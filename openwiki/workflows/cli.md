# CLI workflows

## Main entrypoint

The CLI is implemented in `cmd/cloudbridge-client/main.go` using Cobra. It exposes a root command plus subcommands for the three main product workflows:

- P2P mesh networking
- tunnel forwarding
- WireGuard overlay management
- service lifecycle operations

## Root command flow

When the root command runs, it:

1. loads configuration via `pkg/config.LoadConfig`
2. applies CLI overrides for token, CA path, log level, and TLS verification
3. creates a relay client through `pkg/relay.NewClient`
4. validates command/flag combinations
5. sets the transport mode when requested
6. connects, authenticates, creates a tunnel, and starts heartbeat
7. waits for a shutdown signal

That means root-command changes can affect the startup path for every mode.

## Supported command groups

### `version`
Prints build/version metadata.

### `p2p`
Starts P2P mesh networking using QUIC and ICE/STUN/TURN. This is the best entrypoint when working on peer discovery or overlay behavior.

### `tunnel`
Creates a tunnel to a remote host. Flags include tunnel ID, local port, remote host, and remote port.

### `service`
Lifecycle manager for OS services:

- install
- uninstall
- start
- stop
- restart
- status

### `wireguard`
Manages WireGuard L3-overlay state:

- `wireguard config`
- `wireguard status`

## Interactive setup and onboarding

Two helper paths support initial configuration:

- `pkg/wizard/wizard.go` offers an interactive setup wizard with invite, manual JWT, and OIDC flows.
- `pkg/onboarding/onboarding.go` validates and redeems invite tokens against the control plane and then generates a config.

## Watch-outs

- The wizard’s defaults do not all match the config loader defaults, so do not assume a single canonical port/host value without checking the source.
- Root flags are persistent, so they apply across subcommands unless a subcommand defines its own value.
- The root flow is stateful: config loading, auth creation, and client construction happen before the command enters its steady state.

## Source references

- `cmd/cloudbridge-client/main.go`
- `pkg/wizard/wizard.go`
- `pkg/onboarding/onboarding.go`
- `pkg/service/service.go`
