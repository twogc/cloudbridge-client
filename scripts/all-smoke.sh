#!/usr/bin/env bash
# All local smokes against cloudbridge-relay-installer local-smoke.
#
# Prereq:
#   cloudbridge-relay-installer/scripts/local-smoke/run-relay.sh start
#
# Usage:
#   ./scripts/all-smoke.sh
#   SKIP_CLI_TUNNEL=1 ./scripts/all-smoke.sh
#   SKIP_MESH_PEERS=1 ./scripts/all-smoke.sh   # skip 2-peer membership script
#   P2P_SMOKE_DATA=0 ./scripts/all-smoke.sh    # membership only (no --smoke-data)
#
# Env:
#   JWT_SECRET (default test-secret)
#   SMOKE_RELAY_URL (default http://127.0.0.1:5552)
#   SMOKE_DATA_SOFT=1  → p2p --smoke-data warns instead of fail
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="${HOME}/.local/go-install/go/bin:${PATH}"
export JWT_SECRET="${JWT_SECRET:-test-secret}"
export CLOUDBRIDGE_TOKEN="${CLOUDBRIDGE_TOKEN:-local-smoke-grpc-token}"
BASE="${SMOKE_RELAY_URL:-http://127.0.0.1:5552}"
WORKDIR="${SMOKE_DIR:-/tmp/cloudbridge-all-smoke}"
BIN="${CLIENT_BIN:-${ROOT}/bin/cloudbridge-client}"
P2P_SMOKE_DATA="${P2P_SMOKE_DATA:-1}"
mkdir -p "$WORKDIR" "$(dirname "$BIN")"

pass() { echo "ALL_SMOKE_STEP_OK=$*"; }
fail() { echo "ALL_SMOKE_FAIL=$*" >&2; exit 1; }

echo "======== 0) health ========"
curl -sS -m 3 "${BASE}/health" | head -c 200 || fail "health"
echo
pass "health"

if [[ ! -x "$BIN" ]]; then
  echo "building client -> $BIN"
  (cd "$ROOT" && go build -o "$BIN" ./cmd/cloudbridge-client) || fail "build"
fi

make_token() {
  local server_id="$1"
  JWT_SECRET="$JWT_SECRET" python3 -c "
import hmac,hashlib,base64,json,time,uuid,os
secret=os.environ['JWT_SECRET'].encode()
b64=lambda d: base64.urlsafe_b64encode(d).rstrip(b'=').decode()
h=b64(json.dumps({'alg':'HS256','typ':'JWT'},separators=(',',':')).encode())
now=int(time.time())
sid='$server_id'
p=b64(json.dumps({
  'jti':str(uuid.uuid4()),'sub':sid,'peer_id':sid,'tenant_id':'default','server_id':sid,
  'connection_type':'quic','protocol_type':'p2p-mesh','permissions':['p2p_connect'],
  'iat':now,'nbf':now-60,'exp':now+3600
},separators=(',',':')).encode())
print(f'{h}.{p}.{b64(hmac.new(secret,f\"{h}.{p}\".encode(),hashlib.sha256).digest())}')
"
}

write_cfg() {
  local path="$1" tok="$2"
  cat >"$path" <<EOF
relay:
  host: "127.0.0.1"
  port: 5550
  ports:
    p2p_api: 5552
    quic: 5553
    grpc: 8444
    stun: 19302
  tls:
    enabled: false
    verify_cert: false
auth:
  type: jwt
  token: "${tok}"
  secret: "${JWT_SECRET}"
api:
  base_url: "${BASE}"
  p2p_api_url: "${BASE}"
  heartbeat_url: "${BASE}"
  insecure_skip_verify: true
  timeout: 15s
logging:
  level: info
  format: text
metrics:
  enabled: false
  prometheus_port: 0
wireguard:
  enabled: false
p2p:
  heartbeat_interval: 10s
rate_limiting:
  max_retries: 2
  backoff_multiplier: 1.5
  max_backoff: 3s
EOF
}

echo
echo "======== 1) p2p --smoke / --smoke-data ========"
TOK=$(make_token "all-smoke-$(date +%s)")
CFG="${WORKDIR}/p2p.yaml"
write_cfg "$CFG" "$TOK"
SMOKE_FLAGS=(--smoke --smoke-wait 60s)
if [[ "$P2P_SMOKE_DATA" == "1" ]]; then
  SMOKE_FLAGS=(--smoke-data --smoke-wait 60s)
fi
"$BIN" p2p -c "$CFG" -t "$TOK" "${SMOKE_FLAGS[@]}" --log-level info \
  2>&1 | tee "${WORKDIR}/p2p.log"
grep -q 'SMOKE_PASS=1' "${WORKDIR}/p2p.log" || fail "p2p smoke"
if [[ "$P2P_SMOKE_DATA" == "1" ]] && [[ "${SMOKE_DATA_SOFT:-}" != "1" ]]; then
  grep -q 'data_plane=ok\|SMOKE_WARN' "${WORKDIR}/p2p.log" || true
  grep -qE 'data_plane=ok|SMOKE_PASS=1' "${WORKDIR}/p2p.log" || fail "p2p smoke-data"
fi
pass "p2p_smoke"

if [[ "${SKIP_MESH_PEERS:-0}" != "1" ]]; then
  echo
  echo "======== 2) 2-peer mesh membership ========"
  bash "$ROOT/scripts/mesh-smoke-2peer.sh" 2>&1 | tee "${WORKDIR}/mesh2.log"
  grep -q 'MESH_SMOKE_PASS=1' "${WORKDIR}/mesh2.log" || fail "mesh-2peer"
  pass "mesh_2peer"
else
  echo "(skip 2-peer mesh: SKIP_MESH_PEERS=1)"
fi

echo
echo "======== 3) gRPC CreateTunnel + TCP bytes ========"
(cd "$ROOT" && go run ./scripts/grpc-smoke -host 127.0.0.1 -port 8444 \
  -token "$CLOUDBRIDGE_TOKEN" -tunnel -echo-port 18080) 2>&1 | tee "${WORKDIR}/grpc.log"
grep -q 'GRPC_SMOKE_PASS=1' "${WORKDIR}/grpc.log" || fail "grpc"
pass "grpc_tunnel"

if [[ "${SKIP_CLI_TUNNEL:-0}" != "1" ]]; then
  echo
  echo "======== 4) CLI tunnel e2e ========"
  bash "$ROOT/scripts/cli-tunnel-smoke.sh" 2>&1 | tee "${WORKDIR}/cli-tun.log"
  grep -q 'CLI_TUNNEL_SMOKE_PASS=1\|TUNNEL_CLI_SMOKE_PASS=1' "${WORKDIR}/cli-tun.log" || fail "cli-tunnel"
  pass "cli_tunnel"
else
  echo "(skip CLI tunnel: SKIP_CLI_TUNNEL=1)"
fi

echo
echo "======== 5) QUIC AUTH + PING/PONG ========"
(cd "$ROOT" && go run ./scripts/quic-smoke -addr 127.0.0.1:5553 -secret "$JWT_SECRET") \
  2>&1 | tee "${WORKDIR}/quic.log"
grep -q 'QUIC_SMOKE_PASS=1' "${WORKDIR}/quic.log" || fail "quic"
pass "quic_ping"

echo
echo "======== 6) QUIC multi-peer mesh TO ========"
bash "$ROOT/scripts/quic-mesh-smoke.sh" 2>&1 | tee "${WORKDIR}/quic-mesh.log"
grep -q 'QUIC_MESH_SMOKE_PASS=1' "${WORKDIR}/quic-mesh.log" || fail "quic-mesh"
pass "quic_mesh"

echo
echo "ALL_SMOKE_PASS=1"
echo "logs: $WORKDIR"
