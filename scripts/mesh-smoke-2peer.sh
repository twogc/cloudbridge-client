#!/usr/bin/env bash
# Track A: two-peer mesh membership smoke against local relay.
# PASS criteria:
#   1) REST health
#   2) peer-A CLI p2p --smoke exits 0 (SMOKE_PASS=1)
#   3) peer-B CLI p2p --smoke exits 0
#   4) GET /peers shows >= 2 peers for tenant default
#
# Prereq: relay local-smoke running (installer scripts/local-smoke/run-relay.sh start)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="${HOME}/.local/go-install/go/bin:${PATH}"
BASE="${SMOKE_RELAY_URL:-http://127.0.0.1:5552}"
SECRET="${JWT_SECRET:-test-secret}"
WORKDIR="${SMOKE_DIR:-/tmp/cloudbridge-mesh-smoke}"
BIN="${CLIENT_BIN:-${WORKDIR}/cbc}"
mkdir -p "$WORKDIR"

make_token() {
  local server_id="$1"
  JWT_SECRET="$SECRET" python3 -c "
import hmac,hashlib,base64,json,time,uuid,os
secret=os.environ['JWT_SECRET'].encode()
b64=lambda d: base64.urlsafe_b64encode(d).rstrip(b'=').decode()
h=b64(json.dumps({'alg':'HS256','typ':'JWT'},separators=(',',':')).encode())
now=int(time.time())
sid='$server_id'
p=b64(json.dumps({
  'jti':str(uuid.uuid4()),'sub':sid,'tenant_id':'default','server_id':sid,
  'connection_type':'quic','protocol_type':'p2p-mesh','permissions':['p2p_connect'],
  'iat':now,'nbf':now-60,'exp':now+3600
},separators=(',',':')).encode())
print(f'{h}.{p}.{b64(hmac.new(secret,f\"{h}.{p}\".encode(),hashlib.sha256).digest())}')
"
}

write_cfg() {
  local path="$1" token="$2"
  cat >"$path" <<EOF
relay:
  host: "127.0.0.1"
  port: 5550
  ports:
    p2p_api: 5552
    quic: 5553
    stun: 19302
  tls:
    enabled: false
    verify_cert: false
auth:
  type: jwt
  token: "${token}"
  secret: "${SECRET}"
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
EOF
}

echo "== health =="
curl -sS -m 3 "${BASE}/health" | head -c 200
echo

if [[ ! -x "$BIN" ]]; then
  echo "building client -> $BIN"
  (cd "$ROOT" && go build -o "$BIN" ./cmd/cloudbridge-client)
fi

TA=$(make_token "mesh-a-$(date +%s)")
TB=$(make_token "mesh-b-$(date +%s)")
write_cfg "${WORKDIR}/a.yaml" "$TA"
write_cfg "${WORKDIR}/b.yaml" "$TB"

echo "== peer A smoke =="
"$BIN" p2p -c "${WORKDIR}/a.yaml" -t "$TA" --smoke --smoke-wait 60s --log-level info \
  2>&1 | tee "${WORKDIR}/a.log"
grep -q 'SMOKE_PASS=1' "${WORKDIR}/a.log" || { echo "FAIL peer A"; exit 1; }

echo "== peer B smoke =="
"$BIN" p2p -c "${WORKDIR}/b.yaml" -t "$TB" --smoke --smoke-wait 60s --log-level info \
  2>&1 | tee "${WORKDIR}/b.log"
grep -q 'SMOKE_PASS=1' "${WORKDIR}/b.log" || { echo "FAIL peer B"; exit 1; }

echo "== discover count =="
DISC=$(curl -sS -m 5 "${BASE}/api/v1/tenants/default/peers" -H "Authorization: Bearer ${TB}")
echo "$DISC" | head -c 500
echo
COUNT=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); print(len(d.get('peers') or []))" "$DISC")
echo "peers_online=$COUNT"
if [[ "$COUNT" -lt 2 ]]; then
  echo "FAIL: expected >=2 peers, got $COUNT"
  exit 1
fi

echo
echo "MESH_SMOKE_PASS=1 peers=$COUNT"
echo "Track A: two-peer control-plane membership OK"
