//go:build smoke

package pathselect

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// Local-smoke stub: run with -tags smoke when relay is up (127.0.0.1 defaults).
// Requires CLOUDBRIDGE_SMOKE_TOKEN for Open paths.
func TestSmoke_AdaptersProbeLocalRelay(t *testing.T) {
	host := os.Getenv("CLOUDBRIDGE_RELAY_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	cfg := &types.Config{
		Relay: types.RelayConfig{
			Host: host,
			Ports: types.RelayPorts{
				QUIC: types.DefaultP2PQUICPort,
				GRPC: types.DefaultGRPCPort,
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := ProbeQUICReachability(ctx, cfg); err != nil {
		t.Skipf("relay QUIC not reachable: %v", err)
	}
	if err := ProbeGRPCReachability(ctx, cfg); err != nil {
		t.Skipf("relay gRPC not reachable: %v", err)
	}
}

func TestSmoke_AdaptersOpenLocalRelay(t *testing.T) {
	token := os.Getenv("CLOUDBRIDGE_SMOKE_TOKEN")
	if token == "" {
		t.Skip("CLOUDBRIDGE_SMOKE_TOKEN not set")
	}
	host := os.Getenv("CLOUDBRIDGE_RELAY_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	cfg := &types.Config{
		Relay: types.RelayConfig{
			Host: host,
			Ports: types.RelayPorts{
				QUIC: types.DefaultP2PQUICPort,
				GRPC: types.DefaultGRPCPort,
			},
		},
	}
	req := OpenRequest{
		SessionID:  "smoke-s1",
		TenantID:   "smoke-tenant",
		RemoteHost: "127.0.0.1",
		RemotePort: 9,
		Meta:       map[string]string{"token": token},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	quicPath := NewRelayQUICPath(cfg)
	h, err := quicPath.Open(ctx, req)
	if err != nil {
		t.Fatalf("relay_quic open: %v", err)
	}
	_ = h.Close(ctx)

	grpcPath := NewGRPCTunnelPath(cfg)
	h2, err := grpcPath.Open(ctx, req)
	if err != nil {
		t.Fatalf("grpc_tunnel open: %v", err)
	}
	_ = h2.Close(ctx)
}
