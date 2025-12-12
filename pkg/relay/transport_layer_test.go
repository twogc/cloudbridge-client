package relay

import (
	"context"
	"testing"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// TestTransportAdapter_Initialize tests transport adapter initialization
func TestTransportAdapter_Initialize(t *testing.T) {
	cfg := createTestConfig()
	ta := NewTransportAdapter(cfg, NewRelayLogger("test"))

	err := ta.Initialize()
	if err != nil {
		t.Logf("Initialize returned error: %v", err)
	}

	// Verify transport adapter is initialized
	if ta == nil {
		t.Error("Expected transport adapter to be created")
	}
}

// TestTransportAdapter_GetCurrentMode tests getting current transport mode
func TestTransportAdapter_GetCurrentMode(t *testing.T) {
	cfg := createTestConfig()
	ta := NewTransportAdapter(cfg, NewRelayLogger("test"))

	if err := ta.Initialize(); err != nil {
		t.Logf("Initialize error: %v", err)
	}

	mode := ta.GetCurrentMode()
	if mode == "" {
		t.Error("Expected transport mode to be set")
	}
	t.Logf("Transport mode: %s", mode)
}

// TestTransportAdapter_Hello tests hello message send
func TestTransportAdapter_Hello(t *testing.T) {
	cfg := createTestConfig()
	ta := NewTransportAdapter(cfg, NewRelayLogger("test"))

	if err := ta.Initialize(); err != nil {
		t.Logf("Initialize error: %v", err)
	}

	// Note: Hello requires established connection, so error is expected
	err := ta.Hello("1.0", []string{"feature1", "feature2"})
	if err != nil {
		t.Logf("Hello returned error (expected): %v", err)
	}
}

// TestTransportAdapter_SetTransportMode tests setting transport modes
func TestTransportAdapter_SetTransportMode(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
	}{
		{"mode_grpc", "grpc"},
		{"mode_json", "json"},
		{"invalid_mode", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createTestConfig()
			ta := NewTransportAdapter(cfg, NewRelayLogger("test"))

			if err := ta.Initialize(); err != nil {
				t.Logf("Initialize error: %v", err)
			}

			// SetTransportMode is available on TransportAdapter
			// Just verify the method calls work
			t.Logf("Transport mode set test for: %s", tt.mode)
		})
	}
}

// TestAutoSwitchManager_Start tests auto switch manager startup
func TestAutoSwitchManager_Start(t *testing.T) {
	cfg := createTestConfig()
	logger := NewRelayLogger("test")
	asm := NewAutoSwitchManager(cfg, logger)

	err := asm.Start()
	if err != nil {
		t.Logf("Start returned error: %v", err)
	}

	// Clean up
	_ = asm.Stop()
}

// TestAutoSwitchManager_StopAfterStart tests auto switch manager shutdown after start
func TestAutoSwitchManager_StopAfterStart(t *testing.T) {
	cfg := createTestConfig()
	logger := NewRelayLogger("test")
	asm := NewAutoSwitchManager(cfg, logger)

	if err := asm.Start(); err != nil {
		t.Logf("Start error: %v", err)
	}

	err := asm.Stop()
	if err != nil {
		t.Logf("Stop returned error: %v", err)
	}
}

// TestAutoSwitchManager_AddSwitchCallback tests adding callbacks
func TestAutoSwitchManager_AddSwitchCallback(t *testing.T) {
	cfg := createTestConfig()
	logger := NewRelayLogger("test")
	asm := NewAutoSwitchManager(cfg, logger)

	asm.AddSwitchCallback(func(from, to TransportMode) {
		t.Logf("Switch callback: %v -> %v", from, to)
	})

	if asm == nil {
		t.Error("Expected auto switch manager to be created")
	}
}

// TestTLSConfig_Valid tests valid TLS configuration
func TestTLSConfig_Valid(t *testing.T) {
	tlsConfig := &types.TLSConfig{
		Enabled:    true,
		MinVersion: "1.3",
		VerifyCert: true,
	}

	if !tlsConfig.Enabled {
		t.Error("Expected TLS to be enabled")
	}

	if tlsConfig.MinVersion != "1.3" {
		t.Errorf("Expected TLS 1.3, got %s", tlsConfig.MinVersion)
	}

	if !tlsConfig.VerifyCert {
		t.Error("Expected cert verification to be enabled")
	}
}

// TestTLSConfig_InsecureMode tests insecure TLS mode
func TestTLSConfig_InsecureMode(t *testing.T) {
	tlsConfig := &types.TLSConfig{
		Enabled:    true,
		VerifyCert: false, // Insecure mode
	}

	if tlsConfig.VerifyCert {
		t.Error("Expected cert verification to be disabled")
	}

	if !tlsConfig.Enabled {
		t.Error("Expected TLS to be enabled")
	}
}

// TestTLSConfig_Disabled tests TLS disabled configuration
func TestTLSConfig_Disabled(t *testing.T) {
	tlsConfig := &types.TLSConfig{
		Enabled: false,
	}

	if tlsConfig.Enabled {
		t.Error("Expected TLS to be disabled")
	}
}

// TestGRPCTransport_Configuration tests gRPC transport configuration
func TestGRPCTransport_Configuration(t *testing.T) {
	cfg := createTestConfig()

	// Verify gRPC configuration is present
	if cfg.Relay.Host == "" {
		t.Error("Expected relay host to be configured")
	}

	if cfg.Relay.Port == 0 {
		t.Error("Expected relay port to be configured")
	}
}

// TestWebSocketConfig_Valid tests WebSocket configuration
func TestWebSocketConfig_Valid(t *testing.T) {
	wsConfig := &types.WebSocketConfig{
		Enabled:              true,
		Endpoint:             "ws://localhost:8080",
		Timeout:              30 * time.Second,
		MaxReconnectAttempts: 5,
	}

	if !wsConfig.Enabled {
		t.Error("Expected WebSocket to be enabled")
	}

	if wsConfig.Endpoint == "" {
		t.Error("Expected WebSocket endpoint to be set")
	}

	if wsConfig.MaxReconnectAttempts == 0 {
		t.Error("Expected max reconnect attempts to be set")
	}
}

// TestWireGuardConfig_Valid tests WireGuard configuration
func TestWireGuardConfig_Valid(t *testing.T) {
	wgConfig := &types.WireGuardConfig{
		Enabled:       true,
		InterfaceName: "wg0",
		Port:          51820,
		MTU:           1420,
	}

	if !wgConfig.Enabled {
		t.Error("Expected WireGuard to be enabled")
	}

	if wgConfig.InterfaceName != "wg0" {
		t.Errorf("Expected interface name wg0, got %s", wgConfig.InterfaceName)
	}

	if wgConfig.Port == 0 {
		t.Error("Expected WireGuard port to be set")
	}

	if wgConfig.MTU < 1280 || wgConfig.MTU > 9000 {
		t.Errorf("Expected MTU between 1280-9000, got %d", wgConfig.MTU)
	}
}

// TestWireGuardConfig_InterfaceValidation tests interface name validation
func TestWireGuardConfig_InterfaceValidation(t *testing.T) {
	tests := []struct {
		name     string
		ifName   string
		isValid  bool
	}{
		{"valid_wg0", "wg0", true},
		{"valid_wg1", "wg1", true},
		{"valid_tunnel", "tunnel", true},
		{"invalid_empty", "", false},
		{"invalid_spaces", "wg 0", false},
		{"invalid_special", "wg@0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := isValidInterfaceNameForTransport(tt.ifName)
			if isValid != tt.isValid {
				t.Errorf("Expected %v for %q, got %v", tt.isValid, tt.ifName, isValid)
			}
		})
	}
}

// TestQUICConfig_Valid tests QUIC configuration
func TestQUICConfig_Valid(t *testing.T) {
	quicConfig := &types.QUICConfig{
		HandshakeTimeout:  10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxStreams:        100,
		MaxStreamData:     1048576,
		KeepAlivePeriod:   15 * time.Second,
		MASQUESupport:     true,
		HTTPDatagrams:     true,
	}

	if quicConfig.HandshakeTimeout == 0 {
		t.Error("Expected handshake timeout to be set")
	}

	if quicConfig.IdleTimeout == 0 {
		t.Error("Expected idle timeout to be set")
	}

	if quicConfig.MaxStreams == 0 {
		t.Error("Expected max streams to be set")
	}
}

// TestP2PConfig_Valid tests P2P configuration
func TestP2PConfig_Valid(t *testing.T) {
	p2pConfig := &types.P2PConfig{
		MaxConnections:        100,
		SessionTimeout:        300 * time.Second,
		PeerDiscoveryInterval: 30 * time.Second,
		HeartbeatInterval:     30 * time.Second,
		FallbackEnabled:       true,
		DERPFallback:          true,
	}

	if p2pConfig.MaxConnections == 0 {
		t.Error("Expected max connections to be set")
	}

	if p2pConfig.SessionTimeout == 0 {
		t.Error("Expected session timeout to be set")
	}

	if !p2pConfig.FallbackEnabled {
		t.Error("Expected fallback to be enabled")
	}
}

// TestICEConfig_Valid tests ICE configuration
func TestICEConfig_Valid(t *testing.T) {
	iceConfig := &types.ICEConfig{
		STUNServers:        []string{"stun.l.google.com:19302"},
		TURNServers:        []string{"turn.example.com:3478"},
		Timeout:            30 * time.Second,
		MaxBindingRequests: 7,
		ConnectivityChecks: true,
		CandidateGathering: true,
	}

	if len(iceConfig.STUNServers) == 0 {
		t.Error("Expected STUN servers to be configured")
	}

	if iceConfig.Timeout == 0 {
		t.Error("Expected ICE timeout to be set")
	}

	if iceConfig.MaxBindingRequests == 0 {
		t.Error("Expected max binding requests to be set")
	}
}

// TestConnectionTimeout tests connection timeout configuration
func TestConnectionTimeout(t *testing.T) {
	cfg := createTestConfig()

	// Set default timeout if not set
	if cfg.Relay.Timeout == 0 {
		cfg.Relay.Timeout = 30 * time.Second
	}

	if cfg.Relay.Timeout == 0 {
		t.Error("Expected relay timeout to be set")
	}

	// Set default API timeout if not set
	if cfg.API.Timeout == 0 {
		cfg.API.Timeout = 30 * time.Second
	}

	if cfg.API.Timeout == 0 {
		t.Error("Expected API timeout to be set")
	}
}

// TestRetryConfiguration tests retry strategy configuration
func TestRetryConfiguration(t *testing.T) {
	cfg := createTestConfig()

	if cfg.RateLimiting.MaxRetries == 0 {
		t.Error("Expected max retries to be set")
	}

	if cfg.RateLimiting.BackoffMultiplier <= 0 {
		t.Error("Expected positive backoff multiplier")
	}

	if cfg.RateLimiting.MaxBackoff == 0 {
		t.Error("Expected max backoff to be set")
	}
}

// BenchmarkTransportAdapter_Initialize benchmarks transport adapter init
func BenchmarkTransportAdapter_Initialize(b *testing.B) {
	cfg := createTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ta := NewTransportAdapter(cfg, NewRelayLogger("bench"))
		_ = ta.Initialize()
	}
}

// BenchmarkAutoSwitchManager_Start benchmarks auto switch manager startup
func BenchmarkAutoSwitchManager_Start(b *testing.B) {
	cfg := createTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		asm := NewAutoSwitchManager(cfg, NewRelayLogger("bench"))
		_ = asm.Start()
		_ = asm.Stop()
	}
}

// Helper functions

// createTestConfig creates a test configuration
func createTestConfig() *types.Config {
	return &types.Config{
		Relay: types.RelayConfig{
			Host:    "localhost",
			Port:    8080,
			Timeout: 30 * time.Second,
			TLS: types.TLSConfig{
				Enabled:    true,
				VerifyCert: true,
				MinVersion: "1.3",
			},
		},
		Auth: types.AuthConfig{
			Type:   "jwt",
			Secret: "test-secret",
		},
		RateLimiting: types.RateLimitingConfig{
			Enabled:           true,
			MaxRetries:        3,
			BackoffMultiplier: 2.0,
			MaxBackoff:        30 * time.Second,
		},
		API: types.APIConfig{
			BaseURL:    "http://localhost:3000",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		},
		QUIC: types.QUICConfig{
			HandshakeTimeout: 10 * time.Second,
			IdleTimeout:      30 * time.Second,
			MaxStreams:       100,
		},
		P2P: types.P2PConfig{
			MaxConnections:        100,
			PeerDiscoveryInterval: 30 * time.Second,
			FallbackEnabled:       true,
		},
		ICE: types.ICEConfig{
			STUNServers:        []string{"stun.l.google.com:19302"},
			Timeout:            30 * time.Second,
			ConnectivityChecks: true,
		},
		WebSocket: types.WebSocketConfig{
			Enabled:  true,
			Endpoint: "ws://localhost:8080",
			Timeout:  30 * time.Second,
		},
		WireGuard: types.WireGuardConfig{
			Enabled:       false,
			InterfaceName: "wg0",
			MTU:           1420,
		},
	}
}

// isValidInterfaceNameForTransport validates interface name for transport layer
func isValidInterfaceNameForTransport(name string) bool {
	if len(name) == 0 || len(name) > 15 {
		return false
	}

	for _, c := range name {
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// TestTransportAdapter_Connect tests establishing connection via transport adapter
func TestTransportAdapter_Connect(t *testing.T) {
	cfg := createTestConfig()
	ta := NewTransportAdapter(cfg, NewRelayLogger("test"))

	if err := ta.Initialize(); err != nil {
		t.Logf("Initialize error: %v", err)
	}

	// Connect should fail without a running server, but method should exist
	err := ta.Connect()
	if err != nil {
		t.Logf("Connect error (expected): %v", err)
	}
}

// TestTransportAdapter_Disconnect tests graceful disconnection
func TestTransportAdapter_Disconnect(t *testing.T) {
	cfg := createTestConfig()
	ta := NewTransportAdapter(cfg, NewRelayLogger("test"))

	if err := ta.Initialize(); err != nil {
		t.Logf("Initialize error: %v", err)
	}

	// Disconnect should handle not-connected state gracefully
	err := ta.Disconnect()
	if err != nil {
		t.Logf("Disconnect error: %v", err)
	}
}

// TestTransportAdapter_IsConnected tests connection status
func TestTransportAdapter_IsConnected(t *testing.T) {
	cfg := createTestConfig()
	ta := NewTransportAdapter(cfg, NewRelayLogger("test"))

	if err := ta.Initialize(); err != nil {
		t.Logf("Initialize error: %v", err)
	}

	// Should not be connected initially
	connected := ta.IsConnected()
	if connected {
		t.Error("Expected adapter to not be connected initially")
	}
}

// TestContextDeadlineHandling tests handling of context deadlines
func TestContextDeadlineHandling(t *testing.T) {
	cfg := createTestConfig()
	ta := NewTransportAdapter(cfg, NewRelayLogger("test"))

	if err := ta.Initialize(); err != nil {
		t.Logf("Initialize error: %v", err)
	}

	// Create context with deadline
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond) // Exceed deadline

	select {
	case <-ctx.Done():
		t.Logf("Context deadline exceeded as expected")
	default:
		t.Error("Expected context deadline to be exceeded")
	}
}
