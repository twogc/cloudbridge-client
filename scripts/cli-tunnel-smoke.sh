#!/usr/bin/env bash
# CLI tunnel e2e: cloudbridge-client tunnel --smoke via local-smoke relay CreateTunnel.
# Path: localhost:LOCAL → relay TCP endpoint → remote echo.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="${HOME}/.local/go-install/go/bin:${PATH}"
export JWT_SECRET="${JWT_SECRET:-test-secret}"
SECRET="$JWT_SECRET"
BASE="${P2P_API_BASE:-http://127.0.0.1:5552}"
BIN="${CLI_BIN:-$ROOT/bin/cloudbridge-client}"
LOCAL_PORT="${LOCAL_PORT:-13389}"
ECHO_PORT="${ECHO_PORT:-18081}"
WORKDIR="${TMPDIR:-/tmp}/cloudbridge-cli-tunnel-smoke"
mkdir -p "$WORKDIR" "$ROOT/bin"

make_token() {
  JWT_SECRET="$SECRET" python3 -c "
import os,hmac,hashlib,base64,json,time
secret=os.environ['JWT_SECRET'].encode()
def b64(b): return base64.urlsafe_b64encode(b).rstrip(b'=').decode()
now=int(time.time())
h=b64(json.dumps({'alg':'HS256','typ':'JWT'},separators=(',',':')).encode())
p=b64(json.dumps({
  'sub':'cli-tunnel-smoke','tenant_id':'default','peer_id':'cli-tunnel-peer',
  'server_id':'cli-tunnel-server','connection_type':'client-server',
  'protocol_type':'client-server','permissions':['tunnel'],
  'iat':now,'nbf':now-60,'exp':now+3600
},separators=(',',':')).encode())
print(f'{h}.{p}.{b64(hmac.new(secret,f\"{h}.{p}\".encode(),hashlib.sha256).digest())}')
"
}

echo "== health =="
curl -sS -m 3 "${BASE}/health" | head -c 200
echo

if [[ ! -x "$BIN" ]]; then
  echo "building client -> $BIN"
  (cd "$ROOT" && go build -o "$BIN" ./cmd/cloudbridge-client)
fi

TOKEN="$(make_token)"
CFG="$WORKDIR/config.yaml"
cat >"$CFG" <<EOF
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
  token: "${TOKEN}"
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
rate_limiting:
  max_retries: 2
  backoff_multiplier: 1.5
  max_backoff: 3s
EOF

# TCP echo on remote side (what relay proxies to)
ECHO_PID_FILE="$WORKDIR/echo.pid"
python3 - <<PY &
import socket, threading, sys
port = int("${ECHO_PORT}")
s = socket.socket()
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", port))
s.listen(5)
print(f"echo listening 127.0.0.1:{port}", flush=True)
def handle(c):
    try:
        while True:
            d = c.recv(4096)
            if not d:
                break
            c.sendall(d)
    finally:
        c.close()
while True:
    c, _ = s.accept()
    threading.Thread(target=handle, args=(c,), daemon=True).start()
PY
ECHO_PID=$!
echo "$ECHO_PID" >"$ECHO_PID_FILE"
trap 'kill "$ECHO_PID" 2>/dev/null || true' EXIT
sleep 0.3

export CLOUDBRIDGE_TOKEN="$TOKEN"
echo "== CLI tunnel --smoke local=$LOCAL_PORT remote=127.0.0.1:$ECHO_PORT =="
"$BIN" tunnel \
  --config "$CFG" \
  --token "$TOKEN" \
  --transport grpc \
  --tunnel-id "cli-smoke-$(date +%s)" \
  --local-port "$LOCAL_PORT" \
  --remote-host 127.0.0.1 \
  --remote-port "$ECHO_PORT" \
  --smoke

echo
echo "CLI_TUNNEL_SMOKE_PASS=1"
