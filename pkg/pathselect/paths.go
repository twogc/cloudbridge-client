package pathselect

import "github.com/twogc/cloudbridge-client/pkg/types"

// NewDefaultPaths returns production RelayQUIC + GRPCTunnel adapters for a config.
func NewDefaultPaths(cfg *types.Config) []Path {
	return []Path{
		NewRelayQUICPath(cfg),
		NewGRPCTunnelPath(cfg),
	}
}

// Ensure compile-time Path interface satisfaction.
var (
	_ Path = (*RelayQUICPath)(nil)
	_ Path = (*GRPCTunnelPath)(nil)
)
