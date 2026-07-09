package pathselect

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

func TestLadderSelector_RealAdapterTypes_InjectedDeps(t *testing.T) {
	cfg := &types.Config{Relay: types.RelayConfig{Host: "edge.example"}}

	quicProber := &mockRelayQUICProber{}
	quicOpener := &mockRelayQUICOpener{}
	quicPath := NewRelayQUICPathWithDeps(cfg, quicProber, quicOpener)

	grpcProber := &mockGRPCTunnelProber{}
	grpcOpener := &mockGRPCTunnelOpener{endpoint: "127.0.0.1:18080"}
	grpcPath := NewGRPCTunnelPathWithDeps(cfg, grpcProber, grpcOpener)

	sel := NewLadderSelector(enabledCfg(PathRelayQUIC, PathGRPCTunnel), []Path{quicPath, grpcPath})
	sess := NewSession("s1", "ten", "a", "b")
	req := OpenRequest{
		SessionID:  "s1",
		TenantID:   "ten",
		LocalPeer:  "a",
		RemotePeer: "b",
		Meta:       map[string]string{"token": "jwt"},
	}

	p, h, err := sel.Ensure(context.Background(), sess, req)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != PathRelayQUIC || h.PathName() != PathRelayQUIC {
		t.Fatalf("want relay_quic, got path=%s handle=%s", p.Name(), h.PathName())
	}
	if grpcOpener.n != 0 {
		t.Fatalf("grpc should not open, n=%d", grpcOpener.n)
	}
}

func TestLadderSelector_RealAdapters_FailoverToGRPC(t *testing.T) {
	cfg := &types.Config{Relay: types.RelayConfig{Host: "edge.example"}}

	quicPath := NewRelayQUICPathWithDeps(cfg,
		&mockRelayQUICProber{err: errors.New("udp blocked")},
		&mockRelayQUICOpener{},
	)
	grpcPath := NewGRPCTunnelPathWithDeps(cfg,
		&mockGRPCTunnelProber{},
		&mockGRPCTunnelOpener{endpoint: "127.0.0.1:18081"},
	)

	sel := NewLadderSelector(enabledCfg(PathRelayQUIC, PathGRPCTunnel), []Path{quicPath, grpcPath})
	sess := NewSession("s1", "ten", "a", "b")

	_, h, err := sel.Ensure(context.Background(), sess, OpenRequest{
		SessionID: "s1",
		Meta:      map[string]string{"token": "jwt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.PathName() != PathGRPCTunnel {
		t.Fatalf("want grpc_tunnel, got %s", h.PathName())
	}
	if sess.ActivePath != PathGRPCTunnel {
		t.Fatal(sess.ActivePath)
	}
}

func TestLadderSelector_RealAdapters_HealthFailover(t *testing.T) {
	cfg := &types.Config{Relay: types.RelayConfig{Host: "edge.example"}}

	quicProber := &mockRelayQUICProber{}
	quicOpener := &mockRelayQUICOpener{}
	quicPath := NewRelayQUICPathWithDeps(cfg, quicProber, quicOpener)

	grpcPath := NewGRPCTunnelPathWithDeps(cfg,
		&mockGRPCTunnelProber{},
		&mockGRPCTunnelOpener{endpoint: "127.0.0.1:18082"},
	)

	sel := NewLadderSelector(enabledCfg(PathRelayQUIC, PathGRPCTunnel), []Path{quicPath, grpcPath})
	sess := NewSession("s1", "ten", "a", "b")

	_, _, err := sel.Ensure(context.Background(), sess, OpenRequest{
		SessionID: "s1",
		Meta:      map[string]string{"token": "jwt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ActivePath != PathRelayQUIC {
		t.Fatal(sess.ActivePath)
	}

	quicProber.err = errors.New("probe fail after ensure")
	if err := sel.HealthTick(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	if sess.ActivePath != PathGRPCTunnel {
		t.Fatalf("want failover to grpc_tunnel, got %s", sess.ActivePath)
	}
}

func TestNewDefaultPaths_ReturnsBothAdapters(t *testing.T) {
	cfg := &types.Config{Relay: types.RelayConfig{Host: "edge.example"}}
	paths := NewDefaultPaths(cfg)
	if len(paths) != 2 {
		t.Fatalf("len=%d", len(paths))
	}
	if paths[0].Name() != PathRelayQUIC || paths[1].Name() != PathGRPCTunnel {
		t.Fatalf("names: %s %s", paths[0].Name(), paths[1].Name())
	}
}

func TestDialHelpers_EmptyConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := ProbeQUICReachability(ctx, &types.Config{}); err == nil {
		t.Fatal("expected empty host error")
	}
	if err := ProbeGRPCReachability(ctx, &types.Config{}); err == nil {
		t.Fatal("expected empty target error")
	}
}
