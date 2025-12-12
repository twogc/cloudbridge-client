package types

import (
	"testing"
	"time"
)

func TestConfig_Defaults(t *testing.T) {
	config := &Config{
		Relay: RelayConfig{
			Host: "edge.2gc.ru",
			Port: 5553,
		},
		Auth: AuthConfig{
			Type:   "jwt",
			Secret: "test-secret",
		},
	}

	if config.Relay.Host != "edge.2gc.ru" {
		t.Errorf("Expected host edge.2gc.ru, got %s", config.Relay.Host)
	}

	if config.Auth.Type != "jwt" {
		t.Errorf("Expected auth type jwt, got %s", config.Auth.Type)
	}
}

func TestRelayConfig_Ports(t *testing.T) {
	ports := RelayPorts{
		HTTPAPI:      5553,
		P2PAPI:       5553,
		QUIC:         5553,
		STUN:         19302,
		MASQUE:       8443,
		EnhancedQUIC: 9092,
		TURN:         3478,
		DERP:         3479,
	}

	if ports.HTTPAPI != 5553 {
		t.Errorf("Expected HTTP API port 5553, got %d", ports.HTTPAPI)
	}

	if ports.STUN != 19302 {
		t.Errorf("Expected STUN port 19302, got %d", ports.STUN)
	}
}

func TestTLSConfig_Settings(t *testing.T) {
	tlsConfig := TLSConfig{
		Enabled:    true,
		MinVersion: "1.3",
		VerifyCert: true,
		ServerName: "b1.2gc.space",
	}

	if !tlsConfig.Enabled {
		t.Error("Expected TLS to be enabled")
	}

	if tlsConfig.MinVersion != "1.3" {
		t.Errorf("Expected TLS version 1.3, got %s", tlsConfig.MinVersion)
	}

	if !tlsConfig.VerifyCert {
		t.Error("Expected certificate verification to be enabled")
	}
}

func TestAuthConfig_JWT(t *testing.T) {
	authConfig := AuthConfig{
		Type:   "jwt",
		Secret: "my-secret",
		Token:  "my-token",
	}

	if authConfig.Type != "jwt" {
		t.Errorf("Expected auth type jwt, got %s", authConfig.Type)
	}

	if authConfig.Secret != "my-secret" {
		t.Errorf("Expected secret my-secret, got %s", authConfig.Secret)
	}
}

func TestAuthConfig_OIDC(t *testing.T) {
	oidcConfig := OIDCConfig{
		IssuerURL: "https://auth.example.com",
		Audience:  "my-client-id",
		JWKSURL:   "https://auth.example.com/jwks",
	}

	authConfig := AuthConfig{
		Type: "oidc",
		OIDC: oidcConfig,
	}

	if authConfig.Type != "oidc" {
		t.Errorf("Expected auth type oidc, got %s", authConfig.Type)
	}

	if authConfig.OIDC.IssuerURL != "https://auth.example.com" {
		t.Errorf("Expected issuer URL https://auth.example.com, got %s", authConfig.OIDC.IssuerURL)
	}
}

func TestRateLimitingConfig(t *testing.T) {
	rateLimiting := RateLimitingConfig{
		Enabled:           true,
		MaxRetries:        3,
		BackoffMultiplier: 2.0,
		MaxBackoff:        30 * time.Second,
	}

	if !rateLimiting.Enabled {
		t.Error("Expected rate limiting to be enabled")
	}

	if rateLimiting.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", rateLimiting.MaxRetries)
	}

	if rateLimiting.BackoffMultiplier != 2.0 {
		t.Errorf("Expected backoff multiplier 2.0, got %.2f", rateLimiting.BackoffMultiplier)
	}
}

func TestMetricsConfig(t *testing.T) {
	metricsConfig := MetricsConfig{
		Enabled:           true,
		PrometheusPort:    9091,
		TenantMetrics:     true,
		BufferMetrics:     true,
		ConnectionMetrics: true,
	}

	if !metricsConfig.Enabled {
		t.Error("Expected metrics to be enabled")
	}

	if metricsConfig.PrometheusPort != 9091 {
		t.Errorf("Expected Prometheus port 9091, got %d", metricsConfig.PrometheusPort)
	}
}

func TestPushgatewayConfig(t *testing.T) {
	pushConfig := PushgatewayConfig{
		Enabled:      true,
		URL:          "http://localhost:9091",
		JobName:      "cloudbridge-client",
		Instance:     "test-instance",
		PushInterval: 30 * time.Second,
	}

	if !pushConfig.Enabled {
		t.Error("Expected pushgateway to be enabled")
	}

	if pushConfig.URL != "http://localhost:9091" {
		t.Errorf("Expected URL http://localhost:9091, got %s", pushConfig.URL)
	}

	if pushConfig.PushInterval != 30*time.Second {
		t.Errorf("Expected push interval 30s, got %v", pushConfig.PushInterval)
	}
}

func TestICEConfig(t *testing.T) {
	iceConfig := ICEConfig{
		STUNServers:        []string{"edge.2gc.ru:19302"},
		TURNServers:        []string{"edge.2gc.ru:3478"},
		DERPServers:        []string{"edge.2gc.ru:3479"},
		Timeout:            30 * time.Second,
		MaxBindingRequests: 7,
		ConnectivityChecks: true,
		CandidateGathering: true,
	}

	if len(iceConfig.STUNServers) != 1 {
		t.Errorf("Expected 1 STUN server, got %d", len(iceConfig.STUNServers))
	}

	if iceConfig.MaxBindingRequests != 7 {
		t.Errorf("Expected max binding requests 7, got %d", iceConfig.MaxBindingRequests)
	}
}

func TestQUICConfig(t *testing.T) {
	quicConfig := QUICConfig{
		HandshakeTimeout:   10 * time.Second,
		IdleTimeout:        30 * time.Second,
		MaxStreams:         100,
		MaxStreamData:      1048576,
		KeepAlivePeriod:    15 * time.Second,
		InsecureSkipVerify: false,
		MASQUESupport:      true,
		HTTPDatagrams:      true,
	}

	if quicConfig.HandshakeTimeout != 10*time.Second {
		t.Errorf("Expected handshake timeout 10s, got %v", quicConfig.HandshakeTimeout)
	}

	if quicConfig.MaxStreams != 100 {
		t.Errorf("Expected max streams 100, got %d", quicConfig.MaxStreams)
	}

	if quicConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be false")
	}
}

func TestP2PConfig(t *testing.T) {
	p2pConfig := P2PConfig{
		MaxConnections:          1000,
		SessionTimeout:          300 * time.Second,
		PeerDiscoveryInterval:   30 * time.Second,
		ConnectionRetryInterval: 5 * time.Second,
		MaxRetryAttempts:        3,
		HeartbeatInterval:       30 * time.Second,
		HeartbeatTimeout:        10 * time.Second,
		FallbackEnabled:         true,
		DERPFallback:            true,
		WebSocketFallback:       true,
	}

	if p2pConfig.MaxConnections != 1000 {
		t.Errorf("Expected max connections 1000, got %d", p2pConfig.MaxConnections)
	}

	if !p2pConfig.FallbackEnabled {
		t.Error("Expected fallback to be enabled")
	}
}

func TestWireGuardConfig(t *testing.T) {
	wgConfig := WireGuardConfig{
		Enabled:             true,
		InterfaceName:       "wg-cloudbridge",
		Port:                51820,
		MTU:                 1420,
		PersistentKeepAlive: 25 * time.Second,
	}

	if !wgConfig.Enabled {
		t.Error("Expected WireGuard to be enabled")
	}

	if wgConfig.InterfaceName != "wg-cloudbridge" {
		t.Errorf("Expected interface name wg-cloudbridge, got %s", wgConfig.InterfaceName)
	}

	if wgConfig.Port != 51820 {
		t.Errorf("Expected port 51820, got %d", wgConfig.Port)
	}

	if wgConfig.MTU != 1420 {
		t.Errorf("Expected MTU 1420, got %d", wgConfig.MTU)
	}
}

func TestPlatformConstants(t *testing.T) {
	if PlatformLinux != "linux" {
		t.Errorf("Expected linux platform constant, got %s", PlatformLinux)
	}

	if PlatformWindows != "windows" {
		t.Errorf("Expected windows platform constant, got %s", PlatformWindows)
	}

	if PlatformDarwin != "darwin" {
		t.Errorf("Expected darwin platform constant, got %s", PlatformDarwin)
	}
}

func TestStatusConstants(t *testing.T) {
	if StatusInactive != "inactive" {
		t.Errorf("Expected inactive status constant, got %s", StatusInactive)
	}
}

func TestWebSocketConfig(t *testing.T) {
	wsConfig := WebSocketConfig{
		Enabled:               true,
		Endpoint:              "wss://edge.2gc.ru:5553/ws",
		Timeout:               30 * time.Second,
		PingInterval:          15 * time.Second,
		MaxReconnectAttempts:  5,
	}

	if !wsConfig.Enabled {
		t.Error("Expected WebSocket to be enabled")
	}

	if wsConfig.Endpoint != "wss://edge.2gc.ru:5553/ws" {
		t.Errorf("Expected endpoint wss://edge.2gc.ru:5553/ws, got %s", wsConfig.Endpoint)
	}
}

func TestTURNConfig(t *testing.T) {
	turnConfig := TURNConfig{
		Enabled:        true,
		Servers:        []string{"localhost:3478"},
		Username:       "cloudbridge",
		Password:       "cloudbridge123",
		Realm:          "cloudbridge.local",
		Timeout:        30 * time.Second,
		MaxAllocations: 10,
	}

	if !turnConfig.Enabled {
		t.Error("Expected TURN to be enabled")
	}

	if len(turnConfig.Servers) != 1 {
		t.Errorf("Expected 1 TURN server, got %d", len(turnConfig.Servers))
	}
}

func TestDERPConfig(t *testing.T) {
	derpConfig := DERPConfig{
		Enabled:          true,
		Servers:          []string{"localhost:3479"},
		Timeout:          30 * time.Second,
		MaxConnections:   100,
		FallbackPriority: 1,
	}

	if !derpConfig.Enabled {
		t.Error("Expected DERP to be enabled")
	}

	if derpConfig.FallbackPriority != 1 {
		t.Errorf("Expected fallback priority 1, got %d", derpConfig.FallbackPriority)
	}
}
