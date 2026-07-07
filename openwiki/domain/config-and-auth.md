# Configuration and authentication

## Configuration loading

`pkg/config/config.go` uses Viper to load settings from:

- the current directory
- `./config`
- `/etc/cloudbridge-client`
- `$HOME/.cloudbridge-client`
- an explicit path passed on the CLI

It also reads environment variables with the `CLOUDBRIDGE_` prefix and fills in a large set of defaults.

## Validation behavior

The loader validates key runtime assumptions, including:

- relay host and port
- TLS version requirements
- CA certificate file existence when specified
- consistency between client certificate and key
- auth requirements for JWT and OIDC
- WireGuard interface name and MTU

## Authentication modes

`pkg/auth/auth.go` supports two main modes:

- `jwt` — validates JWTs using a secret, which may be plain text or base64-encoded
- `oidc` — performs OIDC discovery and loads JWKS for token verification

The auth layer includes claim types for tenant ID, org ID, permissions, network config, mesh config, and peer whitelists.

## Onboarding and wizard flow

- `pkg/onboarding/onboarding.go` validates invite tokens and redeems them with the control plane before generating a config.
- `pkg/wizard/wizard.go` offers interactive setup paths for invite, manual JWT, and OIDC-based configuration.

## Important caveats

- The repository’s docs and source contain several environment-specific default URLs and ports; treat them as deployment-specific unless you verify them against your environment.
- OIDC support depends on discovery/JWKS availability and an issuer/audience pair that matches the deployment.
- The wizard and config loader do not use the same defaults in every case, so future changes should check both paths.

## Source references

- `pkg/config/config.go`
- `pkg/config/watcher.go`
- `pkg/auth/auth.go`
- `pkg/onboarding/onboarding.go`
- `pkg/wizard/wizard.go`
