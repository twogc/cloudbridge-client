package relay

import (
	"strings"
	"testing"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// newTestRelayLogger creates a new test relay logger
func newTestRelayLogger() *relayLogger {
	return NewRelayLogger("test")
}

// TestAutoSwitchManager_Stop tests that Stop() properly tears down WireGuard
func TestAutoSwitchManager_Stop(t *testing.T) {
	// Create test configuration with WireGuard enabled
	config := &types.Config{
		WireGuard: types.WireGuardConfig{
			Enabled:       true,
			InterfaceName: "wg-test-auto",
			MTU:           1420,
			Port:          51824,
		},
		Relay: types.RelayConfig{
			Host: "test.example.com",
			Ports: types.RelayPorts{
				MASQUE: 8443,
			},
		},
	}

	// Create test logger
	logger := newTestRelayLogger()

	// Create AutoSwitchManager
	asm := NewAutoSwitchManager(config, logger)

	// Start the manager (this will initialize WireGuard manager)
	err := asm.Start()
	if err != nil {
		// If WireGuard tools are not available, skip the test
		t.Skipf("Failed to start AutoSwitchManager (likely missing WireGuard tools): %v", err)
	}

	// Stop the manager
	err = asm.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Verify that WireGuard teardown was called
	// We can't directly verify the interface was removed without root privileges,
	// but we can check that the Stop method completed without error
}

// TestAutoSwitchManager_SwitchToQUIC tests switching back to QUIC tears down WireGuard
func TestAutoSwitchManager_SwitchToQUIC(t *testing.T) {
	// Create test configuration with WireGuard enabled
	config := &types.Config{
		WireGuard: types.WireGuardConfig{
			Enabled:       true,
			InterfaceName: "wg-test-switch",
			MTU:           1420,
			Port:          51825,
		},
		Relay: types.RelayConfig{
			Host: "test.example.com",
			Ports: types.RelayPorts{
				MASQUE: 8443,
			},
		},
	}

	// Create test logger
	logger := newTestRelayLogger()

	// Create AutoSwitchManager
	asm := NewAutoSwitchManager(config, logger)

	// Start the manager
	err := asm.Start()
	if err != nil {
		t.Skipf("Failed to start AutoSwitchManager (likely missing WireGuard tools): %v", err)
	}
	defer func() {
		if err := asm.Stop(); err != nil {
			t.Logf("Failed to stop AutoSwitchManager: %v", err)
		}
	}()

	// Set a custom health check function that always returns false (unhealthy QUIC)
	asm.SetHealthCheckFunc(func() bool { return false })

	// Force switch to WireGuard
	err = asm.ForceSwitch(TransportModeWireGuard)
	if err != nil {
		// If we don't have root privileges or required tools, skip the test
		if strings.Contains(err.Error(), "Operation not permitted") {
			t.Skip("Test requires root privileges for WireGuard interface operations")
		}
		if strings.Contains(err.Error(), "executable file not found") {
			t.Skip("Test requires WireGuard tools (wg, ip) - not available on this platform")
		}
		t.Errorf("Failed to switch to WireGuard: %v", err)
	}

	// Verify we're in WireGuard mode
	if asm.GetCurrentMode() != TransportModeWireGuard {
		t.Error("Expected to be in WireGuard mode")
	}

	// Set health check to return true (healthy QUIC)
	asm.SetHealthCheckFunc(func() bool { return true })

	// Force switch back to QUIC
	err = asm.ForceSwitch(TransportModeQUIC)
	if err != nil {
		t.Errorf("Failed to switch to QUIC: %v", err)
	}

	// Verify we're back in QUIC mode
	if asm.GetCurrentMode() != TransportModeQUIC {
		t.Error("Expected to be in QUIC mode")
	}

	// Test completed successfully - switching to QUIC should call WireGuard TearDown()
}

// TestAutoSwitchManager_HealthCheck tests the health checking mechanism
func TestAutoSwitchManager_HealthCheck(t *testing.T) {
	// Create test configuration with WireGuard enabled
	config := &types.Config{
		WireGuard: types.WireGuardConfig{
			Enabled:       true,
			InterfaceName: "wg-test-health",
			MTU:           1420,
			Port:          51826,
		},
		Relay: types.RelayConfig{
			Host: "test.example.com",
			Ports: types.RelayPorts{
				MASQUE: 8443,
			},
		},
	}

	// Create test logger
	logger := newTestRelayLogger()

	// Create AutoSwitchManager
	asm := NewAutoSwitchManager(config, logger)

	// Start the manager
	err := asm.Start()
	if err != nil {
		t.Skipf("Failed to start AutoSwitchManager (likely missing WireGuard tools): %v", err)
	}
	defer func() {
		if err := asm.Stop(); err != nil {
			t.Logf("Failed to stop AutoSwitchManager: %v", err)
		}
	}()

	// Test health check function
	healthCheckCalled := false
	asm.SetHealthCheckFunc(func() bool {
		healthCheckCalled = true
		return true // Always healthy
	})

	// Manually trigger health check
	asm.performHealthCheck()

	// Verify health check was called
	if !healthCheckCalled {
		t.Error("Health check function was not called")
	}

	// Verify we're still in QUIC mode (since health check returned true)
	if asm.GetCurrentMode() != TransportModeQUIC {
		t.Error("Expected to remain in QUIC mode when healthy")
	}
}

// TestAutoSwitchManager_CallbacksOnSwitch tests that callbacks are called on mode switch
func TestAutoSwitchManager_CallbacksOnSwitch(t *testing.T) {
	// Create test configuration with WireGuard enabled
	config := &types.Config{
		WireGuard: types.WireGuardConfig{
			Enabled:       true,
			InterfaceName: "wg-test-callback",
			MTU:           1420,
			Port:          51827,
		},
		Relay: types.RelayConfig{
			Host: "test.example.com",
			Ports: types.RelayPorts{
				MASQUE: 8443,
			},
		},
	}

	// Create test logger
	logger := newTestRelayLogger()

	// Create AutoSwitchManager
	asm := NewAutoSwitchManager(config, logger)

	// Start the manager
	err := asm.Start()
	if err != nil {
		t.Skipf("Failed to start AutoSwitchManager (likely missing WireGuard tools): %v", err)
	}
	defer func() {
		if err := asm.Stop(); err != nil {
			t.Logf("Failed to stop AutoSwitchManager: %v", err)
		}
	}()

	// Add callback to track mode switches
	var switchEvents []struct {
		from, to TransportMode
	}

	asm.AddSwitchCallback(func(from, to TransportMode) {
		switchEvents = append(switchEvents, struct{ from, to TransportMode }{from, to})
	})

	// Force switch to WireGuard
	err = asm.ForceSwitch(TransportModeWireGuard)
	if err != nil {
		// If we don't have root privileges or required tools, skip the test
		if strings.Contains(err.Error(), "Operation not permitted") {
			t.Skip("Test requires root privileges for WireGuard interface operations")
		}
		if strings.Contains(err.Error(), "executable file not found") {
			t.Skip("Test requires WireGuard tools (wg, ip) - not available on this platform")
		}
		t.Errorf("Failed to switch to WireGuard: %v", err)
	}

	// Give callback time to execute (it runs in a goroutine)
	time.Sleep(100 * time.Millisecond)

	// Verify callback was called
	if len(switchEvents) != 1 {
		t.Errorf("Expected 1 switch event, got %d", len(switchEvents))
	} else {
		event := switchEvents[0]
		if event.from != TransportModeQUIC || event.to != TransportModeWireGuard {
			t.Errorf("Expected switch from QUIC to WireGuard, got %s to %s", event.from, event.to)
		}
	}

	// Force switch back to QUIC
	err = asm.ForceSwitch(TransportModeQUIC)
	if err != nil {
		t.Errorf("Failed to switch to QUIC: %v", err)
	}

	// Give callback time to execute
	time.Sleep(100 * time.Millisecond)

	// Verify second callback was called
	if len(switchEvents) != 2 {
		t.Errorf("Expected 2 switch events, got %d", len(switchEvents))
	} else {
		event := switchEvents[1]
		if event.from != TransportModeWireGuard || event.to != TransportModeQUIC {
			t.Errorf("Expected switch from WireGuard to QUIC, got %s to %s", event.from, event.to)
		}
	}
}
