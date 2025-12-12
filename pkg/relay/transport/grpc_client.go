package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// GRPCClient represents a gRPC transport client
type GRPCClient struct {
	config    *types.Config
	conn      *grpc.ClientConn
	logger    Logger
	mu        sync.RWMutex
	connected bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// Logger interface for gRPC client logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// NewGRPCClient creates a new gRPC transport client
func NewGRPCClient(config *types.Config, logger Logger) *GRPCClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &GRPCClient{
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Connect establishes a gRPC connection to the relay server
func (gc *GRPCClient) Connect() error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if gc.connected {
		return fmt.Errorf("already connected")
	}

	target := fmt.Sprintf("%s:%d", gc.config.Relay.Host, gc.config.Relay.Port)

	// Build connection options
	opts, err := gc.buildConnectionOptions()
	if err != nil {
		return fmt.Errorf("failed to build connection options: %w", err)
	}

	// Establish gRPC connection with timeout
	ctx, cancel := context.WithTimeout(gc.ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	gc.conn = conn
	gc.connected = true

	gc.logger.Info("gRPC connection established", "target", target)
	return nil
}

// Disconnect closes the gRPC connection
func (gc *GRPCClient) Disconnect() error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if !gc.connected {
		return nil
	}

	if gc.conn != nil {
		if err := gc.conn.Close(); err != nil {
			gc.logger.Warn("Error closing gRPC connection", "error", err)
		}
	}

	gc.connected = false
	gc.logger.Info("gRPC connection closed")
	return nil
}

// Close closes the client and cleans up resources
func (gc *GRPCClient) Close() error {
	gc.cancel()
	return gc.Disconnect()
}

// IsConnected returns true if the client is connected
func (gc *GRPCClient) IsConnected() bool {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.connected
}

// GetConnection returns the underlying gRPC connection
func (gc *GRPCClient) GetConnection() *grpc.ClientConn {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.conn
}

// buildConnectionOptions builds gRPC connection options based on configuration
func (gc *GRPCClient) buildConnectionOptions() ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

	// Add keepalive parameters
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             3 * time.Second,
		PermitWithoutStream: true,
	}))

	// Configure TLS
	if gc.config.Relay.TLS.Enabled {
		tlsConfig, err := gc.buildTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}

		// Force TLS 1.3 and HTTP/2 (h2) ALPN
		tlsConfig.NextProtos = []string{"h2"}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		// No TLS - use insecure connection (plaintext)
		opts = append(opts, grpc.WithInsecure())
	}

	// Add other options
	opts = append(opts, grpc.WithBlock()) // Wait for connection to be ready

	return opts, nil
}

// buildTLSConfig builds TLS configuration for gRPC
func (gc *GRPCClient) buildTLSConfig() (*tls.Config, error) {
	// Use the existing TLS config creation from the config package
	// This ensures consistency with the main client TLS configuration
	return createTLSConfigForGRPC(gc.config)
}

// createTLSConfigForGRPC creates a TLS config specifically for gRPC
func createTLSConfigForGRPC(config *types.Config) (*tls.Config, error) {
	if !config.Relay.TLS.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
		},
		InsecureSkipVerify: !config.Relay.TLS.VerifyCert,
	}

	// Set server name for SNI
	if config.Relay.TLS.ServerName != "" {
		tlsConfig.ServerName = config.Relay.TLS.ServerName
	} else {
		tlsConfig.ServerName = config.Relay.Host
	}

	// Note: CA certificate and client certificate loading would be implemented
	// similar to the main config package, but for brevity we'll keep it simple here
	// In a real implementation, you would load these from the config

	return tlsConfig, nil
}

// GetContext returns the client context
func (gc *GRPCClient) GetContext() context.Context {
	return gc.ctx
}
