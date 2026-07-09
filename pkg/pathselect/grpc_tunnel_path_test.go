package pathselect

import (
	"context"
	"errors"
	"testing"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

type mockGRPCTunnelProber struct {
	err error
	n   int
}

func (m *mockGRPCTunnelProber) Probe(ctx context.Context, cfg *types.Config) error {
	m.n++
	if err := ctx.Err(); err != nil {
		return err
	}
	return m.err
}

type mockGRPCTunnelOpener struct {
	err      error
	n        int
	sess     grpcTunnelSession
	endpoint string
}

func (m *mockGRPCTunnelOpener) Open(ctx context.Context, cfg *types.Config, req OpenRequest, token string) (grpcTunnelSession, error) {
	m.n++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.err != nil {
		return nil, m.err
	}
	if m.sess != nil {
		return m.sess, nil
	}
	ep := m.endpoint
	if ep == "" {
		ep = "127.0.0.1:19000"
	}
	return &transportTunnelSession{tunnelID: "t1", endpoint: ep}, nil
}

func TestGRPCTunnelPath_Name(t *testing.T) {
	p := NewGRPCTunnelPath(&types.Config{Relay: types.RelayConfig{Host: "edge.example"}})
	if p.Name() != PathGRPCTunnel {
		t.Fatal(p.Name())
	}
}

func TestGRPCTunnelPath_ProbeFail(t *testing.T) {
	prober := &mockGRPCTunnelProber{err: errors.New("tcp blocked")}
	p := NewGRPCTunnelPathWithDeps(&types.Config{Relay: types.RelayConfig{Host: "edge.example"}}, prober, nil)
	if err := p.Probe(context.Background()); err == nil {
		t.Fatal("expected probe error")
	}
}

func TestGRPCTunnelPath_ProbeSuccess(t *testing.T) {
	prober := &mockGRPCTunnelProber{}
	p := NewGRPCTunnelPathWithDeps(&types.Config{Relay: types.RelayConfig{Host: "edge.example"}}, prober, nil)
	if err := p.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if prober.n != 1 {
		t.Fatal(prober.n)
	}
}

func TestGRPCTunnelPath_OpenSuccess(t *testing.T) {
	opener := &mockGRPCTunnelOpener{endpoint: "relay:19999"}
	p := NewGRPCTunnelPathWithDeps(&types.Config{}, &mockGRPCTunnelProber{}, opener)
	h, err := p.Open(context.Background(), OpenRequest{
		SessionID:  "sess-1",
		TenantID:   "ten",
		RemoteHost: "10.0.0.5",
		RemotePort: 443,
		Meta:       map[string]string{"token": "jwt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.PathName() != PathGRPCTunnel {
		t.Fatal(h.PathName())
	}
	gh, ok := h.(*grpcTunnelHandle)
	if !ok {
		t.Fatalf("handle type %T", h)
	}
	if gh.Endpoint() != "relay:19999" {
		t.Fatal(gh.Endpoint())
	}
	if err := h.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCTunnelPath_OpenFail(t *testing.T) {
	opener := &mockGRPCTunnelOpener{err: errors.New("CreateTunnel failed")}
	p := NewGRPCTunnelPathWithDeps(&types.Config{}, &mockGRPCTunnelProber{}, opener)
	_, err := p.Open(context.Background(), OpenRequest{RemoteHost: "127.0.0.1", RemotePort: 80})
	if err == nil {
		t.Fatal("expected open error")
	}
}

func TestGRPCTarget_FromConfig(t *testing.T) {
	cfg := &types.Config{Relay: types.RelayConfig{Host: "edge.2gc.ru"}}
	if got := GRPCTarget(cfg); got != "edge.2gc.ru:8444" {
		t.Fatalf("GRPCTarget: %q", got)
	}
}

func TestGRPCTarget_EmptyHost(t *testing.T) {
	if got := GRPCTarget(&types.Config{}); got != "" {
		t.Fatal(got)
	}
}
