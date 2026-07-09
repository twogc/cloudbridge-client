#!/usr/bin/env bash
# CreateTunnel (gRPC) + full P2P QUIC AUTH against local-smoke relay.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="${HOME}/.local/go-install/go/bin:${PATH}"
export JWT_SECRET="${JWT_SECRET:-test-secret}"
export CLOUDBRIDGE_TOKEN="${CLOUDBRIDGE_TOKEN:-local-smoke-grpc-token}"

echo "======== gRPC CreateTunnel smoke ========"
cd "$ROOT"
go run ./scripts/grpc-smoke -host 127.0.0.1 -port 8444 -token "$CLOUDBRIDGE_TOKEN" -tunnel -echo-port 18080

echo
echo "======== full QUIC AUTH smoke ========"
go run ./scripts/quic-smoke -addr 127.0.0.1:5553 -secret "$JWT_SECRET"

echo
echo "TUNNEL_QUIC_SMOKE_PASS=1"
