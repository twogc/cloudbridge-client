package interfaces

import (
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// ClientInterface defines the common interface for relay clients.
type ClientInterface interface {
	// Connection state
	IsConnected() bool
	Start() error
	Stop() error
	Reconnect() error

	// Heartbeat & telemetry
	SendHeartbeat() error
	LastHeartbeat() time.Time

	// Identification
	GetClientID() string
	GetTenantID() string

	// Configuration
	GetConfig() *types.Config

	// Events (optional async)
	SubscribeState() <-chan ClientStateEvent
}

// ClientStateEvent represents async updates (connection up/down, reconnect, etc.)
type ClientStateEvent struct {
	State   string    // e.g. "connected", "disconnected", "reconnecting"
	Reason  string    // optional human-readable
	Updated time.Time // timestamp
}

// ConfigInterface defines a read-only configuration accessor.
type ConfigInterface interface {
	// Relay
	GetRelayHost() string
	GetRelayPort() int
	GetRelayTimeout() time.Duration

	// TLS
	GetTLSEnabled() bool
	GetTLSMinVersion() string
	GetTLSVerifyCert() bool
	GetTLSCACert() string
	GetTLSClientCert() string
	GetTLSClientKey() string
	GetTLSServerName() string

	// Auth
	GetAuthType() string
	GetAuthSecret() string

	// Rate limiting
	GetRateLimitingEnabled() bool
	GetRateLimitingMaxRetries() int
	GetRateLimitingBackoffMultiplier() float64
	GetRateLimitingMaxBackoff() time.Duration

	// Additional convenience getters (future-proof)
	GetAuthConfig() *types.AuthConfig
	GetTLSConfig() *types.TLSConfig
}
