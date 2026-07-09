# Contract: cloudbridge-client ↔ cloudbridge-relay

**Status:** Canonical (client side)  
**Date:** 2026-07-09  
**Server SoT:** `cloudbridge-relay-installer/openwiki/port-scheme.md`  
**Client implementation plan:** [plans/CONTRACT_ALIGNMENT_PLAN.md](plans/CONTRACT_ALIGNMENT_PLAN.md)

Host placeholder: `{relay}` (default prod example: `edge.2gc.ru`).

---

## 1. L4 listeners (relay) and client dial

| Role | Port | Protocol | Relay source | Client config key | Client dial helper |
|------|-----:|----------|--------------|-------------------|--------------------|
| Main TCP (legacy control) | **5550** | TCP (+TLS) | `server.port` | `relay.port` (legacy) | `LegacyTCPAddr()` |
| P2P REST API | **5552** | TCP / HTTPS | `api.port` | `relay.ports.p2p_api` | `P2PAPIBaseURL()` |
| HTTPS API | **5553** | TCP / TLS | `https_api.port` | `relay.ports.https_api` | optional |
| P2P QUIC | **5553** | **UDP** | hardcoded | `relay.ports.quic` | `P2PQUICAddr()` |
| HTTP API (health/REST) | **9090** | TCP | `http_api.port` | `relay.ports.http_api` | optional HTTP base |
| Main QUIC | **9090** | **UDP** | hardcoded | `relay.ports.quic_main` | `QUICMainAddr()` |
| Enhanced QUIC | **9092** | UDP | hardcoded | `relay.ports.enhanced_quic` | optional |
| Relay metrics | **5551** | TCP | `metrics.port` | (scrape only) | — |
| Health / TLS tunnel | **9091** | TCP | `https.go` | — | do not confuse with client metrics |
| gRPC control | **8444** | TCP / gRPC | `grpc.port` | `relay.ports.grpc` | `GRPCTarget()` |
| MASQUE HTTP/3 | **8443** | TCP/UDP H3 | masque | `relay.ports.masque` | **UNWIRED** in client orchestrator |
| STUN | **19302** | UDP | hardcoded | `ice.stun_servers` / `ports.stun` | ICE |
| TURN | **3478** | UDP/TCP | built-in/coturn | `ice.turn_servers` / `ports.turn` | ICE |
| DERP | **3479** | UDP | hardcoded | `ice.derp_servers` / `ports.derp` | ICE |
| WireGuard | **51820** | UDP | wireguard | `wireguard.port` | local + peer |

### Hard rules

1. **Never** dial gRPC on `relay.port` if that port is used for QUIC UDP (5553).
2. **Never** treat **5553** as the default **HTTPS REST** base; REST = **5552**, P2P QUIC = **5553/UDP**, HTTPS API TCP = **5553** only when explicitly using `https_api`.
3. Client local Prometheus default = **9091** (not relay metrics 5551, not host Prometheus 9090).

---

## 2. REST control plane (aligned paths)

**Base (canonical):** `https://{relay}:5552`

| Method | Path | Status |
|--------|------|--------|
| GET | `/health` | OK on relay P2P API |
| GET | `/ready` | OK |
| POST | `/api/v1/tenants/:tid/peers/register` | OK both sides |
| GET | `/api/v1/tenants/:tid/peers` | Client DiscoverPeers (aligned) |
| GET | `/api/v1/p2p/discover` | Relay alternate |
| PUT | `/api/v1/tenants/:tid/peers/:pid/status` | OK shape |
| POST | `/api/v1/tenants/:tid/peers/:pid/heartbeat` | OK shape (mount verify e2e) |
| GET | `/api/v1/tenants/:tid/peers/:pid/wireguard-config` | OK shape |
| POST | `/api/v1/relay/route` | Relay: `from_peer`,`to_peer`,`data`,`message_type` (message route). **Not** ConnectionRequest monitoring. Client monitoring open **disabled by default** (`api.route_monitoring_enabled=false`, Phase D.1) |
| GET/POST | `/api/v1/wireguard/*` | Often on HTTP mux **9090** — dual-base if 404 on 5552 |
| WSS | `/ws` | Prefer via API/ingress, not “5553 as HTTPS” |

### Known missing / deferred (PR-4)

| Client path | Relay | Action |
|-------------|-------|--------|
| `…/ice-credentials` | **MISSING** | Client **disabled by default** (`ice.signaling_enabled=false`, Phase D.2) |
| `/api/v1/ice/candidates` | **MISSING** | same gate |
| `/api/v1/p2p/connect` | not found | N/A for L0 smoke |
| ~~`/api/v1/relay/connections/*`~~ | mapped to `/relay/route` + peer heartbeat | client WP4 partial |
| POST `/api/v1/relay/route` as ConnectionRequest | wrong schema | **disabled** (`api.route_monitoring_enabled=false`, D.1) |

**ICE status (D.2):** relay signaling **N/A** on installer SoT until routes exist. Local STUN/TURN gather may still run; REST exchange skipped unless `ice.signaling_enabled=true`.

---

## 3. gRPC control plane

**Target:** `{relay}:8444`

| Service | RPCs |
|---------|------|
| `ControlService` | Hello, Authenticate, GetStatus |
| `HeartbeatService` | SendHeartbeat, StreamHeartbeat, GetHealth |
| `TunnelService` | CreateTunnel, CloseTunnel, ListTunnels, GetTunnelStatus, StreamData |

Client protos: `pkg/relay/transport/proto/`.

CLI default: `--transport grpc`.

---

## 4. Data plane

| Flow | Endpoint | Notes |
|------|----------|-------|
| P2P QUIC to relay | `{relay}:5553` UDP | Auth stream `AUTH <token>` (verify e2e) |
| Main QUIC | `{relay}:9090` UDP | Optional primary after align |
| Peer QUIC after ICE | dynamic | STUN/TURN required |
| WireGuard overlay | 51820 UDP | Needs API config fetch |
| MASQUE | 8443 | Package exists; orchestrator **nil** |

---

## 5. Auth

| Mode | Client | Relay (post-cleanup) |
|------|--------|----------------------|
| OIDC / JWKS (Zitadel) | `auth.type=oidc` | Canonical for production |
| JWT HMAC secret | `auth.type=jwt` | **Lab / local-smoke** profile |
| Claims | `tenant_id`, `org_id`, `permissions`, network/mesh | Align field names with relay `RelayClaims` |

Onboarding invites → **control plane** (`/api/v1/invites/*`), not relay.

### 5.1 Lab vs production auth profiles (Phase D.5)

| Profile | When | Client | Relay | gRPC Authenticate |
|---------|------|--------|-------|-------------------|
| **lab-hmac** | `scripts/local-smoke*`, `all-smoke` | `auth.type=jwt` + shared HMAC secret; often `auth.skip_validation` only in extreme dev | `config.local-smoke.json`, TLS often off | **Real** RPC to `:8444` with JWT; not a NoOp transport, but **not** Zitadel JWKS |
| **prod-oidc** | staging/prod | `auth.type=oidc` + `issuer_url` / `audience` / JWKS | Zitadel | Real JWT validation path aligned with OIDC issuer |

**Honesty:** L0 smokes prove **lab-hmac**, not enterprise OIDC e2e. Do not claim “OIDC production-ready” until a live or CI OIDC smoke is green (D.4 residual).

See also: [AUTH_PROFILES.md](./AUTH_PROFILES.md).

---

## 6. Status legend (matrix work)

| Tag | Meaning |
|-----|---------|
| OK | Aligned |
| PORT | Wrong port/base |
| PATH | Path mismatch |
| MISSING | No server route |
| UNWIRED | Code not hooked |
| DOCS | Docs only |

---

## 7. Changelog

| Date | Change |
|------|--------|
| 2026-07-09 | Initial contract from dual-repo analysis; PR-1 alignment started |
| 2026-07-09 | D.1 route monitoring off; D.2 ICE signaling off; D.5 lab-hmac vs prod-oidc profiles |
