# Plan: закрытие недостающих компонентов + предложения по улучшению

**Project:** `cloudbridge-client` (+ минимальные правки relay при необходимости)  
**Created:** 2026-07-09  
**Documented:** 2026-07-09  
**Status:** **documented / ready to execute** (not started coding Phase A)  
**Index:** [plans/README.md](./README.md)

**Depends on:**  
- [PATH_SELECTION_STATE_MACHINE.md](./PATH_SELECTION_STATE_MACHINE.md) — целевая модель (диаграммы SM + ladder)  
- [CONTRACT_ALIGNMENT_PLAN.md](./CONTRACT_ALIGNMENT_PLAN.md) — WP0–WP3 done, WP4+ open  
- [../STATUS.md](../STATUS.md) · [../CHECKLISTS.md](../CHECKLISTS.md) · [../CONTRACT_CLIENT_RELAY.md](../CONTRACT_CLIENT_RELAY.md)

**Honesty rule:** «есть package» ≠ «работает в runtime». Закрытие = **wired + smoke PASS + STATUS update**.

### Phase checklist (track here)

| Phase | Title | Status |
|-------|--------|--------|
| A | Foundation: `pkg/pathselect` | [ ] not started |
| B | Adapters: RelayQUIC + GRPCTunnel | [ ] |
| C | Selector + CLI + chaos smokes (**L1 milestone**) | [ ] |
| D | Control-plane hygiene (WP4 + OIDC) | [ ] |
| E | ICE / STUN / TURN ladder | [ ] |
| F | Handover + session health | [ ] |
| G | Enhanced (MASQUE / SLO / multi-relay) | [ ] |

---

## 0. Цель плана

Собрать из разрозненных LIVE-путей **один управляемый connectivity-слой** для A↔B:

1. Стабильный app-facing endpoint (tunnel / peer face).  
2. Path ladder с fail-over.  
3. Health + (позже) handover.  
4. Доказуемость chaos-smokes, не только happy path.

**Не цель:** переписать monorepo, «добавить ещё 3 протокола», маркетинг GA mesh без e2e.

---

## 1. Инвентарь пробелов (что закрывать)

### 1.1 Критичные для «стабильной связи A↔B»

| ID | Пробел | Сейчас | Риск |
|----|--------|--------|------|
| G1 | Нет **Session** как объекта | CLI/path ad-hoc | Нет reconnect/failover semantics |
| G2 | Нет **PathSelector** / ladder | Пути вызываются вручную | Нет auto-switch |
| G3 | gRPC tunnel и QUIC mesh **не унифицированы** | Два API, два smoke | App не переживает смену path |
| G4 | Reconnect после обрыва | Partial retries on connect | Долгая «мёртвая» сессия |
| G5 | Health = не session-level | UDP probe MASQUE / none | Ложные switch / no switch |
| G6 | REST drift (`/relay/route` 400 и т.п.) | best-effort | Шум, ложные fails |
| G7 | OIDC/Zitadel e2e | local HMAC + NoOp gRPC | Prod auth path не доказан |

### 1.2 Важные, но после ядра

| ID | Пробел | Сейчас |
|----|--------|--------|
| G8 | ICE → direct QUIC → TURN ladder | packages partial |
| G9 | WireGuard как path в ladder | AutoSwitch отдельный, WG often off |
| G10 | Handover dual-path | `handoverManager = nil` |
| G11 | MASQUE / H3 | `masqueClient = nil` |
| G12 | SLO + synthetic probes | nil |
| G13 | Multi-relay / sticky | narrative only |
| G14 | Cross-tenant isolation smoke | unit-ish on relay |
| G15 | Log hygiene relay (19G flood fixed partially) | ops |

### 1.3 Уже LIVE (не «закрывать», а **переиспользовать**)

| Компонент | Smoke |
|-----------|--------|
| REST membership | `p2p --smoke`, mesh-2peer |
| QUIC AUTH+PING | `p2p --smoke-data`, quic-smoke |
| QUIC A↔B `TO:peer` | quic-mesh-smoke |
| gRPC CreateTunnel + bytes | grpc-smoke, `tunnel --smoke` |
| all-smoke harness | `scripts/all-smoke.sh` |

**Принцип:** новые фазы = **adapters + orchestrator**, не дублирование smoke-логики.

---

## 2. Фазовый план закрытия

Каждая фаза: **Deliverables · Code · Acceptance · Out of scope · Effort**.

Effort: **S** ≤2d · **M** 3–5d · **L** 1–2w · **XL** >2w (1 eng, local-smoke).

---

### Phase A — Foundation (Session + Path interfaces) · **M**

**Зачем:** общий язык для всех последующих path.

| Deliverable | Detail |
|-------------|--------|
| A.1 | `pkg/pathselect` (or `pkg/session`): `SessionState`, `Path`, `ProbeResult` |
| A.2 | Config block `path_select:` order, timeouts, health_interval |
| A.3 | Unit tests: state transitions, timeout, order |
| A.4 | Docs: link from PATH_SELECTION + STATUS |

**Interfaces (sketch):**

```go
type Path interface {
    Name() string
    Probe(ctx context.Context) error          // cheap liveness
    Open(ctx context.Context, req OpenRequest) (Handle, error)
    Close(ctx context.Context) error
}

type Selector interface {
    Ensure(ctx context.Context, sess *Session) (Path, Handle, error)
    HealthTick(ctx context.Context, sess *Session) error
}
```

**Acceptance:**

- [ ] `go test ./pkg/pathselect/...` green  
- [ ] No behavior change in production CLI yet (library only)  
- [ ] STATUS: Phase A done  

**Out of scope:** wiring into `p2p`/`tunnel` CLI.

---

### Phase B — Adapters for LIVE paths · **M**

**Зачем:** ladder ходит по уже рабочему коду.

| Deliverable | Wraps |
|-------------|--------|
| B.1 `RelayQUICPath` | dial AUTH + PING / optional TO:peer |
| B.2 `GRPCTunnelPath` | CreateTunnel + local proxy (existing tunnel manager) |
| B.3 Shared dial helpers | host/port from `types.Config` (no hardcode) |
| B.4 Adapter unit/integration tests | with local-smoke optional build tag `//go:build smoke` |

**Acceptance:**

- [ ] Adapter probes succeed against local-smoke (manual or `-tags smoke`)  
- [ ] `all-smoke` still green without new CLI  
- [ ] No nil deref if WG/MASQUE disabled  

**Out of scope:** ICE/WG/MASQUE adapters (Phase E+).

---

### Phase C — Selector + CLI wiring (MVP failover) · **L**

**Зачем:** первый реальный auto-switch между **двумя LIVE** paths.

| Deliverable | Detail |
|-------------|--------|
| C.1 | `Selector.Ensure`: try order from config |
| C.2 | Default order: `relay_quic, grpc_tunnel` (or reverse for service-tunnel product) |
| C.3 | CLI: `p2p --connect <peer_id>` **or** extend `tunnel --smoke` / new `session --smoke` |
| C.4 | On active path death: re-ladder with cooldown |
| C.5 | Metrics counters: path_selected, path_fail, failover_total |

**Config example:**

```yaml
path_select:
  enabled: true
  order: [relay_quic, grpc_tunnel]
  probe_timeout: 5s
  ladder_timeout: 45s
  health_interval: 10s
  failover_cooldown: 15s
  soft_fail: false
```

**Acceptance:**

- [ ] Happy: Ensure opens first path, bytes OK  
- [ ] Chaos smoke **C-chaos-1:** block UDP :5553 → selector uses grpc_tunnel (PASS)  
- [ ] Chaos smoke **C-chaos-2:** block TCP :8444 → selector uses relay_quic (PASS)  
- [ ] Documented in CHECKLISTS §5  
- [ ] `ALL_SMOKE` optional include chaos (flag `RUN_CHAOS=1`)  

**Out of scope:** seamless zero-byte-loss handover (Phase F).

---

### Phase D — Control-plane hygiene (WP4 + auth) · **M**

**Зачем:** убрать шум и prod-auth gap, иначе failover маскируется 400/auth errors.

| ID | Task | Acceptance |
|----|------|------------|
| D.1 | Align `/relay/route` client schema **or** disable probe when unsupported | no WARN spam on happy path |
| D.2 | Discover/ICE/credentials path audit vs relay SoT | contract table updated |
| D.3 | Heartbeat interval defaults in smoke configs | no «interval 0» warn |
| D.4 | OIDC smoke (optional CI job) against test issuer / mock | offline + one live job |
| D.5 | gRPC auth: real JWT validation path in local-smoke optional profile | doc which profile is NoOp |

**Acceptance:** clean logs on `all-smoke`; CONTRACT matrix rows for route/ICE green or explicitly N/A.

---

### Phase E — NAT ladder (ICE/STUN/TURN) · **L–XL**

**Зачем:** «не только когда relay UDP открыт».

| Step | Task |
|------|------|
| E.1 | ICE gather smoke (candidates non-empty) |
| E.2 | Candidate exchange via relay REST/gRPC |
| E.3 | Connectivity check host/srflx |
| E.4 | TURN allocate when direct fails |
| E.5 | `ICEPath` adapter in selector order (before or after relay per policy) |

**Acceptance:** multi-host test (two NATs or docker netns) **or** documented lab topology; smoke script.

**Out of scope:** full mesh L3 WG IPAM productization.

---

### Phase F — Handover + resilience polish · **L**

| Task | Notes |
|------|--------|
| F.1 Wire `handover.Manager` (stop nil) | continuation token |
| F.2 Dual-path drain then cutover | Session stays Open |
| F.3 Fix AutoSwitch health | probe **active path**, not raw MASQUE UDP only |
| F.4 Integrate WG as Path adapter | only if `wireguard.enabled` |
| F.5 Idle TTL / max session lifetime | config |

**Acceptance:** chaos: kill active path mid-stream → handover <5s, app face survives (or documented reconnect with one RST).

---

### Phase G — Enhanced stack (optional product) · **XL**

Только после C+F green:

- MASQUE path adapter  
- SLO controller + synthetic probes feeding scores  
- Multi-relay selection  
- Cross-tenant negative smoke  

**Gate:** Phase C chaos PASS + Phase D clean control plane.

---

## 3. Roadmap (порядок и зависимости)

```text
A Foundation ──► B Adapters ──► C Selector+CLI+Chaos ──► F Handover
                     │                    │
                     └────────► D Control hygiene (parallel after B)
                                          │
                                          ▼
                                    E ICE/TURN (parallel after C)
                                          │
                                          ▼
                                    G MASQUE/SLO/multi-relay
```

| Phase | Depends | Parallel OK |
|-------|---------|-------------|
| A | — | — |
| B | A | D can start late in B |
| C | A,B | — |
| D | — (soft dep B) | yes with C |
| E | C | yes with F start |
| F | C | after C |
| G | C,F,(E optional) | — |

**Recommended first milestone for stakeholders:** **end of Phase C**  
= «auto failover between relay QUIC and gRPC tunnel, chaos-proven».

---

## 4. Критерии «готово» по уровням зрелости

| Level | Meaning | Exit criteria |
|-------|---------|---------------|
| **L0** Local baseline | Today | `all-smoke.sh` PASS |
| **L1** Orchestrated MVP | End Phase C | Selector + 2 chaos smokes PASS |
| **L2** Control clean | End Phase D | No spurious 400/route; OIDC path documented |
| **L3** NAT-aware | End Phase E | ICE/TURN lab PASS |
| **L4** Resilient | End Phase F | Mid-stream failover PASS |
| **L5** Enhanced | Phase G | MASQUE/SLO only if product needs |

**Do not market above the highest level with green exit criteria.**

---

## 5. Предложения по улучшению (architecture & process)

### 5.1 Architecture

| # | Proposal | Why |
|---|----------|-----|
| P1 | **One Session, many Paths** | App never binds to protocol |
| P2 | **Probe ≠ full Open** | Cheap fail before allocating listeners |
| P3 | **Policy YAML for order** | Edge vs mesh products differ (tunnel-first vs quic-first) |
| P4 | **Scores not only binary** | RTT/loss pick better path when both up (Phase F+) |
| P5 | **Relay stays SoT for ports/auth** | Client never hardcodes 5553/8444 |
| P6 | **Fail closed on auth; fail open on optional monitoring** | route 400 must not kill mesh |
| P7 | **Separate control and data health** | Heartbeat OK ≠ data path OK |

### 5.2 Reliability

| # | Proposal | Why |
|---|----------|-----|
| P8 | Chaos smokes as first-class CI optional job | Prevents “happy-only” regressions |
| P9 | Explicit reconnect budget (N tries / T window) | Avoid infinite silent retry |
| P10 | Jittered backoff shared library | Already partial; unify CLI/p2p/session |
| P11 | Connection draining on failover | Reduce app RST |
| P12 | Relay: cap AcceptStream error logs | Disk full incidents |

### 5.3 Observability

| # | Proposal | Why |
|---|----------|-----|
| P13 | Metrics: `path_active`, `failover_total`, `probe_fail{path}` | Ops |
| P14 | Structured log fields: `session_id`, `path`, `tenant` | Debug multi-peer |
| P15 | Smoke emit single `*_PASS=1` line (already pattern) | CI grep-friendly |

### 5.4 Security / multi-tenant

| # | Proposal | Why |
|---|----------|-----|
| P16 | Cross-tenant TO: must fail smoke | Isolation regression |
| P17 | Prefer OIDC profile in non-dev compose | Align Zitadel SoT |
| P18 | No default weak TURN passwords (done in WP1 — keep) | |

### 5.5 Product / UX

| # | Proposal | Why |
|---|----------|-----|
| P19 | `cloudbridge-client session status` | Show active path |
| P20 | Tunnel logs always print relay endpoint | Already partial — make SoT |
| P21 | `--smoke-data` remains membership+quic; session chaos separate | Clear semantics |
| P22 | Feature flags: `path_select.enabled` default false until C green | Safe rollout |

### 5.6 Process

| # | Proposal | Why |
|---|----------|-----|
| P23 | Every phase ends with STATUS + CHECKLISTS update | Honesty rule |
| P24 | No new protocol package without Path adapter + smoke | Prevents more nil stubs |
| P25 | Freeze installer as SoT; client adapts | Already agreed |
| P26 | PR size: one phase ≈ one PR stack, not mega-merge | Reviewability |

---

## 6. Что сознательно не делать (non-goals)

1. Rewrite `pkg/relay.Client` god-object in one shot.  
2. Enable MASQUE/handover/SLO before Phase C.  
3. Claim “P2P always direct” without Phase E evidence.  
4. Auto-switch that only flips a label without moving bytes.  
5. Big-bang monorepo merge of unrelated 2GC components.

---

## 7. Work breakdown → PR stack (Phase A–C)

| PR | Title | Approx |
|----|-------|--------|
| PR-A | `pkg/pathselect`: types, state machine, config, tests | M |
| PR-B1 | `RelayQUICPath` adapter + smoke tag test | S–M |
| PR-B2 | `GRPCTunnelPath` adapter + smoke tag test | S–M |
| PR-C1 | Selector.Ensure + failover cooldown | M |
| PR-C2 | CLI/session smoke + metrics | M |
| PR-C3 | Chaos scripts block 5553 / 8444 | S |
| PR-D* | Route/ICE/heartbeat hygiene (can interleave) | M |

Relay PRs only if adapter needs server change (prefer client-only).

---

## 8. Success metrics (engineering)

| Metric | Baseline (now) | Target after C |
|--------|----------------|----------------|
| Happy local all-smoke | PASS | PASS |
| Failover QUIC→tunnel | N/A | PASS chaos |
| Failover tunnel→QUIC | N/A | PASS chaos |
| Spurious route WARN | present | gone or rate-limited |
| Time to first path | manual | < ladder_timeout |
| Docs honesty | STATUS updated | each phase |

---

## 9. Decision requests (for product/eng)

1. **Default ladder order:** mesh-first (`relay_quic` then `grpc_tunnel`) vs service-tunnel-first?  
2. **App face for mesh product:** only tunnel localhost, or also raw peer streams?  
3. **WG required for L1?** Recommendation: **no** — L1 without WG.  
4. **Phase E (ICE) before or after F (handover)?** Recommendation: **E parallel, F after C**.

---

## 10. Immediate next action

If this plan is approved without changes:

1. Start **Phase A** (`pkg/pathselect` skeleton + tests).  
2. Open STATUS section «Path select roadmap» with Phase A–C checkboxes.  
3. Do **not** wire MASQUE/handover until C chaos green.

---

## 11. Summary one-pager

| | |
|--|--|
| **Problem** | Paths work in isolation; no orchestrated failover A↔B |
| **Fix** | Session + Path adapters + Selector + chaos smokes |
| **First shippable** | Phase C (QUIC ↔ gRPC tunnel auto-failover) |
| **Later** | ICE/TURN, handover, MASQUE, multi-relay |
| **Improve** | policy config, session health, metrics, no new stubs without adapters |
| **Done means** | smoke PASS + STATUS, not “package exists” |
