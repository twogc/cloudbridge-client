package types

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// SaveToFile saves the configuration to a YAML file
func (c *Config) SaveToFile(path string) error {
	if path == "" {
		return fmt.Errorf("config file path cannot be empty")
	}

	// Create a viper instance to handle serialization
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Convert Config struct to map for viper
	v.Set("relay.host", c.Relay.Host)
	v.Set("relay.port", c.Relay.Port)
	v.Set("relay.timeout", c.Relay.Timeout)
	v.Set("relay.tls.enabled", c.Relay.TLS.Enabled)
	v.Set("relay.tls.min_version", c.Relay.TLS.MinVersion)
	v.Set("relay.tls.verify_cert", c.Relay.TLS.VerifyCert)
	v.Set("relay.tls.ca_cert", c.Relay.TLS.CACert)
	v.Set("relay.tls.client_cert", c.Relay.TLS.ClientCert)
	v.Set("relay.tls.client_key", c.Relay.TLS.ClientKey)
	v.Set("relay.tls.server_name", c.Relay.TLS.ServerName)

	// Relay ports
	v.Set("relay.ports.http_api", c.Relay.Ports.HTTPAPI)
	v.Set("relay.ports.p2p_api", c.Relay.Ports.P2PAPI)
	v.Set("relay.ports.quic", c.Relay.Ports.QUIC)
	v.Set("relay.ports.stun", c.Relay.Ports.STUN)
	v.Set("relay.ports.masque", c.Relay.Ports.MASQUE)
	v.Set("relay.ports.enhanced_quic", c.Relay.Ports.EnhancedQUIC)
	v.Set("relay.ports.turn", c.Relay.Ports.TURN)
	v.Set("relay.ports.derp", c.Relay.Ports.DERP)

	// Authentication
	v.Set("auth.type", c.Auth.Type)
	v.Set("auth.token", c.Auth.Token)
	v.Set("auth.secret", c.Auth.Secret)
	v.Set("auth.fallback_secret", c.Auth.FallbackSecret)
	v.Set("auth.skip_validation", c.Auth.SkipValidation)
	v.Set("auth.oidc.issuer_url", c.Auth.OIDC.IssuerURL)
	v.Set("auth.oidc.audience", c.Auth.OIDC.Audience)
	v.Set("auth.oidc.jwks_url", c.Auth.OIDC.JWKSURL)

	// Rate limiting
	v.Set("rate_limiting.enabled", c.RateLimiting.Enabled)
	v.Set("rate_limiting.max_retries", c.RateLimiting.MaxRetries)
	v.Set("rate_limiting.backoff_multiplier", c.RateLimiting.BackoffMultiplier)
	v.Set("rate_limiting.max_backoff", c.RateLimiting.MaxBackoff)

	// Logging
	v.Set("logging.level", c.Logging.Level)
	v.Set("logging.format", c.Logging.Format)
	v.Set("logging.output", c.Logging.Output)

	// Metrics
	v.Set("metrics.enabled", c.Metrics.Enabled)
	v.Set("metrics.prometheus_port", c.Metrics.PrometheusPort)
	v.Set("metrics.tenant_metrics", c.Metrics.TenantMetrics)
	v.Set("metrics.buffer_metrics", c.Metrics.BufferMetrics)
	v.Set("metrics.connection_metrics", c.Metrics.ConnectionMetrics)
	v.Set("metrics.pushgateway.enabled", c.Metrics.Pushgateway.Enabled)
	v.Set("metrics.pushgateway.push_url", c.Metrics.Pushgateway.URL)
	v.Set("metrics.pushgateway.job_name", c.Metrics.Pushgateway.JobName)
	v.Set("metrics.pushgateway.instance", c.Metrics.Pushgateway.Instance)
	v.Set("metrics.pushgateway.push_interval", c.Metrics.Pushgateway.PushInterval)

	// Performance
	v.Set("performance.enabled", c.Performance.Enabled)
	v.Set("performance.optimization_mode", c.Performance.OptimizationMode)
	v.Set("performance.gc_percent", c.Performance.GCPercent)
	v.Set("performance.memory_ballast", c.Performance.MemoryBallast)

	// API
	v.Set("api.base_url", c.API.BaseURL)
	v.Set("api.p2p_api_url", c.API.P2PAPIURL)
	v.Set("api.heartbeat_url", c.API.HeartbeatURL)
	v.Set("api.insecure_skip_verify", c.API.InsecureSkipVerify)
	v.Set("api.timeout", c.API.Timeout)
	v.Set("api.max_retries", c.API.MaxRetries)
	v.Set("api.backoff_multiplier", c.API.BackoffMultiplier)
	v.Set("api.max_backoff", c.API.MaxBackoff)

	// ICE
	v.Set("ice.stun_servers", c.ICE.STUNServers)
	v.Set("ice.turn_servers", c.ICE.TURNServers)
	v.Set("ice.derp_servers", c.ICE.DERPServers)
	v.Set("ice.timeout", c.ICE.Timeout)
	v.Set("ice.max_binding_requests", c.ICE.MaxBindingRequests)
	v.Set("ice.connectivity_checks", c.ICE.ConnectivityChecks)
	v.Set("ice.candidate_gathering", c.ICE.CandidateGathering)

	// QUIC
	v.Set("quic.handshake_timeout", c.QUIC.HandshakeTimeout)
	v.Set("quic.idle_timeout", c.QUIC.IdleTimeout)
	v.Set("quic.max_streams", c.QUIC.MaxStreams)
	v.Set("quic.max_stream_data", c.QUIC.MaxStreamData)
	v.Set("quic.keep_alive_period", c.QUIC.KeepAlivePeriod)
	v.Set("quic.insecure_skip_verify", c.QUIC.InsecureSkipVerify)
	v.Set("quic.masque_support", c.QUIC.MASQUESupport)
	v.Set("quic.http_datagrams", c.QUIC.HTTPDatagrams)

	// P2P
	v.Set("p2p.max_connections", c.P2P.MaxConnections)
	v.Set("p2p.session_timeout", c.P2P.SessionTimeout)
	v.Set("p2p.peer_discovery_interval", c.P2P.PeerDiscoveryInterval)
	v.Set("p2p.connection_retry_interval", c.P2P.ConnectionRetryInterval)
	v.Set("p2p.max_retry_attempts", c.P2P.MaxRetryAttempts)
	v.Set("p2p.heartbeat_interval", c.P2P.HeartbeatInterval)
	v.Set("p2p.heartbeat_timeout", c.P2P.HeartbeatTimeout)
	v.Set("p2p.fallback_enabled", c.P2P.FallbackEnabled)
	v.Set("p2p.derp_fallback", c.P2P.DERPFallback)
	v.Set("p2p.websocket_fallback", c.P2P.WebSocketFallback)

	// TURN
	v.Set("turn.enabled", c.TURN.Enabled)
	v.Set("turn.servers", c.TURN.Servers)
	v.Set("turn.username", c.TURN.Username)
	v.Set("turn.password", c.TURN.Password)
	v.Set("turn.realm", c.TURN.Realm)
	v.Set("turn.timeout", c.TURN.Timeout)
	v.Set("turn.max_allocations", c.TURN.MaxAllocations)

	// DERP
	v.Set("derp.enabled", c.DERP.Enabled)
	v.Set("derp.servers", c.DERP.Servers)
	v.Set("derp.timeout", c.DERP.Timeout)
	v.Set("derp.max_connections", c.DERP.MaxConnections)
	v.Set("derp.fallback_priority", c.DERP.FallbackPriority)

	// WebSocket
	v.Set("websocket.enabled", c.WebSocket.Enabled)
	v.Set("websocket.endpoint", c.WebSocket.Endpoint)
	v.Set("websocket.timeout", c.WebSocket.Timeout)
	v.Set("websocket.ping_interval", c.WebSocket.PingInterval)
	v.Set("websocket.max_reconnect_attempts", c.WebSocket.MaxReconnectAttempts)

	// WireGuard
	v.Set("wireguard.enabled", c.WireGuard.Enabled)
	v.Set("wireguard.interface_name", c.WireGuard.InterfaceName)
	v.Set("wireguard.port", c.WireGuard.Port)
	v.Set("wireguard.mtu", c.WireGuard.MTU)
	v.Set("wireguard.persistent_keepalive", c.WireGuard.PersistentKeepAlive)

	// Create directory if it doesn't exist
	dir := ""
	if len(path) > 0 {
		for i := len(path) - 1; i >= 0; i-- {
			if path[i] == '/' || path[i] == '\\' {
				dir = path[:i]
				break
			}
		}
	}

	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	// Write configuration to file
	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
