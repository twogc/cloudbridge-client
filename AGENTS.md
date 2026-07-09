## OpenWiki and docs

This repository has agent-oriented documentation in `/openwiki` and operational docs in `/docs`.

**Start here:**

1. [OpenWiki quickstart](openwiki/quickstart.md)
2. [docs/README.md](docs/README.md) — documentation index
3. [docs/CONTRACT_CLIENT_RELAY.md](docs/CONTRACT_CLIENT_RELAY.md) — **canonical** ports/paths vs relay
4. [docs/plans/CONTRACT_ALIGNMENT_PLAN.md](docs/plans/CONTRACT_ALIGNMENT_PLAN.md) — active alignment plan
5. [docs/CHECKLISTS.md](docs/CHECKLISTS.md) — build / test / smoke
6. [docs/STATUS.md](docs/STATUS.md) — honest maturity snapshot

When changing ports, dial logic, or REST paths: update **CONTRACT** + plan checkboxes in the same change.

**Server SoT (do not contradict):**  
`cloudbridge-relay-installer/openwiki/port-scheme.md`

**Env prefix:** `CLOUDBRIDGE_` (Viper). Do not reintroduce `CBR_*` as primary docs.
