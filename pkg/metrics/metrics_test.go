package metrics

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	metrics := NewMetrics(true, 9091)

	if !metrics.enabled {
		t.Error("Expected metrics to be enabled")
	}

	if metrics.port != 9091 {
		t.Errorf("Expected port 9091, got %d", metrics.port)
	}

	if metrics.registry == nil {
		t.Error("Expected registry to be initialized")
	}
}

func TestNewMetrics_Disabled(t *testing.T) {
	metrics := NewMetrics(false, 9091)

	if metrics.enabled {
		t.Error("Expected metrics to be disabled")
	}
}

func TestNewMetricsWithPushgateway(t *testing.T) {
	pushConfig := &PushgatewayConfig{
		Enabled:      true,
		URL:          "http://localhost:9091",
		JobName:      "test-job",
		Instance:     "test-instance",
		PushInterval: time.Second * 30,
	}

	metrics := NewMetricsWithPushgateway(true, 9091, pushConfig)

	if !metrics.enabled {
		t.Error("Expected metrics to be enabled")
	}

	if metrics.pushgatewayConfig == nil {
		t.Error("Expected pushgateway config to be set")
	}

	if metrics.pushgatewayConfig.URL != pushConfig.URL {
		t.Errorf("Expected URL %s, got %s", pushConfig.URL, metrics.pushgatewayConfig.URL)
	}

	// Clean up
	if metrics.pushCancel != nil {
		metrics.pushCancel()
	}
}

func TestMetrics_Start(t *testing.T) {
	metrics := NewMetrics(true, 19091) // Use non-standard port to avoid conflicts

	err := metrics.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Try to access metrics endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", 19091))
	if err != nil {
		t.Fatalf("Failed to access metrics endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Clean up
	if err := metrics.Stop(); err != nil {
		t.Errorf("Failed to stop metrics server: %v", err)
	}
}

func TestMetrics_Start_AlreadyRunning(t *testing.T) {
	metrics := NewMetrics(true, 19092)

	err := metrics.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}
	defer metrics.Stop()

	// Try to start again
	err = metrics.Start()
	if err == nil {
		t.Error("Expected error when starting already running server")
	}
}

func TestMetrics_Start_Disabled(t *testing.T) {
	metrics := NewMetrics(false, 9091)

	err := metrics.Start()
	if err != nil {
		t.Errorf("Expected no error when starting disabled metrics, got: %v", err)
	}
}

func TestMetrics_Stop(t *testing.T) {
	metrics := NewMetrics(true, 19093)

	err := metrics.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	err = metrics.Stop()
	if err != nil {
		t.Errorf("Failed to stop metrics server: %v", err)
	}

	// Verify server is stopped
	time.Sleep(100 * time.Millisecond)
	_, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", 19093))
	if err == nil {
		t.Error("Expected error accessing stopped server")
	}
}

func TestMetrics_RecordBytesTransferred(t *testing.T) {
	metrics := NewMetrics(true, 19094)

	metrics.RecordBytesTransferred("tenant-1", "sent", 1024)
	metrics.RecordBytesTransferred("tenant-1", "recv", 2048)

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.RecordBytesTransferred("tenant-1", "sent", 1024) // Should not panic
}

func TestMetrics_RecordConnectionHandled(t *testing.T) {
	metrics := NewMetrics(true, 19095)

	metrics.RecordConnectionHandled("tenant-1")
	metrics.RecordConnectionHandled("tenant-2")

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.RecordConnectionHandled("tenant-1") // Should not panic
}

func TestMetrics_SetActiveConnections(t *testing.T) {
	metrics := NewMetrics(true, 19096)

	metrics.SetActiveConnections("tenant-1", 10)
	metrics.SetActiveConnections("tenant-1", 5)

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.SetActiveConnections("tenant-1", 10) // Should not panic
}

func TestMetrics_RecordConnectionDuration(t *testing.T) {
	metrics := NewMetrics(true, 19097)

	metrics.RecordConnectionDuration("tenant-1", time.Second*5)
	metrics.RecordConnectionDuration("tenant-2", time.Millisecond*500)

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.RecordConnectionDuration("tenant-1", time.Second) // Should not panic
}

func TestMetrics_SetBufferPoolSize(t *testing.T) {
	metrics := NewMetrics(true, 19098)

	metrics.SetBufferPoolSize(100)
	metrics.SetBufferPoolSize(200)

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.SetBufferPoolSize(100) // Should not panic
}

func TestMetrics_SetBufferPoolUsage(t *testing.T) {
	metrics := NewMetrics(true, 19099)

	metrics.SetBufferPoolUsage(50)
	metrics.SetBufferPoolUsage(75)

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.SetBufferPoolUsage(50) // Should not panic
}

func TestMetrics_RecordError(t *testing.T) {
	metrics := NewMetrics(true, 19100)

	metrics.RecordError("connection_error", "tenant-1")
	metrics.RecordError("auth_error", "tenant-2")

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.RecordError("error", "tenant-1") // Should not panic
}

func TestMetrics_RecordHeartbeatLatency(t *testing.T) {
	metrics := NewMetrics(true, 19101)

	metrics.RecordHeartbeatLatency("tenant-1", time.Millisecond*50)
	metrics.RecordHeartbeatLatency("tenant-2", time.Millisecond*100)

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.RecordHeartbeatLatency("tenant-1", time.Millisecond*50) // Should not panic
}

func TestMetrics_RecordClientBytes(t *testing.T) {
	metrics := NewMetrics(true, 19102)

	metrics.RecordClientBytesSent(1024)
	metrics.RecordClientBytesRecv(2048)

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.RecordClientBytesSent(1024) // Should not panic
	disabledMetrics.RecordClientBytesRecv(2048) // Should not panic
}

func TestMetrics_SetP2PSessions(t *testing.T) {
	metrics := NewMetrics(true, 19103)

	metrics.SetP2PSessions(5)
	metrics.SetP2PSessions(10)

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.SetP2PSessions(5) // Should not panic
}

func TestMetrics_SetTransportMode(t *testing.T) {
	metrics := NewMetrics(true, 19104)

	metrics.SetTransportMode(0) // QUIC
	metrics.SetTransportMode(1) // WireGuard
	metrics.SetTransportMode(2) // gRPC

	// Test with disabled metrics
	disabledMetrics := NewMetrics(false, 9091)
	disabledMetrics.SetTransportMode(0) // Should not panic
}

func TestMetrics_GetMetrics(t *testing.T) {
	metrics := NewMetrics(true, 19105)

	result := metrics.GetMetrics()

	if !result["enabled"].(bool) {
		t.Error("Expected enabled to be true")
	}

	if result["port"].(int) != 19105 {
		t.Errorf("Expected port 19105, got %v", result["port"])
	}
}

func TestMetrics_GetMetrics_Disabled(t *testing.T) {
	metrics := NewMetrics(false, 9091)

	result := metrics.GetMetrics()

	if result["enabled"].(bool) {
		t.Error("Expected enabled to be false")
	}
}

func TestMetrics_GetMetrics_WithPushgateway(t *testing.T) {
	pushConfig := &PushgatewayConfig{
		Enabled:      true,
		URL:          "http://localhost:9091",
		JobName:      "test-job",
		Instance:     "test-instance",
		PushInterval: time.Second * 30,
	}

	metrics := NewMetricsWithPushgateway(true, 19106, pushConfig)
	defer func() {
		if metrics.pushCancel != nil {
			metrics.pushCancel()
		}
	}()

	result := metrics.GetMetrics()

	pgMap, ok := result["pushgateway"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected pushgateway config in result")
	}

	if pgMap["url"] != pushConfig.URL {
		t.Errorf("Expected URL %s, got %v", pushConfig.URL, pgMap["url"])
	}

	if pgMap["job_name"] != pushConfig.JobName {
		t.Errorf("Expected job name %s, got %v", pushConfig.JobName, pgMap["job_name"])
	}
}

func TestMetrics_ForcePush_Disabled(t *testing.T) {
	metrics := NewMetrics(true, 19107)

	err := metrics.ForcePush()
	if err == nil {
		t.Error("Expected error when pushing without pushgateway configured")
	}
}

func TestMetrics_GetPushgatewayConfig(t *testing.T) {
	pushConfig := &PushgatewayConfig{
		Enabled:      true,
		URL:          "http://localhost:9091",
		JobName:      "test-job",
		Instance:     "test-instance",
		PushInterval: time.Second * 30,
	}

	metrics := NewMetricsWithPushgateway(true, 19108, pushConfig)
	defer func() {
		if metrics.pushCancel != nil {
			metrics.pushCancel()
		}
	}()

	config := metrics.GetPushgatewayConfig()

	if config == nil {
		t.Fatal("Expected non-nil pushgateway config")
	}

	if config.URL != pushConfig.URL {
		t.Errorf("Expected URL %s, got %s", pushConfig.URL, config.URL)
	}
}

func TestMetrics_PrometheusEndpoint(t *testing.T) {
	metrics := NewMetrics(true, 19109)

	err := metrics.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}
	defer metrics.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Record some metrics
	metrics.RecordClientBytesSent(1024)
	metrics.RecordClientBytesRecv(2048)
	metrics.SetP2PSessions(5)
	metrics.SetTransportMode(2)

	// Fetch metrics
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", 19109))
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check for expected metrics in output
	bodyStr := string(body)
	expectedMetrics := []string{
		"cloudbridge_client_bytes_total",
		"p2p_sessions",
		"transport_mode",
	}

	for _, metric := range expectedMetrics {
		if !contains(bodyStr, metric) {
			t.Errorf("Expected metric %s not found in output", metric)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 1; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	metrics := NewMetrics(true, 19110)

	// Test concurrent access to metrics
	done := make(chan bool)
	iterations := 100

	// Goroutine 1: Record bytes
	go func() {
		for i := 0; i < iterations; i++ {
			metrics.RecordClientBytesSent(int64(i))
		}
		done <- true
	}()

	// Goroutine 2: Record connections
	go func() {
		for i := 0; i < iterations; i++ {
			metrics.RecordConnectionHandled("tenant-1")
		}
		done <- true
	}()

	// Goroutine 3: Set active connections
	go func() {
		for i := 0; i < iterations; i++ {
			metrics.SetActiveConnections("tenant-1", i)
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done
}
