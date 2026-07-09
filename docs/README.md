# CloudBridge Client — Documentation

Human- and agent-oriented documentation for the client. Prefer **source + this tree + openwiki** over aspirational sections in root `Architecture.md`.

## Start here

| Document | Purpose |
|----------|---------|
| [openwiki/quickstart.md](../openwiki/quickstart.md) | Repo map, CLI surface, agent entry |
| [CONTRACT_CLIENT_RELAY.md](CONTRACT_CLIENT_RELAY.md) | **Canonical** client↔relay ports, protocols, paths |
| [plans/README.md](plans/README.md) | **Plans index** (alignment + gap closure + path SM) |
| [plans/GAP_CLOSURE_AND_IMPROVEMENTS.md](plans/GAP_CLOSURE_AND_IMPROVEMENTS.md) | **Roadmap:** Session/PathSelector/failover phases A–G |
| [plans/PATH_SELECTION_STATE_MACHINE.md](plans/PATH_SELECTION_STATE_MACHINE.md) | Target path-selection architecture (diagrams) |
| [plans/CONTRACT_ALIGNMENT_PLAN.md](plans/CONTRACT_ALIGNMENT_PLAN.md) | Port/dial alignment (WP0–WP5), status, decisions |
| [CHECKLISTS.md](CHECKLISTS.md) | Rebuild, test, smoke, release checklists |
| [STATUS.md](STATUS.md) | Current maturity snapshot (honest) |

## OpenWiki (domain detail)

| Path | Topic |
|------|-------|
| [architecture/overview.md](../openwiki/architecture/overview.md) | Runtime layers |
| [domain/networking.md](../openwiki/domain/networking.md) | P2P, QUIC, tunnel, WG |
| [domain/config-and-auth.md](../openwiki/domain/config-and-auth.md) | Config, JWT/OIDC |
| [workflows/cli.md](../openwiki/workflows/cli.md) | CLI modes |
| [operations/build-and-service.md](../openwiki/operations/build-and-service.md) | Build / package / service |
| [testing.md](../openwiki/testing.md) | What to run when changing code |

## Related (server)

Relay installer SoT (do not contradict when aligning ports):

- `cloudbridge-relay-installer/openwiki/port-scheme.md`
- `cloudbridge-relay-installer/docs/operations/SERVICE_CATALOG.md`

## Rules

1. **Port / endpoint truth** → [CONTRACT_CLIENT_RELAY.md](CONTRACT_CLIENT_RELAY.md).
2. If code and docs disagree, **fix docs or code in the same change**; never leave silent drift.
3. Root `Architecture.md` is high-level; features marked **UNWIRED** in STATUS must not be sold as GA.
4. Env prefix for config: **`CLOUDBRIDGE_`** (Viper). Older README `CBR_*` examples are deprecated.
