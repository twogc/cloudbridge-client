# Plans index — cloudbridge-client

Active and historical engineering plans.  
**Honesty rule:** plan status ≠ production readiness; see [../STATUS.md](../STATUS.md).

| Plan | Role | Status |
|------|------|--------|
| [CONTRACT_ALIGNMENT_PLAN.md](./CONTRACT_ALIGNMENT_PLAN.md) | Client↔relay ports, dial, docs rebuild (WP0–WP5) | **Active** — WP0–WP3 largely done; WP4–WP5 open |
| [GAP_CLOSURE_AND_IMPROVEMENTS.md](./GAP_CLOSURE_AND_IMPROVEMENTS.md) | **Primary roadmap:** Session / PathSelector / failover / phases A–G | **Documented** — ready to execute from Phase A |
| [PATH_SELECTION_STATE_MACHINE.md](./PATH_SELECTION_STATE_MACHINE.md) | Target architecture: session SM + probe ladder diagrams | **Design reference** for GAP plan |

## Reading order

1. **Where we are:** [../STATUS.md](../STATUS.md) + [../CHECKLISTS.md](../CHECKLISTS.md)  
2. **Contract SoT:** [../CONTRACT_CLIENT_RELAY.md](../CONTRACT_CLIENT_RELAY.md)  
3. **What to build next:** [GAP_CLOSURE_AND_IMPROVEMENTS.md](./GAP_CLOSURE_AND_IMPROVEMENTS.md)  
4. **How path selection should work:** [PATH_SELECTION_STATE_MACHINE.md](./PATH_SELECTION_STATE_MACHINE.md)  
5. **Port alignment backlog:** [CONTRACT_ALIGNMENT_PLAN.md](./CONTRACT_ALIGNMENT_PLAN.md)

## Milestone summary (GAP plan)

| Level | Meaning | Exit |
|-------|---------|------|
| L0 | Local baseline | `scripts/all-smoke.sh` PASS (**today**) |
| L1 | Orchestrated MVP | Phase C: selector + chaos QUIC↔tunnel |
| L2 | Control clean | Phase D: REST/OIDC hygiene |
| L3 | NAT-aware | Phase E: ICE/TURN lab |
| L4 | Resilient | Phase F: mid-stream handover |
| L5 | Enhanced | Phase G: MASQUE/SLO/multi-relay (optional) |

**First shippable product milestone:** end of **Phase C** (auto-failover between `relay_quic` and `grpc_tunnel`).

## Related server docs

- `cloudbridge-relay-installer/openwiki/port-scheme.md`
- Installer frozen as port/protocol SoT for client alignment
