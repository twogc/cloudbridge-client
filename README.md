# CloudBridge Relay Client

Cross-platform client for CloudBridge Relay P2P mesh networking.

## Documentation

| Doc | Description |
|-----|-------------|
| **[docs/README.md](docs/README.md)** | Full documentation index |
| **[docs/CONTRACT_CLIENT_RELAY.md](docs/CONTRACT_CLIENT_RELAY.md)** | Canonical ports & API contract vs relay |
| **[docs/CHECKLISTS.md](docs/CHECKLISTS.md)** | Build / test / smoke checklists |
| **[docs/STATUS.md](docs/STATUS.md)** | Current maturity (honest) |
| [openwiki/quickstart.md](openwiki/quickstart.md) | Agent / developer map |

## Quick Start

1. **Get a token** from your CloudBridge operator (Zitadel/OIDC JWT preferred).

2. **Configure** — copy and edit config (see also defaults in `pkg/config`):

   ```yaml
   relay:
     host: "your-relay-server.example"
     port: 5550                    # legacy main TCP
     ports:
       p2p_api: 5552               # REST
       grpc: 8444
       quic: 5553                  # P2P QUIC (UDP)
       http_api: 9090
       quic_main: 9090             # main QUIC (UDP)
       masque: 8443
       stun: 19302
       turn: 3478
   api:
     base_url: "https://your-relay-server.example:5552"
     p2p_api_url: "https://your-relay-server.example:5552"
     heartbeat_url: "https://your-relay-server.example:5552"
   auth:
     type: "oidc"                  # or "jwt" for legacy HMAC
     token: "YOUR_JWT_TOKEN_HERE"
     # oidc: issuer_url, audience, jwks_url as needed
   p2p:
     server_id: "my-server-001"
     tenant_id: "my-organization"
   ```

3. **Run**:

   ```bash
   # P2P mesh (prefer gRPC control plane)
   ./cloudbridge-client p2p --config config.yaml --token "$TOKEN" --transport grpc

   # Tunnel mode
   ./cloudbridge-client tunnel --config config.yaml --token "$TOKEN" \
     --local-port 3389 --remote-host target.example --remote-port 3389

   # WireGuard L3-overlay helpers
   ./cloudbridge-client wireguard config --config config.yaml --token "$TOKEN"
   ./cloudbridge-client wireguard status --config config.yaml --token "$TOKEN"
   ```

## Canonical ports (client ↔ relay)

| Role | Port | Protocol |
|------|-----:|----------|
| P2P REST API | **5552** | TCP / HTTPS |
| gRPC control | **8444** | TCP |
| P2P QUIC | **5553** | UDP |
| Main QUIC | **9090** | UDP |
| STUN | **19302** | UDP |
| TURN | **3478** | UDP/TCP |
| WireGuard | **51820** | UDP |
| Client metrics (local) | **9091** | TCP |

Full matrix and path status: [docs/CONTRACT_CLIENT_RELAY.md](docs/CONTRACT_CLIENT_RELAY.md).

> **Obsolete:** older docs used port **8081** — do not use.

## Security & secrets

1. **Environment** (Viper prefix `CLOUDBRIDGE_`):

   ```bash
   export CLOUDBRIDGE_RELAY_HOST="your-relay-server.example"
   # token often passed via --token or auth.token in a 0600 config file
   ./cloudbridge-client p2p --config config.yaml --token "$TOKEN"
   ```

2. **File permissions:** `chmod 600 config.yaml`

3. **Tokens:** short-lived JWT; prefer OIDC/Zitadel in production.

4. **OS keyring:** optional patterns in packaging docs; validate for your OS.

## Transport protocols

- **gRPC** — preferred control plane (`--transport grpc`, port 8444)
- **QUIC** — P2P / data plane
- **WebSocket** — fallback (when enabled)
- **WireGuard** — L3-overlay
- **MASQUE** — code present, **not wired** in orchestrator (see STATUS)

## Build

```bash
make build          # current platform
make build-all      # cross-platform
make build-windows  # Windows
```

Requirements: Go 1.25+, network access to relay, valid token.

## Verify release (if published)

```bash
curl -L https://github.com/twogc/cloudbridge-client/releases/latest/download/checksums.txt
sha256sum -c checksums.txt
```

## License

Copyright 2025 2GC — Apache License 2.0. See [LICENSE](LICENSE).
