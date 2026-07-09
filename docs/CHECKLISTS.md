# Checklists — rebuild, test, smoke, release

Use these when bringing the client back to a testable baseline after long dormancy.

Related: [STATUS.md](STATUS.md) · [plans/CONTRACT_ALIGNMENT_PLAN.md](plans/CONTRACT_ALIGNMENT_PLAN.md) · [CONTRACT_CLIENT_RELAY.md](CONTRACT_CLIENT_RELAY.md)

---

## 0. Repo health (once)

- [ ] `git status` clean or intentional WIP branch
- [ ] Go toolchain installed (`go version` ≥ 1.25)
- [ ] `go mod download` succeeds
- [ ] Read [docs/README.md](README.md) + [CONTRACT_CLIENT_RELAY.md](CONTRACT_CLIENT_RELAY.md)
- [ ] Know target relay host (staging / edge / local)

---

## 1. Documentation rebuild

- [x] `docs/` index + contract + plan + STATUS + checklists
- [x] openwiki links updated (quickstart, config, networking, testing)
- [x] README ports match contract (no 8081)
- [x] AGENTS.md points to `docs/`
- [x] Architecture.md notes UNWIRED features
- [x] packaging configs match defaults (WP3)
- [x] root `config.yaml` template aligned
- [x] wizard / onboarding generate canonical ports

---

## 2. Build

```bash
# from repo root
make build
# or
go build -o bin/cloudbridge-client ./cmd/cloudbridge-client
```

- [ ] Build succeeds on current OS
- [ ] `./bin/cloudbridge-client version` prints metadata
- [ ] `./bin/cloudbridge-client --help` lists `p2p`, `tunnel`, `service`, `wireguard`

Optional cross-build:

- [ ] `make build-linux` / `build-windows` / `build-darwin` (as needed)

---

## 3. Unit / package tests

```bash
go test ./pkg/config/ ./pkg/types/ -count=1
go test ./pkg/auth/ ./pkg/errors/ -count=1
go test ./pkg/relay/transport/ -count=1
go test ./pkg/p2p/ ./pkg/quic/ -count=1 -short
go test ./... -count=1 -short
```

- [ ] `pkg/config` green
- [ ] `pkg/types` green (incl. addr helpers)
- [ ] `pkg/auth` green (short)
- [ ] `pkg/relay/transport` green
- [ ] Full `./... -short` recorded (note failures in STATUS.md — do not ignore silently)

Lint (if golangci available):

```bash
golangci-lint run ./...
```

- [ ] Lint run or explicitly skipped with reason

---

## 4. Config contract self-check

After WP1 defaults:

- [ ] Empty/minimal config loads with `p2p_api=5552`, `grpc=8444`, `quic=5553`
- [ ] `api.base_url` default ends with `:5552`
- [ ] `GRPCTarget()` → `host:8444`
- [ ] `P2PQUICAddr()` → `host:5553`
- [ ] No default TURN password `cloudbridge123`
- [ ] Env override works, e.g. `CLOUDBRIDGE_RELAY_HOST=...`

Manual:

```bash
# after helpers land — small go test or debug print
go test ./pkg/types/ -run Addr -v
go test ./pkg/config/ -count=1 -v
```

---

## 5. Smoke against relay (live / staging)

**Prerequisites:** valid JWT or OIDC setup; network to relay; firewall allows ports below.

### 5.0 Local loop (dev host)

Relay: `cloudbridge-relay-installer/scripts/local-smoke/run-relay.sh start`  
Client REST: `cloudbridge-client/scripts/local-smoke.sh`  
Optional CLI: `RUN_CLI=1 cloudbridge-client/scripts/local-smoke.sh`  
**2-peer membership:** `scripts/mesh-smoke-2peer.sh` (uses `p2p --smoke`)

**2026-07-09 local results**

| Check | Result |
|-------|--------|
| Relay start (`config.local-smoke.json`, `-tls=false`) | PASS — :5550 :5551 :5552 :5553 |
| `GET :5552/health` | PASS |
| JWT register (secret `test-secret`) | PASS — needs `protocol_type=p2p-mesh`, `connection_type=quic`, `p2p_connect` |
| `GET …/peers` discover | PASS |
| `PUT …/status` | PASS |
| `POST …/heartbeat` | PASS (route wired on relay 2026-07-09) |
| CLI `p2p` register | PASS HTTP 200 (then runs long mesh loop; use timeout) |
| gRPC :8444 | PASS listen (local-smoke enables grpc, tls off) |
| Host note | Prometheus already uses TCP **9090**; smoke config moves HTTP API to **19090** |

### 5.1 L4 reachability

| Check | Command / method | Pass |
|-------|------------------|------|
| REST health :5552 | `curl -sk https://{relay}:5552/health` | [x] local http |
| HTTP API :9090 | `curl -s http://{relay}:9090/health` (if exposed) | [ ] local uses 19090 / often off |
| gRPC :8444 | `nc -vz {relay} 8444` or grpcurl | [ ] enable in relay config |
| QUIC UDP :5553 | client p2p log / packet capture | [~] server listens |
| STUN :19302 | ICE gather logs | [~] server started |
| TURN :3478 | ICE relay candidate (if needed) | [~] server started |

### 5.2 Client CLI

```bash
export CLOUDBRIDGE_AUTH_TOKEN="..."   # if using env substitution patterns
# Prefer explicit config:
./bin/cloudbridge-client p2p \
  --config config.yaml \
  --token "$TOKEN" \
  --transport grpc \
  --log-level debug
```

- [ ] Connect / Hello (gRPC) succeeds on **8444**
- [ ] Authenticate succeeds (token validated)
- [ ] Peer register REST hits **5552** (debug logs show URL)
- [ ] Heartbeat starts without tight error loop
- [ ] Graceful Ctrl+C shutdown

Tunnel mode (optional):

```bash
./bin/cloudbridge-client tunnel \
  --config config.yaml --token "$TOKEN" \
  --local-port 13389 --remote-host 127.0.0.1 --remote-port 3389
```

- [ ] Tunnel create OK or documented failure

WireGuard (optional):

```bash
./bin/cloudbridge-client wireguard status --config config.yaml --token "$TOKEN"
```

- [ ] Config fetch OK or 404 documented (dual-base WP4)

### 5.3 Failure recording

For each fail, log:

| Field | Value |
|-------|-------|
| Date | |
| Host | |
| Step | |
| Expected | |
| Actual | |
| Logs snippet | |
| Follow-up WP | WP2 / WP4 / server |

---

## 6. Auth matrix

| Mode | Config | Result |
|------|--------|--------|
| JWT HMAC (dev) | `auth.type=jwt` + secret | [ ] |
| OIDC Zitadel | `auth.type=oidc` + issuer/audience | [ ] |
| Skip validation | **dev only** | [ ] must stay off in prod templates |

- [ ] Claims `tenant_id` present after auth
- [ ] Expired token rejected

---

## 7. Packaging / service (optional)

- [ ] `install.sh` dry-run reviewed
- [ ] systemd unit paths match binary/config
- [ ] Windows/macOS notes still valid or marked stale

---

## 8. Pre-release / handoff

- [ ] STATUS.md updated
- [ ] Plan WP checkboxes updated
- [ ] No secrets in committed `config.yaml`
- [ ] Tag / changelog note (if releasing)
- [ ] Relay version / commit tested against recorded

---

## 9. Regression guards (after WP1–2)

```bash
# Must not reappear as sole gRPC dial:
rg 'Relay\.Host, gc.config.Relay.Port' pkg/relay/transport || true
rg 'edge\.2gc\.ru:5553' pkg/p2p/manager.go || true
rg '8081' README.md docs/ || true
rg 'cloudbridge123' pkg/config/config.go || true
```

- [ ] Guards clean or justified

---

## Sign-off

| Role | Name | Date | Notes |
|------|------|------|-------|
| Implementer | | | |
| Reviewer | | | |
