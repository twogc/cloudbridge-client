//go:build windows

package relay

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// WireGuardManager manages WireGuard VPN connections on Windows
type WireGuardManager struct {
	config        *types.Config
	logger        *relayLogger
	mu            sync.RWMutex
	connected     bool
	interfaceName string
}

// NewWireGuardManager creates a new WireGuardManager for Windows
func NewWireGuardManager(config *types.Config, logger *relayLogger) (*WireGuardManager, error) {
	if !config.WireGuard.Enabled {
		return nil, fmt.Errorf("WireGuard is disabled in configuration")
	}

	// Check if WireGuard is installed on Windows
	if _, err := exec.LookPath("wg.exe"); err != nil {
		return nil, fmt.Errorf("WireGuard for Windows not found: %w", err)
	}

	return &WireGuardManager{
		config:        config,
		logger:        logger,
		interfaceName: config.WireGuard.InterfaceName,
	}, nil
}

// Connect establishes a WireGuard connection on Windows
func (wgm *WireGuardManager) Connect() error {
	wgm.mu.Lock()
	defer wgm.mu.Unlock()

	if wgm.connected {
		return nil // Already connected
	}

	wgm.logger.Info("Establishing WireGuard connection on Windows", "interface", wgm.interfaceName)

	// Check if interface already exists
	if wgm.interfaceExists() {
		wgm.logger.Info("WireGuard interface already exists, reusing", "interface", wgm.interfaceName)
		wgm.connected = true
		return nil
	}

	// Create WireGuard interface using Windows netsh
	if err := wgm.createInterface(); err != nil {
		return fmt.Errorf("failed to create WireGuard interface: %w", err)
	}

	// Configure interface
	if err := wgm.configureInterface(); err != nil {
		// Clean up on failure
		wgm.destroyInterface()
		return fmt.Errorf("failed to configure WireGuard interface: %w", err)
	}

	// Bring interface up
	if err := wgm.bringInterfaceUp(); err != nil {
		// Clean up on failure
		wgm.destroyInterface()
		return fmt.Errorf("failed to bring WireGuard interface up: %w", err)
	}

	wgm.connected = true
	wgm.logger.Info("WireGuard connection established successfully on Windows", "interface", wgm.interfaceName)
	return nil
}

// Disconnect closes the WireGuard connection
func (wgm *WireGuardManager) Disconnect() error {
	wgm.mu.Lock()
	defer wgm.mu.Unlock()

	if !wgm.connected {
		return nil // Already disconnected
	}

	wgm.logger.Info("Disconnecting WireGuard on Windows", "interface", wgm.interfaceName)

	// Destroy interface
	if err := wgm.destroyInterface(); err != nil {
		wgm.logger.Warn("Failed to destroy WireGuard interface cleanly", "error", err)
	}

	wgm.connected = false
	wgm.logger.Info("WireGuard disconnected on Windows", "interface", wgm.interfaceName)
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

	wgm.logger.Info("Tearing down WireGuard connection on Windows", "interface", wgm.interfaceName)

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
	wgm.logger.Info("WireGuard connection torn down on Windows", "interface", wgm.interfaceName)
	return nil
}

// IsConnected returns true if WireGuard is connected
func (wgm *WireGuardManager) IsConnected() bool {
	wgm.mu.RLock()
	defer wgm.mu.RUnlock()
	return wgm.connected
}

// interfaceExists checks if the WireGuard interface exists on Windows
func (wgm *WireGuardManager) interfaceExists() bool {
	cmd := exec.Command("netsh", "interface", "show", "interface", "name="+wgm.interfaceName)
	err := cmd.Run()
	return err == nil
}

// createInterface creates a WireGuard interface on Windows
func (wgm *WireGuardManager) createInterface() error {
	wgm.logger.Debug("Creating WireGuard interface on Windows", "interface", wgm.interfaceName)

	// On Windows, WireGuard interfaces are created differently
	// We use the WireGuard Windows service approach
	cmd := exec.Command("wg-quick.exe", "up", wgm.interfaceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Fallback: try using netsh to create tunnel interface
		cmd = exec.Command("netsh", "interface", "ipv6", "add", "v6v4tunnel",
			"interface="+wgm.interfaceName, "localaddress=0.0.0.0", "remoteaddress=0.0.0.0")
		if output2, err2 := cmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("failed to create interface: %w, output: %s, fallback output: %s", err, string(output), string(output2))
		}
	}

	return nil
}

// configureInterface configures the WireGuard interface on Windows
func (wgm *WireGuardManager) configureInterface() error {
	wgm.logger.Debug("Configuring WireGuard interface on Windows", "interface", wgm.interfaceName)

	// Generate a temporary private key for this session
	privateKey, err := wgm.generatePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Set private key using wg.exe
	cmd := exec.Command("wg.exe", "set", wgm.interfaceName, "private-key", "-")
	cmd.Stdin = strings.NewReader(privateKey)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set private key: %w, output: %s", err, string(output))
	}

	// Set MTU using netsh
	cmd = exec.Command("netsh", "interface", "ipv4", "set", "subinterface",
		wgm.interfaceName, "mtu="+fmt.Sprintf("%d", wgm.config.WireGuard.MTU))
	if output, err := cmd.CombinedOutput(); err != nil {
		wgm.logger.Warn("Failed to set MTU via netsh", "error", err, "output", string(output))
		// MTU setting failure is not critical, continue
	}

	wgm.logger.Debug("WireGuard interface configured on Windows", "interface", wgm.interfaceName, "mtu", wgm.config.WireGuard.MTU)
	return nil
}

// bringInterfaceUp brings the WireGuard interface up on Windows
func (wgm *WireGuardManager) bringInterfaceUp() error {
	wgm.logger.Debug("Bringing WireGuard interface up on Windows", "interface", wgm.interfaceName)

	cmd := exec.Command("netsh", "interface", "set", "interface", wgm.interfaceName, "admin=enabled")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %w, output: %s", err, string(output))
	}

	return nil
}

// bringInterfaceDown brings the WireGuard interface down on Windows
func (wgm *WireGuardManager) bringInterfaceDown() error {
	wgm.logger.Debug("Bringing WireGuard interface down on Windows", "interface", wgm.interfaceName)

	cmd := exec.Command("netsh", "interface", "set", "interface", wgm.interfaceName, "admin=disabled")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface down: %w, output: %s", err, string(output))
	}

	return nil
}

// destroyInterface destroys the WireGuard interface on Windows
func (wgm *WireGuardManager) destroyInterface() error {
	wgm.logger.Debug("Destroying WireGuard interface on Windows", "interface", wgm.interfaceName)

	// Try wg-quick down first
	cmd := exec.Command("wg-quick.exe", "down", wgm.interfaceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		wgm.logger.Debug("wg-quick down failed, trying netsh", "output", string(output))

		// Fallback: remove tunnel interface
		cmd = exec.Command("netsh", "interface", "ipv6", "delete", "v6v4tunnel", "interface="+wgm.interfaceName)
		if output2, err2 := cmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("failed to destroy interface: %w, output: %s, fallback output: %s", err, string(output), string(output2))
		}
	}

	return nil
}

// removePeers removes all peers from the WireGuard interface on Windows
func (wgm *WireGuardManager) removePeers() error {
	wgm.logger.Debug("Removing WireGuard peers on Windows", "interface", wgm.interfaceName)

	// Get current peers
	cmd := exec.Command("wg.exe", "show", wgm.interfaceName, "peers")
	output, err := cmd.Output()
	if err != nil {
		// Interface might not exist or have no peers
		return nil
	}

	peers := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, peer := range peers {
		if peer = strings.TrimSpace(peer); peer != "" {
			cmd := exec.Command("wg.exe", "set", wgm.interfaceName, "peer", peer, "remove")
			if output, err := cmd.CombinedOutput(); err != nil {
				wgm.logger.Debug("Failed to remove peer", "peer", peer, "error", err, "output", string(output))
			}
		}
	}

	return nil
}

// generatePrivateKey generates a WireGuard private key on Windows
func (wgm *WireGuardManager) generatePrivateKey() (string, error) {
	cmd := exec.Command("wg.exe", "genkey")
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
		"platform":       "windows",
	}

	if wgm.connected && wgm.interfaceExists() {
		// Get interface statistics
		if stats, err := wgm.getInterfaceStats(); err == nil {
			status["stats"] = stats
		}
	}

	return status, nil
}

// getInterfaceStats gets interface statistics on Windows
func (wgm *WireGuardManager) getInterfaceStats() (map[string]interface{}, error) {
	cmd := exec.Command("wg.exe", "show", wgm.interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface stats: %w", err)
	}

	stats := map[string]interface{}{
		"raw_output": strings.TrimSpace(string(output)),
		"timestamp":  time.Now().Unix(),
		"platform":   "windows",
	}

	return stats, nil
}
