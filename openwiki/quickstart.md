# OpenWiki quickstart

CloudBridge Client is a cross-platform Go CLI for connecting to CloudBridge Relay services, with three main user-facing paths:

- P2P mesh networking over QUIC and ICE/STUN/TURN
- TCP tunnel creation to a remote host
- WireGuard/L3-overlay network management and status

The repository is organized around a Cobra CLI entrypoint, a central relay client/orchestrator, configuration loading with Viper, authentication with JWT or OIDC/JWKS, service installation helpers, and an interactive onboarding wizard.

Start here when you need to understand the project quickly:

1. [`architecture/overview.md`](architecture/overview.md) — how the codebase is structured
2. [`workflows/cli.md`](workflows/cli.md) — command-line modes and runtime flow
3. [`domain/networking.md`](domain/networking.md) — relay, P2P, tunnel, and overlay concepts
4. [`domain/config-and-auth.md`](domain/config-and-auth.md) — config, auth, onboarding, and wizard behavior
5. [`operations/build-and-service.md`](operations/build-and-service.md) — build, packaging, and service installation
6. [`testing.md`](testing.md) — what to run when changing core code

## What this repo does

At a high level, the client:

- loads configuration from YAML, environment variables, and command-line flags
- authenticates with a relay server using JWT or OIDC-backed JWT validation
- establishes transport and network connections for P2P or tunnel operation
- starts a heartbeat loop and exposes metrics
- can install itself as a service on Linux, Windows, or macOS

## Repository map

- `cmd/cloudbridge-client/main.go` — main CLI entrypoint and subcommand wiring
- `cmd/quic-tester/main.go` — standalone QUIC diagnostics utility
- `pkg/config/` — configuration loading, defaults, validation, and hot reload
- `pkg/auth/` — JWT and OIDC/JWKS authentication
- `pkg/relay/` — central client orchestration and transport management
- `pkg/p2p/` — P2P mesh manager and WireGuard overlay integration
- `pkg/heartbeat/` — heartbeat manager with retry/backoff behavior
- `pkg/service/` — OS-specific service install/start/stop helpers
- `pkg/onboarding/` and `pkg/wizard/` — invite-based onboarding and interactive setup
- `Makefile`, `build-all-platforms.sh`, `install.sh`, `.goreleaser.yml` — build/package entrypoints

## Verified CLI surface

The root CLI is built with Cobra and exposes these top-level paths in `cmd/cloudbridge-client/main.go`:

- `cloudbridge-client version`
- `cloudbridge-client p2p`
- `cloudbridge-client tunnel`
- `cloudbridge-client service install|uninstall|start|stop|restart|status`
- `cloudbridge-client wireguard config|status`

The root command also supports persistent flags such as `--config`, `--token`, `--verbose`, `--ca`, `--insecure-skip-tls-verify`, `--log-level`, and `--transport`.

## Build/runtime baseline

- Go toolchain: `go 1.25.3` in `go.mod`
- Primary binary target: `./cmd/cloudbridge-client`
- Alternate binary: `./cmd/quic-tester`
- Build tags/ldflags are managed in the `Makefile`

## Notes for future changes

- Be careful with production defaults in `pkg/config/config.go`; many values are environment-specific and should not be treated as generic examples.
- The relay client initializes several subsystems during construction, so changes in config/auth/metrics can affect startup behavior broadly.
- Some docs in the repo are more aspirational than code-backed; when in doubt, prefer the source files linked from this wiki.
