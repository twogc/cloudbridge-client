# CloudBridge Relay Client

Cross-platform client for CloudBridge Relay P2P mesh networking.

## Quick Start

1. **Get JWT Token**: Contact your CloudBridge Relay administrator for a JWT token.

2. **Configure**: Copy `config-example.yaml` to `config.yaml` and update:
   ```yaml
   relay:
     host: "your-relay-server.com"
     port: 8081
   
   auth:
     token: "YOUR_JWT_TOKEN_HERE"
   
   p2p:
     server_id: "my-server-001"
     tenant_id: "my-organization"
   ```

3. **Run**:
   ```bash
   # P2P mesh mode with L3-overlay network
   ./cloudbridge-client p2p --config config.yaml --server-id my-server-001
   
   # Tunnel mode
   ./cloudbridge-client tunnel --config config.yaml --local-port 3389 --remote-host target.com --remote-port 3389
   
   # WireGuard L3-overlay network management
   ./cloudbridge-client wireguard config --config config.yaml --token YOUR_JWT_TOKEN
   ./cloudbridge-client wireguard status --config config.yaml --token YOUR_JWT_TOKEN
   ```

## Security & Secrets

**Production Security Best Practices:**

1. **Environment Variables** (Recommended):
   ```bash
   export CBR_AUTH_TOKEN="your-jwt-token"
   export CBR_RELAY_HOST="your-relay-server.com"
   export CBR_RELAY_PORT="8081"
   ./cloudbridge-client p2p --config config.yaml
   ```

2. **OS Keyring Integration**:
   - **Windows**: Uses Windows Credential Manager
   - **macOS**: Uses Keychain Access
   - **Linux**: Uses libsecret (GNOME Keyring/KDE Wallet)

3. **File Permissions**:
   ```bash
   chmod 600 config.yaml  # Restrict config file access
   chown $USER:$USER config.yaml
   ```

4. **Token Rotation**:
   - Use short-lived JWT tokens (1-24 hours)
   - Implement automatic token refresh
   - Rotate tokens regularly

## Verify Release

**Before running, verify the release integrity:**

1. **Download checksums**:
   ```bash
   curl -L https://github.com/twogc/cloudbridge-client/releases/latest/download/checksums.txt
   ```

2. **Verify binary**:
   ```bash
   sha256sum -c checksums.txt
   ```

3. **Verify signature** (if available):
   ```bash
   cosign verify-blob --certificate-identity="*" --certificate-oidc-issuer="*" \
     --signature cloudbridge-client-linux-amd64.sig cloudbridge-client-linux-amd64
   ```

## Configuration Files

- `config-example.yaml` - Configuration template
- `config-production.yaml` - Production template for edge.2gc.ru
- `config.yaml` - Your configuration (create from example)

## Transport Protocols

- **QUIC** - Primary high-performance transport
- **WebSocket** - Fallback for restricted networks
- **gRPC** - API communication
- **WireGuard** - L3-overlay network support

## L3-overlay Network Features

- **Per-peer IPAM** - Automatic IP address allocation for each peer
- **WireGuard Integration** - Ready-to-use WireGuard configurations
- **Tenant Isolation** - Complete network isolation between tenants
- **Hybrid Architecture** - SCORE for tenant subnets, local DB for per-peer IPs
- **Event-driven Sync** - Real-time configuration updates

## Build

```bash
make build          # Current platform
make build-all      # Cross-platform
make build-windows  # Windows
```

## Requirements

- Go 1.25+
- Valid JWT token from CloudBridge Relay
- Network access to relay server

## License

See LICENSE file for details.