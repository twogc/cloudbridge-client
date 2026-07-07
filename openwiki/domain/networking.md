# Networking domain

CloudBridge Client’s core product value is network connectivity. The implementation centers on relay-assisted P2P networking, tunnel forwarding, and an L3 overlay built around WireGuard.

## Core concepts

### Relay client
`pkg/relay/client.go` is the high-level runtime that coordinates transport, auth, metrics, heartbeat, and P2P features.

### P2P mesh
`pkg/p2p/manager.go` handles peer connectivity using QUIC plus ICE/STUN/TURN. It also interacts with the relay API and can apply WireGuard configuration for overlay connectivity.

### Tunnel mode
Tunnel support is exposed from the CLI and backed by relay/tunnel code. It forwards a local port to a remote host/port.

### WireGuard overlay
The repo includes WireGuard client and configuration logic as part of the P2P and wireguard workflows. This is used for L3-overlay networking and peer-level network configuration.

### Transport helpers
The `pkg/relay/transport/` and `pkg/quic/` packages contain transport-specific code and protobuf/gRPC descriptors.

## What the code currently emphasizes

- QUIC is the primary performance-oriented transport
- ICE/STUN/TURN are used for connectivity and fallback
- WireGuard is used for overlay networking
- Relay/API integration provides control-plane coordination and tenant-aware configuration

## Change guidance

When editing networking code, verify the effect on:

- `pkg/p2p/manager.go`
- `pkg/relay/client.go`
- `pkg/quic/connection.go`
- `pkg/ice/agent.go`
- `pkg/relay/transport/*`
- the integration test surface in `integration_test.go` and `pkg/p2p/*_test.go`

Networking changes often need coordinated updates to config defaults and auth claims, because those values are threaded through startup and peer setup.

## Source references

- `pkg/p2p/manager.go`
- `pkg/relay/client.go`
- `pkg/quic/connection.go`
- `pkg/ice/agent.go`
- `pkg/relay/transport/`
