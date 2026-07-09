package pathselect

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/relay/transport"
	"github.com/twogc/cloudbridge-client/pkg/types"
)

// grpcTunnelSession holds an opened gRPC tunnel control/data-plane binding.
type grpcTunnelSession interface {
	Close() error
	TunnelID() string
	Endpoint() string
}

type transportTunnelSession struct {
	mgr      *transport.TransportManager
	tunnelID string
	endpoint string
}

func (s *transportTunnelSession) Close() error {
	if s == nil || s.mgr == nil {
		return nil
	}
	return s.mgr.Close()
}

func (s *transportTunnelSession) TunnelID() string {
	if s == nil {
		return ""
	}
	return s.tunnelID
}

func (s *transportTunnelSession) Endpoint() string {
	if s == nil {
		return ""
	}
	return s.endpoint
}

// GRPCTunnelProber checks gRPC control-plane reachability.
type GRPCTunnelProber interface {
	Probe(ctx context.Context, cfg *types.Config) error
}

// GRPCTunnelOpener performs CreateTunnel-style open via gRPC transport.
type GRPCTunnelOpener interface {
	Open(ctx context.Context, cfg *types.Config, req OpenRequest, token string) (grpcTunnelSession, error)
}

type defaultGRPCTunnelProber struct{}

func (defaultGRPCTunnelProber) Probe(ctx context.Context, cfg *types.Config) error {
	return ProbeGRPCReachability(ctx, cfg)
}

type defaultGRPCTunnelOpener struct{}

func (defaultGRPCTunnelOpener) Open(ctx context.Context, cfg *types.Config, req OpenRequest, token string) (grpcTunnelSession, error) {
	return openGRPCTunnelSession(ctx, cfg, req, token)
}

// GRPCTunnelPath adapts gRPC CreateTunnel for the path ladder.
type GRPCTunnelPath struct {
	cfg    *types.Config
	prober GRPCTunnelProber
	opener GRPCTunnelOpener
	mu     sync.Mutex
	active grpcTunnelSession
}

// NewGRPCTunnelPath returns a production GRPCTunnelPath from config.
func NewGRPCTunnelPath(cfg *types.Config) *GRPCTunnelPath {
	return NewGRPCTunnelPathWithDeps(cfg, defaultGRPCTunnelProber{}, defaultGRPCTunnelOpener{})
}

// NewGRPCTunnelPathWithDeps allows injecting probe/open deps (tests, smoke).
func NewGRPCTunnelPathWithDeps(cfg *types.Config, prober GRPCTunnelProber, opener GRPCTunnelOpener) *GRPCTunnelPath {
	if prober == nil {
		prober = defaultGRPCTunnelProber{}
	}
	if opener == nil {
		opener = defaultGRPCTunnelOpener{}
	}
	return &GRPCTunnelPath{cfg: cfg, prober: prober, opener: opener}
}

func (p *GRPCTunnelPath) Name() string { return PathGRPCTunnel }

func (p *GRPCTunnelPath) Probe(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("grpc_tunnel: nil path")
	}
	return p.prober.Probe(ctx, p.cfg)
}

func (p *GRPCTunnelPath) Open(ctx context.Context, req OpenRequest) (Handle, error) {
	if p == nil {
		return nil, fmt.Errorf("grpc_tunnel: nil path")
	}
	token := tokenFromRequest(req)
	sess, err := p.opener.Open(ctx, p.cfg, req, token)
	if err != nil {
		return nil, err
	}
	h := &grpcTunnelHandle{name: PathGRPCTunnel, session: sess}
	p.mu.Lock()
	if p.active != nil {
		_ = p.active.Close()
	}
	p.active = sess
	p.mu.Unlock()
	return h, nil
}

func (p *GRPCTunnelPath) Close(ctx context.Context) error {
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

type grpcTunnelHandle struct {
	name    string
	session grpcTunnelSession
}

func (h *grpcTunnelHandle) PathName() string {
	if h == nil {
		return PathGRPCTunnel
	}
	return h.name
}

func (h *grpcTunnelHandle) Close(ctx context.Context) error {
	if h == nil || h.session == nil {
		return nil
	}
	return h.session.Close()
}

// Endpoint returns relay-side tunnel endpoint when available.
func (h *grpcTunnelHandle) Endpoint() string {
	if h == nil || h.session == nil {
		return ""
	}
	return h.session.Endpoint()
}

type nopTransportLogger struct{}

func (nopTransportLogger) Info(string, ...interface{})  {}
func (nopTransportLogger) Error(string, ...interface{}) {}
func (nopTransportLogger) Debug(string, ...interface{}) {}
func (nopTransportLogger) Warn(string, ...interface{})  {}

// openGRPCTunnelSession connects gRPC transport and creates a tunnel.
func openGRPCTunnelSession(ctx context.Context, cfg *types.Config, req OpenRequest, token string) (grpcTunnelSession, error) {
	tm := transport.NewTransportManager(cfg, nopTransportLogger{})
	if err := tm.Initialize(); err != nil {
		return nil, fmt.Errorf("transport init: %w", err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := connectTransport(connectCtx, tm); err != nil {
		_ = tm.Close()
		return nil, err
	}

	if token != "" {
		authCtx, authCancel := context.WithTimeout(ctx, 10*time.Second)
		defer authCancel()
		if _, err := authenticateTransport(authCtx, tm, token); err != nil {
			_ = tm.Close()
			return nil, err
		}
	}

	tunnelID := req.SessionID
	if tunnelID == "" {
		tunnelID = fmt.Sprintf("ps_%d", time.Now().UnixNano())
	}
	remoteHost := req.RemoteHost
	if remoteHost == "" {
		remoteHost = "127.0.0.1"
	}
	remotePort := req.RemotePort
	if remotePort <= 0 {
		remotePort = 80
	}

	tunCtx, tunCancel := context.WithTimeout(ctx, 15*time.Second)
	defer tunCancel()
	result, err := createTunnelTransport(tunCtx, tm, tunnelID, req.TenantID, remoteHost, remotePort)
	if err != nil {
		_ = tm.Close()
		return nil, err
	}
	if result.Status != "ok" {
		_ = tm.Close()
		return nil, fmt.Errorf("CreateTunnel: %s", result.ErrorMessage)
	}
	if result.Endpoint == "" {
		_ = tm.Close()
		return nil, fmt.Errorf("CreateTunnel returned empty endpoint")
	}

	return &transportTunnelSession{
		mgr:      tm,
		tunnelID: result.TunnelID,
		endpoint: result.Endpoint,
	}, nil
}

func connectTransport(ctx context.Context, tm *transport.TransportManager) error {
	type result struct{ err error }
	ch := make(chan result, 1)
	go func() {
		ch <- result{err: tm.Connect()}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return fmt.Errorf("grpc connect: %w", r.err)
		}
		return nil
	}
}

func authenticateTransport(ctx context.Context, tm *transport.TransportManager, token string) (*transport.AuthResult, error) {
	type result struct {
		res *transport.AuthResult
		err error
	}
	ch := make(chan result, 1)
	go func() {
		res, err := tm.Authenticate(token)
		ch <- result{res: res, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("grpc authenticate: %w", r.err)
		}
		if r.res.Status != "ok" {
			return nil, fmt.Errorf("grpc authenticate: %s", r.res.ErrorMessage)
		}
		return r.res, nil
	}
}

func createTunnelTransport(ctx context.Context, tm *transport.TransportManager, tunnelID, tenantID, remoteHost string, remotePort int) (*transport.TunnelResult, error) {
	type result struct {
		res *transport.TunnelResult
		err error
	}
	ch := make(chan result, 1)
	go func() {
		res, err := tm.CreateTunnel(tunnelID, tenantID, 0, remoteHost, remotePort)
		ch <- result{res: res, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("CreateTunnel: %w", r.err)
		}
		return r.res, nil
	}
}
