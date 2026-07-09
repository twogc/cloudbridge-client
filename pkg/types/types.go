package types

import (
	"time"
)

// Platform constants
const (
	PlatformLinux   = "linux"
	PlatformWindows = "windows"
	PlatformDarwin  = "darwin"
)

// Status constants
const (
	StatusInactive = "inactive"
)

// Config represents the complete client configuration
type Config struct {
	Relay        RelayConfig        `mapstructure:"relay"`
	Auth         AuthConfig         `mapstructure:"auth"`
	RateLimiting RateLimitingConfig `mapstructure:"rate_limiting"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	Metrics      MetricsConfig      `mapstructure:"metrics"`
	Performance  PerformanceConfig  `mapstructure:"performance"`
	API          APIConfig          `mapstructure:"api"`
	ICE          ICEConfig          `mapstructure:"ice"`
	QUIC         QUICConfig         `mapstructure:"quic"`
	P2P          P2PConfig          `mapstructure:"p2p"`
	TURN         TURNConfig         `mapstructure:"turn"`
	DERP         DERPConfig         `mapstructure:"derp"`
	WebSocket    WebSocketConfig    `mapstructure:"websocket"`
	WireGuard    WireGuardConfig    `mapstructure:"wireguard"`
	// PathSelect is the probe ladder / session path policy (GAP Phase A+).
	// Disabled by default until Phase C CLI wiring.
	PathSelect PathSelectConfig `mapstructure:"path_select"`
}

// PathSelectConfig is the path_select: block (see pkg/pathselect, GAP plan).
type PathSelectConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	Order            []string      `mapstructure:"order"`
	ProbeTimeout     time.Duration `mapstructure:"probe_timeout"`
	LadderTimeout    time.Duration `mapstructure:"ladder_timeout"`
	HealthInterval   time.Duration `mapstructure:"health_interval"`
	FailoverCooldown time.Duration `mapstructure:"failover_cooldown"`
	SoftFail         bool          `mapstructure:"soft_fail"`
}

// RelayConfig contains relay server connection settings
type RelayConfig struct {
	Host    string        `mapstructure:"host"`
	Port    int           `mapstructure:"port"` // Legacy port for backward compatibility
	Ports   RelayPorts    `mapstructure:"ports"`
	Timeout time.Duration `mapstructure:"timeout"`
	TLS     TLSConfig     `mapstructure:"tls"`
}

// RelayPorts contains all relay server ports.
// Canonical mapping: docs/CONTRACT_CLIENT_RELAY.md (align with relay port-scheme).
type RelayPorts struct {
	HTTPAPI      int `mapstructure:"http_api"`       // 9090 TCP — HTTP API / health
	HTTPSAPI     int `mapstructure:"https_api"`      // 5553 TCP — HTTPS API
	P2PAPI       int `mapstructure:"p2p_api"`        // 5552 TCP — P2P REST
	QUIC         int `mapstructure:"quic"`           // 5553 UDP — P2P QUIC
	QUICMain     int `mapstructure:"quic_main"`      // 9090 UDP — main QUIC transport
	GRPC         int `mapstructure:"grpc"`           // 8444 TCP — gRPC control plane
	STUN         int `mapstructure:"stun"`           // 19302 UDP
	MASQUE       int `mapstructure:"masque"`         // 8443
	EnhancedQUIC int `mapstructure:"enhanced_quic"`  // 9092 UDP
	TURN         int `mapstructure:"turn"`           // 3478
	DERP         int `mapstructure:"derp"`           // 3479
}

// Canonical default ports (relay installer port-scheme).
const (
	DefaultLegacyTCPPort = 5550
	DefaultP2PAPIPort    = 5552
	DefaultHTTPSAPIPort  = 5553
	DefaultP2PQUICPort   = 5553
	DefaultHTTPAPIPort   = 9090
	DefaultQUICMainPort  = 9090
	DefaultGRPCPort      = 8444
	DefaultMASQUEPort    = 8443
	DefaultSTUNPort      = 19302
	DefaultTURNPort      = 3478
	DefaultDERPPort      = 3479
	DefaultEnhancedQUIC  = 9092
	DefaultWireGuardPort = 51820
	DefaultClientMetrics = 9091
)

// TLSConfig contains TLS-specific settings
type TLSConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	MinVersion string `mapstructure:"min_version"`
	VerifyCert bool   `mapstructure:"verify_cert"`
	CACert     string `mapstructure:"ca_cert"`
	ClientCert string `mapstructure:"client_cert"`
	ClientKey  string `mapstructure:"client_key"`
	ServerName string `mapstructure:"server_name"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	Type           string     `mapstructure:"type"`
	Token          string     `mapstructure:"token"`
	Secret         string     `mapstructure:"secret"`
	FallbackSecret string     `mapstructure:"fallback_secret"`
	SkipValidation bool       `mapstructure:"skip_validation"`
	OIDC           OIDCConfig `mapstructure:"oidc"`
}

// OIDCConfig contains OIDC integration settings
type OIDCConfig struct {
	IssuerURL string `mapstructure:"issuer_url"`
	Audience  string `mapstructure:"audience"`
	JWKSURL   string `mapstructure:"jwks_url"`
}

// RateLimitingConfig contains rate limiting settings
type RateLimitingConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	MaxRetries        int           `mapstructure:"max_retries"`
	BackoffMultiplier float64       `mapstructure:"backoff_multiplier"`
	MaxBackoff        time.Duration `mapstructure:"max_backoff"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled           bool              `mapstructure:"enabled"`
	PrometheusPort    int               `mapstructure:"prometheus_port"`
	TenantMetrics     bool              `mapstructure:"tenant_metrics"`
	BufferMetrics     bool              `mapstructure:"buffer_metrics"`
	ConnectionMetrics bool              `mapstructure:"connection_metrics"`
	Pushgateway       PushgatewayConfig `mapstructure:"pushgateway"`
}

// PushgatewayConfig contains Pushgateway configuration
type PushgatewayConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	URL          string        `mapstructure:"push_url"`
	JobName      string        `mapstructure:"job_name"`
	Instance     string        `mapstructure:"instance"`
	PushInterval time.Duration `mapstructure:"push_interval"`
}

// PerformanceConfig contains performance optimization settings
type PerformanceConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	OptimizationMode string `mapstructure:"optimization_mode"`
	GCPercent        int    `mapstructure:"gc_percent"`
	MemoryBallast    bool   `mapstructure:"memory_ballast"`
}

// APIConfig contains HTTP API settings for P2P operations
type APIConfig struct {
	BaseURL            string        `mapstructure:"base_url"`
	P2PAPIURL          string        `mapstructure:"p2p_api_url"`
	HeartbeatURL       string        `mapstructure:"heartbeat_url"`
	InsecureSkipVerify bool          `mapstructure:"insecure_skip_verify"`
	Timeout            time.Duration `mapstructure:"timeout"`
	MaxRetries         int           `mapstructure:"max_retries"`
	BackoffMultiplier  float64       `mapstructure:"backoff_multiplier"`
	MaxBackoff         time.Duration `mapstructure:"max_backoff"`
	// RouteMonitoringEnabled posts ConnectionRequest-shaped bodies to POST /relay/route.
	// Relay SoT expects RelayRouteRequest (from_peer,to_peer,data,message_type) — schema mismatch
	// caused 400 WARN spam on happy path. Default false (GAP Phase D.1).
	RouteMonitoringEnabled bool `mapstructure:"route_monitoring_enabled"`
}

// ICEConfig contains ICE/STUN/TURN configuration
type ICEConfig struct {
	STUNServers        []string      `mapstructure:"stun_servers"`
	TURNServers        []string      `mapstructure:"turn_servers"`
	DERPServers        []string      `mapstructure:"derp_servers"`
	Timeout            time.Duration `mapstructure:"timeout"`
	MaxBindingRequests int           `mapstructure:"max_binding_requests"`
	ConnectivityChecks bool          `mapstructure:"connectivity_checks"`
	CandidateGathering bool          `mapstructure:"candidate_gathering"`
}

// QUICConfig contains QUIC connection configuration
type QUICConfig struct {
	HandshakeTimeout   time.Duration `mapstructure:"handshake_timeout"`
	IdleTimeout        time.Duration `mapstructure:"idle_timeout"`
	MaxStreams         int           `mapstructure:"max_streams"`
	MaxStreamData      int           `mapstructure:"max_stream_data"`
	KeepAlivePeriod    time.Duration `mapstructure:"keep_alive_period"`
	InsecureSkipVerify bool          `mapstructure:"insecure_skip_verify"`
	MASQUESupport      bool          `mapstructure:"masque_support"`
	HTTPDatagrams      bool          `mapstructure:"http_datagrams"`
}

// P2PConfig contains P2P mesh configuration
type P2PConfig struct {
	MaxConnections          int           `mapstructure:"max_connections"`
	SessionTimeout          time.Duration `mapstructure:"session_timeout"`
	PeerDiscoveryInterval   time.Duration `mapstructure:"peer_discovery_interval"`
	ConnectionRetryInterval time.Duration `mapstructure:"connection_retry_interval"`
	MaxRetryAttempts        int           `mapstructure:"max_retry_attempts"`
	HeartbeatInterval       time.Duration `mapstructure:"heartbeat_interval"`
	HeartbeatTimeout        time.Duration `mapstructure:"heartbeat_timeout"`
	FallbackEnabled         bool          `mapstructure:"fallback_enabled"`
	DERPFallback            bool          `mapstructure:"derp_fallback"`
	WebSocketFallback       bool          `mapstructure:"websocket_fallback"`
}

// TURNConfig contains TURN server configuration
type TURNConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	Servers        []string      `mapstructure:"servers"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	Realm          string        `mapstructure:"realm"`
	Timeout        time.Duration `mapstructure:"timeout"`
	MaxAllocations int           `mapstructure:"max_allocations"`
}

// DERPConfig contains DERP server configuration
type DERPConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	Servers          []string      `mapstructure:"servers"`
	Timeout          time.Duration `mapstructure:"timeout"`
	MaxConnections   int           `mapstructure:"max_connections"`
	FallbackPriority int           `mapstructure:"fallback_priority"`
}

// WebSocketConfig contains WebSocket tunneling configuration
type WebSocketConfig struct {
	Enabled              bool          `mapstructure:"enabled"`
	Endpoint             string        `mapstructure:"endpoint"`
	Timeout              time.Duration `mapstructure:"timeout"`
	PingInterval         time.Duration `mapstructure:"ping_interval"`
	MaxReconnectAttempts int           `mapstructure:"max_reconnect_attempts"`
}

// WireGuardConfig contains WireGuard VPN configuration
type WireGuardConfig struct {
	Enabled             bool          `mapstructure:"enabled"`
	InterfaceName       string        `mapstructure:"interface_name"`
	Port                int           `mapstructure:"port"`
	MTU                 int           `mapstructure:"mtu"`
	PersistentKeepAlive time.Duration `mapstructure:"persistent_keepalive"`
}
