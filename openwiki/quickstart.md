# OpenWiki quickstart

CloudBridge Client is a cross-platform Go CLI for connecting to CloudBridge Relay services, with three main user-facing paths:

- P2P mesh networking over QUIC and ICE/STUN/TURN
- TCP tunnel creation to a remote host
- WireGuard/L3-overlay network management and status

The repository is organized around a Cobra CLI entrypoint, a central relay client/orchestrator, configuration loading with Viper, authentication with JWT or OIDC/JWKS, service installation helpers, and an interactive onboarding wizard.

## Documentation (start here)

| Doc | Purpose |
|-----|---------|
| [docs/README.md](../docs/README.md) | Documentation index |
| [docs/CONTRACT_CLIENT_RELAY.md](../docs/CONTRACT_CLIENT_RELAY.md) | **Canonical** ports/paths vs relay |
| [docs/plans/README.md](../docs/plans/README.md) | **Plans index** |
| [docs/plans/GAP_CLOSURE_AND_IMPROVEMENTS.md](../docs/plans/GAP_CLOSURE_AND_IMPROVEMENTS.md) | Roadmap: Session / PathSelector / failover |
| [docs/plans/PATH_SELECTION_STATE_MACHINE.md](../docs/plans/PATH_SELECTION_STATE_MACHINE.md) | Path-selection design diagrams |
| [docs/plans/CONTRACT_ALIGNMENT_PLAN.md](../docs/plans/CONTRACT_ALIGNMENT_PLAN.md) | Port/dial alignment + WP checklists |
| [docs/CHECKLISTS.md](../docs/CHECKLISTS.md) | Build / unit / smoke / release |
| [docs/STATUS.md](../docs/STATUS.md) | Honest maturity snapshot |

Then domain notes:

1. [`architecture/overview.md`](architecture/overview.md) — codebase structure
2. [`workflows/cli.md`](workflows/cli.md) — command-line modes
3. [`domain/networking.md`](domain/networking.md) — relay, P2P, tunnel, overlay
4. [`domain/config-and-auth.md`](domain/config-and-auth.md) — config, auth, onboarding
5. [`operations/build-and-service.md`](operations/build-and-service.md) — build, packaging, service
6. [`testing.md`](testing.md) — what to run when changing core code

## What this repo does

At a high level, the client:

- loads configuration from YAML, environment variables (`CLOUDBRIDGE_*`), and CLI flags
- authenticates with a relay using JWT or OIDC-backed JWT validation
- establishes transport (prefer **gRPC :8444**) and network connections for P2P or tunnel
- starts a heartbeat loop and exposes local metrics (default **:9091**)
- can install itself as a service on Linux, Windows, or macOS

## Repository map

- `cmd/cloudbridge-client/main.go` — main CLI entrypoint and subcommand wiring
- `pkg/config/` — configuration loading, defaults, validation, hot reload
- `pkg/auth/` — JWT and OIDC/JWKS authentication
- `pkg/relay/` — central client orchestration and transport management
- `pkg/p2p/` — P2P mesh manager and WireGuard overlay integration
- `pkg/heartbeat/` — heartbeat manager with retry/backoff behavior
- `pkg/service/` — OS-specific service install/start/stop helpers
- `pkg/onboarding/` and `pkg/wizard/` — invite-based onboarding and interactive setup
- `docs/` — contract, plan, checklists, status
- `Makefile`, `build-all-platforms.sh`, `install.sh`, `.goreleaser.yml` — build/package entrypoints

## Verified CLI surface

- `cloudbridge-client version`
- `cloudbridge-client p2p`
- `cloudbridge-client tunnel`
- `cloudbridge-client service install|uninstall|start|stop|restart|status`
- `cloudbridge-client wireguard config|status`

Persistent flags: `--config`, `--token`, `--verbose`, `--ca`, `--insecure-skip-tls-verify`, `--log-level`, `--transport`.

## Build/runtime baseline

- Go toolchain: `go 1.25.3` in `go.mod` (align toward 1.25.12 with installer when convenient)
- Primary binary: `./cmd/cloudbridge-client`
- Env prefix: **`CLOUDBRIDGE_`** (not legacy `CBR_*`)

## Canonical ports (short)

| Role | Port |
|------|-----:|
| P2P REST API | **5552** TCP |
| gRPC control | **8444** TCP |
| P2P QUIC | **5553** UDP |
| Main QUIC | **9090** UDP |
| STUN / TURN / WG | 19302 / 3478 / 51820 |

Full matrix: [docs/CONTRACT_CLIENT_RELAY.md](../docs/CONTRACT_CLIENT_RELAY.md).

## Notes for future changes

- Prefer [CONTRACT_CLIENT_RELAY.md](../docs/CONTRACT_CLIENT_RELAY.md) over historical defaults in code comments.
- `relay.NewClient` initializes many subsystems at once; config/auth/metrics changes affect startup broadly.
- Some root docs are aspirational; if in doubt, prefer source + `docs/STATUS.md`.
