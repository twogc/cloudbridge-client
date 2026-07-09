package pathselect

import (
	"context"
	"errors"
	"testing"
	"time"
)

func enabledCfg(order ...string) Config {
	c := DefaultConfig()
	c.Enabled = true
	if len(order) > 0 {
		c.Order = order
	}
	c.ProbeTimeout = 200 * time.Millisecond
	c.LadderTimeout = 2 * time.Second
	c.FailoverCooldown = 0 // allow immediate failover in tests
	return c
}

func TestEnsure_FirstPathWins(t *testing.T) {
	quic := NewStubPath(PathRelayQUIC)
	tun := NewStubPath(PathGRPCTunnel)
	sel := NewLadderSelector(enabledCfg(PathRelayQUIC, PathGRPCTunnel), []Path{quic, tun})
	sess := NewSession("s1", "ten", "a", "b")
	req := OpenRequest{SessionID: "s1", TenantID: "ten", LocalPeer: "a", RemotePeer: "b"}

	p, h, err := sel.Ensure(context.Background(), sess, req)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != PathRelayQUIC || h.PathName() != PathRelayQUIC {
		t.Fatalf("want relay_quic, got path=%s handle=%s", p.Name(), h.PathName())
	}
	if sess.State != StatePathSelected || sess.ActivePath != PathRelayQUIC {
		t.Fatalf("session: state=%s path=%s", sess.State, sess.ActivePath)
	}
	if tun.OpenCount() != 0 {
		t.Fatalf("second path should not open, openN=%d", tun.OpenCount())
	}
}

func TestEnsure_FallsThroughOnProbeFail(t *testing.T) {
	quic := NewStubPath(PathRelayQUIC)
	quic.SetProbeErr(errors.New("udp blocked"))
	tun := NewStubPath(PathGRPCTunnel)
	sel := NewLadderSelector(enabledCfg(PathRelayQUIC, PathGRPCTunnel), []Path{quic, tun})
	sess := NewSession("s1", "ten", "a", "b")

	_, h, err := sel.Ensure(context.Background(), sess, OpenRequest{SessionID: "s1"})
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

func TestEnsure_AllFail(t *testing.T) {
	quic := NewStubPath(PathRelayQUIC)
	quic.SetProbeErr(errors.New("down"))
	tun := NewStubPath(PathGRPCTunnel)
	tun.SetOpenErr(errors.New("open fail"))
	sel := NewLadderSelector(enabledCfg(), []Path{quic, tun})
	sess := NewSession("s1", "ten", "a", "b")

	_, _, err := sel.Ensure(context.Background(), sess, OpenRequest{})
	if !errors.Is(err, ErrAllPathsFailed) {
		t.Fatalf("want ErrAllPathsFailed, got %v", err)
	}
	if sess.State != StateFailed {
		t.Fatalf("want failed, got %s", sess.State)
	}
}

func TestEnsure_Disabled(t *testing.T) {
	c := DefaultConfig() // Enabled=false
	sel := NewLadderSelector(c, []Path{NewStubPath(PathRelayQUIC)})
	_, _, err := sel.Ensure(context.Background(), NewSession("s", "t", "a", "b"), OpenRequest{})
	if !errors.Is(err, ErrSelectorDisabled) {
		t.Fatal(err)
	}
}

func TestEnsure_OrderRespected(t *testing.T) {
	// Prefer tunnel first
	quic := NewStubPath(PathRelayQUIC)
	tun := NewStubPath(PathGRPCTunnel)
	sel := NewLadderSelector(enabledCfg(PathGRPCTunnel, PathRelayQUIC), []Path{quic, tun})
	sess := NewSession("s1", "ten", "a", "b")

	_, h, err := sel.Ensure(context.Background(), sess, OpenRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if h.PathName() != PathGRPCTunnel {
		t.Fatal(h.PathName())
	}
}

func TestHealthTick_Failover(t *testing.T) {
	quic := NewStubPath(PathRelayQUIC)
	// First Ensure probe+open succeed (2 probes if health uses probe again — Ensure: 1 probe)
	// After Ensure: probeN=1. Fail after 1 success → next probe fails.
	quic.FailAfterNProbes(1)
	tun := NewStubPath(PathGRPCTunnel)
	sel := NewLadderSelector(enabledCfg(PathRelayQUIC, PathGRPCTunnel), []Path{quic, tun})
	sess := NewSession("s1", "ten", "a", "b")

	_, _, err := sel.Ensure(context.Background(), sess, OpenRequest{SessionID: "s1", TenantID: "ten", LocalPeer: "a", RemotePeer: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ActivePath != PathRelayQUIC {
		t.Fatal(sess.ActivePath)
	}

	// Health: quic probe fails (probeN becomes 2 > 1), should move to grpc_tunnel
	if err := sel.HealthTick(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	if sess.ActivePath != PathGRPCTunnel {
		t.Fatalf("want failover to grpc_tunnel, got %s (state=%s err=%v)", sess.ActivePath, sess.State, sess.LastError)
	}
	if sess.State != StatePathSelected {
		t.Fatal(sess.State)
	}
}

func TestConfig_Normalize(t *testing.T) {
	c := Config{Enabled: true}.Normalize()
	if len(c.Order) != 2 || c.ProbeTimeout <= 0 || c.LadderTimeout <= 0 {
		t.Fatalf("%+v", c)
	}
}

func TestEnsure_ProbeTimeout(t *testing.T) {
	// path that blocks until ctx cancel
	slow := &blockingPath{name: "slow"}
	cfg := enabledCfg("slow")
	cfg.ProbeTimeout = 50 * time.Millisecond
	cfg.LadderTimeout = 200 * time.Millisecond
	sel := NewLadderSelector(cfg, []Path{slow})
	sess := NewSession("s1", "t", "a", "b")
	start := time.Now()
	_, _, err := sel.Ensure(context.Background(), sess, OpenRequest{})
	if !errors.Is(err, ErrAllPathsFailed) {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout did not fire promptly")
	}
}

type blockingPath struct{ name string }

func (p *blockingPath) Name() string { return p.name }
func (p *blockingPath) Probe(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
func (p *blockingPath) Open(ctx context.Context, req OpenRequest) (Handle, error) {
	return nil, errors.New("nope")
}
func (p *blockingPath) Close(ctx context.Context) error { return nil }
