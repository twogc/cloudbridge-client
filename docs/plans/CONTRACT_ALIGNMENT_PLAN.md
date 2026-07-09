# Plan: Client ↔ Relay contract alignment

**Project:** `cloudbridge-client`  
**Created:** 2026-07-09  
**Owner:** engineering (2GC)  
**Companion:** [../CONTRACT_CLIENT_RELAY.md](../CONTRACT_CLIENT_RELAY.md) · [../CHECKLISTS.md](../CHECKLISTS.md) · [../STATUS.md](../STATUS.md)

## Context

- Client long-lived; incomplete test coverage; docs/README drift (e.g. port **8081**).
- Relay installer underwent major cleanup (Zitadel-only, port-scheme SoT, SCORE wiring).
- Without client alignment, pilot `client → relay` fails on **ports/URLs**, not missing features.

## Goals

1. Rebuild **operational documentation** (contract, checklists, status).
2. Align **defaults + dial** with relay port-scheme.
3. Establish **testable baseline** (unit + smoke checklist).
4. Defer non-blocking work (MASQUE wire, full path redesign) behind flags/checklists.

## Non-goals

- Full monorepo refactor of `pkg/relay.Client`.
- Wiring MASQUE/handover/SLO in this plan.
- Control-plane rewrite.
- Relay full merge of k8s trees (server-side).

---

## Target port map (summary)

| Role | Port | Proto | Client key |
|------|-----:|-------|------------|
| Legacy main TCP | 5550 | TCP | `relay.port` |
| P2P REST | 5552 | TCP | `relay.ports.p2p_api` |
| HTTPS API | 5553 | TCP | `relay.ports.https_api` |
| P2P QUIC | 5553 | UDP | `relay.ports.quic` |
| HTTP API | 9090 | TCP | `relay.ports.http_api` |
| Main QUIC | 9090 | UDP | `relay.ports.quic_main` |
| gRPC | 8444 | TCP | `relay.ports.grpc` |
| MASQUE | 8443 | H3 | `relay.ports.masque` |
| Client metrics | 9091 | TCP local | `metrics.prometheus_port` |

---

## Work packages

### WP0 — Documentation rebuild

| ID | Task | Status |
|----|------|--------|
| WP0.1 | `docs/README.md` index | [x] |
| WP0.2 | `docs/CONTRACT_CLIENT_RELAY.md` SoT | [x] |
| WP0.3 | This plan + checklists + STATUS | [x] |
| WP0.4 | Update `openwiki/quickstart.md` links | [x] |
| WP0.5 | Update `openwiki/domain/config-and-auth.md` ports | [x] |
| WP0.6 | Update `openwiki/domain/networking.md` dial rules | [x] |
| WP0.7 | Update `openwiki/testing.md` smoke section | [x] |
| WP0.8 | Rewrite README Quick Start ports (drop 8081) | [x] |
| WP0.9 | AGENTS.md → docs entry | [x] |
| WP0.10 | Architecture.md honesty note (UNWIRED) | [x] |

### WP1 — Schema + defaults + helpers (PR-1)

| ID | Task | File(s) | Status |
|----|------|---------|--------|
| WP1.1 | Extend `RelayPorts` (grpc, https_api, quic_main) | `pkg/types/types.go` | [x] |
| WP1.2 | Add `addrs.go` helpers | `pkg/types/addrs.go` | [x] |
| WP1.3 | Canonical `setDefaults()` | `pkg/config/config.go` | [x] |
| WP1.4 | SaveToFile new port fields | `pkg/types/config_methods.go` | [x] |
| WP1.5 | Unit tests defaults/helpers | `*_test.go` | [x] |
| WP1.6 | TURN weak default password → empty | `pkg/config/config.go` | [x] |
| WP1.7 | Align turn/derp servers with ice (not localhost) | `pkg/config/config.go` | [x] |

### WP2 — Dial wiring (PR-2)

| ID | Task | File(s) | Status |
|----|------|---------|--------|
| WP2.1 | gRPC dial → `GRPCTarget()` / ports.grpc | `pkg/relay/transport/grpc_client.go` | [x] |
| WP2.2 | P2P QUIC derive from config | `pkg/p2p/manager.go` + SetRelayQUICEndpoint | [x] |
| WP2.3 | Legacy JSON Connect effective port | `pkg/relay/client.go` | [x] |
| WP2.4 | Heartbeat base URL consistency | `pkg/api/manager.go` | [x] via BaseURL / P2PAPIBaseURL |
| WP2.5 | Transport unit tests | `pkg/relay/transport/*_test.go` | [x] suite green |

### WP3 — Templates / wizard / packaging (PR-3)

| ID | Task | Status |
|----|------|--------|
| WP3.1 | Root `config.yaml` template full ports block | [x] |
| WP3.2 | Wizard defaults 5552/8444/5553 | [x] |
| WP3.3 | Onboarding generated config ports | [x] |
| WP3.4 | `packaging/common/config.yaml` + `pkg-build/` | [x] |
| WP3.5 | Env docs: `CLOUDBRIDGE_*` only | [x] README/openwiki |

### WP4 — REST path alignment (PR-4, after smoke)

| ID | Task | Status |
|----|------|--------|
| WP4.1 | Discover path: client ↔ relay | [x] `GET …/peers` |
| WP4.2 | connections/* → route or deprecate | [x] open/close → `/relay/route`; heartbeat deprecated with clear error |
| WP4.3 | ICE endpoints: implement or feature-flag off | [ ] still missing on relay |
| WP4.4 | WireGuard dual-base if 404 on 5552 | [ ] needs live smoke |

### WP5 — Security / hygiene (optional)

| ID | Task | Status |
|----|------|--------|
| WP5.1 | `quic.insecure_skip_verify` default false | [ ] |
| WP5.2 | Go module 1.25.3 → 1.25.12 (align installer) | [ ] |
| WP5.3 | Document TLS server_name vs host | [ ] |

---

## Execution order

```
WP0 docs (this pass)
  → WP1 schema/defaults
  → WP2 dial
  → WP3 templates/docs polish
  → smoke checklist (CHECKLISTS.md)
  → WP4 only if e2e 404/path failures
  → WP5 as capacity allows
```

## Success criteria

- [x] Default REST base is `:5552`
- [x] Default gRPC dial is `:8444`
- [x] Default P2P QUIC is `:5553` from config (no hardcoded host:port only)
- [x] `rg '8081' README.md` → empty
- [x] `go test ./pkg/config/ ./pkg/types/ ./pkg/relay/transport/ -count=1` green (2026-07-09)
- [ ] Smoke checklist filled at least once against a real or staging relay
- [x] STATUS.md reflects remaining UNWIRED / PATH gaps

## Risks

| Risk | Mitigation |
|------|------------|
| Edge live uses non-SoT ports | Keep host/port override via yaml/env; defaults = SoT |
| Breaking old yaml with only `relay.port: 5553` | Document migration; helpers fall back with warnings in logs |
| gRPC not exposed publicly | Document internal/LB; client still dials correct port |
| Incomplete e2e env | Checklists separate unit vs live smoke |

## Decision log

| Date | Decision |
|------|----------|
| 2026-07-09 | Prefer client defaults = installer port-scheme, not ad-hoc edge snapshot |
| 2026-07-09 | gRPC port is first-class `relay.ports.grpc`, not `relay.port` |
| 2026-07-09 | PATH/ICE work deferred to WP4 after dial fix |
| 2026-07-09 | Docs rebuilt under `docs/` + openwiki links; Architecture.md not deleted |
