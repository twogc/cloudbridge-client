// Package wizard provides interactive configuration wizard
package wizard

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/onboarding"
	"github.com/twogc/cloudbridge-client/pkg/types"
)

// SetupWizard provides interactive configuration setup
type SetupWizard struct {
	reader *bufio.Reader
}

// NewSetupWizard creates a new setup wizard
func NewSetupWizard() *SetupWizard {
	return &SetupWizard{
		reader: bufio.NewReader(os.Stdin),
	}
}

// Run executes the setup wizard
func (w *SetupWizard) Run() (*types.Config, error) {
	fmt.Println("╔═══════════════════════════════════════════════════╗")
	fmt.Println("║   CloudBridge Client Configuration Wizard        ║")
	fmt.Println("╚═══════════════════════════════════════════════════╝")
	fmt.Println()

	// Step 1: Authentication method
	authMethod, err := w.selectAuthMethod()
	if err != nil {
		return nil, err
	}

	var cfg *types.Config

	switch authMethod {
	case "invite":
		cfg, err = w.configureViaInvite()
	case "manual":
		cfg, err = w.configureManual()
	case "oidc":
		cfg, err = w.configureOIDC()
	default:
		return nil, fmt.Errorf("invalid auth method: %s", authMethod)
	}

	if err != nil {
		return nil, err
	}

	// Step 2: Network configuration
	if err := w.configureNetwork(cfg); err != nil {
		return nil, err
	}

	// Step 3: Advanced options
	if w.promptYesNo("Configure advanced options?", false) {
		if err := w.configureAdvanced(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func (w *SetupWizard) selectAuthMethod() (string, error) {
	fmt.Println("1️⃣  Authentication Method")
	fmt.Println("────────────────────────────────────────")
	fmt.Println()
	fmt.Println("How would you like to authenticate?")
	fmt.Println("  (1) Invite link/token (recommended)")
	fmt.Println("  (2) Manual JWT token")
	fmt.Println("  (3) OIDC (enterprise)")
	fmt.Println()

	choice, err := w.promptInt("Choice", 1, 1, 3)
	if err != nil {
		return "", err
	}

	switch choice {
	case 1:
		return "invite", nil
	case 2:
		return "manual", nil
	case 3:
		return "oidc", nil
	default:
		return "", fmt.Errorf("invalid choice: %d", choice)
	}
}

func (w *SetupWizard) configureViaInvite() (*types.Config, error) {
	fmt.Println()
	fmt.Println("2️⃣  Invite Configuration")
	fmt.Println("────────────────────────────────────────")
	fmt.Println()

	// Get control plane URL
	controlPlaneURL, err := w.promptString("Control Plane URL", "https://edge.2gc.ru")
	if err != nil {
		return nil, err
	}

	// Get invite token
	fmt.Println()
	fmt.Println("Please enter your invite token or URL:")
	fmt.Println("(You should have received this from your administrator)")
	token, err := w.promptString("Invite token/URL", "")
	if err != nil {
		return nil, err
	}

	// Extract token if it's a URL
	token = extractTokenFromInput(token)

	// Use onboarding manager to validate and join
	fmt.Println()
	fmt.Println("🔍 Validating invite token...")

	mgr := onboarding.NewOnboardingManager(controlPlaneURL)
	cfg, joinResp, err := mgr.Join(nil, token)
	if err != nil {
		return nil, fmt.Errorf("failed to join via invite: %w", err)
	}

	fmt.Println("✅ Successfully validated and configured via invite!")
	if joinResp.DeviceID != "" {
		fmt.Printf("   Device ID: %s\n", joinResp.DeviceID)
	}
	if joinResp.TenantID != "" {
		fmt.Printf("   Tenant ID: %s\n", joinResp.TenantID)
	}

	return cfg, nil
}

func (w *SetupWizard) configureManual() (*types.Config, error) {
	fmt.Println()
	fmt.Println("2️⃣  Server Configuration")
	fmt.Println("────────────────────────────────────────")
	fmt.Println()

	host, err := w.promptString("Relay server URL", "edge.2gc.ru")
	if err != nil {
		return nil, err
	}

	port, err := w.promptInt("Relay server port", 5552, 1, 65535)
	if err != nil {
		return nil, err
	}

	useTLS := w.promptYesNo("Use TLS", true)

	fmt.Println()
	fmt.Println("3️⃣  Authentication")
	fmt.Println("────────────────────────────────────────")
	fmt.Println()

	fmt.Println("Enter your JWT token:")
	token, err := w.promptString("JWT Token", "")
	if err != nil {
		return nil, err
	}

	fmt.Println()
	fmt.Println("4️⃣  Identity")
	fmt.Println("────────────────────────────────────────")
	fmt.Println()

	tenantID, err := w.promptString("Tenant ID", "default")
	if err != nil {
		return nil, err
	}

	// Build config
	cfg := &types.Config{
		Relay: types.RelayConfig{
			Host: host,
			Port: port,
			TLS: types.TLSConfig{
				Enabled: useTLS,
				VerifyCert: true,
			},
		},
		Auth: types.AuthConfig{
			Type:  "jwt",
			Token: token,
		},
		P2P: types.P2PConfig{
			MaxConnections: 10,
		},
		WireGuard: types.WireGuardConfig{
			InterfaceName: "cloudbridge0",
			MTU:           1420,
		},
	}

	// Fill in defaults
	fillDefaults(cfg, host, tenantID)

	return cfg, nil
}

func (w *SetupWizard) configureOIDC() (*types.Config, error) {
	fmt.Println()
	fmt.Println("2️⃣  OIDC Configuration")
	fmt.Println("────────────────────────────────────────")
	fmt.Println()

	issuerURL, err := w.promptString("OIDC Issuer URL", "https://auth.2gc.ru")
	if err != nil {
		return nil, err
	}

	audience, err := w.promptString("Audience", "cloudbridge")
	if err != nil {
		return nil, err
	}

	// Create base config
	cfg, err := w.configureManual()
	if err != nil {
		return nil, err
	}

	// Override auth config
	cfg.Auth.Type = "oidc"
	cfg.Auth.OIDC = types.OIDCConfig{
		IssuerURL: issuerURL,
		Audience:  audience,
	}

	return cfg, nil
}

func (w *SetupWizard) configureNetwork(cfg *types.Config) error {
	fmt.Println()
	fmt.Println("🌐 Network Configuration")
	fmt.Println("────────────────────────────────────────")
	fmt.Println()

	interfaceName, err := w.promptString("Interface name", "cloudbridge0")
	if err != nil {
		return err
	}
	cfg.WireGuard.InterfaceName = interfaceName

	mtu, err := w.promptInt("MTU", 1420, 500, 9000)
	if err != nil {
		return err
	}
	cfg.WireGuard.MTU = mtu

	enableWG := w.promptYesNo("Enable WireGuard VPN", true)
	cfg.WireGuard.Enabled = enableWG

	return nil
}

func (w *SetupWizard) configureAdvanced(cfg *types.Config) error {
	fmt.Println()
	fmt.Println("⚙️  Advanced Configuration")
	fmt.Println("────────────────────────────────────────")
	fmt.Println()

	// Logging
	if w.promptYesNo("Configure logging?", false) {
		logLevel, err := w.promptString("Log level (debug/info/warn/error)", "info")
		if err != nil {
			return err
		}
		cfg.Logging.Level = logLevel

		logFormat, err := w.promptString("Log format (json/text)", "json")
		if err != nil {
			return err
		}
		cfg.Logging.Format = logFormat
	}

	// Performance
	if w.promptYesNo("Configure performance tuning?", false) {
		maxConns, err := w.promptInt("Max P2P connections", 10, 1, 1000)
		if err != nil {
			return err
		}
		cfg.P2P.MaxConnections = maxConns
	}

	// Metrics
	if w.promptYesNo("Configure metrics?", false) {
		metricsPort, err := w.promptInt("Metrics port", 9090, 1024, 65535)
		if err != nil {
			return err
		}
		cfg.Metrics.PrometheusPort = metricsPort
		cfg.Metrics.Enabled = true
	}

	return nil
}

func (w *SetupWizard) promptString(prompt string, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Printf("   %s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("   %s: ", prompt)
	}

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue, nil
	}

	return input, nil
}

func (w *SetupWizard) promptInt(prompt string, defaultValue int, min int, max int) (int, error) {
	for {
		input, err := w.promptString(prompt, strconv.Itoa(defaultValue))
		if err != nil {
			return 0, err
		}

		if input == strconv.Itoa(defaultValue) {
			return defaultValue, nil
		}

		val, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("   ⚠️  Invalid number. Please try again.\n")
			continue
		}

		if val < min || val > max {
			fmt.Printf("   ⚠️  Value must be between %d and %d. Please try again.\n", min, max)
			continue
		}

		return val, nil
	}
}

func (w *SetupWizard) promptYesNo(prompt string, defaultValue bool) bool {
	defaultStr := "Y/n"
	if !defaultValue {
		defaultStr = "y/N"
	}

	for {
		fmt.Printf("   %s [%s]: ", prompt, defaultStr)
		input, err := w.reader.ReadString('\n')
		if err != nil {
			fmt.Printf("   ⚠️  Failed to read input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(strings.ToLower(input))

		if input == "" {
			return defaultValue
		}

		if input == "y" || input == "yes" {
			return true
		}

		if input == "n" || input == "no" {
			return false
		}

		fmt.Println("   ⚠️  Please answer 'y' or 'n'")
	}
}

func extractTokenFromInput(input string) string {
	// Check if input is a URL
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		// Extract token from URL query parameter
		if idx := strings.Index(input, "token="); idx != -1 {
			token := input[idx+6:]
			// Remove any trailing query parameters or fragments
			if idx := strings.IndexAny(token, "&#"); idx != -1 {
				token = token[:idx]
			}
			return token
		}
	}

	// Assume it's already a token
	return input
}

func getDefaultServerID() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return hostname
}

func fillDefaults(cfg *types.Config, host string, tenantID string) {
	// Relay defaults
	if cfg.Relay.TLS.ServerName == "" {
		cfg.Relay.TLS.ServerName = host
	}

	// ICE/STUN/TURN
	if len(cfg.ICE.STUNServers) == 0 {
		cfg.ICE.STUNServers = []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
		}
	}
	if cfg.ICE.Timeout == 0 {
		cfg.ICE.Timeout = 30 * time.Second
	}

	// WebSocket
	if cfg.WebSocket.Endpoint == "" {
		protocol := "wss"
		if !cfg.Relay.TLS.Enabled {
			protocol = "ws"
		}
		cfg.WebSocket.Endpoint = fmt.Sprintf("%s://%s:%d/ws/peers", protocol, host, cfg.Relay.Port)
	}
	if cfg.WebSocket.Timeout == 0 {
		cfg.WebSocket.Timeout = 30 * time.Second
	}
	if cfg.WebSocket.MaxReconnectAttempts == 0 {
		cfg.WebSocket.MaxReconnectAttempts = 10
	}
	cfg.WebSocket.Enabled = true

	// API
	protocol := "https"
	if !cfg.Relay.TLS.Enabled {
		protocol = "http"
	}
	if cfg.API.BaseURL == "" {
		cfg.API.BaseURL = fmt.Sprintf("%s://%s:%d", protocol, host, cfg.Relay.Port)
	}
	if cfg.API.P2PAPIURL == "" {
		cfg.API.P2PAPIURL = fmt.Sprintf("%s://%s:%d/api", protocol, host, cfg.Relay.Port)
	}
	if cfg.API.HeartbeatURL == "" {
		cfg.API.HeartbeatURL = fmt.Sprintf("%s://%s:%d/api/v1/tenants/%s/peers/register", protocol, host, cfg.Relay.Port, tenantID)
	}
	if cfg.API.Timeout == 0 {
		cfg.API.Timeout = 30 * time.Second
	}

	// Logging
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
	if cfg.Logging.Output == "" {
		cfg.Logging.Output = "stdout"
	}

	// P2P defaults
	if cfg.P2P.MaxConnections == 0 {
		cfg.P2P.MaxConnections = 10
	}
	if cfg.P2P.PeerDiscoveryInterval == 0 {
		cfg.P2P.PeerDiscoveryInterval = 30 * time.Second
	}
	if cfg.P2P.HeartbeatInterval == 0 {
		cfg.P2P.HeartbeatInterval = 30 * time.Second
	}

	// WireGuard defaults
	if cfg.WireGuard.MTU == 0 {
		cfg.WireGuard.MTU = 1420
	}

	// Metrics defaults
	if cfg.Metrics.PrometheusPort == 0 {
		cfg.Metrics.PrometheusPort = 9090
	}
}

func getDefaultLogPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join("C:", "ProgramData", "cloudbridge-client", "logs", "cloudbridge-client.log")
	case "darwin":
		return "/var/log/cloudbridge-client/cloudbridge-client.log"
	default:
		return "/var/log/cloudbridge-client/cloudbridge-client.log"
	}
}
