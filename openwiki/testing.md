# Testing guidance

Full operational checklists: **[docs/CHECKLISTS.md](../docs/CHECKLISTS.md)**.

## Observed test areas

- `integration_test.go`
- `pkg/auth/auth_test.go`
- `pkg/config/config_test.go`
- `pkg/errors/errors_test.go`
- `pkg/heartbeat/payload_test.go`
- `pkg/metrics/metrics_test.go`
- `pkg/onboarding/onboarding_test.go`
- `pkg/p2p/p2p_test.go`, `pkg/p2p/types_test.go`
- `pkg/quic/connection_test.go`
- `pkg/relay/autoswitch_test.go`, `transport_layer_test.go`, `transport/grpc_test.go`, `wireguard_test.go`
- `pkg/types/types_test.go`, `pkg/types/addrs_test.go` (after WP1)
- `pkg/utils/command_test.go`

## Minimum after config/transport changes

```bash
go test ./pkg/config/ ./pkg/types/ -count=1
go test ./pkg/relay/transport/ -count=1
go test ./pkg/auth/ -count=1 -short
```

## Broader

```bash
go test ./... -short -count=1
```

Record failures in [docs/STATUS.md](../docs/STATUS.md) test log — do not claim green without running.

## Live smoke

See CHECKLISTS §5 (REST :5552, gRPC :8444, `p2p` CLI). Requires token and reachable relay.

## Quality tools

- `.golangci.yml` — run `golangci-lint` when available
- Contract guards (after WP1–2): see CHECKLISTS §9

## Source references

- `integration_test.go`
- `pkg/*/*_test.go`
- `.golangci.yml`
- `docs/CHECKLISTS.md`
