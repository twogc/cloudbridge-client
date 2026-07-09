package pathselect

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/twogc/cloudbridge-client/pkg/types"
)

// relayQUICSession is an authenticated QUIC path reusable for TO:peer (Phase C+).
type relayQUICSession interface {
	Close() error
}

type quicConnSession struct {
	conn *quic.Conn
}

func (s *quicConnSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.CloseWithError(0, "pathselect relay_quic close")
}

// RelayQUICProber performs cheap reachability checks.
type RelayQUICProber interface {
	Probe(ctx context.Context, cfg *types.Config) error
}

// RelayQUICOpener establishes AUTH+PING and returns a reusable session.
type RelayQUICOpener interface {
	Open(ctx context.Context, cfg *types.Config, req OpenRequest, token string) (relayQUICSession, error)
}

type defaultRelayQUICProber struct{}

func (defaultRelayQUICProber) Probe(ctx context.Context, cfg *types.Config) error {
	return ProbeQUICReachability(ctx, cfg)
}

type defaultRelayQUICOpener struct{}

func (defaultRelayQUICOpener) Open(ctx context.Context, cfg *types.Config, req OpenRequest, token string) (relayQUICSession, error) {
	return openRelayQUICSession(ctx, cfg, token)
}

// RelayQUICPath adapts relay P2P QUIC (AUTH + PING) for the path ladder.
type RelayQUICPath struct {
	cfg    *types.Config
	prober RelayQUICProber
	opener RelayQUICOpener
	mu     sync.Mutex
	active relayQUICSession
}

// NewRelayQUICPath returns a production RelayQUICPath from config.
func NewRelayQUICPath(cfg *types.Config) *RelayQUICPath {
	return NewRelayQUICPathWithDeps(cfg, defaultRelayQUICProber{}, defaultRelayQUICOpener{})
}

// NewRelayQUICPathWithDeps allows injecting probe/open deps (tests, smoke).
func NewRelayQUICPathWithDeps(cfg *types.Config, prober RelayQUICProber, opener RelayQUICOpener) *RelayQUICPath {
	if prober == nil {
		prober = defaultRelayQUICProber{}
	}
	if opener == nil {
		opener = defaultRelayQUICOpener{}
	}
	return &RelayQUICPath{cfg: cfg, prober: prober, opener: opener}
}

func (p *RelayQUICPath) Name() string { return PathRelayQUIC }

func (p *RelayQUICPath) Probe(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("relay_quic: nil path")
	}
	return p.prober.Probe(ctx, p.cfg)
}

func (p *RelayQUICPath) Open(ctx context.Context, req OpenRequest) (Handle, error) {
	if p == nil {
		return nil, fmt.Errorf("relay_quic: nil path")
	}
	token := tokenFromRequest(req)
	if token == "" {
		return nil, fmt.Errorf("relay_quic: token required in OpenRequest.Meta[\"token\"]")
	}
	sess, err := p.opener.Open(ctx, p.cfg, req, token)
	if err != nil {
		return nil, err
	}
	h := &relayQUICHandle{name: PathRelayQUIC, session: sess}
	p.mu.Lock()
	if p.active != nil {
		_ = p.active.Close()
	}
	p.active = sess
	p.mu.Unlock()
	return h, nil
}

func (p *RelayQUICPath) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active == nil {
		return nil
	}
	err := p.active.Close()
	p.active = nil
	return err
}

type relayQUICHandle struct {
	name    string
	session relayQUICSession
}

func (h *relayQUICHandle) PathName() string {
	if h == nil {
		return PathRelayQUIC
	}
	return h.name
}

func (h *relayQUICHandle) Close(ctx context.Context) error {
	if h == nil || h.session == nil {
		return nil
	}
	return h.session.Close()
}

// openRelayQUICSession dials relay QUIC, AUTH, then PING/PONG (data plane sanity).
func openRelayQUICSession(ctx context.Context, cfg *types.Config, token string) (relayQUICSession, error) {
	addr := RelayQUICAddr(cfg)
	if addr == "" {
		return nil, fmt.Errorf("relay QUIC address not configured")
	}
	conn, err := quic.DialAddr(ctx, addr, QUICClientTLS(cfg), DefaultQUICConfig())
	if err != nil {
		return nil, fmt.Errorf("quic dial %s: %w", addr, err)
	}

	authStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		_ = conn.CloseWithError(0, "auth stream failed")
		return nil, fmt.Errorf("open auth stream: %w", err)
	}
	if _, err := authStream.Write([]byte("AUTH " + token)); err != nil {
		_ = authStream.Close()
		_ = conn.CloseWithError(0, "auth write failed")
		return nil, fmt.Errorf("write AUTH: %w", err)
	}
	_ = authStream.SetReadDeadline(time.Now().Add(8 * time.Second))
	buf := make([]byte, 256)
	n, err := authStream.Read(buf)
	if err != nil {
		_ = authStream.Close()
		_ = conn.CloseWithError(0, "auth read failed")
		return nil, fmt.Errorf("read AUTH response: %w", err)
	}
	resp := string(buf[:n])
	_ = authStream.Close()
	if resp != "AUTH_OK" {
		_ = conn.CloseWithError(0, "auth rejected")
		return nil, fmt.Errorf("expected AUTH_OK got %q", resp)
	}

	dataStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		_ = conn.CloseWithError(0, "data stream failed")
		return nil, fmt.Errorf("open data stream: %w", err)
	}
	payload := "pathselect-relay-quic"
	pingMsg := "PING " + payload
	if _, err := dataStream.Write([]byte(pingMsg)); err != nil {
		_ = dataStream.Close()
		_ = conn.CloseWithError(0, "ping write failed")
		return nil, fmt.Errorf("write PING: %w", err)
	}
	_ = dataStream.SetReadDeadline(time.Now().Add(8 * time.Second))
	n, err = dataStream.Read(buf)
	if err != nil {
		_ = dataStream.Close()
		_ = conn.CloseWithError(0, "ping read failed")
		return nil, fmt.Errorf("read PONG: %w", err)
	}
	pong := string(buf[:n])
	_ = dataStream.Close()
	want := "PONG " + payload
	if pong != want {
		_ = conn.CloseWithError(0, "ping mismatch")
		return nil, fmt.Errorf("expected %q got %q", want, pong)
	}

	return &quicConnSession{conn: conn}, nil
}
