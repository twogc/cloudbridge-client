# Networking domain

CloudBridge Client’s product value is network connectivity: relay-assisted P2P, tunnel forwarding, and L3 overlay (WireGuard).

**Contract SoT:** [docs/CONTRACT_CLIENT_RELAY.md](../../docs/CONTRACT_CLIENT_RELAY.md)

## Core concepts

### Relay client

`pkg/relay/client.go` coordinates transport, auth, metrics, heartbeat, and P2P.

### Transports

| Mode | Target | Notes |
|------|--------|-------|
| gRPC (preferred) | `{host}:8444` | `--transport grpc`; Hello / Auth / Tunnel / Heartbeat protos |
| Legacy JSON/TLS | main TCP (canonical **5550**) | Deprecated path |
| P2P QUIC | `{host}:5553` UDP | Mesh / relay data plane |
| Main QUIC | `{host}:9090` UDP | Optional; align with server |
| WebSocket | via REST/ingress | Fallback; not primary |

### P2P mesh

`pkg/p2p/manager.go` — ICE/STUN/TURN, QUIC peers, WireGuard overlay helpers, relay API.

### Tunnel mode

CLI `tunnel` + relay/tunnel managers — local port forward to remote host:port.

### WireGuard overlay

Config fetch from REST (`…/wireguard-config` or `/api/v1/wireguard/*`). If 404 on :5552, try HTTP API :9090 (WP4 dual-base).

### MASQUE

`pkg/masque` exists but is **UNWIRED** in `initializeEnhancedComponents()` — not GA.

## Dial rules (implementation)

1. REST → `P2PAPIBaseURL()` → port **5552**
2. gRPC → `GRPCTarget()` → port **8444**
3. P2P QUIC → `P2PQUICAddr()` → port **5553** UDP
4. Never use a single `relay.port` for all three roles

## Change guidance

When editing networking, verify:

- `pkg/p2p/manager.go`
- `pkg/relay/client.go`
- `pkg/relay/transport/*`
- `pkg/quic/connection.go`
- `pkg/ice/agent.go`
- tests in `pkg/p2p`, `pkg/quic`, `pkg/relay/transport`
- smoke items in [docs/CHECKLISTS.md](../../docs/CHECKLISTS.md) §5

## Source references

- `pkg/p2p/manager.go`
- `pkg/relay/client.go`
- `pkg/relay/transport/`
- `pkg/quic/connection.go`
- `pkg/ice/agent.go`
