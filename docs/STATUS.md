# Project status — cloudbridge-client

**Snapshot:** 2026-07-09  
**Honesty rule:** if not wired or not smoke-tested, it is not “production ready”.

## Summary

| Area | State |
|------|-------|
| Codebase size | ~20k LOC Go + ~6k tests; single main binary |
| CLI surface | `p2p`, `tunnel`, `session`, `service`, `wireguard`, `version` |
| Docs | **Rebuilding** under `docs/` + openwiki (this pass) |
| Port/API contract vs relay | **WP0–WP2 largely done** (defaults + gRPC/QUIC dial helpers); live smoke still open |
| Unit tests | `pkg/types`, `pkg/config` updated; re-run full suite and record |
| Live e2e vs current relay | **Not verified** in this snapshot |
| Enhanced stack (MASQUE, handover, SLO, probes) | Packages exist; **orchestrator sets nil** |
| Path selection / auto-failover | **Phase A–C partial L1** (`pathselect` + `session --smoke` + unit chaos); live port-block optional |
| Docs | **Rebuilt** under `docs/` + openwiki (2026-07-09) |

## What works in code (implementation present)

- Config load (Viper): YAML, `CLOUDBRIDGE_*`, validation
- Auth: JWT HMAC + OIDC/JWKS
- Relay client orchestrator: connect, auth, tunnel, heartbeat, metrics
- Transport: gRPC preferred, legacy JSON path
- P2P manager: ICE, QUIC peer flows, WG config apply helpers
- Packaging hooks: systemd, multi-OS build Makefile, goreleaser

## What is broken / drifted

| Issue | Severity | Track |
|-------|----------|-------|
| ~~Defaults 5553-as-API~~ | fixed defaults → 5552 / grpc 8444 | WP1 [x] |
| ~~gRPC dials relay.port~~ | uses `GRPCTarget()` | WP2 [x] |
| ~~Hardcoded edge:5553 in p2p~~ | `SetRelayQUICEndpoint` | WP2 [x] |
| ~~Wizard/onboarding/packaging old ports~~ | fixed WP3 | [x] |
| ~~REST `/relay/route` monitoring 400 spam~~ | — | Phase D.1 [x] `route_monitoring_enabled=false` |
| ~~ICE credentials/candidates 404 spam~~ | — | Phase D.2 [x] `ice.signaling_enabled=false` |
| REST path drift discover (LIVE) / ICE (N/A) | P2 | CONTRACT; ICE wait relay |
| ~~No Session / PathSelector foundation~~ | — | Phase A [x] `pkg/pathselect` |
| ~~PathSelector foundation / adapters / unit chaos~~ | — | Phase A–C partial [x] |
| ~~Live pathselect in all-smoke~~ | — | `RUN_PATHSELECT=1` step 7 [x] |
| Real iptables chaos (block 5553/8444) | P2 | optional ops |
| OIDC live e2e | P2 | D.4 residual; lab-hmac is L0 |
| MASQUE/handover/SLO unwired | P2 | GAP Phase F–G |
| Go 1.25.3 vs installer 1.25.12 | P2 | WP5 |
| ~~Live smoke not run~~ | — | `all-smoke` PASS (L0) |

## Active plans

| Plan | Role |
|------|------|
| [plans/README.md](plans/README.md) | Index of all plans |
| [plans/GAP_CLOSURE_AND_IMPROVEMENTS.md](plans/GAP_CLOSURE_AND_IMPROVEMENTS.md) | **Primary roadmap** (phases A–G, improvements) |
| [plans/PATH_SELECTION_STATE_MACHINE.md](plans/PATH_SELECTION_STATE_MACHINE.md) | Target SM + probe ladder |
| [plans/CONTRACT_ALIGNMENT_PLAN.md](plans/CONTRACT_ALIGNMENT_PLAN.md) | Port/dial WP0–WP5 |

### Maturity levels (GAP plan)

| Level | Meaning | State |
|-------|---------|--------|
| **L0** Local baseline | `scripts/all-smoke.sh` | **PASS** |
| L1 Orchestrated MVP | Phase C unit chaos + session CLI | **partial** |
| L2 Control clean | Phase D | **D.1–D.2 + D.5 docs**; D.4 live OIDC open |
| L3 NAT-aware | Phase E | not started |
| L4 Resilient handover | Phase F | not started |
| L5 Enhanced | Phase G | not started |

**Now:** L0 green + Phase A–C partial L1 + **D.1/D.2/D.5**: route mon off, ICE signaling off, [AUTH_PROFILES.md](AUTH_PROFILES.md) (lab-hmac vs prod-oidc).  
Defaults off: `path_select.enabled`, `api.route_monitoring_enabled`, `ice.signaling_enabled`.  
**Next:** D.4 live OIDC optional; live `session --smoke --force`; disk 160G when volume ready.

## Test log (fill as you run)

| Date | Command | Result | Notes |
|------|---------|--------|-------|
| 2026-07-09 | `go test ./pkg/types/ ./pkg/config/ ./pkg/relay/transport/ -count=1` | **OK** | Go 1.25.12 |
| 2026-07-09 | `go test ./pkg/onboarding/ -count=1` | **OK** | ports assertions |
| 2026-07-09 | `go build ./cmd/cloudbridge-client` | **OK** | after WP3 |
| 2026-07-09 | `go test ./... -short` | **partial FAIL** | auth OIDC network — fixed offline mock |
| 2026-07-09 | `go test ./pkg/auth/ …` after OIDC fix | **OK** | |
| 2026-07-09 | WP4 path align + build | **OK** | discover/route |
| 2026-07-09 | Local relay + REST smoke | **OK** | register/discover/status |
| 2026-07-09 | CLI p2p register against local | **OK** | HTTP 200 then long-run mesh |
| 2026-07-09 | Heartbeat route + gRPC local-smoke | **OK** | :8444 listen; POST heartbeat 200 |
| 2026-07-09 | Track A `p2p --smoke` + 2-peer script | **OK** | MESH_SMOKE_PASS=1 |
| 2026-07-09 | Track B gRPC Hello/Auth | **OK** | GRPC_SMOKE_PASS=1 :8444 |
| 2026-07-09 | `go test ./pkg/pathselect/ -count=1` | **OK** | Phase A foundation |
| 2026-07-09 | `go test ./pkg/pathselect/ -count=1 -v` (Phase B verify) | **OK** | 33 tests PASS — RelayQUIC + GRPCTunnel + LadderSelector |
| 2026-07-09 | `go test ./pkg/types/ ./pkg/config/ -count=1` (Phase B verify) | **OK** | config `path_select:` defaults |
| 2026-07-09 | `go build -o /tmp/cloudbridge-client-check ./cmd/cloudbridge-client` | **OK** | no CLI wiring; `path_select.enabled` default false |
| 2026-07-09 | CreateTunnel + full QUIC AUTH | **OK** | TUNNEL_QUIC_SMOKE_PASS=1 |
| 2026-07-09 | CreateTunnel **TCP bytes** | **OK** | tunnel_bytes_ok payload echo |
| 2026-07-09 | QUIC post-AUTH **PING/PONG** | **OK** | second stream after AUTH_OK |
| 2026-07-09 | Phase C `go test ./pkg/pathselect/` + chaos | **OK** | C-chaos-1/2 unit; metrics; session CLI |
| 2026-07-09 | `go build` + `session --help` | **OK** | Phase C orchestrator finish after cursor fail |
| 2026-07-09 | Phase D.1 build + tests | **OK** | route_monitoring_enabled=false |
| 2026-07-09 | Phase D.2/D.5 build + tests | **OK** | ice.signaling_enabled=false; AUTH_PROFILES |
| 2026-07-09 | local-smoke relay + `all-smoke.sh` | **ALL_SMOKE_PASS=1** | lab-hmac; D.1 skip route open |
| 2026-07-09 | `session --smoke --force` live | **SESSION_SMOKE_PASS=1** | active_path=relay_quic |
| 2026-07-09 | D.3 p2p smoke heartbeat | **OK** | interval 10s from YAML; no skip warn |
| 2026-07-09 | `RUN_PATHSELECT=1 all-smoke.sh` | **ALL_SMOKE_PASS=1** | step 7 SESSION_SMOKE_PASS active_path=relay_quic |
| 2026-07-09 | **CLI tunnel e2e** `tunnel --smoke` | **OK** | localPort → relay endpoint → remote |
| 2026-07-09 | **QUIC multi-peer mesh** A↔B `TO:<peer>` | **OK** | scripts/quic-mesh-smoke |
| 2026-07-09 | **`p2p --smoke-data`** | **OK** | membership + QUIC AUTH/PING |
| 2026-07-09 | **`scripts/all-smoke.sh`** | **OK** | ALL_SMOKE_PASS=1 |
| | smoke REST :5552 | | |
| | smoke gRPC :8444 | | |
| | smoke `p2p` CLI | | |

## UNWIRED / aspirational (do not market as GA)

- MASQUE client in `pkg/masque` — not attached in `relay.Client.initializeEnhancedComponents`
- Handover manager, SLO controller, synthetic probes — same
- Full multi-relay load balancing narrative in Architecture.md — not proven e2e here

## Last meaningful git note

Repo previously sparse on commits (OpenWiki add). Treat operational readiness as **rebuild from contract**, not “resume green CI”.
