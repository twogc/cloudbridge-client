package heartbeat

import (
	"runtime"
	"testing"
	"time"
)

func TestPayloadBuilder_Basic(t *testing.T) {
	builder := NewPayloadBuilder("server-001", "tenant-123")

	payload := builder.
		WithVersion("1.0.0", "2025-10-21").
		WithHostname("test-host").
		WithUptime().
		Build()

	if payload.ServerID != "server-001" {
		t.Errorf("Expected ServerID 'server-001', got '%s'", payload.ServerID)
	}

	if payload.TenantID != "tenant-123" {
		t.Errorf("Expected TenantID 'tenant-123', got '%s'", payload.TenantID)
	}

	if payload.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got '%s'", payload.Version)
	}

	if payload.Hostname != "test-host" {
		t.Errorf("Expected Hostname 'test-host', got '%s'", payload.Hostname)
	}

	if payload.OS != runtime.GOOS {
		t.Errorf("Expected OS '%s', got '%s'", runtime.GOOS, payload.OS)
	}

	if payload.Arch != runtime.GOARCH {
		t.Errorf("Expected Arch '%s', got '%s'", runtime.GOARCH, payload.Arch)
	}

	if payload.HeartbeatSeqNum != 1 {
		t.Errorf("Expected SeqNum 1, got %d", payload.HeartbeatSeqNum)
	}
}

func TestPayloadBuilder_NetworkQuality(t *testing.T) {
	tests := []struct {
		name          string
		avgRTT        float64
		packetLoss    float64
		expectedQual  string
	}{
		{"Excellent", 30.0, 0.1, "excellent"},
		{"Good", 80.0, 0.8, "good"},
		{"Fair", 150.0, 1.5, "fair"},
		{"Poor", 250.0, 3.0, "poor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewPayloadBuilder("test", "test")
			payload := builder.
				WithNetworkQuality(tt.avgRTT, tt.avgRTT*1.2, tt.packetLoss, 5.0).
				Build()

			if payload.ConnectionQuality != tt.expectedQual {
				t.Errorf("Expected quality '%s', got '%s'", tt.expectedQual, payload.ConnectionQuality)
			}

			if payload.AverageRTT != tt.avgRTT {
				t.Errorf("Expected avgRTT %.2f, got %.2f", tt.avgRTT, payload.AverageRTT)
			}

			if payload.PacketLossPercent != tt.packetLoss {
				t.Errorf("Expected packetLoss %.2f, got %.2f", tt.packetLoss, payload.PacketLossPercent)
			}
		})
	}
}

func TestPayloadBuilder_TrafficStats(t *testing.T) {
	builder := NewPayloadBuilder("test", "test")

	// Simulate some uptime - need to set startTime in the past
	time.Sleep(100 * time.Millisecond)

	payload := builder.
		WithUptime().                                  // This must be called BEFORE WithTrafficStats
		WithTrafficStats(1000000, 2000000, 5000, 10000).
		Build()

	if payload.BytesSent != 1000000 {
		t.Errorf("Expected BytesSent 1000000, got %d", payload.BytesSent)
	}

	if payload.BytesReceived != 2000000 {
		t.Errorf("Expected BytesReceived 2000000, got %d", payload.BytesReceived)
	}

	if payload.PacketsSent != 5000 {
		t.Errorf("Expected PacketsSent 5000, got %d", payload.PacketsSent)
	}

	if payload.PacketsReceived != 10000 {
		t.Errorf("Expected PacketsReceived 10000, got %d", payload.PacketsReceived)
	}

	// Throughput should be calculated based on uptime
	// With ~0.1 seconds uptime and 3MB total data, throughput should be ~240 Mbps
	if payload.ThroughputMbps <= 0 {
		t.Errorf("Expected positive throughput, got %.2f", payload.ThroughputMbps)
	}

	t.Logf("Calculated throughput: %.2f Mbps (uptime: %d seconds)", payload.ThroughputMbps, payload.UptimeSeconds)
}

func TestPayloadBuilder_Features(t *testing.T) {
	builder := NewPayloadBuilder("test", "test")

	payload := builder.
		WithFeatures(true, true, false, true, true).
		Build()

	if !payload.Features.P2PEnabled {
		t.Error("Expected P2P enabled")
	}

	if !payload.Features.WireGuardEnabled {
		t.Error("Expected WireGuard enabled")
	}

	if payload.Features.QUICEnabled {
		t.Error("Expected QUIC disabled")
	}

	if !payload.Features.WebSocketEnabled {
		t.Error("Expected WebSocket enabled")
	}
}

func TestPayloadBuilder_SequenceNumber(t *testing.T) {
	builder := NewPayloadBuilder("test", "test")

	payload1 := builder.Build()
	payload2 := builder.Build()
	payload3 := builder.Build()

	if payload1.HeartbeatSeqNum != 1 {
		t.Errorf("Expected SeqNum 1, got %d", payload1.HeartbeatSeqNum)
	}

	if payload2.HeartbeatSeqNum != 2 {
		t.Errorf("Expected SeqNum 2, got %d", payload2.HeartbeatSeqNum)
	}

	if payload3.HeartbeatSeqNum != 3 {
		t.Errorf("Expected SeqNum 3, got %d", payload3.HeartbeatSeqNum)
	}
}

func TestErrorTracker_Basic(t *testing.T) {
	tracker := NewErrorTracker()

	tracker.AddError("connection", "Failed to connect to relay")
	tracker.AddError("auth", "Authentication failed")
	tracker.AddError("connection", "Failed to connect to relay") // Duplicate

	errors := tracker.GetRecentErrors(10)

	if len(errors) != 2 {
		t.Errorf("Expected 2 unique errors, got %d", len(errors))
	}

	// Find connection error
	var connErr *ErrorInfo
	for i := range errors {
		if errors[i].Type == "connection" {
			connErr = &errors[i]
			break
		}
	}

	if connErr == nil {
		t.Fatal("Connection error not found")
	}

	if connErr.Count != 2 {
		t.Errorf("Expected connection error count 2, got %d", connErr.Count)
	}
}

func TestErrorTracker_RecentErrors(t *testing.T) {
	tracker := NewErrorTracker()

	// Add multiple errors
	for i := 0; i < 10; i++ {
		tracker.AddError("test", "Error "+string(rune('A'+i)))
		time.Sleep(1 * time.Millisecond) // Small delay to ensure different timestamps
	}

	// Get only 5 most recent
	recent := tracker.GetRecentErrors(5)

	if len(recent) != 5 {
		t.Errorf("Expected 5 recent errors, got %d", len(recent))
	}

	// Should be in reverse chronological order (most recent first)
	// Most recent should be "Error J"
	if recent[0].Message != "Error J" {
		t.Errorf("Expected most recent error 'Error J', got '%s'", recent[0].Message)
	}
}

func TestErrorTracker_CountLastMinute(t *testing.T) {
	tracker := NewErrorTracker()

	// Add recent errors
	for i := 0; i < 5; i++ {
		tracker.AddError("recent", "Recent error")
	}

	// Add old error (simulated by manipulating timestamp - in real code this would be old)
	// For this test, all errors are recent
	count := tracker.GetErrorCountLastMinute()

	if count != 5 {
		t.Errorf("Expected error count 5, got %d", count)
	}
}

func TestPayloadBuilder_WithErrors(t *testing.T) {
	builder := NewPayloadBuilder("test", "test")

	payload := builder.
		AddError("connection", "Connection timeout").
		AddError("auth", "Invalid token").
		AddError("connection", "Connection timeout"). // Duplicate
		Build()

	if len(payload.RecentErrors) != 2 {
		t.Errorf("Expected 2 unique errors, got %d", len(payload.RecentErrors))
	}

	if payload.ErrorCount != 3 {
		t.Errorf("Expected error count 3, got %d", payload.ErrorCount)
	}
}

func TestPayloadBuilder_HealthStatus(t *testing.T) {
	builder := NewPayloadBuilder("test", "test")

	// Default should be healthy
	payload := builder.Build()
	if payload.HealthStatus != "healthy" {
		t.Errorf("Expected default health status 'healthy', got '%s'", payload.HealthStatus)
	}

	// Can be changed
	payload = builder.WithHealthStatus("degraded").Build()
	if payload.HealthStatus != "degraded" {
		t.Errorf("Expected health status 'degraded', got '%s'", payload.HealthStatus)
	}
}

func TestPayloadBuilder_SystemStats(t *testing.T) {
	builder := NewPayloadBuilder("test", "test")

	payload := builder.
		WithSystemStats(45.5, 512, 2048).
		WithConnectionCount(42).
		Build()

	if payload.CPUUsagePercent != 45.5 {
		t.Errorf("Expected CPU 45.5%%, got %.2f%%", payload.CPUUsagePercent)
	}

	if payload.MemoryUsedMB != 512 {
		t.Errorf("Expected MemoryUsed 512MB, got %d", payload.MemoryUsedMB)
	}

	if payload.MemoryTotalMB != 2048 {
		t.Errorf("Expected MemoryTotal 2048MB, got %d", payload.MemoryTotalMB)
	}

	if payload.ConnectionsActive != 42 {
		t.Errorf("Expected 42 active connections, got %d", payload.ConnectionsActive)
	}

	if payload.GoRoutines <= 0 {
		t.Error("Expected positive goroutine count")
	}
}
