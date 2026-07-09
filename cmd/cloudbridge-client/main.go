package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/spf13/cobra" // Required for CLI interface
	"github.com/twogc/cloudbridge-client/pkg/api"
	"github.com/twogc/cloudbridge-client/pkg/auth"
	"github.com/twogc/cloudbridge-client/pkg/config"
	"github.com/twogc/cloudbridge-client/pkg/errors"
	"github.com/twogc/cloudbridge-client/pkg/p2p"
	"github.com/twogc/cloudbridge-client/pkg/relay"
	"github.com/twogc/cloudbridge-client/pkg/service"
	"github.com/twogc/cloudbridge-client/pkg/types"
	"github.com/twogc/cloudbridge-client/pkg/utils"
)

// Build-time variables (set via ldflags)
var (
	version       string = "dev"
	buildType     string = "unknown"
	buildOS       string = "unknown"
	buildArch     string = "unknown"
	buildTime     string = "unknown"
	jwtSecret     string = ""
	buildAPIBase  string = ""
	buildTenantID string = ""
)

var (
	configFile string
	token      string
	caPath     string
	tunnelID   string
	localPort  int
	remoteHost string
	remotePort int
	verbose    bool

	// P2P Mesh specific flags
	p2pMode bool
	peerID  string
	// smoke: exit after control-plane mesh membership is ready (register + optional checks)
	p2pSmoke     bool
	p2pSmokeWait time.Duration
	// smoke-data: after membership, also verify QUIC AUTH + PING/PONG (data plane)
	p2pSmokeData bool
	// tunnel --smoke: create tunnel, push bytes via local port, exit
	tunnelSmoke bool

	// HTTP API specific flags
	insecureSkipTLSVerify bool
	logLevel              string
	transportMode         string
)

func main() {
	// Ensure cobra is used to prevent go mod tidy from removing it
	_ = cobra.Command{}

	rootCmd := &cobra.Command{
		Use:   "cloudbridge-client",
		Short: "CloudBridge Relay Client",
		Long: "A cross-platform client for CloudBridge Relay with TLS 1.3 support, " +
			"JWT authentication, and QUIC + ICE/STUN/TURN P2P mesh networking",
		RunE: run,
	}

	// Add version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			showVersion()
		},
	})

	// Add basic flags as persistent flags so they're available to subcommands
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().StringVarP(&token, "token", "t", "", "JWT token for authentication")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	// Custom CA path for TLS root trust
	rootCmd.PersistentFlags().StringVar(&caPath, "ca", os.Getenv("CLOUDBRIDGE_CA"), "Path to custom Root CA PEM file (env CLOUDBRIDGE_CA)")

	// Tunnel mode flags
	rootCmd.Flags().StringVarP(&tunnelID, "tunnel-id", "i", "tunnel_001", "Tunnel ID")
	rootCmd.Flags().IntVarP(&localPort, "local-port", "l", 3389, "Local port to bind")
	rootCmd.Flags().StringVarP(&remoteHost, "remote-host", "r", "192.168.1.100", "Remote host")
	rootCmd.Flags().IntVarP(&remotePort, "remote-port", "p", 3389, "Remote port")

	// P2P Mesh mode flags
	rootCmd.Flags().BoolVar(&p2pMode, "p2p", false, "Enable P2P mesh mode")
	rootCmd.Flags().StringVar(&peerID, "peer-id", "", "Peer ID for P2P mesh")

	// HTTP API flags (URLs are hardcoded in the code)
	rootCmd.PersistentFlags().BoolVar(&insecureSkipTLSVerify, "insecure-skip-tls-verify", false,
		"Skip TLS certificate verification (dev only)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&transportMode, "transport", "grpc", "Transport mode (grpc, json)")

	// Note: token flag is checked in validateFlags() function instead of marking it required
	// This allows version and help commands to work without requiring a token

	// Add subcommands
	rootCmd.AddCommand(createP2PCommand())
	rootCmd.AddCommand(createTunnelCommand())
	rootCmd.AddCommand(createServiceCommand())
	rootCmd.AddCommand(createWireGuardCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Log platform information
	log.Printf("Running on %s/%s", runtime.GOOS, runtime.GOARCH)

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override config with command line flags if provided
	if token != "" {
		cfg.Auth.Token = token // For JWT auth, token is the JWT token
	}
	// API URLs are hardcoded in the code
	if insecureSkipTLSVerify {
		cfg.API.InsecureSkipVerify = insecureSkipTLSVerify
	}
	if logLevel != "" {
		cfg.Logging.Level = logLevel
	}
	// Apply custom CA if provided
	if caPath != "" {
		cfg.Relay.TLS.CACert = caPath
	}

	// Create client
	client, err := relay.NewClient(cfg, configFile)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Validate CLI flags for incompatible modes
	if err := validateFlags(cfg, transportMode); err != nil {
		return fmt.Errorf("invalid flag combination: %w", err)
	}

	// Set transport mode if specified
	if transportMode != "" {
		if err := client.SetTransportMode(transportMode); err != nil {
			return fmt.Errorf("failed to set transport mode: %w", err)
		}
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Failed to close client: %v", err)
		}
	}()

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use the utility function for cross-platform signal handling
	sigChan := utils.SetupSignalHandler()

	// Start connection with retry logic
	if err := connectWithRetry(client); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	log.Printf("Successfully connected to relay server %s:%d", cfg.Relay.Host, cfg.Relay.Port)

	// Authenticate
	if err := authenticateWithRetry(client, token); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	log.Printf("Successfully authenticated with client ID: %s", client.GetClientID())

	// Create tunnel
	if err := createTunnelWithRetry(client, tunnelID, localPort, remoteHost, remotePort); err != nil {
		return fmt.Errorf("failed to create tunnel: %w", err)
	}

	log.Printf("Successfully created tunnel %s: localhost:%d -> %s:%d",
		tunnelID, localPort, remoteHost, remotePort)

	// Start heartbeat
	if err := client.StartHeartbeat(); err != nil {
		return fmt.Errorf("failed to start heartbeat: %w", err)
	}

	log.Printf("Heartbeat started")
	log.Printf("Press Ctrl+C to stop the client gracefully")

	// Wait for shutdown signal
	select {
	case <-sigChan:
		log.Println("Received shutdown signal (Ctrl+C), closing gracefully...")
	case <-ctx.Done():
		log.Println("Context canceled, closing...")
	}

	return nil
}

// connectWithRetry connects to the relay server with retry logic
func connectWithRetry(client *relay.Client) error {
	retryStrategy := client.GetRetryStrategy()

	for {
		err := client.Connect()
		if err == nil {
			return nil
		}

		relayErr, handleErr := errors.HandleError(err)
		if handleErr != nil {
			log.Printf("Error handling error: %v", handleErr)
		}
		if relayErr == nil || !retryStrategy.ShouldRetry(err) {
			return err
		}

		delay := retryStrategy.GetNextDelay(err)
		log.Printf("Connection failed: %v, retrying in %v...", err, delay)
		time.Sleep(delay)
	}
}

// authenticateWithRetry authenticates with retry logic
func authenticateWithRetry(client *relay.Client, token string) error {
	retryStrategy := client.GetRetryStrategy()

	for {
		err := client.Authenticate(token)
		if err == nil {
			return nil
		}

		relayErr, handleErr := errors.HandleError(err)
		if handleErr != nil {
			log.Printf("Error handling error: %v", handleErr)
		}
		if relayErr == nil || !retryStrategy.ShouldRetry(err) {
			return err
		}

		delay := retryStrategy.GetNextDelay(err)
		log.Printf("Authentication failed: %v, retrying in %v...", err, delay)
		time.Sleep(delay)
	}
}

// createTunnelWithRetry creates a tunnel with retry logic
func createTunnelWithRetry(client *relay.Client, tunnelID string, localPort int, remoteHost string, remotePort int) error {
	retryStrategy := client.GetRetryStrategy()

	for {
		err := client.CreateTunnel(tunnelID, localPort, remoteHost, remotePort)
		if err == nil {
			return nil
		}

		relayErr, handleErr := errors.HandleError(err)
		if handleErr != nil {
			log.Printf("Error handling error: %v", handleErr)
		}
		if relayErr == nil || !retryStrategy.ShouldRetry(err) {
			return err
		}

		delay := retryStrategy.GetNextDelay(err)
		log.Printf("Tunnel creation failed: %v, retrying in %v...", err, delay)
		time.Sleep(delay)
	}
}

// createP2PCommand creates the P2P mesh subcommand
func createP2PCommand() *cobra.Command {
	p2pCmd := &cobra.Command{
		Use:   "p2p",
		Short: "Start P2P mesh networking",
		Long:  "Connect to P2P mesh network using QUIC + ICE/STUN/TURN",
		RunE:  runP2P,
	}

	// P2P specific flags
	p2pCmd.Flags().StringVar(&peerID, "peer-id", "", "Peer ID for P2P mesh (optional, auto-generated if not provided)")
	p2pCmd.Flags().BoolVar(&p2pSmoke, "smoke", false,
		"Smoke mode: start mesh membership then exit (CI/local loop)")
	p2pCmd.Flags().BoolVar(&p2pSmokeData, "smoke-data", false,
		"Also verify QUIC data plane (AUTH + PING/PONG) after membership; implies --smoke")
	p2pCmd.Flags().DurationVar(&p2pSmokeWait, "smoke-wait", 45*time.Second,
		"Max time to wait for P2P Start in --smoke / --smoke-data mode")

	return p2pCmd
}

// createTunnelCommand creates the tunnel subcommand
func createTunnelCommand() *cobra.Command {
	tunnelCmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Start tunnel mode",
		Long:  "Create a tunnel to remote host (localPort → relay CreateTunnel endpoint → remote)",
		RunE:  runTunnel,
	}

	// Tunnel specific flags
	tunnelCmd.Flags().StringVarP(&tunnelID, "tunnel-id", "i", "tunnel_001", "Tunnel ID")
	tunnelCmd.Flags().IntVarP(&localPort, "local-port", "l", 3389, "Local port to bind on this host")
	tunnelCmd.Flags().StringVarP(&remoteHost, "remote-host", "r", "192.168.1.100", "Remote host (from relay)")
	tunnelCmd.Flags().IntVarP(&remotePort, "remote-port", "p", 3389, "Remote port")
	tunnelCmd.Flags().BoolVar(&tunnelSmoke, "smoke", false,
		"Smoke mode: create tunnel, send/receive bytes via local port, then exit")

	return tunnelCmd
}

// createServiceCommand creates the service management subcommand
func createServiceCommand() *cobra.Command {
	svcCmd := &cobra.Command{
		Use:   "service",
		Short: "Manage CloudBridge Client service",
		Long:  "Install, uninstall, start, stop, restart, or check status of CloudBridge Client service",
	}

	// Install service command
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install CloudBridge Client as a service",
		Long:  "Install CloudBridge Client as a system service with auto-start",
		RunE:  runServiceInstall,
	}
	installCmd.Flags().StringVarP(&configFile, "config", "c", "", "Configuration file path")
	installCmd.Flags().StringVarP(&token, "token", "t", "", "JWT token for authentication")
	if err := installCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag required: %v\n", err)
		os.Exit(1)
	}
	if err := installCmd.MarkFlagRequired("token"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag required: %v\n", err)
		os.Exit(1)
	}

	// Uninstall service command
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall CloudBridge Client service",
		Long:  "Remove CloudBridge Client service from the system",
		RunE:  runServiceUninstall,
	}

	// Start service command
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start CloudBridge Client service",
		Long:  "Start the CloudBridge Client service",
		RunE:  runServiceStart,
	}

	// Stop service command
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop CloudBridge Client service",
		Long:  "Stop the CloudBridge Client service",
		RunE:  runServiceStop,
	}

	// Restart service command
	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart CloudBridge Client service",
		Long:  "Restart the CloudBridge Client service",
		RunE:  runServiceRestart,
	}

	// Status service command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check CloudBridge Client service status",
		Long:  "Check the current status of CloudBridge Client service",
		RunE:  runServiceStatus,
	}

	svcCmd.AddCommand(installCmd)
	svcCmd.AddCommand(uninstallCmd)
	svcCmd.AddCommand(startCmd)
	svcCmd.AddCommand(stopCmd)
	svcCmd.AddCommand(restartCmd)
	svcCmd.AddCommand(statusCmd)

	return svcCmd
}

// createWireGuardCommand creates the WireGuard subcommand
func createWireGuardCommand() *cobra.Command {
	wgCmd := &cobra.Command{
		Use:   "wireguard",
		Short: "Manage WireGuard L3-overlay network",
		Long:  "Get WireGuard configuration for L3-overlay network",
	}

	// Get config command
	getConfigCmd := &cobra.Command{
		Use:   "config",
		Short: "Get WireGuard configuration",
		Long:  "Get WireGuard client configuration for L3-overlay network",
		RunE:  runWireGuardConfig,
	}

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check WireGuard status",
		Long:  "Check WireGuard L3-overlay network status",
		RunE:  runWireGuardStatus,
	}

	wgCmd.AddCommand(getConfigCmd)
	wgCmd.AddCommand(statusCmd)

	return wgCmd
}

// runWireGuardConfig gets WireGuard configuration
func runWireGuardConfig(cmd *cobra.Command, args []string) error {
	log.Printf("Getting WireGuard configuration for L3-overlay network...")

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get token from config or command line
	tokenToUse := token
	if tokenToUse == "" {
		tokenToUse = cfg.Auth.Token
	}

	if tokenToUse == "" {
		return fmt.Errorf("JWT token is required (use --token flag or set auth.token in config)")
	}

	// Create authentication manager
	authManager, err := auth.NewAuthManager(&auth.AuthConfig{
		Type:           cfg.Auth.Type,
		Secret:         cfg.Auth.Secret,
		FallbackSecret: cfg.Auth.FallbackSecret,
		SkipValidation: cfg.Auth.SkipValidation,
		OIDC: &auth.OIDCConfig{
			IssuerURL: cfg.Auth.OIDC.IssuerURL,
			Audience:  cfg.Auth.OIDC.Audience,
			JWKSURL:   cfg.Auth.OIDC.JWKSURL,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create auth manager: %w", err)
	}

	// Validate JWT token
	validatedToken, err := authManager.ValidateToken(tokenToUse)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	// Extract P2P configuration from JWT token
	p2pConfig, err := p2p.ExtractP2PConfigFromToken(authManager, validatedToken)
	if err != nil {
		return fmt.Errorf("failed to extract P2P config from token: %w", err)
	}

	// Create API manager configuration
	apiConfig := &api.ManagerConfig{
		BaseURL:            cfg.API.BaseURL,
		InsecureSkipVerify: cfg.API.InsecureSkipVerify,
		Timeout:            cfg.API.Timeout,
		MaxRetries:         cfg.API.MaxRetries,
		BackoffMultiplier:  cfg.API.BackoffMultiplier,
		MaxBackoff:         cfg.API.MaxBackoff,
		Token:              tokenToUse,
		TenantID:           p2pConfig.TenantID,
		HeartbeatInterval:  30 * time.Second,
	}

	// Create P2P logger
	p2pLogger := &p2pLogger{}

	// Create P2P manager with HTTP API support
	p2pManager := p2p.NewManagerWithAPI(p2pConfig, apiConfig, authManager, tokenToUse, p2pLogger)

	// Get WireGuard configuration
	config, err := p2pManager.GetWireGuardConfig()
	if err != nil {
		return fmt.Errorf("failed to get WireGuard config: %w", err)
	}

	// Display configuration
	fmt.Printf("WireGuard Configuration:\n")
	fmt.Printf("=======================\n")
	fmt.Printf("Success: %t\n", config.Success)
	fmt.Printf("Message: %s\n", config.Message)
	fmt.Printf("Peer IP: %s\n", config.PeerIP)
	fmt.Printf("Tenant CIDR: %s\n", config.TenantCIDR)
	fmt.Printf("\nClient Configuration:\n")
	fmt.Printf("---------------------\n")
	fmt.Printf("%s\n", config.ClientConfig)

	return nil
}

// runWireGuardStatus checks WireGuard status
func runWireGuardStatus(cmd *cobra.Command, args []string) error {
	log.Printf("Checking WireGuard L3-overlay network status...")

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get token from config or command line
	tokenToUse := token
	if tokenToUse == "" {
		tokenToUse = cfg.Auth.Token
	}

	if tokenToUse == "" {
		return fmt.Errorf("JWT token is required (use --token flag or set auth.token in config)")
	}

	// Create authentication manager
	authManager, err := auth.NewAuthManager(&auth.AuthConfig{
		Type:           cfg.Auth.Type,
		Secret:         cfg.Auth.Secret,
		FallbackSecret: cfg.Auth.FallbackSecret,
		SkipValidation: cfg.Auth.SkipValidation,
		OIDC: &auth.OIDCConfig{
			IssuerURL: cfg.Auth.OIDC.IssuerURL,
			Audience:  cfg.Auth.OIDC.Audience,
			JWKSURL:   cfg.Auth.OIDC.JWKSURL,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create auth manager: %w", err)
	}

	// Validate JWT token
	validatedToken, err := authManager.ValidateToken(tokenToUse)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	// Extract P2P configuration from JWT token
	p2pConfig, err := p2p.ExtractP2PConfigFromToken(authManager, validatedToken)
	if err != nil {
		return fmt.Errorf("failed to extract P2P config from token: %w", err)
	}

	// Create API manager configuration
	apiConfig := &api.ManagerConfig{
		BaseURL:            cfg.API.BaseURL,
		InsecureSkipVerify: cfg.API.InsecureSkipVerify,
		Timeout:            cfg.API.Timeout,
		MaxRetries:         cfg.API.MaxRetries,
		BackoffMultiplier:  cfg.API.BackoffMultiplier,
		MaxBackoff:         cfg.API.MaxBackoff,
		Token:              tokenToUse,
		TenantID:           p2pConfig.TenantID,
		HeartbeatInterval:  30 * time.Second,
	}

	// Create P2P logger
	p2pLogger := &p2pLogger{}

	// Create P2P manager with HTTP API support
	p2pManager := p2p.NewManagerWithAPI(p2pConfig, apiConfig, authManager, tokenToUse, p2pLogger)

	// Get status
	status := p2pManager.GetStatus()

	// Display status
	fmt.Printf("WireGuard L3-overlay Network Status:\n")
	fmt.Printf("====================================\n")
	fmt.Printf("L3 Overlay Ready: %t\n", status.L3OverlayReady)
	fmt.Printf("WireGuard Ready: %t\n", status.WireGuardReady)
	fmt.Printf("Peer IP: %s\n", status.PeerIP)
	fmt.Printf("Tenant CIDR: %s\n", status.TenantCIDR)
	fmt.Printf("Connection Type: %s\n", status.ConnectionType)
	fmt.Printf("Mesh Enabled: %t\n", status.MeshEnabled)
	fmt.Printf("Active Connections: %d\n", status.ActiveConnections)

	return nil
}

// runServiceInstall installs the service
func runServiceInstall(cmd *cobra.Command, args []string) error {
	log.Printf("Installing CloudBridge Client service...")

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create service configuration with token
	if err := createServiceConfig(configFile, token); err != nil {
		return fmt.Errorf("failed to create service config: %w", err)
	}

	// Install service
	if err := service.Install(execPath); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	log.Printf("Service installed successfully")
	return nil
}

// runServiceUninstall uninstalls the service
func runServiceUninstall(cmd *cobra.Command, args []string) error {
	log.Printf("Uninstalling CloudBridge Client service...")

	if err := service.Uninstall(); err != nil {
		return fmt.Errorf("failed to uninstall service: %w", err)
	}

	log.Printf("Service uninstalled successfully")
	return nil
}

// runServiceStart starts the service
func runServiceStart(cmd *cobra.Command, args []string) error {
	log.Printf("Starting CloudBridge Client service...")

	if err := service.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	log.Printf("Service started successfully")
	return nil
}

// runServiceStop stops the service
func runServiceStop(cmd *cobra.Command, args []string) error {
	log.Printf("Stopping CloudBridge Client service...")

	if err := service.Stop(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	log.Printf("Service stopped successfully")
	return nil
}

// runServiceRestart restarts the service
func runServiceRestart(cmd *cobra.Command, args []string) error {
	log.Printf("Restarting CloudBridge Client service...")

	if err := service.Restart(); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	log.Printf("Service restarted successfully")
	return nil
}

// runServiceStatus checks the service status
func runServiceStatus(cmd *cobra.Command, args []string) error {
	status, err := service.Status()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	fmt.Printf("Service status: %s\n", status)
	return nil
}

// createServiceConfig creates a service-specific configuration file
func createServiceConfig(configPath, token string) error {
	// Load base configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load base configuration: %w", err)
	}

	// Set token in configuration
	cfg.Auth.Secret = token

	// Create service config directory
	serviceConfigDir := "/etc/cloudbridge-client"
	if runtime.GOOS == types.PlatformWindows {
		serviceConfigDir = filepath.Join(os.Getenv("ProgramData"), "cloudbridge-client")
	}

	if err := os.MkdirAll(serviceConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create service config directory: %w", err)
	}

	// Write service configuration
	serviceConfigPath := filepath.Join(serviceConfigDir, "config.yaml")
	// Note: In a real implementation, you would marshal the config to YAML
	// For now, we'll copy the original config and modify it
	if err := copyFile(configPath, serviceConfigPath); err != nil {
		return fmt.Errorf("failed to copy configuration: %w", err)
	}

	log.Printf("Service configuration created at: %s", serviceConfigPath)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644) //nolint:gosec // Config files need readable permissions
}

// p2pLogger implements the p2p.Logger interface
type p2pLogger struct{}

func (pl *p2pLogger) Info(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		log.Printf("[P2P] INFO: %s %v", msg, fields)
	} else {
		log.Printf("[P2P] INFO: %s", msg)
	}
}

func (pl *p2pLogger) Error(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		log.Printf("[P2P] ERROR: %s %v", msg, fields)
	} else {
		log.Printf("[P2P] ERROR: %s", msg)
	}
}

func (pl *p2pLogger) Debug(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		log.Printf("[P2P] DEBUG: %s %v", msg, fields)
	} else {
		log.Printf("[P2P] DEBUG: %s", msg)
	}
}

func (pl *p2pLogger) Warn(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		log.Printf("[P2P] WARN: %s %v", msg, fields)
	} else {
		log.Printf("[P2P] WARN: %s", msg)
	}
}

// showVersion displays version information
func showVersion() {
	fmt.Printf("CloudBridge Client\n")
	fmt.Printf("==================\n")
	fmt.Printf("Version:     %s\n", version)
	fmt.Printf("Build Type:  %s\n", buildType)
	fmt.Printf("Build OS:    %s\n", buildOS)
	fmt.Printf("Build Arch:  %s\n", buildArch)
	fmt.Printf("Build Time:  %s\n", buildTime)
	fmt.Printf("Go Version:  %s\n", runtime.Version())
	fmt.Printf("Go OS/Arch:  %s/%s\n", runtime.GOOS, runtime.GOARCH)

	if buildType != "production" {
		fmt.Printf("\nBuild Configuration:\n")
		if jwtSecret != "" {
			fmt.Printf("JWT Secret:     %s...\n", jwtSecret[:minInt(8, len(jwtSecret))])
		}
		if buildAPIBase != "" {
			fmt.Printf("API Base:       %s\n", buildAPIBase)
		}
		if buildTenantID != "" {
			fmt.Printf("Tenant ID:      %s\n", buildTenantID)
		}
	}
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runP2P runs the P2P mesh mode
func runP2P(cmd *cobra.Command, args []string) error {
	log.Printf("Starting P2P mesh mode with QUIC + ICE/STUN/TURN...")

	// --smoke-data implies membership smoke + data-plane check
	if p2pSmokeData {
		p2pSmoke = true
	}

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override config with command line flags if provided
	// API URLs are hardcoded in the code
	if insecureSkipTLSVerify {
		cfg.API.InsecureSkipVerify = insecureSkipTLSVerify
	}
	if logLevel != "" {
		cfg.Logging.Level = logLevel
	}
	// Apply custom CA if provided
	if caPath != "" {
		cfg.Relay.TLS.CACert = caPath
	}

	// Generate peer ID if not provided
	if peerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		peerID = fmt.Sprintf("peer-%s", hostname)
	}

	log.Printf("Peer ID: %s", peerID)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use the utility function for cross-platform signal handling
	sigChan := utils.SetupSignalHandler()

	// Create authentication manager for JWT validation
	authManager, err := auth.NewAuthManager(&auth.AuthConfig{
		Type:           cfg.Auth.Type,
		Secret:         cfg.Auth.Secret,
		FallbackSecret: cfg.Auth.FallbackSecret,
		SkipValidation: cfg.Auth.SkipValidation,
		OIDC: &auth.OIDCConfig{
			IssuerURL: cfg.Auth.OIDC.IssuerURL,
			Audience:  cfg.Auth.OIDC.Audience,
			JWKSURL:   cfg.Auth.OIDC.JWKSURL,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create auth manager: %w", err)
	}

	// Get token from config or command line
	tokenToUse := token
	if tokenToUse == "" {
		tokenToUse = cfg.Auth.Token
	}

	// Validate JWT token
	validatedToken, err := authManager.ValidateToken(tokenToUse)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	// Extract P2P configuration from JWT token
	p2pConfig, err := p2p.ExtractP2PConfigFromToken(authManager, validatedToken)
	if err != nil {
		return fmt.Errorf("failed to extract P2P config from token: %w", err)
	}

	// Create API manager configuration
	apiConfig := &api.ManagerConfig{
		BaseURL:            cfg.API.BaseURL,
		HeartbeatURL:       cfg.API.HeartbeatURL,
		InsecureSkipVerify: cfg.API.InsecureSkipVerify,
		Timeout:            cfg.API.Timeout,
		MaxRetries:         cfg.API.MaxRetries,
		BackoffMultiplier:  cfg.API.BackoffMultiplier,
		MaxBackoff:         cfg.API.MaxBackoff,
		Token:              tokenToUse,
		TenantID:           p2pConfig.TenantID,
		HeartbeatInterval:  cfg.P2P.HeartbeatInterval,
	}

	// Create P2P logger
	p2pLogger := &p2pLogger{}

	// Create P2P manager with HTTP API support
	p2pManager := p2p.NewManagerWithAPI(p2pConfig, apiConfig, authManager, tokenToUse, p2pLogger)
	// Dial P2P QUIC to relay using canonical ports from config (docs/CONTRACT_CLIENT_RELAY.md)
	p2pManager.SetRelayQUICEndpoint(cfg.Relay.Host, cfg.EffectiveP2PQUICPort())
	if p2pSmoke {
		// Membership smoke: register + heartbeat + discovery without ICE/QUIC hang risks.
		// Data plane is verified separately via smokeQUICDataPlane when --smoke-data.
		p2pManager.SetSkipDataPlane(true)
	}

	// Start P2P mesh with retry to survive temporary relay outages
	startDeadline := time.Time{}
	if p2pSmoke {
		startDeadline = time.Now().Add(p2pSmokeWait)
	}
	{
		backoff := 1 * time.Second
		maxBackoff := 30 * time.Second
		for {
			if p2pSmoke && time.Now().After(startDeadline) {
				return fmt.Errorf("smoke: P2P Start did not succeed within %s", p2pSmokeWait)
			}
			if err := p2pManager.Start(); err != nil {
				log.Printf("Failed to start P2P mesh: %v, retrying in %v...", err, backoff)
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
			break
		}
	}

	defer func() {
		if err := p2pManager.Stop(); err != nil {
			log.Printf("Failed to stop P2P manager: %v", err)
		}
	}()

	log.Printf("P2P mesh started successfully")
	livePeerID := peerID
	if st := p2pManager.GetStatus(); st != nil {
		log.Printf("P2P status: connected=%v active_peers=%d total_peers=%d",
			st.IsConnected, st.ActivePeers, st.TotalPeers)
	}
	log.Printf("SMOKE_STEP=start_ok peer_id=%s", livePeerID)

	// Check L3-overlay network status
	if p2pManager.IsL3OverlayReady() {
		log.Printf("L3-overlay network ready: Peer IP=%s, Tenant CIDR=%s",
			p2pManager.GetPeerIP(), p2pManager.GetTenantCIDR())

		// Display WireGuard configuration
		if config := p2pManager.GetWireGuardConfigString(); config != "" {
			log.Printf("WireGuard configuration available (length: %d chars)", len(config))
		}
	} else {
		log.Printf("L3-overlay network not ready yet")
	}

	if p2pSmoke {
		log.Printf("SMOKE_STEP=control_plane_ok peer=%s", livePeerID)

		if p2pSmokeData {
			quicHost := cfg.Relay.Host
			if quicHost == "" {
				quicHost = "127.0.0.1"
			}
			quicPort := cfg.EffectiveP2PQUICPort()
			if err := smokeQUICDataPlane(quicHost, quicPort, tokenToUse, livePeerID); err != nil {
				// Hard-fail by default when --smoke-data is requested.
				// Soft-fail only if SMOKE_DATA_SOFT=1 (CI can opt into warn-only).
				if soft := os.Getenv("SMOKE_DATA_SOFT"); soft == "1" || strings.EqualFold(soft, "true") {
					log.Printf("SMOKE_WARN data_plane_failed soft=%v err=%v", soft, err)
					log.Printf("SMOKE_PASS=1 peer=%s control_plane=ok data_plane=soft_fail", livePeerID)
					return nil
				}
				return fmt.Errorf("smoke-data: QUIC data plane failed: %w", err)
			}
			log.Printf("SMOKE_PASS=1 peer=%s control_plane=ok data_plane=ok", livePeerID)
		} else {
			log.Printf("SMOKE_PASS=1 peer=%s control_plane=ok", livePeerID)
		}
		log.Printf("Smoke mode complete — exiting successfully")
		return nil
	}

	log.Printf("Press Ctrl+C to stop the client gracefully")

	// Wait for shutdown signal
	select {
	case <-sigChan:
		log.Println("Received shutdown signal (Ctrl+C), closing gracefully...")
	case <-ctx.Done():
		log.Println("Context canceled, closing...")
	}

	return nil
}

// runTunnel runs the tunnel mode
func runTunnel(cmd *cobra.Command, args []string) error {
	log.Printf("Starting tunnel mode...")
	log.Printf("Tunnel ID: %s", tunnelID)
	log.Printf("Local Port: %d", localPort)
	log.Printf("Remote Host: %s", remoteHost)
	log.Printf("Remote Port: %d", remotePort)
	if tunnelSmoke {
		log.Printf("Smoke mode: will verify bytes then exit")
	}

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override config with CLI flags
	if token != "" {
		cfg.Auth.Token = token
	}
	if insecureSkipTLSVerify {
		cfg.API.InsecureSkipVerify = true
		cfg.Relay.TLS.VerifyCert = false
	}
	if logLevel != "" {
		cfg.Logging.Level = logLevel
	}
	if caPath != "" {
		cfg.Relay.TLS.CACert = caPath
	}

	// Create client
	client, err := relay.NewClient(cfg, configFile)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Failed to close client: %v", err)
		}
	}()

	if err := validateFlags(cfg, transportMode); err != nil {
		return fmt.Errorf("invalid flag combination: %w", err)
	}
	if transportMode != "" {
		if err := client.SetTransportMode(transportMode); err != nil {
			return fmt.Errorf("failed to set transport mode: %w", err)
		}
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use the utility function for cross-platform signal handling
	sigChan := utils.SetupSignalHandler()

	// Connect to relay server
	if err := connectWithRetry(client); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	log.Printf("Successfully connected to relay server %s:%d (grpc target ready)", cfg.Relay.Host, cfg.Relay.Port)

	authToken := token
	if authToken == "" {
		authToken = cfg.Auth.Token
	}
	// gRPC metadata also reads CLOUDBRIDGE_TOKEN
	if authToken != "" {
		_ = os.Setenv("CLOUDBRIDGE_TOKEN", authToken)
	}

	// Authenticate
	if err := authenticateWithRetry(client, authToken); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	log.Printf("Successfully authenticated with client ID: %s", client.GetClientID())

	// Unique tunnel id per smoke run if default
	if tunnelSmoke && tunnelID == "tunnel_001" {
		tunnelID = fmt.Sprintf("cli-tun-%d", time.Now().UnixNano())
	}

	// Create tunnel
	if err := createTunnelWithRetry(client, tunnelID, localPort, remoteHost, remotePort); err != nil {
		return fmt.Errorf("failed to create tunnel: %w", err)
	}

	log.Printf("Successfully created tunnel %s: localhost:%d -> (relay) -> %s:%d",
		tunnelID, localPort, remoteHost, remotePort)

	// Local listener needs a brief moment to bind
	time.Sleep(100 * time.Millisecond)

	if tunnelSmoke {
		if err := smokeTunnelBytes(localPort); err != nil {
			return fmt.Errorf("tunnel smoke bytes failed: %w", err)
		}
		log.Printf("TUNNEL_CLI_SMOKE_PASS=1 tunnel_id=%s local_port=%d remote=%s:%d",
			tunnelID, localPort, remoteHost, remotePort)
		fmt.Println("CLI tunnel e2e: localPort → relay → remote bytes OK")
		return nil
	}

	// Start heartbeat
	if err := client.StartHeartbeat(); err != nil {
		return fmt.Errorf("failed to start heartbeat: %w", err)
	}

	log.Printf("Heartbeat started")
	log.Printf("Press Ctrl+C to stop the client gracefully")

	// Wait for shutdown signal
	select {
	case <-sigChan:
		log.Println("Received shutdown signal (Ctrl+C), closing gracefully...")
	case <-ctx.Done():
		log.Println("Context canceled, closing...")
	}

	return nil
}

// smokeQUICDataPlane dials relay QUIC, AUTH with JWT, then PING/PONG on a second stream.
// Used by p2p --smoke-data (membership already verified; skipDataPlane still on).
func smokeQUICDataPlane(host string, quicPort int, jwtToken, peerHint string) error {
	if quicPort <= 0 {
		quicPort = types.DefaultP2PQUICPort
	}
	if host == "" {
		host = "127.0.0.1"
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", quicPort))
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	tlsConf := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		NextProtos:         []string{"cloudbridge-p2p", "h3"},
		ServerName:         "localhost",
	}
	if host != "127.0.0.1" && host != "localhost" {
		tlsConf.ServerName = host
	}
	qconf := &quic.Config{
		HandshakeIdleTimeout:  8 * time.Second,
		MaxIdleTimeout:        20 * time.Second,
		KeepAlivePeriod:       10 * time.Second,
		MaxIncomingStreams:    64,
		MaxIncomingUniStreams: 64,
	}

	log.Printf("SMOKE_STEP=quic_dial addr=%s peer_hint=%s", addr, peerHint)
	conn, err := quic.DialAddr(ctx, addr, tlsConf, qconf)
	if err != nil {
		return fmt.Errorf("quic dial %s: %w", addr, err)
	}
	defer conn.CloseWithError(0, "smoke-data done")

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("open auth stream: %w", err)
	}
	authMsg := "AUTH " + jwtToken
	if _, err := stream.Write([]byte(authMsg)); err != nil {
		_ = stream.Close()
		return fmt.Errorf("write AUTH: %w", err)
	}
	_ = stream.SetReadDeadline(time.Now().Add(8 * time.Second))
	buf := make([]byte, 256)
	n, err := stream.Read(buf)
	if err != nil {
		_ = stream.Close()
		return fmt.Errorf("read AUTH response: %w", err)
	}
	resp := string(buf[:n])
	if resp != "AUTH_OK" {
		_ = stream.Close()
		return fmt.Errorf("expected AUTH_OK got %q", resp)
	}
	_ = stream.Close()
	log.Printf("SMOKE_STEP=quic_auth_ok")

	time.Sleep(50 * time.Millisecond)
	dataStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("open data stream: %w", err)
	}
	defer dataStream.Close()

	payload := "p2p-smoke-data"
	pingMsg := "PING " + payload
	if _, err := dataStream.Write([]byte(pingMsg)); err != nil {
		return fmt.Errorf("write PING: %w", err)
	}
	_ = dataStream.SetReadDeadline(time.Now().Add(8 * time.Second))
	n, err = dataStream.Read(buf)
	if err != nil {
		return fmt.Errorf("read PONG: %w", err)
	}
	pong := string(buf[:n])
	want := "PONG " + payload
	if pong != want {
		return fmt.Errorf("expected %q got %q", want, pong)
	}
	log.Printf("SMOKE_STEP=quic_ping_ok payload=%q", payload)
	return nil
}

// smokeTunnelBytes dials localhost:localPort and checks echo through the relay hop.
func smokeTunnelBytes(port int) error {
	payload := []byte("cli-tunnel-e2e-smoke")
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			lastErr = err
			time.Sleep(100 * time.Millisecond)
			continue
		}
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write(payload); err != nil {
			_ = conn.Close()
			return fmt.Errorf("write: %w", err)
		}
		buf := make([]byte, len(payload))
		if _, err := io.ReadFull(conn, buf); err != nil {
			_ = conn.Close()
			return fmt.Errorf("read echo: %w", err)
		}
		_ = conn.Close()
		if string(buf) != string(payload) {
			return fmt.Errorf("payload mismatch got=%q", buf)
		}
		log.Printf("CLI_TUNNEL_STEP=bytes_ok payload=%q via %s", buf, addr)
		return nil
	}
	return fmt.Errorf("dial %s: %v", addr, lastErr)
}

// validateFlags validates CLI flags for incompatible combinations
func validateFlags(cfg *types.Config, transportMode string) error {
	// gRPC without TLS is allowed for local-smoke (plaintext :8444); warn only.
	if transportMode == "grpc" && !cfg.Relay.TLS.Enabled {
		log.Printf("WARNING: gRPC transport with TLS disabled (local-smoke / dev only)")
	}

	// Check WireGuard requirements
	if cfg.WireGuard.Enabled {
		// Check if running with administrative privileges
		if !utils.IsRunningAsAdmin() {
			log.Print(utils.GetPrivilegeWarning())
		}
	}

	// Check Pushgateway URL format
	if cfg.Metrics.Pushgateway.Enabled {
		if cfg.Metrics.Pushgateway.URL == "" {
			return fmt.Errorf("pushgateway enabled but no URL specified")
		}

		// Basic URL validation
		if !strings.HasPrefix(cfg.Metrics.Pushgateway.URL, "http://") &&
			!strings.HasPrefix(cfg.Metrics.Pushgateway.URL, "https://") {
			return fmt.Errorf("pushgateway URL must start with http:// or https://")
		}
	}

	// Check token requirement
	if token == "" && cfg.Auth.Secret == "" {
		return fmt.Errorf("JWT token is required (use --token flag or set auth.secret in config)")
	}

	return nil
}
