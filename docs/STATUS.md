# Project status — cloudbridge-client

**Snapshot:** 2026-07-09  
**Honesty rule:** if not wired or not smoke-tested, it is not “production ready”.

## Summary

| Area | State |
|------|-------|
| Codebase size | ~20k LOC Go + ~6k tests; single main binary |
| CLI surface | `p2p`, `tunnel`, `service`, `wireguard`, `version` |
| Docs | **Rebuilding** under `docs/` + openwiki (this pass) |
| Port/API contract vs relay | **WP0–WP2 largely done** (defaults + gRPC/QUIC dial helpers); live smoke still open |
| Unit tests | `pkg/types`, `pkg/config` updated; re-run full suite and record |
| Live e2e vs current relay | **Not verified** in this snapshot |
| Enhanced stack (MASQUE, handover, SLO, probes) | Packages exist; **orchestrator sets nil** |
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
| REST path drift (discover, connections, ICE) | P1 | WP4 |
| MASQUE/handover/SLO unwired | P2 | later |
| Go 1.25.3 vs installer 1.25.12 | P2 | WP5 |
| Live smoke not run | P0 ops | CHECKLISTS §5 |

## Active plan

See [plans/CONTRACT_ALIGNMENT_PLAN.md](plans/CONTRACT_ALIGNMENT_PLAN.md).

**Now:** local smoke baseline recorded (CHECKLISTS §5.0). Next: enable gRPC in local-smoke, fix peer heartbeat mount, optional full CLI mesh; or push commits.

## Test log (fill as you run)

| Date | Command | Result | Notes |
|------|---------|--------|-------|
| 2026-07-09 | `go test ./pkg/types/ ./pkg/config/ ./pkg/relay/transport/ -count=1` | **OK** | Go 1.25.12 |
| 2026-07-09 | `go test ./pkg/onboarding/ -count=1` | **OK** | ports assertions |
| 2026-07-09 | `go build ./cmd/cloudbridge-client` | **OK** | after WP3 |
| 2026-07-09 | `go test ./... -short` | **partial FAIL** | auth OIDC network — fixed offline mock |
| 2026-07-09 | `go test ./pkg/auth/ …` after OIDC fix | **OK** | |
| 2026-07-09 | WP4 path align + build | **OK** | discover/route |
| 2026-07-09 | Local relay + REST smoke | **OK** | register/discover/status; heartbeat 404 |
| 2026-07-09 | CLI p2p register against local | **OK** | HTTP 200 then long-run mesh |
| | smoke REST :5552 | | |
| | smoke gRPC :8444 | | |
| | smoke `p2p` CLI | | |

## UNWIRED / aspirational (do not market as GA)

- MASQUE client in `pkg/masque` — not attached in `relay.Client.initializeEnhancedComponents`
- Handover manager, SLO controller, synthetic probes — same
- Full multi-relay load balancing narrative in Architecture.md — not proven e2e here

## Last meaningful git note

Repo previously sparse on commits (OpenWiki add). Treat operational readiness as **rebuild from contract**, not “resume green CI”.
