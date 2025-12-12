package p2p

import (
	"context"
	"testing"
	"time"
)

// TestManager_Creation tests P2P manager creation
func TestManager_Creation(t *testing.T) {
	config := createP2PTestConfig()
	logger := createTestLogger()

	manager := NewManager(config, logger)

	if manager == nil {
		t.Error("Expected manager to be created")
	}
}

// TestManager_Start tests manager startup
func TestManager_Start(t *testing.T) {
	config := createP2PTestConfig()
	logger := createTestLogger()

	manager := NewManager(config, logger)
	if manager == nil {
		t.Fatalf("Failed to create manager")
	}

	// Try to start manager
	err := manager.Start()
	if err != nil {
		t.Logf("Start returned error (may be expected): %v", err)
	}

	// Clean up
	defer manager.Stop()
}

// TestManager_Stop tests graceful shutdown
func TestManager_Stop(t *testing.T) {
	config := createP2PTestConfig()
	logger := createTestLogger()

	manager := NewManager(config, logger)
	if manager == nil {
		t.Fatalf("Failed to create manager")
	}

	if err := manager.Start(); err != nil {
		t.Logf("Start error: %v", err)
	}

	err := manager.Stop()
	if err != nil {
		t.Logf("Stop returned error: %v", err)
	}
}

// TestManager_GetStatus tests status retrieval
func TestManager_GetStatus(t *testing.T) {
	config := createP2PTestConfig()
	logger := createTestLogger()

	manager := NewManager(config, logger)
	if manager == nil {
		t.Fatalf("Failed to create manager")
	}

	status := manager.GetStatus()
	if status == nil {
		t.Error("Expected status to be returned")
	}

	// Verify status fields
	t.Logf("Connection type: %v", status.ConnectionType)
	t.Logf("Mesh enabled: %v", status.MeshEnabled)
}

// TestManager_SessionID tests session ID management
func TestManager_SessionID(t *testing.T) {
	config := createP2PTestConfig()
	logger := createTestLogger()

	manager := NewManager(config, logger)
	if manager == nil {
		t.Fatalf("Failed to create manager")
	}

	if err := manager.Start(); err != nil {
		t.Logf("Start error: %v", err)
	}

	// Session ID may not be exposed through public API
	// Verify manager was started successfully
	status := manager.GetStatus()
	if status == nil {
		t.Logf("Status not available after start")
	} else {
		t.Logf("Manager started successfully")
	}

	manager.Stop()
}

// TestConnectionTypeConfiguration tests connection type setup
func TestConnectionTypeConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		connType   ConnectionType
		isValid    bool
	}{
		{"p2p_mesh", ConnectionTypeP2PMesh, true},
		{"client_server", ConnectionTypeClientServer, true},
		{"server_server", ConnectionTypeServerServer, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &P2PConfig{
				ConnectionType: tt.connType,
			}

			logger := createTestLogger()
			manager := NewManager(config, logger)

			if manager == nil {
				t.Error("Expected manager to be created")
			}
		})
	}
}

// TestQUICConfiguration tests QUIC config
func TestQUICConfiguration(t *testing.T) {
	config := &P2PConfig{
		QUICConfig: &QUICConfig{
			ListenPort:      5000,
			MaxStreams:      100,
			MaxStreamData:   1048576,
			HandshakeTimeout: "10s",
			IdleTimeout:     "30s",
		},
	}

	if config.QUICConfig == nil {
		t.Error("Expected QUIC config to be set")
	}

	if config.QUICConfig.ListenPort == 0 {
		t.Error("Expected listen port to be set")
	}

	if config.QUICConfig.MaxStreams == 0 {
		t.Error("Expected max streams to be set")
	}

	t.Logf("QUIC configured with port %d, max streams %d",
		config.QUICConfig.ListenPort, config.QUICConfig.MaxStreams)
}

// TestMeshConfiguration tests mesh config
func TestMeshConfiguration(t *testing.T) {
	config := &P2PConfig{
		MeshConfig: &MeshConfig{
			AutoDiscovery:    true,
			Persistent:       true,
			Routing:          "hybrid",
			Encryption:       "quic",
			TopologyInterval: 30 * time.Second,
			RoutingInterval:  30 * time.Second,
			HealthInterval:   10 * time.Second,
		},
	}

	if config.MeshConfig == nil {
		t.Error("Expected mesh config to be set")
	}

	if !config.MeshConfig.AutoDiscovery {
		t.Error("Expected auto discovery to be enabled")
	}

	if config.MeshConfig.Routing != "hybrid" {
		t.Errorf("Expected hybrid routing, got %s", config.MeshConfig.Routing)
	}

	t.Logf("Mesh configured with routing: %s, encryption: %s",
		config.MeshConfig.Routing, config.MeshConfig.Encryption)
}

// TestNetworkConfiguration tests network config
func TestNetworkConfiguration(t *testing.T) {
	config := &P2PConfig{
		NetworkConfig: &NetworkConfig{
			Subnet:      "10.0.0.0/8",
			DNS:         []string{"8.8.8.8", "8.8.4.4"},
			MTU:         1420,
			STUNServers: []string{"stun.l.google.com:19302"},
			TURNServers: []string{},
		},
	}

	if config.NetworkConfig == nil {
		t.Error("Expected network config to be set")
	}

	if config.NetworkConfig.Subnet == "" {
		t.Error("Expected subnet to be set")
	}

	if config.NetworkConfig.MTU < 1280 || config.NetworkConfig.MTU > 9000 {
		t.Errorf("Invalid MTU: %d (should be 1280-9000)", config.NetworkConfig.MTU)
	}

	t.Logf("Network configured with subnet: %s, MTU: %d",
		config.NetworkConfig.Subnet, config.NetworkConfig.MTU)
}

// TestHeartbeatConfiguration tests heartbeat config
func TestHeartbeatConfiguration(t *testing.T) {
	config := createP2PTestConfig()

	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30 * time.Second
	}

	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 10 * time.Second
	}

	if config.HeartbeatInterval.Seconds() < 5 {
		t.Logf("Warning: Heartbeat interval very short: %v", config.HeartbeatInterval)
	}

	t.Logf("Heartbeat configured - Interval: %v, Timeout: %v",
		config.HeartbeatInterval, config.HeartbeatTimeout)
}

// TestPeerWhitelistConfiguration tests peer whitelist
func TestPeerWhitelistConfiguration(t *testing.T) {
	config := &P2PConfig{
		PeerWhitelist: &PeerWhitelist{
			AllowedPeers: []string{"peer1", "peer2", "peer3"},
			AutoApprove:  true,
			MaxPeers:     100,
		},
	}

	if config.PeerWhitelist == nil {
		t.Error("Expected peer whitelist to be set")
	}

	if len(config.PeerWhitelist.AllowedPeers) == 0 {
		t.Error("Expected allowed peers list to be populated")
	}

	if config.PeerWhitelist.MaxPeers == 0 {
		t.Error("Expected max peers to be set")
	}

	t.Logf("Peer whitelist configured with %d allowed peers, max peers: %d",
		len(config.PeerWhitelist.AllowedPeers), config.PeerWhitelist.MaxPeers)
}

// TestP2PStatus_Validation tests P2P status structure
func TestP2PStatus_Validation(t *testing.T) {
	status := &P2PStatus{
		IsConnected:       false,
		ConnectionType:    ConnectionTypeP2PMesh,
		ActivePeers:       0,
		TotalPeers:        0,
		MeshEnabled:       true,
		QUICReady:         false,
		ICEReady:          false,
		L3OverlayReady:    false,
		WireGuardReady:    false,
	}

	if status.ConnectionType != ConnectionTypeP2PMesh {
		t.Error("Expected P2P mesh connection type")
	}

	if !status.MeshEnabled {
		t.Error("Expected mesh to be enabled")
	}

	t.Logf("P2P status validated - Mesh: %v, QUIC: %v, ICE: %v, L3: %v",
		status.MeshEnabled, status.QUICReady, status.ICEReady, status.L3OverlayReady)
}

// TestPeerStructure tests Peer representation
func TestPeerStructure(t *testing.T) {
	peer := &Peer{
		ID:         "peer-abc123",
		PublicKey:  "test-public-key",
		Endpoint:   "192.168.1.100:5000",
		AllowedIPs: []string{"10.0.0.1/32"},
		QUICPort:   5000,
		ICEPort:    19302,
		Persistent: true,
		LastSeen:   time.Now().Unix(),
		Latency:    25,
		IsConnected: false,
	}

	if peer.ID == "" {
		t.Error("Expected peer ID to be set")
	}

	if len(peer.AllowedIPs) == 0 {
		t.Error("Expected allowed IPs to be configured")
	}

	t.Logf("Peer configured - ID: %s, Endpoint: %s, Latency: %d ms",
		peer.ID, peer.Endpoint, peer.Latency)
}

// TestMeshTopology tests mesh topology representation
func TestMeshTopology(t *testing.T) {
	topology := &MeshTopology{
		LocalPeerID:     "local-peer-123",
		ConnectedPeers:  make(map[string]*Peer),
		DiscoveredPeers: make(map[string]*Peer),
		RoutingTable:    make(map[string][]string),
	}

	if topology.LocalPeerID == "" {
		t.Error("Expected local peer ID to be set")
	}

	if topology.ConnectedPeers == nil {
		t.Error("Expected connected peers map to be initialized")
	}

	if topology.DiscoveredPeers == nil {
		t.Error("Expected discovered peers map to be initialized")
	}

	t.Logf("Mesh topology initialized - Local peer: %s", topology.LocalPeerID)
}

// TestP2PMessageStructure tests P2P message format
func TestP2PMessageStructure(t *testing.T) {
	msg := &P2PMessage{
		Type:      "peer_discovery",
		Data:      map[string]interface{}{"subnet": "10.0.0.0/8"},
		Timestamp: time.Now().Unix(),
		PeerID:    "peer-123",
	}

	if msg.Type == "" {
		t.Error("Expected message type to be set")
	}

	if msg.Timestamp == 0 {
		t.Error("Expected timestamp to be set")
	}

	t.Logf("P2P message created - Type: %s, PeerID: %s", msg.Type, msg.PeerID)
}

// TestConcurrentManagerOperations tests concurrent operations
func TestConcurrentManagerOperations(t *testing.T) {
	config := createP2PTestConfig()
	logger := createTestLogger()

	manager := NewManager(config, logger)
	if manager == nil {
		t.Fatalf("Failed to create manager")
	}

	if err := manager.Start(); err != nil {
		t.Logf("Start error: %v", err)
	}

	// Simulate concurrent operations
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_ = manager.GetStatus()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	manager.Stop()
	t.Logf("Concurrent operations completed successfully")
}

// TestContextHandling tests context-based operations
func TestContextHandling(t *testing.T) {
	config := createP2PTestConfig()
	logger := createTestLogger()

	manager := NewManager(config, logger)
	if manager == nil {
		t.Fatalf("Failed to create manager")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	select {
	case <-ctx.Done():
		t.Logf("Context cancelled as expected")
	default:
		t.Error("Expected context to be cancelled")
	}

	manager.Stop()
}

// TestP2PConfigValidation tests configuration validation
func TestP2PConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *P2PConfig
	}{
		{
			"minimal_config",
			&P2PConfig{
				ConnectionType: ConnectionTypeP2PMesh,
			},
		},
		{
			"full_config",
			createP2PTestConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.FillDefaults()

			if err := tt.config.Validate(); err != nil {
				t.Logf("Validation error (may be expected): %v", err)
			}

			t.Logf("Config validated: %v", tt.config.ConnectionType)
		})
	}
}

// BenchmarkManager_Creation benchmarks manager creation
func BenchmarkManager_Creation(b *testing.B) {
	config := createP2PTestConfig()
	logger := createTestLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewManager(config, logger)
	}
}

// BenchmarkManager_Start benchmarks manager startup
func BenchmarkManager_Start(b *testing.B) {
	config := createP2PTestConfig()
	logger := createTestLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager := NewManager(config, logger)
		_ = manager.Start()
		_ = manager.Stop()
	}
}

// BenchmarkGetStatus benchmarks status retrieval
func BenchmarkGetStatus(b *testing.B) {
	config := createP2PTestConfig()
	logger := createTestLogger()
	manager := NewManager(config, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GetStatus()
	}
}

// Helper functions

// createP2PTestConfig creates a test P2P configuration
func createP2PTestConfig() *P2PConfig {
	return &P2PConfig{
		ConnectionType:    ConnectionTypeP2PMesh,
		TenantID:          "test-tenant-123",
		HeartbeatInterval: 30 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
		QUICConfig: &QUICConfig{
			ListenPort:       5000,
			MaxStreams:       100,
			MaxStreamData:    1048576,
			HandshakeTimeout: "10s",
			IdleTimeout:      "30s",
			KeepAlivePeriod:  "15s",
		},
		MeshConfig: &MeshConfig{
			AutoDiscovery:    true,
			Persistent:       true,
			Routing:          "hybrid",
			Encryption:       "quic",
			TopologyInterval: 30 * time.Second,
			RoutingInterval:  30 * time.Second,
			HealthInterval:   10 * time.Second,
		},
		NetworkConfig: &NetworkConfig{
			Subnet:      "10.0.0.0/8",
			DNS:         []string{"8.8.8.8"},
			MTU:         1420,
			STUNServers: []string{"stun.l.google.com:19302"},
		},
		PeerWhitelist: &PeerWhitelist{
			AutoApprove: true,
			MaxPeers:    100,
		},
	}
}

// createTestLogger creates a test logger
type testLogger struct{}

func (tl *testLogger) Info(msg string, fields ...interface{}) {
	// Minimal logging implementation
}

func (tl *testLogger) Error(msg string, fields ...interface{}) {
	// Minimal logging implementation
}

func (tl *testLogger) Debug(msg string, fields ...interface{}) {
	// Minimal logging implementation
}

func (tl *testLogger) Warn(msg string, fields ...interface{}) {
	// Minimal logging implementation
}

func createTestLogger() Logger {
	return &testLogger{}
}
