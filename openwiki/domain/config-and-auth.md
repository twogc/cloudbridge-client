# Configuration and authentication

## Canonical ports and bases

See **[docs/CONTRACT_CLIENT_RELAY.md](../../docs/CONTRACT_CLIENT_RELAY.md)** for the full client↔relay contract.

| Purpose | Default |
|---------|---------|
| REST / P2P API base | `https://{host}:5552` (`api.base_url`, `relay.ports.p2p_api`) |
| gRPC | `{host}:8444` (`relay.ports.grpc`) |
| P2P QUIC | `{host}:5553` UDP (`relay.ports.quic`) |
| Client metrics | `:9091` local |
| Env prefix | `CLOUDBRIDGE_` |

Helpers (after WP1): `P2PAPIBaseURL()`, `GRPCTarget()`, `P2PQUICAddr()` on `*types.Config`.

## Configuration loading

`pkg/config/config.go` uses Viper to load settings from:

- the current directory
- `./config`
- `/etc/cloudbridge-client`
- `$HOME/.cloudbridge-client`
- an explicit path passed on the CLI

It also reads environment variables with the `CLOUDBRIDGE_` prefix and fills defaults from `setDefaults()`.

## Validation behavior

The loader validates key runtime assumptions, including:

- relay host and port
- TLS version requirements (TLS 1.3 when enabled)
- CA certificate file existence when specified
- consistency between client certificate and key
- auth requirements for JWT and OIDC
- WireGuard interface name and MTU

## Authentication modes

`pkg/auth/auth.go` supports two main modes:

- `jwt` — validates JWTs using a secret (plain text or base64)
- `oidc` — OIDC discovery + JWKS (Zitadel-ready)

Claims include tenant ID, org ID, permissions, network config, mesh config, and peer whitelists.

**Production path with current relay:** prefer **OIDC/Zitadel-issued JWT**. HMAC `jwt` secret mode is for legacy/dev and may not validate server-issued tokens.

## Onboarding and wizard

- `pkg/onboarding/onboarding.go` — invite validate/redeem against **control plane**, then generates config
- `pkg/wizard/wizard.go` — interactive invite / manual JWT / OIDC setup

Wizard and config loader defaults must both match [CONTRACT_CLIENT_RELAY.md](../../docs/CONTRACT_CLIENT_RELAY.md) (WP3).

## Caveats

- Do not use obsolete README port **8081**.
- Do not dial gRPC on the QUIC UDP port (5553).
- `relay.tls.server_name` may differ from `relay.host` (cert SAN); set explicitly per environment.

## Source references

- `pkg/config/config.go`
- `pkg/types/types.go`, `pkg/types/addrs.go`
- `pkg/auth/auth.go`
- `pkg/onboarding/onboarding.go`
- `pkg/wizard/wizard.go`
