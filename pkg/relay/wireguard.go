//go:build !windows

package relay

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// WireGuardManager manages WireGuard VPN connections
type WireGuardManager struct {
	config        *types.Config
	logger        *relayLogger
	mu            sync.RWMutex
	connected     bool
	interfaceName string
}

// NewWireGuardManager creates a new WireGuardManager
func NewWireGuardManager(config *types.Config, logger *relayLogger) (*WireGuardManager, error) {
	if !config.WireGuard.Enabled {
		return nil, fmt.Errorf("WireGuard is disabled in configuration")
	}

	// Check if wg command is available
	if _, err := exec.LookPath("wg"); err != nil {
		return nil, fmt.Errorf("WireGuard tools not found: %w", err)
	}

	return &WireGuardManager{
		config:        config,
		logger:        logger,
		interfaceName: config.WireGuard.InterfaceName,
	}, nil
}

// Connect establishes a WireGuard connection
func (wgm *WireGuardManager) Connect() error {
	wgm.mu.Lock()
	defer wgm.mu.Unlock()

	if wgm.connected {
		return nil // Already connected
	}

	wgm.logger.Info("Establishing WireGuard connection", "interface", wgm.interfaceName)

	// Check if interface already exists
	if wgm.interfaceExists() {
		wgm.logger.Info("WireGuard interface already exists, reusing", "interface", wgm.interfaceName)
		wgm.connected = true
		return nil
	}

	// Create WireGuard interface
	if err := wgm.createInterface(); err != nil {
		return fmt.Errorf("failed to create WireGuard interface: %w", err)
	}

	// Configure interface
	if err := wgm.configureInterface(); err != nil {
		// Clean up on failure
		if cleanupErr := wgm.destroyInterface(); cleanupErr != nil {
			wgm.logger.Error("Failed to cleanup interface after configuration failure", "error", cleanupErr)
		}
		return fmt.Errorf("failed to configure WireGuard interface: %w", err)
	}

	// Bring interface up
	if err := wgm.bringInterfaceUp(); err != nil {
		// Clean up on failure
		if cleanupErr := wgm.destroyInterface(); cleanupErr != nil {
			wgm.logger.Error("Failed to cleanup interface after bringup failure", "error", cleanupErr)
		}
		return fmt.Errorf("failed to bring WireGuard interface up: %w", err)
	}

	wgm.connected = true
	wgm.logger.Info("WireGuard connection established successfully", "interface", wgm.interfaceName)
	return nil
}

// Disconnect closes the WireGuard connection
func (wgm *WireGuardManager) Disconnect() error {
	wgm.mu.Lock()
	defer wgm.mu.Unlock()

	if !wgm.connected {
		return nil // Already disconnected
	}

	wgm.logger.Info("Disconnecting WireGuard", "interface", wgm.interfaceName)

	// Destroy interface
	if err := wgm.destroyInterface(); err != nil {
		wgm.logger.Warn("Failed to destroy WireGuard interface cleanly", "error", err)
	}

	wgm.connected = false
	wgm.logger.Info("WireGuard disconnected", "interface", wgm.interfaceName)
	return nil
}

// Stop stops the WireGuard manager
func (wgm *WireGuardManager) Stop() error {
	return wgm.TearDown()
}

// TearDown properly tears down WireGuard connection with peer cleanup
func (wgm *WireGuardManager) TearDown() error {
	wgm.mu.Lock()
	defer wgm.mu.Unlock()

	if !wgm.connected {
		return nil // Already disconnected
	}

	wgm.logger.Info("Tearing down WireGuard connection", "interface", wgm.interfaceName)

	// Remove all peers first (proper cleanup)
	if err := wgm.removePeers(); err != nil {
		wgm.logger.Warn("Failed to remove WireGuard peers", "error", err)
	}

	// Bring interface down
	if err := wgm.bringInterfaceDown(); err != nil {
		wgm.logger.Warn("Failed to bring WireGuard interface down", "error", err)
	}

	// Destroy interface
	if err := wgm.destroyInterface(); err != nil {
		wgm.logger.Warn("Failed to destroy WireGuard interface cleanly", "error", err)
	}

	wgm.connected = false
	wgm.logger.Info("WireGuard interface removed", "interface", wgm.interfaceName)
	return nil
}

// IsConnected returns true if WireGuard is connected
func (wgm *WireGuardManager) IsConnected() bool {
	wgm.mu.RLock()
	defer wgm.mu.RUnlock()
	return wgm.connected
}

// interfaceExists checks if the WireGuard interface exists
func (wgm *WireGuardManager) interfaceExists() bool {
	cmd := exec.Command("ip", "link", "show", wgm.interfaceName)
	err := cmd.Run()
	return err == nil
}

// createInterface creates a WireGuard interface
func (wgm *WireGuardManager) createInterface() error {
	wgm.logger.Debug("Creating WireGuard interface", "interface", wgm.interfaceName)

	cmd := exec.Command("ip", "link", "add", "dev", wgm.interfaceName, "type", "wireguard")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create interface: %w, output: %s", err, string(output))
	}

	return nil
}

// configureInterface configures the WireGuard interface
func (wgm *WireGuardManager) configureInterface() error {
	wgm.logger.Debug("Configuring WireGuard interface", "interface", wgm.interfaceName)

	// Generate a temporary private key for this session
	// In a real implementation, this would be managed more securely
	privateKey, err := wgm.generatePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Set private key
	cmd := exec.Command("wg", "set", wgm.interfaceName, "private-key", "/dev/stdin")
	cmd.Stdin = strings.NewReader(privateKey)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set private key: %w, output: %s", err, string(output))
	}

	// Set MTU
	cmd = exec.Command("ip", "link", "set", "dev", wgm.interfaceName, "mtu", fmt.Sprintf("%d", wgm.config.WireGuard.MTU))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set MTU: %w, output: %s", err, string(output))
	}

	// In a real implementation, you would also configure:
	// - Peer public key
	// - Endpoint (relay server WireGuard endpoint)
	// - Allowed IPs
	// - Persistent keepalive

	wgm.logger.Debug("WireGuard interface configured", "interface", wgm.interfaceName, "mtu", wgm.config.WireGuard.MTU)
	return nil
}

// bringInterfaceUp brings the WireGuard interface up
func (wgm *WireGuardManager) bringInterfaceUp() error {
	wgm.logger.Debug("Bringing WireGuard interface up", "interface", wgm.interfaceName)

	cmd := exec.Command("ip", "link", "set", "up", "dev", wgm.interfaceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %w, output: %s", err, string(output))
	}

	return nil
}

// destroyInterface destroys the WireGuard interface
func (wgm *WireGuardManager) destroyInterface() error {
	wgm.logger.Debug("Destroying WireGuard interface", "interface", wgm.interfaceName)

	cmd := exec.Command("ip", "link", "del", "dev", wgm.interfaceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to destroy interface: %w, output: %s", err, string(output))
	}

	return nil
}

// generatePrivateKey generates a WireGuard private key
func (wgm *WireGuardManager) generatePrivateKey() (string, error) {
	cmd := exec.Command("wg", "genkey")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate private key: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetInterfaceName returns the WireGuard interface name
func (wgm *WireGuardManager) GetInterfaceName() string {
	return wgm.interfaceName
}

// GetStatus returns the current status of the WireGuard connection
func (wgm *WireGuardManager) GetStatus() (map[string]interface{}, error) {
	wgm.mu.RLock()
	defer wgm.mu.RUnlock()

	status := map[string]interface{}{
		"connected":      wgm.connected,
		"interface_name": wgm.interfaceName,
		"mtu":            wgm.config.WireGuard.MTU,
		"port":           wgm.config.WireGuard.Port,
	}

	if wgm.connected && wgm.interfaceExists() {
		// Get interface statistics
		if stats, err := wgm.getInterfaceStats(); err == nil {
			status["stats"] = stats
		}
	}

	return status, nil
}

// removePeers removes all peers from the WireGuard interface
func (wgm *WireGuardManager) removePeers() error {
	wgm.logger.Debug("Removing WireGuard peers", "interface", wgm.interfaceName)

	// In a real implementation, you would:
	// 1. Get list of current peers: wg show <interface> peers
	// 2. Remove each peer: wg set <interface> peer <pubkey> remove
	// For now, we'll use a simplified approach

	cmd := exec.Command("wg", "set", wgm.interfaceName, "peer", "remove-all")
	if output, err := cmd.CombinedOutput(); err != nil {
		// This command might not exist, so we'll try individual peer removal
		wgm.logger.Debug("Bulk peer removal failed, trying individual removal", "output", string(output))
		return wgm.removeIndividualPeers()
	}

	return nil
}

// removeIndividualPeers removes peers individually
func (wgm *WireGuardManager) removeIndividualPeers() error {
	// Get current peers
	cmd := exec.Command("wg", "show", wgm.interfaceName, "peers")
	output, err := cmd.Output()
	if err != nil {
		// Interface might not exist or have no peers
		return nil
	}

	peers := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, peer := range peers {
		if peer = strings.TrimSpace(peer); peer != "" {
			cmd := exec.Command("wg", "set", wgm.interfaceName, "peer", peer, "remove")
			if output, err := cmd.CombinedOutput(); err != nil {
				wgm.logger.Debug("Failed to remove peer", "peer", peer, "error", err, "output", string(output))
			}
		}
	}

	return nil
}

// bringInterfaceDown brings the WireGuard interface down
func (wgm *WireGuardManager) bringInterfaceDown() error {
	wgm.logger.Debug("Bringing WireGuard interface down", "interface", wgm.interfaceName)

	cmd := exec.Command("ip", "link", "set", "down", "dev", wgm.interfaceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface down: %w, output: %s", err, string(output))
	}

	return nil
}

// getInterfaceStats gets interface statistics
func (wgm *WireGuardManager) getInterfaceStats() (map[string]interface{}, error) {
	cmd := exec.Command("wg", "show", wgm.interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface stats: %w", err)
	}

	stats := map[string]interface{}{
		"raw_output": strings.TrimSpace(string(output)),
		"timestamp":  time.Now().Unix(),
	}

	return stats, nil
}
