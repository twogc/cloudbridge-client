#!/usr/bin/env bash
# Phase D.4 — OIDC smoke
#
# Offline (default, CI-safe): local JWKS mock + RS256 validate via go test.
# Live (optional): validate a real token against issuer JWKS.
#
# Usage:
#   ./scripts/oidc-smoke.sh
#   ./scripts/oidc-smoke.sh offline
#   OIDC_LIVE_TOKEN=... OIDC_LIVE_ISSUER=... OIDC_LIVE_AUDIENCE=... ./scripts/oidc-smoke.sh live
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="/usr/local/go/bin:${HOME}/.local/go-install/go/bin:${PATH}"
MODE="${1:-offline}"

pass() { echo "OIDC_SMOKE_OK=$*"; }
fail() { echo "OIDC_SMOKE_FAIL=$*" >&2; exit 1; }

case "$MODE" in
  offline|"")
    echo "======== OIDC offline (mock JWKS + RS256) ========"
    (cd "$ROOT" && go test ./pkg/auth/ -count=1 \
      -run 'OIDC_Offline|TestAuthManager_OIDC_Configuration|TestOIDCConfig_Validation' \
      -timeout 90s) || fail "offline go test"
    pass "offline"
    echo "OIDC_SMOKE_PASS=1 mode=offline"
    ;;
  live)
    echo "======== OIDC live ========"
    : "${OIDC_LIVE_ISSUER:?set OIDC_LIVE_ISSUER}"
    : "${OIDC_LIVE_AUDIENCE:?set OIDC_LIVE_AUDIENCE}"
    : "${OIDC_LIVE_TOKEN:?set OIDC_LIVE_TOKEN}"
    (cd "$ROOT" && \
      OIDC_LIVE=1 \
      OIDC_LIVE_ISSUER="$OIDC_LIVE_ISSUER" \
      OIDC_LIVE_AUDIENCE="$OIDC_LIVE_AUDIENCE" \
      OIDC_LIVE_TOKEN="$OIDC_LIVE_TOKEN" \
      OIDC_LIVE_JWKS_URL="${OIDC_LIVE_JWKS_URL:-}" \
      go test ./pkg/auth/ -count=1 -run TestOIDC_LiveOptional -timeout 90s -v) \
      || fail "live validate"
    pass "live"
    echo "OIDC_SMOKE_PASS=1 mode=live"
    ;;
  *)
    echo "usage: $0 [offline|live]" >&2
    exit 2
    ;;
esac
