#!/usr/bin/env bash
# Local smoke: REST register/discover against a running relay on 127.0.0.1:5552.
# Optional: short-lived `p2p` CLI (json transport; gRPC often disabled in local-smoke config).
#
# Prereq: relay local-smoke running (installer scripts/local-smoke/run-relay.sh start)
#          JWT_SECRET=test-secret on relay (default in run-relay.sh)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="${HOME}/.local/go-install/go/bin:${PATH}"
BASE="${SMOKE_RELAY_URL:-http://127.0.0.1:5552}"
SECRET="${JWT_SECRET:-test-secret}"
BIN="${CLIENT_BIN:-/tmp/cloudbridge-smoke/cbc}"
WORKDIR="${SMOKE_DIR:-/tmp/cloudbridge-smoke}"
mkdir -p "$WORKDIR"

echo "== health =="
curl -sS -m 3 "${BASE}/health" | head -c 300
echo

TOKEN=$(python3 - <<'PY'
import hmac, hashlib, base64, json, time, uuid, os
secret = os.environ.get("JWT_SECRET", "test-secret").encode()
def b64(d):
    return base64.urlsafe_b64encode(d).rstrip(b"=").decode()
h = b64(json.dumps({"alg": "HS256", "typ": "JWT"}, separators=(",", ":")).encode())
now = int(time.time())
# Relay token.Validate / P2PClaims:
#   protocol_type == "p2p-mesh"
#   connection_type == "quic"
#   permissions contain "p2p_connect"
p = b64(json.dumps({
    "jti": str(uuid.uuid4()),
    "sub": "local-smoke-client",
    "tenant_id": "default",
    "server_id": "local-smoke-server",
    "connection_type": "quic",
    "protocol_type": "p2p-mesh",
    "permissions": ["p2p_connect"],
    "iat": now,
    "nbf": now - 60,
    "exp": now + 3600,
}, separators=(",", ":")).encode())
sig = hmac.new(secret, f"{h}.{p}".encode(), hashlib.sha256).digest()
print(f"{h}.{p}.{b64(sig)}")
PY
)

echo "== register =="
REG=$(curl -sS -m 5 -X POST "${BASE}/api/v1/tenants/default/peers/register" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"public_key":"smoke-pubkey","allowed_ips":["10.200.0.0/24"]}')
echo "$REG"
echo "$REG" | grep -q '"success":true' || { echo "register failed"; exit 1; }

echo "== discover (GET …/peers) =="
DISC=$(curl -sS -m 5 "${BASE}/api/v1/tenants/default/peers" \
  -H "Authorization: Bearer ${TOKEN}")
echo "$DISC" | head -c 400
echo
echo "$DISC" | grep -q '"success":true' || { echo "discover failed"; exit 1; }

PEER=$(python3 -c "import json,sys; print(json.loads(sys.argv[1]).get('peer_id',''))" "$REG")
SID=$(python3 -c "import json,sys; print(json.loads(sys.argv[1]).get('relay_session_id',''))" "$REG")

echo "== status PUT =="
curl -sS -m 5 -X PUT "${BASE}/api/v1/tenants/default/peers/${PEER}/status" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"status\":\"online\",\"relay_session_id\":\"${SID}\"}"
echo

if [[ "${RUN_CLI:-0}" == "1" ]]; then
  if [[ ! -x "$BIN" ]]; then
    echo "building client -> $BIN"
    (cd "$ROOT" && go build -o "$BIN" ./cmd/cloudbridge-client)
  fi
  cat >"${WORKDIR}/client.yaml" <<EOF
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
  token: "${TOKEN}"
  secret: "${SECRET}"
api:
  base_url: "${BASE}"
  p2p_api_url: "${BASE}"
  heartbeat_url: "${BASE}"
  insecure_skip_verify: true
  timeout: 10s
logging:
  level: info
  format: text
metrics:
  enabled: false
  prometheus_port: 19091
wireguard:
  enabled: false
EOF
  echo "== CLI p2p (json, 12s) =="
  # gRPC is disabled in local-smoke relay config; use json.
  timeout 12 "$BIN" p2p -c "${WORKDIR}/client.yaml" -t "$TOKEN" --transport json --log-level info \
    | tee "${WORKDIR}/cli.log" || true
  grep -E 'status 200|Starting P2P|register' "${WORKDIR}/cli.log" || true
fi

echo
echo "SMOKE REST: PASS"
echo "JWT claims required: protocol_type=p2p-mesh connection_type=quic permissions=[p2p_connect]"
echo "JWT secret: JWT_SECRET (default test-secret)"
