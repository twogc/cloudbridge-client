package pathselect

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/twogc/cloudbridge-client/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// RelayQUICAddr returns host:port for P2P QUIC from config (no hardcoded ports).
func RelayQUICAddr(cfg *types.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.P2PQUICAddr()
}

// GRPCTarget returns host:port for gRPC control plane from config.
func GRPCTarget(cfg *types.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.GRPCTarget()
}

// DefaultQUICConfig returns client-side QUIC settings aligned with smoke paths.
func DefaultQUICConfig() *quic.Config {
	return &quic.Config{
		HandshakeIdleTimeout:  8 * time.Second,
		MaxIdleTimeout:        20 * time.Second,
		KeepAlivePeriod:       10 * time.Second,
		MaxIncomingStreams:    64,
		MaxIncomingUniStreams: 64,
	}
}

// QUICClientTLS builds TLS config for relay P2P QUIC dial.
func QUICClientTLS(cfg *types.Config) *tls.Config {
	host := ""
	if cfg != nil {
		host = cfg.RelayHost()
	}
	tlsConf := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		NextProtos:         []string{"cloudbridge-p2p", "h3"},
		ServerName:         "localhost",
	}
	if host != "" && host != "127.0.0.1" && host != "localhost" {
		tlsConf.ServerName = host
	}
	if cfg != nil && cfg.Relay.TLS.Enabled {
		tlsConf.InsecureSkipVerify = !cfg.Relay.TLS.VerifyCert
		if cfg.Relay.TLS.ServerName != "" {
			tlsConf.ServerName = cfg.Relay.TLS.ServerName
		} else if host != "" {
			tlsConf.ServerName = host
		}
	}
	return tlsConf
}

// ProbeQUICReachability performs a cheap QUIC handshake to relay P2P QUIC.
func ProbeQUICReachability(ctx context.Context, cfg *types.Config) error {
	addr := RelayQUICAddr(cfg)
	if addr == "" {
		return fmt.Errorf("relay QUIC address not configured")
	}
	conn, err := quic.DialAddr(ctx, addr, QUICClientTLS(cfg), DefaultQUICConfig())
	if err != nil {
		return fmt.Errorf("quic probe dial %s: %w", addr, err)
	}
	return conn.CloseWithError(0, "probe done")
}

// ProbeGRPCReachability dials the gRPC control plane (no auth required for probe).
func ProbeGRPCReachability(ctx context.Context, cfg *types.Config) error {
	target := GRPCTarget(cfg)
	if target == "" {
		return fmt.Errorf("gRPC target not configured")
	}
	opts, err := GRPCDialOptions(cfg, true)
	if err != nil {
		return fmt.Errorf("grpc dial options: %w", err)
	}
	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return fmt.Errorf("grpc probe dial %s: %w", target, err)
	}
	return conn.Close()
}

// GRPCDialOptions builds grpc.Dial options from relay TLS settings.
// When block is true, waits until the connection is ready (for probes/opens).
func GRPCDialOptions(cfg *types.Config, block bool) ([]grpc.DialOption, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             3 * time.Second,
		PermitWithoutStream: true,
	}))
	if cfg != nil && cfg.Relay.TLS.Enabled {
		tlsConf := &tls.Config{
			MinVersion:         tls.VersionTLS13,
			InsecureSkipVerify: !cfg.Relay.TLS.VerifyCert,
			NextProtos:         []string{"h2"},
		}
		if cfg.Relay.TLS.ServerName != "" {
			tlsConf.ServerName = cfg.Relay.TLS.ServerName
		} else if host := cfg.RelayHost(); host != "" {
			tlsConf.ServerName = host
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConf)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if block {
		opts = append(opts, grpc.WithBlock())
	}
	return opts, nil
}

// tokenFromRequest reads JWT/token from OpenRequest.Meta["token"].
func tokenFromRequest(req OpenRequest) string {
	if req.Meta == nil {
		return ""
	}
	return req.Meta["token"]
}

// splitHostPort parses host:port; used when LocalListen is set.
func splitHostPort(listen string) (host string, port int, err error) {
	h, ps, err := net.SplitHostPort(listen)
	if err != nil {
		return "", 0, err
	}
	p, err := net.LookupPort("tcp", ps)
	if err != nil {
		return "", 0, err
	}
	return h, p, nil
}
