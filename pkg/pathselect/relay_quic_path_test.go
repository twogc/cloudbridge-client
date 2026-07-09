package pathselect

import (
	"context"
	"errors"
	"testing"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

type mockRelayQUICProber struct {
	err error
	n   int
}

func (m *mockRelayQUICProber) Probe(ctx context.Context, cfg *types.Config) error {
	m.n++
	if err := ctx.Err(); err != nil {
		return err
	}
	return m.err
}

type mockRelayQUICOpener struct {
	err   error
	n     int
	sess  relayQUICSession
}

func (m *mockRelayQUICOpener) Open(ctx context.Context, cfg *types.Config, req OpenRequest, token string) (relayQUICSession, error) {
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
	return &quicConnSession{}, nil
}

func TestRelayQUICPath_Name(t *testing.T) {
	p := NewRelayQUICPath(&types.Config{Relay: types.RelayConfig{Host: "edge.example"}})
	if p.Name() != PathRelayQUIC {
		t.Fatal(p.Name())
	}
}

func TestRelayQUICPath_ProbeFail(t *testing.T) {
	prober := &mockRelayQUICProber{err: errors.New("udp blocked")}
	p := NewRelayQUICPathWithDeps(&types.Config{Relay: types.RelayConfig{Host: "edge.example"}}, prober, nil)
	if err := p.Probe(context.Background()); err == nil {
		t.Fatal("expected probe error")
	}
}

func TestRelayQUICPath_ProbeSuccess(t *testing.T) {
	prober := &mockRelayQUICProber{}
	p := NewRelayQUICPathWithDeps(&types.Config{Relay: types.RelayConfig{Host: "edge.example"}}, prober, nil)
	if err := p.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if prober.n != 1 {
		t.Fatal(prober.n)
	}
}

func TestRelayQUICPath_OpenRequiresToken(t *testing.T) {
	p := NewRelayQUICPathWithDeps(&types.Config{}, &mockRelayQUICProber{}, &mockRelayQUICOpener{})
	_, err := p.Open(context.Background(), OpenRequest{})
	if err == nil {
		t.Fatal("expected token error")
	}
}

func TestRelayQUICPath_OpenSuccess(t *testing.T) {
	opener := &mockRelayQUICOpener{}
	p := NewRelayQUICPathWithDeps(&types.Config{}, &mockRelayQUICProber{}, opener)
	h, err := p.Open(context.Background(), OpenRequest{Meta: map[string]string{"token": "jwt-test"}})
	if err != nil {
		t.Fatal(err)
	}
	if h.PathName() != PathRelayQUIC {
		t.Fatal(h.PathName())
	}
	if opener.n != 1 {
		t.Fatal(opener.n)
	}
	if err := h.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRelayQUICPath_OpenFail(t *testing.T) {
	opener := &mockRelayQUICOpener{err: errors.New("auth rejected")}
	p := NewRelayQUICPathWithDeps(&types.Config{}, &mockRelayQUICProber{}, opener)
	_, err := p.Open(context.Background(), OpenRequest{Meta: map[string]string{"token": "bad"}})
	if err == nil {
		t.Fatal("expected open error")
	}
}

func TestRelayQUICAddr_FromConfig(t *testing.T) {
	cfg := &types.Config{Relay: types.RelayConfig{Host: "edge.2gc.ru"}}
	if got := RelayQUICAddr(cfg); got != "edge.2gc.ru:5553" {
		t.Fatalf("RelayQUICAddr: %q", got)
	}
}

func TestTokenFromRequest(t *testing.T) {
	if tokenFromRequest(OpenRequest{}) != "" {
		t.Fatal("empty meta")
	}
	req := OpenRequest{Meta: map[string]string{"token": "abc"}}
	if tokenFromRequest(req) != "abc" {
		t.Fatal(tokenFromRequest(req))
	}
}
