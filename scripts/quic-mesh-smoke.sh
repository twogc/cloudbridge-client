#!/usr/bin/env bash
# Multi-peer QUIC mesh data plane: two AUTH peers + TO:<peer>:<payload> both directions.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="${HOME}/.local/go-install/go/bin:${PATH}"
export JWT_SECRET="${JWT_SECRET:-test-secret}"
ADDR="${QUIC_ADDR:-127.0.0.1:5553}"

cd "$ROOT"
go run ./scripts/quic-mesh-smoke -addr "$ADDR" -secret "$JWT_SECRET"
echo "QUIC_MESH_SMOKE_PASS=1"
