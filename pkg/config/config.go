package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"regexp"

	"github.com/twogc/cloudbridge-client/pkg/types"
	"github.com/spf13/viper"
)

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*types.Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/cloudbridge-client")
	viper.AddConfigPath("$HOME/.cloudbridge-client")

	// Set defaults
	setDefaults()

	// Read config file if specified
	if configPath != "" {
		viper.SetConfigFile(configPath)
	}

	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("CLOUDBRIDGE")

	// Read config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var config types.Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Substitute environment variables in string fields
	substituteEnvVars(&config)

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Relay configuration - синхронизировано с реальными портами edge.2gc.ru
	viper.SetDefault("relay.host", "edge.2gc.ru")       // Реальный домен
	viper.SetDefault("relay.port", 5553)                // P2P QUIC порт
	viper.SetDefault("relay.ports.http_api", 5553)      // P2P API через QUIC
	viper.SetDefault("relay.ports.p2p_api", 5553)       // P2P API прямой доступ
	viper.SetDefault("relay.ports.quic", 5553)          // QUIC Transport
	viper.SetDefault("relay.ports.stun", 19302)         // STUN Server (реально открыт)
	viper.SetDefault("relay.ports.masque", 8443)        // MASQUE Proxy через nginx
	viper.SetDefault("relay.ports.enhanced_quic", 9092) // Enhanced QUIC (реально открыт)
	viper.SetDefault("relay.ports.turn", 3478)          // TURN Server
	viper.SetDefault("relay.ports.derp", 3479)          // DERP Server
	viper.SetDefault("relay.timeout", "30s")
	// Enable TLS by default for secure communications
	viper.SetDefault("relay.tls.enabled", true)
	viper.SetDefault("relay.tls.min_version", "1.3")
	// Verify relay server certificate by default
	viper.SetDefault("relay.tls.verify_cert", true)
	viper.SetDefault("relay.tls.server_name", "b1.2gc.space") // Используем имя из серверного сертификата

	// Authentication
	viper.SetDefault("auth.type", "jwt")
	viper.SetDefault("auth.fallback_secret", "")
	viper.SetDefault("auth.skip_validation", false)
	// OIDC (Zitadel)
	viper.SetDefault("auth.oidc.issuer_url", "") // напр. https://<your-zitadel>
	viper.SetDefault("auth.oidc.audience", "")   // ваш client_id
	// auth.secret используется только при type=jwt

	// Rate limiting
	viper.SetDefault("rate_limiting.enabled", true)
	viper.SetDefault("rate_limiting.max_retries", 3)
	viper.SetDefault("rate_limiting.backoff_multiplier", 2.0)
	viper.SetDefault("rate_limiting.max_backoff", "30s")

	// Logging
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")

	// Metrics and Pushgateway
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.prometheus_port", 9091)
	viper.SetDefault("metrics.pushgateway.enabled", false)
	viper.SetDefault("metrics.pushgateway.push_url", "http://localhost:9091")
	viper.SetDefault("metrics.pushgateway.job_name", "cloudbridge-client")
	viper.SetDefault("metrics.pushgateway.instance", "")
	viper.SetDefault("metrics.pushgateway.push_interval", "30s")

	// API configuration - синхронизировано с edge.2gc.ru
	viper.SetDefault("api.base_url", "https://edge.2gc.ru:5553")      // HTTPS основной домен через QUIC порт
	viper.SetDefault("api.p2p_api_url", "https://edge.2gc.ru:5553")   // P2P API через QUIC порт
	viper.SetDefault("api.heartbeat_url", "https://edge.2gc.ru:5553") // Heartbeat через QUIC порт
	viper.SetDefault("api.insecure_skip_verify", false)               // Проверять SSL сертификаты
	viper.SetDefault("api.timeout", "30s")
	viper.SetDefault("api.max_retries", 3)
	viper.SetDefault("api.backoff_multiplier", 2.0)
	viper.SetDefault("api.max_backoff", "60s")

	// ICE configuration - синхронизировано с реальными серверами
	viper.SetDefault("ice.stun_servers", []string{"edge.2gc.ru:19302", "stun.l.google.com:19302"})
	viper.SetDefault("ice.turn_servers", []string{"edge.2gc.ru:3478"})
	viper.SetDefault("ice.derp_servers", []string{"edge.2gc.ru:3479"})
	viper.SetDefault("ice.timeout", "30s")
	viper.SetDefault("ice.max_binding_requests", 7)
	viper.SetDefault("ice.connectivity_checks", true)
	viper.SetDefault("ice.candidate_gathering", true)

	// QUIC configuration
	viper.SetDefault("quic.handshake_timeout", "10s")
	viper.SetDefault("quic.idle_timeout", "30s")
	viper.SetDefault("quic.max_streams", 100)
	viper.SetDefault("quic.max_stream_data", 1048576) // 1MB
	viper.SetDefault("quic.keep_alive_period", "15s")
	viper.SetDefault("quic.insecure_skip_verify", true)
	viper.SetDefault("quic.masque_support", true)
	viper.SetDefault("quic.http_datagrams", true)

	// P2P configuration
	viper.SetDefault("p2p.max_connections", 1000)
	viper.SetDefault("p2p.session_timeout", "300s")
	viper.SetDefault("p2p.peer_discovery_interval", "30s")
	viper.SetDefault("p2p.connection_retry_interval", "5s")
	viper.SetDefault("p2p.max_retry_attempts", 3)
	viper.SetDefault("p2p.heartbeat_interval", "30s")
	viper.SetDefault("p2p.heartbeat_timeout", "10s")
	viper.SetDefault("p2p.fallback_enabled", true)
	viper.SetDefault("p2p.derp_fallback", true)
	viper.SetDefault("p2p.websocket_fallback", true)

	// TURN configuration
	viper.SetDefault("turn.enabled", true)
	viper.SetDefault("turn.servers", []string{"localhost:3478"})
	viper.SetDefault("turn.username", "cloudbridge")
	viper.SetDefault("turn.password", "cloudbridge123")
	viper.SetDefault("turn.realm", "cloudbridge.local")
	viper.SetDefault("turn.timeout", "30s")
	viper.SetDefault("turn.max_allocations", 10)

	// DERP configuration
	viper.SetDefault("derp.enabled", true)
	viper.SetDefault("derp.servers", []string{"localhost:3479"})
	viper.SetDefault("derp.timeout", "30s")
	viper.SetDefault("derp.max_connections", 100)
	viper.SetDefault("derp.fallback_priority", 1)

	// WebSocket configuration
	viper.SetDefault("websocket.enabled", true)
	viper.SetDefault("websocket.endpoint", "wss://edge.2gc.ru:5553/ws") // Правильный WebSocket endpoint
	viper.SetDefault("websocket.timeout", "30s")
	viper.SetDefault("websocket.ping_interval", "15s")
	viper.SetDefault("websocket.max_reconnect_attempts", 5)

	// WireGuard configuration
	viper.SetDefault("wireguard.enabled", true)
	viper.SetDefault("wireguard.interface_name", "wg-cloudbridge")
	viper.SetDefault("wireguard.port", 51820)
	viper.SetDefault("wireguard.mtu", 1420)
	viper.SetDefault("wireguard.persistent_keepalive", "25s")
}

// validateConfig validates the configuration
func validateConfig(c *types.Config) error {
	if c.Relay.Host == "" {
		return fmt.Errorf("relay host is required")
	}

	if c.Relay.Port <= 0 || c.Relay.Port > 65535 {
		return fmt.Errorf("invalid relay port")
	}

	if c.Relay.TLS.Enabled && c.Relay.TLS.MinVersion != "1.3" {
		return fmt.Errorf("only TLS 1.3 is supported")
	}

	if c.Relay.TLS.Enabled && c.Relay.TLS.CACert != "" {
		if _, err := os.Stat(c.Relay.TLS.CACert); os.IsNotExist(err) {
			return fmt.Errorf("CA certificate file not found: %s", c.Relay.TLS.CACert)
		}
	}

	if c.Relay.TLS.Enabled && c.Relay.TLS.ClientCert != "" && c.Relay.TLS.ClientKey == "" {
		return fmt.Errorf("client key is required when client certificate is provided")
	}

	if c.Relay.TLS.Enabled && c.Relay.TLS.ClientKey != "" && c.Relay.TLS.ClientCert == "" {
		return fmt.Errorf("client certificate is required when client key is provided")
	}

	if c.Auth.Type == "jwt" && c.Auth.Secret == "" {
		return fmt.Errorf("JWT secret is required for JWT authentication")
	}

	if c.Auth.Type == "oidc" {
		if c.Auth.OIDC.IssuerURL == "" {
			return fmt.Errorf("oidc issuer_url is required")
		}
		if c.Auth.OIDC.Audience == "" {
			return fmt.Errorf("oidc audience (client_id) is required")
		}
	}

	if c.RateLimiting.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	if c.RateLimiting.BackoffMultiplier <= 0 {
		return fmt.Errorf("backoff multiplier must be positive")
	}

	// Validate WireGuard configuration if enabled
	if c.WireGuard.Enabled {
		if err := ValidateInterfaceName(c.WireGuard.InterfaceName); err != nil {
			return fmt.Errorf("invalid wireguard interface name: %w", err)
		}

		if c.WireGuard.MTU > 0 && (c.WireGuard.MTU < 500 || c.WireGuard.MTU > 9000) {
			return fmt.Errorf("wireguard MTU must be between 500 and 9000")
		}
	}

	return nil
}

// ValidateInterfaceName validates interface name to prevent command injection
func ValidateInterfaceName(name string) error {
	const maxInterfaceNameLen = 15 // Linux interface name limit
	validInterfaceRegex := regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

	if len(name) == 0 || len(name) > maxInterfaceNameLen {
		return fmt.Errorf("invalid interface name length: %d (must be 1-%d chars)", len(name), maxInterfaceNameLen)
	}
	if !validInterfaceRegex.MatchString(name) {
		return fmt.Errorf("interface name contains invalid characters: %s (allowed: alphanumeric, _, -)", name)
	}
	return nil
}

// CreateTLSConfig creates a TLS configuration from the config
func CreateTLSConfig(c *types.Config) (*tls.Config, error) {
	if !c.Relay.TLS.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
		},
		InsecureSkipVerify: !c.Relay.TLS.VerifyCert,
	}

	// Set server name for SNI
	if c.Relay.TLS.ServerName != "" {
		tlsConfig.ServerName = c.Relay.TLS.ServerName
	} else {
		tlsConfig.ServerName = c.Relay.Host
	}

	// Build root CA pool from system trust store first
	var rootCAs *x509.CertPool
	var sysPoolErr error
	rootCAs, sysPoolErr = x509.SystemCertPool()
	if sysPoolErr != nil || rootCAs == nil {
		// Fallback to empty pool if system pool unavailable (e.g., Windows in Go <1.20)
		rootCAs = x509.NewCertPool()
	}

	// Load additional CA certificate provided via config or CLOUDBRIDGE_CA env
	caPath := c.Relay.TLS.CACert
	if caPath == "" {
		// Allow override via environment variable (simple flat name)
		caPath = os.Getenv("CLOUDBRIDGE_CA")
	}

	if caPath != "" {
		caCert, readErr := os.ReadFile(caPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", readErr)
		}

		if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
	}

	// Only set RootCAs if we have something (non-nil)
	if rootCAs != nil {
		tlsConfig.RootCAs = rootCAs
	}

	// Load client certificate if provided
	if c.Relay.TLS.ClientCert != "" && c.Relay.TLS.ClientKey != "" {
		cert, certErr := tls.LoadX509KeyPair(c.Relay.TLS.ClientCert, c.Relay.TLS.ClientKey)
		if certErr != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", certErr)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// substituteEnvVars substitutes environment variables in configuration strings
func substituteEnvVars(config *types.Config) {
	// Substitute in auth config
	config.Auth.Secret = substituteEnvVar(config.Auth.Secret)
	config.Auth.FallbackSecret = substituteEnvVar(config.Auth.FallbackSecret)
	config.Auth.OIDC.IssuerURL = substituteEnvVar(config.Auth.OIDC.IssuerURL)
	config.Auth.OIDC.Audience = substituteEnvVar(config.Auth.OIDC.Audience)
	config.Auth.OIDC.JWKSURL = substituteEnvVar(config.Auth.OIDC.JWKSURL)

	// Substitute in relay config
	config.Relay.Host = substituteEnvVar(config.Relay.Host)
	config.Relay.TLS.CACert = substituteEnvVar(config.Relay.TLS.CACert)
	config.Relay.TLS.ClientCert = substituteEnvVar(config.Relay.TLS.ClientCert)
	config.Relay.TLS.ClientKey = substituteEnvVar(config.Relay.TLS.ClientKey)
	config.Relay.TLS.ServerName = substituteEnvVar(config.Relay.TLS.ServerName)

	// Substitute in logging config
	config.Logging.Output = substituteEnvVar(config.Logging.Output)
}

// substituteEnvVar substitutes environment variables in a string
// Supports format: ${VAR_NAME} or ${VAR_NAME:default_value}
func substituteEnvVar(value string) string {
	if value == "" {
		return value
	}

	// Regular expression to match ${VAR_NAME} or ${VAR_NAME:default_value}
	re := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)

	return re.ReplaceAllStringFunc(value, func(match string) string {
		// Extract variable name and default value
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		varName := matches[1]
		defaultValue := ""
		if len(matches) > 2 {
			defaultValue = matches[2]
		}

		// Get environment variable value
		envValue := os.Getenv(varName)
		if envValue == "" {
			envValue = defaultValue
		}

		return envValue
	})
}
