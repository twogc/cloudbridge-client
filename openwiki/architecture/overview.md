# Architecture overview

CloudBridge Client is organized around a single CLI entrypoint that configures and drives a central relay client. The codebase is small enough that a few packages define most of the runtime behavior.

## Main runtime layers

### CLI layer
`cmd/cloudbridge-client/main.go` wires the Cobra command tree and processes root-level flags before creating the client.

### Configuration layer
`pkg/config/config.go` loads YAML, environment variables, and defaults through Viper. It also validates TLS, relay, auth, and network settings.

### Orchestration layer
`pkg/relay/client.go` is the main coordination point. It creates and wires together auth, retry, metrics, optimization, transport-adapter, tunnel, heartbeat, and P2P components.

### Network layer
`pkg/p2p/manager.go` is the P2P/overlay manager. It combines ICE, QUIC, WireGuard, relay API integration, and peer discovery.

### Security layer
`pkg/auth/auth.go` handles JWT and OIDC/JWKS validation. The code supports JWT secrets as plain text or base64-encoded input.

### Background services
`pkg/heartbeat/manager.go` maintains a retrying heartbeat loop. `pkg/config/watcher.go` adds config hot reload.

## Package map

- `pkg/api/` — API client and manager code used by relay/P2P flows
- `pkg/auth/` — token validation and JWKS support
- `pkg/config/` — config loading, defaults, validation, watching
- `pkg/heartbeat/` — heartbeat lifecycle and backoff
- `pkg/ice/` — ICE agent setup
- `pkg/metrics/` — Prometheus/Pushgateway metrics
- `pkg/p2p/` — mesh and overlay networking logic
- `pkg/quic/` — QUIC connection helpers
- `pkg/relay/` — relay client, transport adapter, auto-switch behavior
- `pkg/service/` — OS service installation and process control
- `pkg/tunnel/` — tunnel manager
- `pkg/wizard/` and `pkg/onboarding/` — setup and invite-based onboarding

## Architecture notes that matter for changes

- The client is not a thin wrapper over one package; startup happens through `relay.NewClient`, which eagerly initializes several subsystems.
- Configuration values influence transport, metrics, heartbeat, and network behavior early in startup.
- P2P and overlay features share relay/API state, so changes to auth or config can cascade into multiple runtime paths.
- Some subsystems exist as scaffolding or optional integrations; confirm implementation status before documenting a feature as fully available.

## Source references

- `cmd/cloudbridge-client/main.go`
- `pkg/relay/client.go`
- `pkg/config/config.go`
- `pkg/p2p/manager.go`
- `pkg/auth/auth.go`
- `pkg/heartbeat/manager.go`
- `pkg/config/watcher.go`
