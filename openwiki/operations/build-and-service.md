# Build, packaging, and service operations

## Build entrypoints

The main build target is the CLI binary under `./cmd/cloudbridge-client`. The alternate utility is `./cmd/quic-tester`.

### Makefile
`Makefile` defines the canonical local build and packaging targets used in the repo:

- `make build`
- `make build-linux`
- `make build-windows`
- `make build-darwin`
- `make build-all`
- package targets for Linux, Windows, macOS, and all platforms

It also uses `tag.txt` and ldflags to stamp version/build metadata.

### Other build scripts
Additional release/build tooling exists in:

- `build-all-platforms.sh`
- `.goreleaser.yml`
- `install.sh`
- `cloudbridge-client.service`
- `MSI_INSTALLATION.md`
- `pkg-build/README.md`

## Service management

`pkg/service/service.go` provides OS-specific service lifecycle helpers:

- Linux: `systemctl`
- Windows: `nssm`
- macOS: `launchctl`

The install path differs by OS, and the code writes or configures service definitions with fixed names and config locations.

## Practical notes

- Service install/start/stop behavior is code-backed and should be checked in `pkg/service/service.go` before changing packaging docs.
- `MSI_INSTALLATION.md` should be treated as a packaging-specific document that may need validation against the current Windows path.
- If you change the binary name, config file location, or CLI flags, you may need to update the service helpers and packaging scripts together.

## Source references

- `Makefile`
- `build-all-platforms.sh`
- `install.sh`
- `cloudbridge-client.service`
- `.goreleaser.yml`
- `pkg/service/service.go`
- `MSI_INSTALLATION.md`
