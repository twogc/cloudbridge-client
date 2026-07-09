# Auth profiles — lab vs production

**Date:** 2026-07-09  
**Phase:** GAP D.5  
**Related:** [CONTRACT_CLIENT_RELAY.md](./CONTRACT_CLIENT_RELAY.md) §5, [STATUS.md](./STATUS.md)

---

## Profiles

### `lab-hmac` (default for local smoke)

| Item | Value |
|------|--------|
| Client `auth.type` | `jwt` |
| Secret | Shared HMAC with relay local-smoke config |
| TLS | Often disabled on loopback (`-tls=false` / `insecure_skip_verify`) |
| gRPC | `Hello` + `Authenticate(token)` against `:8444` — **wired**, not a nil NoOp |
| REST | Register / discover / peer heartbeat with Bearer JWT |
| Used by | `scripts/local-smoke.sh`, `scripts/all-smoke.sh`, `p2p --smoke`, tunnel smokes |

**Proves:** control + data plane with shared secret.  
**Does not prove:** Zitadel / JWKS / multi-tenant IdP.

### `prod-oidc` (target production)

| Item | Value |
|------|--------|
| Client `auth.type` | `oidc` |
| Config | `auth.oidc.issuer_url`, `audience`, optional `jwks_url` |
| Relay | Zitadel-issued access tokens validated via JWKS |
| Used by | staging/prod; **not** default all-smoke |

**Status:** client package supports OIDC.  
**Offline D.4:** `scripts/oidc-smoke.sh offline` — mock discovery + JWKS + RS256 round-trip (CI).  
**Live D.4:** `OIDC_LIVE_*=… scripts/oidc-smoke.sh live` — optional when Zitadel credentials available.

---

## How to select

```yaml
# lab (smoke)
auth:
  type: jwt
  secret: "<same as relay local-smoke>"

# production-shaped
auth:
  type: oidc
  oidc:
    issuer_url: "https://auth.example/..."
    audience: "cloudbridge-client"
```

Env overrides follow existing `CLOUDBRIDGE_*` / Viper rules.

---

## Checklist

| Check | lab-hmac | prod-oidc |
|-------|----------|-----------|
| `go test ./pkg/auth/ -short` | required | required |
| `scripts/oidc-smoke.sh offline` | N/A | **D.4 offline** (mock RS256) |
| local-smoke / all-smoke | primary | optional later |
| `scripts/oidc-smoke.sh live` | N/A | optional with real token |

---

## Changelog

| Date | Note |
|------|------|
| 2026-07-09 | Initial profiles for Phase D.5 honesty |
| 2026-07-09 | D.4 offline OIDC smoke + live optional path |
