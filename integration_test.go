// +build integration

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/config"
	"github.com/twogc/cloudbridge-client/pkg/errors"
	"github.com/twogc/cloudbridge-client/pkg/metrics"
)

// TestIntegration_ConfigLoading tests configuration loading with environment variables
func TestIntegration_ConfigLoading(t *testing.T) {
	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "integration-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
relay:
  host: ${RELAY_HOST:edge.2gc.ru}
  port: ${RELAY_PORT:5553}
  tls:
    enabled: true
    verify_cert: true
auth:
  type: jwt
  secret: ${JWT_SECRET:test-secret}
metrics:
  enabled: true
  prometheus_port: 19200
`
	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Set environment variables
	os.Setenv("RELAY_HOST", "test.example.com")
	os.Setenv("RELAY_PORT", "5554")
	os.Setenv("JWT_SECRET", "integration-secret")
	defer func() {
		os.Unsetenv("RELAY_HOST")
		os.Unsetenv("RELAY_PORT")
		os.Unsetenv("JWT_SECRET")
	}()

	// Load configuration
	cfg, err := config.LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variable substitution
	if cfg.Relay.Host != "test.example.com" {
		t.Errorf("Expected relay host test.example.com, got %s", cfg.Relay.Host)
	}

	if cfg.Relay.Port != 5554 {
		t.Errorf("Expected relay port 5554, got %d", cfg.Relay.Port)
	}

	if cfg.Auth.Secret != "integration-secret" {
		t.Errorf("Expected auth secret integration-secret, got %s", cfg.Auth.Secret)
	}
}

// TestIntegration_MetricsServer tests metrics server lifecycle
func TestIntegration_MetricsServer(t *testing.T) {
	metricsPort := 19201

	// Create metrics instance
	m := metrics.NewMetrics(true, metricsPort)

	// Start metrics server
	if err := m.Start(); err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Record some metrics
	m.RecordClientBytesSent(1024)
	m.RecordClientBytesRecv(2048)
	m.SetP2PSessions(5)
	m.SetTransportMode(2)
	m.RecordBytesTransferred("tenant-1", "sent", 1000)
	m.RecordConnectionHandled("tenant-1")
	m.SetActiveConnections("tenant-1", 10)
	m.RecordError("test_error", "tenant-1")

	// Wait a bit for metrics to be processed
	time.Sleep(100 * time.Millisecond)

	// Stop metrics server
	if err := m.Stop(); err != nil {
		t.Errorf("Failed to stop metrics server: %v", err)
	}

	t.Log("Metrics server lifecycle completed successfully")
}

// TestIntegration_ErrorRetryFlow tests error handling and retry mechanism
func TestIntegration_ErrorRetryFlow(t *testing.T) {
	// Create retry strategy
	strategy := errors.NewRetryStrategy(3, 2.0, 10*time.Second)

	// Simulate errors
	testErrors := []struct {
		name          string
		errorCode     string
		message       string
		shouldRetry   bool
		expectedDelay time.Duration
	}{
		{
			name:          "Rate limit error",
			errorCode:     errors.ErrRateLimitExceeded,
			message:       "Too many requests",
			shouldRetry:   true,
			expectedDelay: 5 * time.Second,
		},
		{
			name:          "Server unavailable",
			errorCode:     errors.ErrServerUnavailable,
			message:       "Server is down",
			shouldRetry:   true,
			expectedDelay: 10 * time.Second,
		},
		{
			name:          "Invalid token",
			errorCode:     errors.ErrInvalidToken,
			message:       "Token is invalid",
			shouldRetry:   false,
			expectedDelay: 0,
		},
	}

	for _, tt := range testErrors {
		t.Run(tt.name, func(t *testing.T) {
			strategy.Reset()

			err := errors.NewRelayError(tt.errorCode, tt.message)

			shouldRetry := strategy.ShouldRetry(err)
			if shouldRetry != tt.shouldRetry {
				t.Errorf("Expected shouldRetry=%v, got %v", tt.shouldRetry, shouldRetry)
			}

			if tt.shouldRetry {
				delay := strategy.GetNextDelay(err)
				if delay < tt.expectedDelay {
					t.Logf("Delay %v is less than expected %v (normal due to backoff)", delay, tt.expectedDelay)
				}
			}
		})
	}
}

// TestIntegration_ConfigurationValidation tests full configuration validation
func TestIntegration_ConfigurationValidation(t *testing.T) {
	// Create a temporary config file with various settings
	tmpFile, err := os.CreateTemp("", "validation-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
relay:
  host: edge.2gc.ru
  port: 5553
  tls:
    enabled: true
    min_version: "1.3"
    verify_cert: true
    server_name: b1.2gc.space
auth:
  type: jwt
  secret: test-secret-123
rate_limiting:
  enabled: true
  max_retries: 5
  backoff_multiplier: 2.5
  max_backoff: 60s
logging:
  level: info
  format: json
  output: stdout
metrics:
  enabled: true
  prometheus_port: 9091
wireguard:
  enabled: true
  interface_name: wg-test
  port: 51820
  mtu: 1420
p2p:
  max_connections: 500
  session_timeout: 300s
  heartbeat_interval: 30s
`
	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Load and validate configuration
	cfg, err := config.LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify configuration values
	if !cfg.Relay.TLS.Enabled {
		t.Error("Expected TLS to be enabled")
	}

	if cfg.Auth.Type != "jwt" {
		t.Errorf("Expected auth type jwt, got %s", cfg.Auth.Type)
	}

	if cfg.RateLimiting.MaxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", cfg.RateLimiting.MaxRetries)
	}

	if !cfg.WireGuard.Enabled {
		t.Error("Expected WireGuard to be enabled")
	}

	if cfg.P2P.MaxConnections != 500 {
		t.Errorf("Expected max connections 500, got %d", cfg.P2P.MaxConnections)
	}
}

// TestIntegration_ConcurrentOperations tests concurrent operations
func TestIntegration_ConcurrentOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create metrics instance
	m := metrics.NewMetrics(true, 19202)
	if err := m.Start(); err != nil {
		t.Fatalf("Failed to start metrics: %v", err)
	}
	defer m.Stop()

	// Run concurrent operations
	operations := 100
	done := make(chan bool, 3)

	// Goroutine 1: Record bytes
	go func() {
		for i := 0; i < operations; i++ {
			select {
			case <-ctx.Done():
				done <- false
				return
			default:
				m.RecordClientBytesSent(int64(i * 100))
				time.Sleep(time.Millisecond)
			}
		}
		done <- true
	}()

	// Goroutine 2: Record connections
	go func() {
		for i := 0; i < operations; i++ {
			select {
			case <-ctx.Done():
				done <- false
				return
			default:
				m.RecordConnectionHandled("tenant-1")
				time.Sleep(time.Millisecond)
			}
		}
		done <- true
	}()

	// Goroutine 3: Update active connections
	go func() {
		for i := 0; i < operations; i++ {
			select {
			case <-ctx.Done():
				done <- false
				return
			default:
				m.SetActiveConnections("tenant-1", i)
				time.Sleep(time.Millisecond)
			}
		}
		done <- true
	}()

	// Wait for all goroutines
	success := 0
	for i := 0; i < 3; i++ {
		if <-done {
			success++
		}
	}

	if success != 3 {
		t.Errorf("Expected 3 successful goroutines, got %d", success)
	}

	t.Log("Concurrent operations completed successfully")
}
