//go:build !windows

package relay

import (
	"os/exec"
	"testing"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// TestWireGuardManager_TearDown tests the WireGuard teardown functionality
func TestWireGuardManager_TearDown(t *testing.T) {
	// Skip test if WireGuard tools are not available
	if _, err := exec.LookPath("wg"); err != nil {
		t.Skip("WireGuard tools not available, skipping test")
	}

	// Create test configuration
	config := &types.Config{
		WireGuard: types.WireGuardConfig{
			Enabled:       true,
			InterfaceName: "wg-test",
			MTU:           1420,
			Port:          51820,
		},
	}

	// Create test logger
	logger := newTestRelayLogger()

	// Create WireGuard manager
	wgm, err := NewWireGuardManager(config, logger)
	if err != nil {
		t.Fatalf("Failed to create WireGuard manager: %v", err)
	}

	// Test TearDown when not connected (should not fail)
	err = wgm.TearDown()
	if err != nil {
		t.Errorf("TearDown failed when not connected: %v", err)
	}

	// Test completed successfully - we can't easily verify log messages with real logger
	// but the fact that TearDown() completed without error is sufficient
}

// TestWireGuardManager_Stop tests the Stop method
func TestWireGuardManager_Stop(t *testing.T) {
	// Skip test if WireGuard tools are not available
	if _, err := exec.LookPath("wg"); err != nil {
		t.Skip("WireGuard tools not available, skipping test")
	}

	// Create test configuration
	config := &types.Config{
		WireGuard: types.WireGuardConfig{
			Enabled:       true,
			InterfaceName: "wg-test-stop",
			MTU:           1420,
			Port:          51821,
		},
	}

	// Create test logger
	logger := newTestRelayLogger()

	// Create WireGuard manager
	wgm, err := NewWireGuardManager(config, logger)
	if err != nil {
		t.Fatalf("Failed to create WireGuard manager: %v", err)
	}

	// Test Stop method
	err = wgm.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Test completed successfully - Stop() calls TearDown() internally
}

// TestWireGuardManager_RemovePeers tests peer removal functionality
func TestWireGuardManager_RemovePeers(t *testing.T) {
	// Skip test if WireGuard tools are not available
	if _, err := exec.LookPath("wg"); err != nil {
		t.Skip("WireGuard tools not available, skipping test")
	}

	// Create test configuration
	config := &types.Config{
		WireGuard: types.WireGuardConfig{
			Enabled:       true,
			InterfaceName: "wg-test-peers",
			MTU:           1420,
			Port:          51822,
		},
	}

	// Create test logger
	logger := newTestRelayLogger()

	// Create WireGuard manager
	wgm, err := NewWireGuardManager(config, logger)
	if err != nil {
		t.Fatalf("Failed to create WireGuard manager: %v", err)
	}

	// Test removePeers method (should not fail even if interface doesn't exist)
	err = wgm.removePeers()
	if err != nil {
		t.Errorf("removePeers failed: %v", err)
	}
}

// TestWireGuardManager_InterfaceOperations tests interface creation/destruction
func TestWireGuardManager_InterfaceOperations(t *testing.T) {
	// Skip test if WireGuard tools are not available
	if _, err := exec.LookPath("wg"); err != nil {
		t.Skip("WireGuard tools not available, skipping test")
	}

	// Skip test if not running as root (required for interface operations)
	if !isRunningAsRoot() {
		t.Skip("Test requires root privileges for interface operations")
	}

	// Create test configuration
	config := &types.Config{
		WireGuard: types.WireGuardConfig{
			Enabled:       true,
			InterfaceName: "wg-test-iface",
			MTU:           1420,
			Port:          51823,
		},
	}

	// Create test logger
	logger := newTestRelayLogger()

	// Create WireGuard manager
	wgm, err := NewWireGuardManager(config, logger)
	if err != nil {
		t.Fatalf("Failed to create WireGuard manager: %v", err)
	}

	// Clean up any existing interface first
	_ = wgm.destroyInterface() //nolint:errcheck // Explicitly ignore errors in test cleanup

	// Test interface creation
	err = wgm.createInterface()
	if err != nil {
		t.Errorf("createInterface failed: %v", err)
	}

	// Verify interface exists
	if !wgm.interfaceExists() {
		t.Error("Interface should exist after creation")
	}

	// Test interface destruction
	err = wgm.destroyInterface()
	if err != nil {
		t.Errorf("destroyInterface failed: %v", err)
	}

	// Verify interface no longer exists
	if wgm.interfaceExists() {
		t.Error("Interface should not exist after destruction")
	}
}

// isRunningAsRoot checks if the current process is running as root
func isRunningAsRoot() bool {
	cmd := exec.Command("id", "-u")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return string(output) == "0\n"
}
