package transport

import (
	"fmt"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// TransportMode represents the transport protocol mode
type TransportMode string

const (
	TransportModeJSON TransportMode = "json"
	TransportModeGRPC TransportMode = "grpc"
)

// Transport interface defines the transport layer operations
type Transport interface {
	// Connect establishes connection to the relay server
	Connect() error

	// Disconnect closes the connection
	Disconnect() error

	// Hello performs initial handshake
	Hello(version string, features []string) (*HelloResult, error)

	// Authenticate performs authentication
	Authenticate(token string) (*AuthResult, error)

	// CreateTunnel creates a new tunnel
	CreateTunnel(tunnelID, tenantID string, localPort int, remoteHost string, remotePort int) (*TunnelResult, error)

	// SendHeartbeat sends a heartbeat
	SendHeartbeat(clientID, tenantID string, metrics *ClientMetrics) (*HeartbeatResult, error)

	// IsConnected returns connection status
	IsConnected() bool

	// Close closes the transport and cleans up resources
	Close() error
}

// HelloResult contains hello response data
type HelloResult struct {
	Status            string
	ServerVersion     string
	SupportedFeatures []string
	SessionID         string
	ErrorMessage      string
}

// AuthResult contains authentication response data
type AuthResult struct {
	Status       string
	ClientID     string
	TenantID     string
	SessionToken string
	ExpiresAt    time.Time
	ErrorMessage string
}

// TunnelResult contains tunnel creation response data
type TunnelResult struct {
	Status       string
	TunnelID     string
	Endpoint     string
	ErrorMessage string
}

// HeartbeatResult contains heartbeat response data
type HeartbeatResult struct {
	Status          string
	ServerTimestamp time.Time
	IntervalSeconds int32
	ErrorMessage    string
}

// ClientMetrics contains client metrics for heartbeat
type ClientMetrics struct {
	BytesSent         int64
	BytesReceived     int64
	PacketsSent       int64
	PacketsReceived   int64
	ActiveTunnels     int32
	ActiveP2PSessions int32
	CPUUsage          float64
	MemoryUsage       float64
	TransportMode     string
	LastSwitch        time.Time
}

// TransportManager manages multiple transport implementations
type TransportManager struct {
	config        *types.Config
	logger        Logger
	currentMode   TransportMode
	grpcTransport *GRPCTransport
	jsonTransport Transport // Legacy JSON transport
	mu            sync.RWMutex
}

// NewTransportManager creates a new transport manager
func NewTransportManager(config *types.Config, logger Logger) *TransportManager {
	return &TransportManager{
		config:      config,
		logger:      logger,
		currentMode: TransportModeGRPC, // Default to gRPC
	}
}

// Initialize initializes the transport manager
func (tm *TransportManager) Initialize() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Initialize gRPC transport
	grpcClient := NewGRPCClient(tm.config, tm.logger)
	tm.grpcTransport = NewGRPCTransport(grpcClient, tm.logger)

	// TODO: Initialize legacy JSON transport as fallback
	// tm.jsonTransport = NewJSONTransport(tm.config, tm.logger)

	tm.logger.Info("Transport manager initialized", "default_mode", tm.currentMode)
	return nil
}

// GetTransport returns the current active transport
func (tm *TransportManager) GetTransport() Transport {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	switch tm.currentMode {
	case TransportModeGRPC:
		return tm.grpcTransport
	case TransportModeJSON:
		if tm.jsonTransport != nil {
			return tm.jsonTransport
		}
		// Fallback to gRPC if JSON transport not available
		tm.logger.Warn("JSON transport not available, falling back to gRPC")
		return tm.grpcTransport
	default:
		return tm.grpcTransport
	}
}

// SwitchTransport switches to a different transport mode
func (tm *TransportManager) SwitchTransport(mode TransportMode) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.currentMode == mode {
		return nil // Already using the requested mode
	}

	// Disconnect current transport
	currentTransport := tm.getCurrentTransportUnsafe()
	if currentTransport != nil && currentTransport.IsConnected() {
		if err := currentTransport.Disconnect(); err != nil {
			tm.logger.Warn("Failed to disconnect current transport", "mode", tm.currentMode, "error", err)
		}
	}

	// Switch to new mode
	oldMode := tm.currentMode
	tm.currentMode = mode

	tm.logger.Info("Switched transport mode", "from", oldMode, "to", mode)
	return nil
}

// getCurrentTransportUnsafe returns current transport without locking (internal use)
func (tm *TransportManager) getCurrentTransportUnsafe() Transport {
	switch tm.currentMode {
	case TransportModeGRPC:
		return tm.grpcTransport
	case TransportModeJSON:
		return tm.jsonTransport
	default:
		return tm.grpcTransport
	}
}

// GetCurrentMode returns the current transport mode
func (tm *TransportManager) GetCurrentMode() TransportMode {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.currentMode
}

// Close closes all transports and cleans up resources
func (tm *TransportManager) Close() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	var lastErr error

	if tm.grpcTransport != nil {
		if err := tm.grpcTransport.Close(); err != nil {
			tm.logger.Error("Failed to close gRPC transport", "error", err)
			lastErr = err
		}
	}

	if tm.jsonTransport != nil {
		if err := tm.jsonTransport.Close(); err != nil {
			tm.logger.Error("Failed to close JSON transport", "error", err)
			lastErr = err
		}
	}

	tm.logger.Info("Transport manager closed")
	return lastErr
}

// IsConnected returns true if the current transport is connected
func (tm *TransportManager) IsConnected() bool {
	transport := tm.GetTransport()
	if transport == nil {
		return false
	}
	return transport.IsConnected()
}

// Connect connects using the current transport
func (tm *TransportManager) Connect() error {
	transport := tm.GetTransport()
	if transport == nil {
		return fmt.Errorf("no transport available")
	}
	return transport.Connect()
}

// Disconnect disconnects the current transport
func (tm *TransportManager) Disconnect() error {
	transport := tm.GetTransport()
	if transport == nil {
		return nil
	}
	return transport.Disconnect()
}

// Hello performs hello handshake using current transport
func (tm *TransportManager) Hello(version string, features []string) (*HelloResult, error) {
	transport := tm.GetTransport()
	if transport == nil {
		return nil, fmt.Errorf("no transport available")
	}
	return transport.Hello(version, features)
}

// Authenticate performs authentication using current transport
func (tm *TransportManager) Authenticate(token string) (*AuthResult, error) {
	transport := tm.GetTransport()
	if transport == nil {
		return nil, fmt.Errorf("no transport available")
	}
	return transport.Authenticate(token)
}

// CreateTunnel creates a tunnel using current transport
func (tm *TransportManager) CreateTunnel(tunnelID, tenantID string, localPort int,
	remoteHost string, remotePort int) (*TunnelResult, error) {
	transport := tm.GetTransport()
	if transport == nil {
		return nil, fmt.Errorf("no transport available")
	}
	return transport.CreateTunnel(tunnelID, tenantID, localPort, remoteHost, remotePort)
}

// SendHeartbeat sends heartbeat using current transport
func (tm *TransportManager) SendHeartbeat(clientID, tenantID string, metrics *ClientMetrics) (*HeartbeatResult, error) {
	transport := tm.GetTransport()
	if transport == nil {
		return nil, fmt.Errorf("no transport available")
	}
	return transport.SendHeartbeat(clientID, tenantID, metrics)
}

// Note: timeToTimestamp and timestampToTime utility functions would be implemented
// when actual protobuf integration is added
