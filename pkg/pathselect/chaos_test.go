package pathselect

import (
	"context"
	"errors"
	"testing"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// C-chaos-1: block relay_quic (probe fail) → selector uses grpc_tunnel.
func TestChaos_C1_BlockQUIC_UsesGRPCTunnel(t *testing.T) {
	ResetMetrics()
	cfg := &types.Config{Relay: types.RelayConfig{Host: "edge.example"}}

	quic := NewRelayQUICPathWithDeps(cfg,
		&mockRelayQUICProber{err: errors.New("chaos: block udp 5553")},
		&mockRelayQUICOpener{},
	)
	grpc := NewGRPCTunnelPathWithDeps(cfg,
		&mockGRPCTunnelProber{},
		&mockGRPCTunnelOpener{endpoint: "127.0.0.1:18444"},
	)
	sel := NewLadderSelector(enabledCfg(PathRelayQUIC, PathGRPCTunnel), []Path{quic, grpc})
	sess := NewSession("chaos1", "ten", "a", "b")

	_, h, err := sel.Ensure(context.Background(), sess, OpenRequest{
		SessionID: "chaos1",
		Meta:      map[string]string{"token": "t"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.PathName() != PathGRPCTunnel {
		t.Fatalf("C-chaos-1: want grpc_tunnel got %s", h.PathName())
	}
	m := SnapshotMetrics()
	if m.PathSelected < 1 {
		t.Fatalf("expected path_selected>=1 got %d", m.PathSelected)
	}
}

// C-chaos-2: block grpc_tunnel → selector uses relay_quic.
func TestChaos_C2_BlockGRPC_UsesRelayQUIC(t *testing.T) {
	ResetMetrics()
	cfg := &types.Config{Relay: types.RelayConfig{Host: "edge.example"}}

	quic := NewRelayQUICPathWithDeps(cfg,
		&mockRelayQUICProber{},
		&mockRelayQUICOpener{},
	)
	grpc := NewGRPCTunnelPathWithDeps(cfg,
		&mockGRPCTunnelProber{err: errors.New("chaos: block tcp 8444")},
		&mockGRPCTunnelOpener{},
	)
	// Prefer grpc first so block forces fallthrough to quic
	sel := NewLadderSelector(enabledCfg(PathGRPCTunnel, PathRelayQUIC), []Path{quic, grpc})
	sess := NewSession("chaos2", "ten", "a", "b")

	_, h, err := sel.Ensure(context.Background(), sess, OpenRequest{
		SessionID: "chaos2",
		Meta:      map[string]string{"token": "t"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.PathName() != PathRelayQUIC {
		t.Fatalf("C-chaos-2: want relay_quic got %s", h.PathName())
	}
}

func TestMetrics_OnFailAndFailover(t *testing.T) {
	ResetMetrics()
	cfg := &types.Config{Relay: types.RelayConfig{Host: "h"}}
	quic := NewRelayQUICPathWithDeps(cfg,
		&mockRelayQUICProber{err: errors.New("down")},
		&mockRelayQUICOpener{},
	)
	grpc := NewGRPCTunnelPathWithDeps(cfg,
		&mockGRPCTunnelProber{err: errors.New("down")},
		&mockGRPCTunnelOpener{},
	)
	sel := NewLadderSelector(enabledCfg(), []Path{quic, grpc})
	_, _, err := sel.Ensure(context.Background(), NewSession("x", "t", "a", "b"), OpenRequest{Meta: map[string]string{"token": "t"}})
	if err == nil {
		t.Fatal("want fail")
	}
	if SnapshotMetrics().PathFail < 1 {
		t.Fatal("path_fail not incremented")
	}
}
