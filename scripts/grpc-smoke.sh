#!/usr/bin/env bash
# Track B: gRPC control-plane smoke (Connect + Hello + Authenticate).
# Prereq: local-smoke relay with grpc.enabled=true on :8444 (tls off).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="${HOME}/.local/go-install/go/bin:${PATH}"
HOST="${GRPC_HOST:-127.0.0.1}"
PORT="${GRPC_PORT:-8444}"
TOKEN="${CLOUDBRIDGE_TOKEN:-local-smoke-grpc-token}"

echo "== TCP probe ${HOST}:${PORT} =="
if ! (echo >/dev/tcp/"$HOST"/"$PORT") 2>/dev/null; then
  # bash /dev/tcp may be disabled; fall back to ss/curl
  if ! ss -lntp 2>/dev/null | grep -q ":${PORT}"; then
    echo "FAIL: nothing listening on ${HOST}:${PORT}"
    echo "Start: cloudbridge-relay-installer/scripts/local-smoke/run-relay.sh start"
    exit 1
  fi
fi

echo "== go run grpc-smoke =="
cd "$ROOT"
CLOUDBRIDGE_TOKEN="$TOKEN" go run ./scripts/grpc-smoke \
  -host "$HOST" -port "$PORT" -token "$TOKEN"
