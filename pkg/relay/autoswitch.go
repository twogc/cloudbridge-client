package relay

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// TransportMode represents the current transport mode
type TransportMode string

const (
	// TransportModeQUIC represents QUIC transport mode
	TransportModeQUIC TransportMode = "quic"
	// TransportModeWireGuard represents WireGuard transport mode
	TransportModeWireGuard TransportMode = "wireguard"
)

// AutoSwitchManager manages automatic switching between QUIC and WireGuard transports
type AutoSwitchManager struct {
	config            *types.Config
	currentMode       TransportMode
	wgManager         *WireGuardManager
	logger            *relayLogger
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	healthCheckTicker *time.Ticker
	switchCallbacks   []func(from, to TransportMode)
	// healthCheckFunc allows overriding health check for testing
	healthCheckFunc func() bool
}

// NewAutoSwitchManager creates a new AutoSwitchManager
func NewAutoSwitchManager(config *types.Config, logger *relayLogger) *AutoSwitchManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &AutoSwitchManager{
		config:          config,
		currentMode:     TransportModeQUIC, // Start with QUIC by default
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		switchCallbacks: make([]func(from, to TransportMode), 0),
	}
}

// Start starts the AutoSwitchManager
func (asm *AutoSwitchManager) Start() error {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	if !asm.config.WireGuard.Enabled {
		asm.logger.Info("WireGuard fallback disabled in configuration")
		return nil
	}

	// Initialize WireGuard manager
	wgManager, err := NewWireGuardManager(asm.config, asm.logger)
	if err != nil {
		return fmt.Errorf("failed to create WireGuard manager: %w", err)
	}
	asm.wgManager = wgManager

	// Start health checking
	asm.startHealthChecking()

	asm.logger.Info("AutoSwitchManager started", "initial_mode", asm.currentMode)
	return nil
}

// Stop stops the AutoSwitchManager
func (asm *AutoSwitchManager) Stop() error {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	if asm.healthCheckTicker != nil {
		asm.healthCheckTicker.Stop()
	}

	if asm.wgManager != nil {
		if err := asm.wgManager.Stop(); err != nil {
			asm.logger.Error("Failed to stop WireGuard manager", "error", err)
		}
	}

	asm.cancel()
	asm.logger.Info("AutoSwitchManager stopped")
	return nil
}

// GetCurrentMode returns the current transport mode
func (asm *AutoSwitchManager) GetCurrentMode() TransportMode {
	asm.mu.RLock()
	defer asm.mu.RUnlock()
	return asm.currentMode
}

// AddSwitchCallback adds a callback to be called when transport mode switches
func (asm *AutoSwitchManager) AddSwitchCallback(callback func(from, to TransportMode)) {
	asm.mu.Lock()
	defer asm.mu.Unlock()
	asm.switchCallbacks = append(asm.switchCallbacks, callback)
}

// startHealthChecking starts periodic health checking of QUIC connection
func (asm *AutoSwitchManager) startHealthChecking() {
	// Check every 10 seconds
	asm.healthCheckTicker = time.NewTicker(10 * time.Second)

	go func() {
		for {
			select {
			case <-asm.ctx.Done():
				return
			case <-asm.healthCheckTicker.C:
				asm.performHealthCheck()
			}
		}
	}()
}

// performHealthCheck checks the health of current transport and switches if needed
func (asm *AutoSwitchManager) performHealthCheck() {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	switch asm.currentMode {
	case TransportModeQUIC:
		if !asm.isQUICHealthy() {
			asm.logger.Warn("QUIC connection unhealthy, switching to WireGuard")
			if err := asm.switchToWireGuard(); err != nil {
				asm.logger.Error("Failed to switch to WireGuard", "error", err)
			}
		}
	case TransportModeWireGuard:
		if asm.isQUICHealthy() {
			asm.logger.Info("QUIC connection restored, switching back from WireGuard")
			if err := asm.switchToQUIC(); err != nil {
				asm.logger.Error("Failed to switch back to QUIC", "error", err)
			}
		}
	}
}

// isQUICHealthy checks if QUIC connection is healthy by testing UDP connectivity
func (asm *AutoSwitchManager) isQUICHealthy() bool {
	// Use custom health check function if set (for testing)
	if asm.healthCheckFunc != nil {
		return asm.healthCheckFunc()
	}

	// Test UDP connectivity to MASQUE port (8443)
	address := net.JoinHostPort(asm.config.Relay.Host, fmt.Sprintf("%d", asm.config.Relay.Ports.MASQUE))

	conn, err := net.DialTimeout("udp", address, 5*time.Second)
	if err != nil {
		asm.logger.Debug("UDP connectivity test failed", "address", address, "error", err)
		return false
	}
	defer conn.Close()

	// Try to write a small test packet
	_, err = conn.Write([]byte("health_check"))
	if err != nil {
		asm.logger.Debug("UDP write test failed", "error", err)
		return false
	}

	return true
}

// SetHealthCheckFunc sets a custom health check function (for testing)
func (asm *AutoSwitchManager) SetHealthCheckFunc(fn func() bool) {
	asm.mu.Lock()
	defer asm.mu.Unlock()
	asm.healthCheckFunc = fn
}

// switchToWireGuard switches transport mode to WireGuard
func (asm *AutoSwitchManager) switchToWireGuard() error {
	if asm.currentMode == TransportModeWireGuard {
		return nil // Already in WireGuard mode
	}

	if asm.wgManager == nil {
		return fmt.Errorf("WireGuard manager not initialized")
	}

	// Start WireGuard connection
	if err := asm.wgManager.Connect(); err != nil {
		return fmt.Errorf("failed to connect via WireGuard: %w", err)
	}

	oldMode := asm.currentMode
	asm.currentMode = TransportModeWireGuard

	// Call switch callbacks
	for _, callback := range asm.switchCallbacks {
		go callback(oldMode, asm.currentMode)
	}

	asm.logger.Info("SwitchedToWG", "from", oldMode, "to", asm.currentMode)
	return nil
}

// switchToQUIC switches transport mode back to QUIC
func (asm *AutoSwitchManager) switchToQUIC() error {
	if asm.currentMode == TransportModeQUIC {
		return nil // Already in QUIC mode
	}

	if asm.wgManager != nil {
		// Properly tear down WireGuard connection
		if err := asm.wgManager.TearDown(); err != nil {
			asm.logger.Warn("Failed to tear down WireGuard cleanly", "error", err)
		}
	}

	oldMode := asm.currentMode
	asm.currentMode = TransportModeQUIC

	// Call switch callbacks
	for _, callback := range asm.switchCallbacks {
		go callback(oldMode, asm.currentMode)
	}

	asm.logger.Info("SwitchedToQUIC", "from", oldMode, "to", asm.currentMode)
	return nil
}

// ForceSwitch forces a switch to the specified transport mode
func (asm *AutoSwitchManager) ForceSwitch(mode TransportMode) error {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	switch mode {
	case TransportModeQUIC:
		return asm.switchToQUIC()
	case TransportModeWireGuard:
		return asm.switchToWireGuard()
	default:
		return fmt.Errorf("unsupported transport mode: %s", mode)
	}
}
