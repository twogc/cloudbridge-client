package config

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches configuration files for changes and triggers hot-reload
type ConfigWatcher struct {
	configPath    string
	currentConfig *types.Config
	watcher       *fsnotify.Watcher
	logger        Logger
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	callbacks     []ConfigChangeCallback
	debounceTimer *time.Timer
	debounceDelay time.Duration
}

// Logger interface for config watcher logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// ConfigChangeCallback is called when configuration changes
type ConfigChangeCallback func(oldConfig, newConfig *types.Config) error

// ConfigChange represents a configuration change event
type ConfigChange struct {
	Field    string
	OldValue interface{}
	NewValue interface{}
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configPath string, logger Logger) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	cw := &ConfigWatcher{
		configPath:    configPath,
		watcher:       watcher,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		callbacks:     make([]ConfigChangeCallback, 0),
		debounceDelay: 500 * time.Millisecond, // Debounce file changes
	}

	return cw, nil
}

// Start starts watching the configuration file
func (cw *ConfigWatcher) Start() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	// Load initial configuration
	config, err := LoadConfig(cw.configPath)
	if err != nil {
		return fmt.Errorf("failed to load initial config: %w", err)
	}
	cw.currentConfig = config

	// Watch the configuration file
	if err := cw.watcher.Add(cw.configPath); err != nil {
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	// Also watch the directory to catch file replacements (common with editors)
	configDir := filepath.Dir(cw.configPath)
	if err := cw.watcher.Add(configDir); err != nil {
		cw.logger.Warn("Failed to watch config directory", "dir", configDir, "error", err)
	}

	// Start the watch loop
	go cw.watchLoop()

	cw.logger.Info("Config watcher started", "config_path", cw.configPath)
	return nil
}

// Stop stops the configuration watcher
func (cw *ConfigWatcher) Stop() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.debounceTimer != nil {
		cw.debounceTimer.Stop()
	}

	cw.cancel()

	if cw.watcher != nil {
		if err := cw.watcher.Close(); err != nil {
			cw.logger.Error("Failed to close file watcher", "error", err)
			return err
		}
	}

	cw.logger.Info("Config watcher stopped")
	return nil
}

// AddCallback adds a callback to be called when configuration changes
func (cw *ConfigWatcher) AddCallback(callback ConfigChangeCallback) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.callbacks = append(cw.callbacks, callback)
}

// GetCurrentConfig returns the current configuration
func (cw *ConfigWatcher) GetCurrentConfig() *types.Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.currentConfig
}

// watchLoop runs the file watching loop
func (cw *ConfigWatcher) watchLoop() {
	for {
		select {
		case <-cw.ctx.Done():
			return
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			cw.handleFileEvent(event)
		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			cw.logger.Error("File watcher error", "error", err)
		}
	}
}

// handleFileEvent handles file system events
func (cw *ConfigWatcher) handleFileEvent(event fsnotify.Event) {
	// Only handle events for our config file
	if event.Name != cw.configPath {
		// Check if it's a file in the config directory that might be our config
		if filepath.Dir(event.Name) == filepath.Dir(cw.configPath) &&
			filepath.Base(event.Name) == filepath.Base(cw.configPath) {
			// Handle case where file was moved/renamed
			cw.logger.Debug("Config file event in directory", "event", event.String())
		} else {
			return
		}
	}

	// Debounce rapid file changes (common with editors that create temp files)
	cw.mu.Lock()
	if cw.debounceTimer != nil {
		cw.debounceTimer.Stop()
	}
	cw.debounceTimer = time.AfterFunc(cw.debounceDelay, func() {
		cw.reloadConfig(event)
	})
	cw.mu.Unlock()

	cw.logger.Debug("Config file event", "event", event.String())
}

// reloadConfig reloads the configuration and triggers callbacks
func (cw *ConfigWatcher) reloadConfig(event fsnotify.Event) {
	cw.logger.Info("Reloading configuration", "event", event.Op.String())

	// Load new configuration
	newConfig, err := LoadConfig(cw.configPath)
	if err != nil {
		cw.logger.Error("Failed to reload config", "error", err)
		return
	}

	cw.mu.Lock()
	oldConfig := cw.currentConfig
	cw.currentConfig = newConfig
	callbacks := make([]ConfigChangeCallback, len(cw.callbacks))
	copy(callbacks, cw.callbacks)
	cw.mu.Unlock()

	// Detect and log changes
	changes := cw.detectChanges(oldConfig, newConfig)
	if len(changes) > 0 {
		cw.logger.Info("Configuration changes detected", "changes", len(changes))
		for _, change := range changes {
			cw.logger.Info("Config changed",
				"field", change.Field,
				"old_value", change.OldValue,
				"new_value", change.NewValue)
		}
	} else {
		cw.logger.Debug("No configuration changes detected")
		return
	}

	// Call callbacks
	for _, callback := range callbacks {
		if err := callback(oldConfig, newConfig); err != nil {
			cw.logger.Error("Config change callback failed", "error", err)
		}
	}

	cw.logger.Info("Configuration reloaded successfully")
}

// detectChanges detects changes between old and new configuration
func (cw *ConfigWatcher) detectChanges(oldConfig, newConfig *types.Config) []ConfigChange {
	var changes []ConfigChange

	// Check logging level changes
	if oldConfig.Logging.Level != newConfig.Logging.Level {
		changes = append(changes, ConfigChange{
			Field:    "logging.level",
			OldValue: oldConfig.Logging.Level,
			NewValue: newConfig.Logging.Level,
		})
	}

	// Check TLS verify changes
	if oldConfig.Relay.TLS.VerifyCert != newConfig.Relay.TLS.VerifyCert {
		changes = append(changes, ConfigChange{
			Field:    "relay.tls.verify_cert",
			OldValue: oldConfig.Relay.TLS.VerifyCert,
			NewValue: newConfig.Relay.TLS.VerifyCert,
		})
	}

	// Check TLS enabled changes
	if oldConfig.Relay.TLS.Enabled != newConfig.Relay.TLS.Enabled {
		changes = append(changes, ConfigChange{
			Field:    "relay.tls.enabled",
			OldValue: oldConfig.Relay.TLS.Enabled,
			NewValue: newConfig.Relay.TLS.Enabled,
		})
	}

	// Check relay host changes
	if oldConfig.Relay.Host != newConfig.Relay.Host {
		changes = append(changes, ConfigChange{
			Field:    "relay.host",
			OldValue: oldConfig.Relay.Host,
			NewValue: newConfig.Relay.Host,
		})
	}

	// Check relay port changes
	if oldConfig.Relay.Port != newConfig.Relay.Port {
		changes = append(changes, ConfigChange{
			Field:    "relay.port",
			OldValue: oldConfig.Relay.Port,
			NewValue: newConfig.Relay.Port,
		})
	}

	// Check auth secret changes (credential changes)
	if oldConfig.Auth.Secret != newConfig.Auth.Secret {
		changes = append(changes, ConfigChange{
			Field:    "auth.secret",
			OldValue: "[REDACTED]", // Don't log actual secrets
			NewValue: "[REDACTED]",
		})
	}

	// Check API endpoints
	if oldConfig.API.BaseURL != newConfig.API.BaseURL {
		changes = append(changes, ConfigChange{
			Field:    "api.base_url",
			OldValue: oldConfig.API.BaseURL,
			NewValue: newConfig.API.BaseURL,
		})
	}

	return changes
}

// ForceReload forces a configuration reload
func (cw *ConfigWatcher) ForceReload() error {
	cw.logger.Info("Forcing configuration reload")

	// Create a synthetic write event
	event := fsnotify.Event{
		Name: cw.configPath,
		Op:   fsnotify.Write,
	}

	cw.reloadConfig(event)
	return nil
}
