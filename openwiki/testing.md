# Testing guidance

This repository has a meaningful unit and integration test surface. Before changing core behavior, check the relevant package tests and the top-level integration test.

## Observed test areas

- `integration_test.go`
- `pkg/auth/auth_test.go`
- `pkg/config/config_test.go`
- `pkg/errors/errors_test.go`
- `pkg/heartbeat/payload_test.go`
- `pkg/metrics/metrics_test.go`
- `pkg/onboarding/onboarding_test.go`
- `pkg/p2p/p2p_test.go`
- `pkg/p2p/types_test.go`
- `pkg/quic/connection_test.go`
- `pkg/relay/autoswitch_test.go`
- `pkg/relay/transport_layer_test.go`
- `pkg/relay/transport/grpc_test.go`
- `pkg/relay/wireguard_test.go`
- `pkg/types/types_test.go`
- `pkg/utils/command_test.go`

## What to run after changing code

- CLI/startup changes: exercise `cmd/cloudbridge-client` paths and related integration tests
- config/auth changes: run the tests under `pkg/config` and `pkg/auth`
- networking changes: run `pkg/p2p`, `pkg/quic`, and relay transport tests
- heartbeat/metrics changes: run the corresponding package tests and any integration coverage that depends on them

## Quality tools

The repository includes `.golangci.yml`, so lint expectations matter. If you change core flow, also check formatting and lint-related failures before considering the change complete.

## Source references

- `integration_test.go`
- `pkg/*/*_test.go`
- `.golangci.yml`
