package relay

import (
	"fmt"

	"github.com/twogc/cloudbridge-client/pkg/relay/transport"
	"github.com/twogc/cloudbridge-client/pkg/types"
)

// TransportAdapter adapts the new gRPC transport to the existing client interface
type TransportAdapter struct {
	transportManager *transport.TransportManager
	logger           *relayLogger
}

// NewTransportAdapter creates a new transport adapter
func NewTransportAdapter(config *types.Config, logger *relayLogger) *TransportAdapter {
	// Create transport logger adapter
	transportLogger := &transportLoggerAdapter{logger: logger}

	return &TransportAdapter{
		transportManager: transport.NewTransportManager(config, transportLogger),
		logger:           logger,
	}
}

// Initialize initializes the transport adapter
func (ta *TransportAdapter) Initialize() error {
	return ta.transportManager.Initialize()
}

// SetTransportMode sets the transport mode (grpc or json)
func (ta *TransportAdapter) SetTransportMode(mode string) error {
	var transportMode transport.TransportMode

	switch mode {
	case "grpc":
		transportMode = transport.TransportModeGRPC
	case "json":
		transportMode = transport.TransportModeJSON
	default:
		return fmt.Errorf("unsupported transport mode: %s", mode)
	}

	return ta.transportManager.SwitchTransport(transportMode)
}

// Connect connects using the current transport
func (ta *TransportAdapter) Connect() error {
	return ta.transportManager.Connect()
}

// Disconnect disconnects the current transport
func (ta *TransportAdapter) Disconnect() error {
	return ta.transportManager.Disconnect()
}

// Hello performs hello handshake
func (ta *TransportAdapter) Hello(version string, features []string) error {
	result, err := ta.transportManager.Hello(version, features)
	if err != nil {
		return err
	}

	if result.Status != "ok" {
		return fmt.Errorf("hello failed: %s", result.ErrorMessage)
	}

	ta.logger.Info("Hello completed via transport",
		"mode", ta.transportManager.GetCurrentMode(),
		"server_version", result.ServerVersion,
		"session_id", result.SessionID)

	return nil
}

// Authenticate performs authentication
func (ta *TransportAdapter) Authenticate(token string) (clientID, tenantID string, err error) {
	result, err := ta.transportManager.Authenticate(token)
	if err != nil {
		return "", "", err
	}

	if result.Status != "ok" {
		return "", "", fmt.Errorf("authentication failed: %s", result.ErrorMessage)
	}

	ta.logger.Info("Authentication completed via transport",
		"mode", ta.transportManager.GetCurrentMode(),
		"client_id", result.ClientID,
		"tenant_id", result.TenantID)

	return result.ClientID, result.TenantID, nil
}

// CreateTunnel creates a tunnel
func (ta *TransportAdapter) CreateTunnel(tunnelID, tenantID string, localPort int, remoteHost string, remotePort int) error {
	result, err := ta.transportManager.CreateTunnel(tunnelID, tenantID, localPort, remoteHost, remotePort)
	if err != nil {
		return err
	}

	if result.Status != "ok" {
		return fmt.Errorf("tunnel creation failed: %s", result.ErrorMessage)
	}

	ta.logger.Info("Tunnel created via transport",
		"mode", ta.transportManager.GetCurrentMode(),
		"tunnel_id", result.TunnelID,
		"endpoint", result.Endpoint)

	return nil
}

// SendHeartbeat sends a heartbeat
func (ta *TransportAdapter) SendHeartbeat(clientID, tenantID string) error {
	// Create basic metrics
	metrics := &transport.ClientMetrics{
		TransportMode: string(ta.transportManager.GetCurrentMode()),
	}

	result, err := ta.transportManager.SendHeartbeat(clientID, tenantID, metrics)
	if err != nil {
		return err
	}

	if result.Status != "ok" {
		return fmt.Errorf("heartbeat failed: %s", result.ErrorMessage)
	}

	ta.logger.Debug("Heartbeat sent via transport",
		"mode", ta.transportManager.GetCurrentMode(),
		"interval", result.IntervalSeconds)

	return nil
}

// IsConnected returns connection status
func (ta *TransportAdapter) IsConnected() bool {
	return ta.transportManager.IsConnected()
}

// GetCurrentMode returns the current transport mode
func (ta *TransportAdapter) GetCurrentMode() string {
	mode := ta.transportManager.GetCurrentMode()
	return string(mode)
}

// Close closes the transport adapter
func (ta *TransportAdapter) Close() error {
	return ta.transportManager.Close()
}

// transportLoggerAdapter adapts relayLogger to transport.Logger interface
type transportLoggerAdapter struct {
	logger *relayLogger
}

func (tla *transportLoggerAdapter) Info(msg string, fields ...interface{}) {
	tla.logger.Info(msg, fields...)
}

func (tla *transportLoggerAdapter) Error(msg string, fields ...interface{}) {
	tla.logger.Error(msg, fields...)
}

func (tla *transportLoggerAdapter) Debug(msg string, fields ...interface{}) {
	tla.logger.Debug(msg, fields...)
}

func (tla *transportLoggerAdapter) Warn(msg string, fields ...interface{}) {
	tla.logger.Warn(msg, fields...)
}
