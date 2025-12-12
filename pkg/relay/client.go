package relay

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/api"
	"github.com/twogc/cloudbridge-client/pkg/auth"
	"github.com/twogc/cloudbridge-client/pkg/config"
	"github.com/twogc/cloudbridge-client/pkg/errors"
	"github.com/twogc/cloudbridge-client/pkg/heartbeat"
	"github.com/twogc/cloudbridge-client/pkg/interfaces"
	"github.com/twogc/cloudbridge-client/pkg/metrics"
	"github.com/twogc/cloudbridge-client/pkg/p2p"
	"github.com/twogc/cloudbridge-client/pkg/performance"
	"github.com/twogc/cloudbridge-client/pkg/tunnel"
	"github.com/twogc/cloudbridge-client/pkg/types"
	"github.com/golang-jwt/jwt/v5"
)

// Константы для метрик транспорта
const (
	MetricTransportQUIC      = 0
	MetricTransportWireGuard = 1
	MetricTransportGRPC      = 2
)

// Client represents a CloudBridge Relay client
type Client struct {
	config              *types.Config
	configPath          string
	configWatcher       *config.ConfigWatcher
	conn                net.Conn
	encoder             *json.Encoder
	decoder             *json.Decoder
	authManager         *auth.AuthManager
	tunnelManager       *tunnel.Manager
	heartbeatMgr        *heartbeat.Manager
	retryStrategy       *errors.RetryStrategy
	metrics             *metrics.Metrics
	optimizer           *performance.Optimizer
	p2pManager          *p2p.Manager
	autoSwitchMgr       *AutoSwitchManager
	transportAdapter    *TransportAdapter
	useTransportAdapter bool
	connectionType      string

	// Новые компоненты для улучшенного клиента
	masqueClient    interface{} // *masque.MASQUEClient
	handoverManager interface{} // *handover.HandoverManager
	sloController   interface{} // *slo.SLOController
	probeManager    interface{} // *probes.SyntheticProbeManager
	logger          *relayLogger
	mu              sync.RWMutex
	connected       bool
	clientID        string
	tenantID        string
	tokenString     string
	ctx             context.Context
	cancel          context.CancelFunc
	lastHeartbeat   time.Time
	stateEvents     chan interfaces.ClientStateEvent
}

// Message types as defined in the requirements
const (
	MessageTypeHello             = "hello"
	MessageTypeHelloResponse     = "hello_response"
	MessageTypeAuth              = "auth"
	MessageTypeAuthResponse      = "auth_response"
	MessageTypeTunnelInfo        = "tunnel_info"
	MessageTypeTunnelResponse    = "tunnel_response"
	MessageTypeHeartbeat         = "heartbeat"
	MessageTypeHeartbeatResponse = "heartbeat_response"
	MessageTypeError             = "error"
	// P2P message types
	MessageTypeP2PHandshake     = "p2p_handshake"
	MessageTypeP2PHandshakeResp = "p2p_handshake_response"
	MessageTypePeerDiscovery    = "peer_discovery"
	MessageTypePeerList         = "peer_list"
	MessageTypeMeshConfig       = "mesh_config"
)

// NewClient creates a new CloudBridge Relay client
func NewClient(cfg *types.Config, configPath string) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

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
		cancel()
		return nil, fmt.Errorf("failed to create auth manager: %w", err)
	}

	// Create retry strategy
	retryStrategy := errors.NewRetryStrategy(
		cfg.RateLimiting.MaxRetries,
		cfg.RateLimiting.BackoffMultiplier,
		cfg.RateLimiting.MaxBackoff,
	)

	// Create metrics system with Pushgateway support
	var metricsSystem *metrics.Metrics
	if cfg.Metrics.Pushgateway.Enabled {
		// Set default instance if not specified
		instance := cfg.Metrics.Pushgateway.Instance
		if instance == "" {
			hostname, err := os.Hostname()
			if err != nil {
				hostname = "unknown"
			}
			instance = hostname
		}

		pushConfig := &metrics.PushgatewayConfig{
			Enabled:      cfg.Metrics.Pushgateway.Enabled,
			URL:          cfg.Metrics.Pushgateway.URL,
			JobName:      cfg.Metrics.Pushgateway.JobName,
			Instance:     instance,
			PushInterval: cfg.Metrics.Pushgateway.PushInterval,
		}
		metricsSystem = metrics.NewMetricsWithPushgateway(cfg.Metrics.Enabled, cfg.Metrics.PrometheusPort, pushConfig)
	} else {
		metricsSystem = metrics.NewMetrics(cfg.Metrics.Enabled, cfg.Metrics.PrometheusPort)
	}

	// Create performance optimizer
	optimizer := performance.NewOptimizer(cfg.Performance.Enabled)

	client := &Client{
		config:        cfg,
		configPath:    configPath,
		authManager:   authManager,
		retryStrategy: retryStrategy,
		metrics:       metricsSystem,
		optimizer:     optimizer,
		logger:        NewRelayLogger("relay-client"),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Create AutoSwitchManager for WireGuard fallback
	client.autoSwitchMgr = NewAutoSwitchManager(cfg, client.logger)

	// Инициализируем новые компоненты
	client.initializeEnhancedComponents()

	// Add callback to update transport mode metrics
	client.autoSwitchMgr.AddSwitchCallback(func(from, to TransportMode) {
		var modeValue int
		switch to {
		case TransportModeQUIC:
			modeValue = 0
		case TransportModeWireGuard:
			modeValue = 1
		default:
			modeValue = 0
		}
		client.metrics.SetTransportMode(modeValue)
	})

	// Create transport adapter for gRPC support
	client.transportAdapter = NewTransportAdapter(cfg, client.logger)
	client.useTransportAdapter = true // Always use transport adapter for modern clients

	// Create tunnel manager
	client.tunnelManager = tunnel.NewManager(client)

	// Create heartbeat manager
	client.heartbeatMgr = heartbeat.NewManager(client)

	// Initialize performance optimization
	if cfg.Performance.Enabled {
		switch cfg.Performance.OptimizationMode {
		case "high_throughput":
			optimizer.OptimizeForHighThroughput()
		case "low_latency":
			optimizer.OptimizeForLowLatency()
		}
	}

	// Start metrics server
	if err := metricsSystem.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start metrics server: %w", err)
	}

	// Start AutoSwitchManager
	if err := client.autoSwitchMgr.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start AutoSwitchManager: %w", err)
	}

	// Initialize transport adapter
	if err := client.transportAdapter.Initialize(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize transport adapter: %w", err)
	}

	// Определяем использование transport adapter на основе режима
	currentMode := client.transportAdapter.GetCurrentMode()
	client.useTransportAdapter = (currentMode == "grpc")
	client.logger.Info("Transport adapter selected",
		"mode", currentMode,
		"useTransportAdapter", client.useTransportAdapter)

	// Initialize config watcher for hot-reload
	if configPath != "" {
		configWatcher, err := config.NewConfigWatcher(configPath, &configLoggerAdapter{logger: client.logger})
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create config watcher: %w", err)
		}
		client.configWatcher = configWatcher

		// Add config change callback
		configWatcher.AddCallback(client.handleConfigChange)

		// Start config watcher
		if err := configWatcher.Start(); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to start config watcher: %w", err)
		}
	}

	return client, nil
}

// Connect establishes a connection to the relay server
func (c *Client) Connect() error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return fmt.Errorf("already connected")
	}
	useTA := c.useTransportAdapter
	c.mu.Unlock() // CRITICAL: отпускаем мьютекс перед I/O

	c.logger.Info("Connect called", "useTransportAdapter", useTA, "transportAdapter_nil", c.transportAdapter == nil)

	// Use transport adapter if enabled
	if useTA {
		c.logger.Info("Using transport adapter for connection")
		if err := c.transportAdapter.Connect(); err != nil {
			return fmt.Errorf("failed to connect via transport adapter: %w", err)
		}

		// Send hello via transport adapter
		if err := c.transportAdapter.Hello("1.0", []string{"tls", "heartbeat", "tunnel_info", "grpc", "masque", "http3_datagrams"}); err != nil {
			if disconnErr := c.transportAdapter.Disconnect(); disconnErr != nil {
				c.logger.Error("Failed to disconnect transport adapter after hello failure", "error", disconnErr)
			}
			return fmt.Errorf("failed to send hello via transport adapter: %w", err)
		}

		c.mu.Lock()
		c.connected = true
		c.mu.Unlock()
		c.emitStateEvent("connected", "transport adapter")
		return nil
	}

	// Legacy JSON transport
	c.logger.Warn("Using deprecated JSON transport, consider switching to gRPC with --transport grpc")

	// Create TLS config
	tlsConfig, err := config.CreateTLSConfig(c.config)
	if err != nil {
		return fmt.Errorf("failed to create TLS config: %w", err)
	}

	// Establish connection
	var conn net.Conn
	address := net.JoinHostPort(c.config.Relay.Host, strconv.Itoa(c.config.Relay.Port))
	if tlsConfig != nil {
		conn, err = tls.Dial("tcp", address, tlsConfig)
	} else {
		conn, err = net.Dial("tcp", address)
	}

	if err != nil {
		return errors.NewRelayError(errors.ErrTLSHandshakeFailed, fmt.Sprintf("failed to connect: %v", err))
	}

	c.conn = conn
	c.encoder = json.NewEncoder(conn)
	c.decoder = json.NewDecoder(conn)

	// Send hello message
	if err := c.sendHello(); err != nil {
		if cerr := conn.Close(); cerr != nil {
			_ = cerr // Игнорируем ошибку закрытия соединения при ошибке отправки hello
		}
		return fmt.Errorf("failed to send hello: %w", err)
	}

	// Receive hello response
	if err := c.receiveHelloResponse(); err != nil {
		if cerr := conn.Close(); cerr != nil {
			_ = cerr // Игнорируем ошибку закрытия соединения при ошибке получения hello response
		}
		return fmt.Errorf("failed to receive hello response: %w", err)
	}

	c.connected = true
	c.emitStateEvent("connected", "legacy JSON transport")
	return nil
}

// Authenticate authenticates with the relay server
func (c *Client) Authenticate(token string) error {
	c.mu.RLock()
	useTA := c.useTransportAdapter
	connected := c.connected
	c.mu.RUnlock() // CRITICAL: отпускаем мьютекс перед I/O

	c.logger.Info("Authenticate called", "useTransportAdapter", useTA, "transportAdapter_nil", c.transportAdapter == nil)

	if !connected {
		return fmt.Errorf("not connected")
	}

	// CRITICAL: всегда валидируем JWT локально
	validatedToken, err := c.authManager.ValidateToken(token)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	// Извлекаем claims локально
	_, tenantID, err := c.authManager.ExtractClaims(validatedToken)
	if err != nil {
		return fmt.Errorf("failed to extract claims: %w", err)
	}

	// Извлекаем connection type
	connType, err := c.authManager.ExtractConnectionType(validatedToken)
	if err != nil {
		connType = "client-server"
		c.logger.Warn("connection_type missing in JWT", "err", err)
	}

	// If using transport adapter (gRPC), дополнительно аутентифицируемся через адаптер
	if useTA && c.transportAdapter != nil {
		c.logger.Info("Using transport adapter for authentication")
		clientID, _, err := c.transportAdapter.Authenticate(token)
		if err != nil {
			return err
		}

		c.mu.Lock()
		c.clientID = clientID
		c.tenantID = tenantID // используем локальные claims
		c.tokenString = token
		c.connectionType = connType
		c.mu.Unlock()

		// Инициализируем P2P если нужно
		if connType == "p2p-mesh" {
			if err := c.initializeP2PManager(validatedToken); err != nil {
				return fmt.Errorf("p2p init: %w", err)
			}
		}
		return nil
	}

	c.logger.Info("Using legacy JSON authentication")

	// Legacy JSON authentication - дублируем валидацию для совместимости
	// (уже сделано выше, но оставляем для legacy пути)

	// Store tenant ID and token string (используем уже извлеченные значения)
	c.mu.Lock()
	c.tenantID = tenantID
	c.tokenString = token
	c.connectionType = connType
	c.mu.Unlock()

	// Create auth message
	authMsg, err := c.authManager.CreateAuthMessage(token)
	if err != nil {
		return fmt.Errorf("failed to create auth message: %w", err)
	}

	// Send auth message
	if err := c.sendMessage(authMsg); err != nil {
		return fmt.Errorf("failed to send auth message: %w", err)
	}

	// Receive auth response
	response, err := c.receiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive auth response: %w", err)
	}

	// Check response type
	if response["type"] != MessageTypeAuthResponse {
		return fmt.Errorf("unexpected response type: %s", response["type"])
	}

	// Check status
	if status, ok := response["status"].(string); !ok || status != "ok" {
		errorMsg := "authentication failed"
		if msg, ok := response["error"].(string); ok {
			errorMsg = msg
		}
		return errors.NewRelayError(errors.ErrAuthenticationFailed, errorMsg)
	}

	// Store client ID
	if clientID, ok := response["client_id"].(string); ok {
		c.clientID = clientID
	}

	// Determine connection type from JWT token
	connectionType, err := c.authManager.ExtractConnectionType(validatedToken)
	if err != nil {
		// If connection type is not specified, default to client-server
		connectionType = "client-server"
		c.logger.Warn("Failed to extract connection type from token, defaulting to client-server", "error", err)
	}
	c.connectionType = connectionType

	// Initialize P2P manager if connection type is P2P mesh
	if connectionType == "p2p-mesh" {
		if err := c.initializeP2PManager(validatedToken); err != nil {
			return fmt.Errorf("failed to initialize P2P manager: %w", err)
		}
	}

	return nil
}

// CreateTunnel creates a tunnel with the specified parameters
func (c *Client) CreateTunnel(tunnelID string, localPort int, remoteHost string, remotePort int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	// If using transport adapter (gRPC), delegate to transport adapter
	if c.useTransportAdapter && c.transportAdapter != nil {
		c.logger.Info("Creating tunnel via transport adapter",
			"tunnel_id", tunnelID,
			"tenant_id", c.tenantID,
			"local_port", localPort,
			"remote_host", remoteHost,
			"remote_port", remotePort)

		if err := c.transportAdapter.CreateTunnel(tunnelID, c.tenantID, localPort, remoteHost, remotePort); err != nil {
			return fmt.Errorf("failed to create tunnel via transport adapter: %w", err)
		}

		// Register tunnel with tunnel manager
		if err := c.tunnelManager.RegisterTunnel(tunnelID, localPort, remoteHost, remotePort); err != nil {
			return fmt.Errorf("failed to register tunnel: %w", err)
		}

		return nil
	}

	// Legacy JSON transport path
	c.logger.Info("Creating tunnel via legacy JSON transport", "tunnel_id", tunnelID)

	// Create tunnel info message
	tunnelMsg := map[string]interface{}{
		"type":        MessageTypeTunnelInfo,
		"tunnel_id":   tunnelID,
		"tenant_id":   c.tenantID,
		"local_port":  localPort,
		"remote_host": remoteHost,
		"remote_port": remotePort,
	}

	// Send tunnel message
	if err := c.sendMessage(tunnelMsg); err != nil {
		return fmt.Errorf("failed to send tunnel message: %w", err)
	}

	// Receive tunnel response
	response, err := c.receiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive tunnel response: %w", err)
	}

	// Check response type
	if response["type"] != MessageTypeTunnelResponse {
		return fmt.Errorf("unexpected response type: %s", response["type"])
	}

	// Check status
	if status, ok := response["status"].(string); !ok || status != "ok" {
		errorMsg := "tunnel creation failed"
		if msg, ok := response["error"].(string); ok {
			errorMsg = msg
		}
		return errors.NewRelayError(errors.ErrTunnelCreationFailed, errorMsg)
	}

	// Register tunnel with tunnel manager
	if err := c.tunnelManager.RegisterTunnel(tunnelID, localPort, remoteHost, remotePort); err != nil {
		return fmt.Errorf("failed to register tunnel: %w", err)
	}

	return nil
}

// StartHeartbeat starts the heartbeat mechanism
func (c *Client) StartHeartbeat() error {
	return c.heartbeatMgr.Start()
}

// StopHeartbeat stops the heartbeat mechanism
func (c *Client) StopHeartbeat() {
	c.heartbeatMgr.Stop()
}

// Disconnect disconnects from the relay server
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.logger.Info("Disconnecting from relay server")

	// Use transport adapter if enabled
	if c.useTransportAdapter && c.transportAdapter != nil {
		if err := c.transportAdapter.Disconnect(); err != nil {
			c.logger.Warn("Failed to disconnect transport adapter", "error", err)
		}
	}

	// Close legacy connection if exists
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.logger.Warn("Failed to close legacy connection", "error", err)
		}
		c.conn = nil
		c.encoder = nil
		c.decoder = nil
	}

	c.connected = false
	c.emitStateEvent("disconnected", "manual disconnect")
	c.logger.Info("Disconnected from relay server")
	return nil
}

// Close closes the client connection and cleans up resources
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// CRITICAL: всегда отменяем контекст в конце
	defer c.cancel()

	if !c.connected {
		// Still need to clean up resources even if not connected
		return nil
	}

	// Stop heartbeat
	c.heartbeatMgr.Stop()

	// Stop AutoSwitchManager
	if c.autoSwitchMgr != nil {
		if err := c.autoSwitchMgr.Stop(); err != nil {
			// Log error but don't fail close operation
			fmt.Printf("Failed to stop AutoSwitchManager: %v\n", err)
		}
	}

	// Stop config watcher
	if c.configWatcher != nil {
		if err := c.configWatcher.Stop(); err != nil {
			// Log error but don't fail close operation
			fmt.Printf("Failed to stop config watcher: %v\n", err)
		}
	}

	// Stop metrics server
	if c.metrics != nil {
		if err := c.metrics.Stop(); err != nil {
			// Log error but don't fail close operation
			fmt.Printf("Failed to stop metrics: %v\n", err)
		}
	}

	// Close connection
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			fmt.Printf("Failed to close connection: %v\n", err)
		}
	}

	c.connected = false
	return nil
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetClientID returns the client ID
func (c *Client) GetClientID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clientID
}

// sendHello sends a hello message
func (c *Client) sendHello() error {
	helloMsg := map[string]interface{}{
		"type":     MessageTypeHello,
		"version":  "1.0",
		"features": []string{"tls", "heartbeat", "tunnel_info"},
	}
	return c.sendMessage(helloMsg)
}

// receiveHelloResponse receives a hello response
func (c *Client) receiveHelloResponse() error {
	response, err := c.receiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive hello response: %w", err)
	}

	if response["type"] != MessageTypeHelloResponse {
		return fmt.Errorf("unexpected response type: %s", response["type"])
	}

	return nil
}

// sendMessage sends a JSON message
func (c *Client) sendMessage(msg map[string]interface{}) error {
	return c.encoder.Encode(msg)
}

// receiveMessage receives a JSON message
func (c *Client) receiveMessage() (map[string]interface{}, error) {
	var msg map[string]interface{}
	if err := c.decoder.Decode(&msg); err != nil {
		return nil, fmt.Errorf("failed to decode message: %w", err)
	}
	return msg, nil
}

// SendHeartbeat sends a heartbeat message
func (c *Client) SendHeartbeat() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	// If using transport adapter (gRPC), delegate to transport adapter
	if c.useTransportAdapter && c.transportAdapter != nil {
		c.logger.Debug("Sending heartbeat via transport adapter",
			"client_id", c.clientID,
			"tenant_id", c.tenantID)

		err := c.transportAdapter.SendHeartbeat(c.clientID, c.tenantID)
		if err == nil {
			c.mu.Lock()
			c.lastHeartbeat = time.Now()
			c.mu.Unlock()
		}
		return err
	}

	// Legacy JSON transport path
	c.logger.Debug("Sending heartbeat via legacy JSON transport")

	heartbeatMsg := map[string]interface{}{
		"type": MessageTypeHeartbeat,
	}

	if err := c.sendMessage(heartbeatMsg); err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	response, err := c.receiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive heartbeat response: %w", err)
	}

	if response["type"] != MessageTypeHeartbeatResponse {
		return fmt.Errorf("unexpected response type: %s", response["type"])
	}

	// Update last heartbeat time
	c.mu.Lock()
	c.lastHeartbeat = time.Now()
	c.mu.Unlock()

	return nil
}

// GetConfig returns the client configuration
func (c *Client) GetConfig() *types.Config {
	return c.config
}

// GetRetryStrategy returns the retry strategy
func (c *Client) GetRetryStrategy() *errors.RetryStrategy {
	return c.retryStrategy
}

// GetTenantID returns the tenant ID
func (c *Client) GetTenantID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tenantID
}

// GetMetrics returns the metrics system
func (c *Client) GetMetrics() *metrics.Metrics {
	return c.metrics
}

// GetOptimizer returns the performance optimizer
func (c *Client) GetOptimizer() *performance.Optimizer {
	return c.optimizer
}

// initializeP2PManager initializes the P2P manager with configuration from JWT token
func (c *Client) initializeP2PManager(token *jwt.Token) error {
	// Extract P2P configuration from JWT token
	p2pConfig, err := p2p.ExtractP2PConfigFromToken(c.authManager, token)
	if err != nil {
		return fmt.Errorf("failed to extract P2P config from token: %w", err)
	}

	// Create P2P logger
	p2pLogger := &p2pLogger{client: c}

	// Create P2P manager with HTTP API support
	// Use HTTPS API on port 30082 for P2P operations
	apiConfig := &api.ManagerConfig{
		BaseURL:            c.config.API.BaseURL,
		InsecureSkipVerify: c.config.API.InsecureSkipVerify,
		Timeout:            c.config.API.Timeout,
		MaxRetries:         c.config.API.MaxRetries,
		BackoffMultiplier:  c.config.API.BackoffMultiplier,
		MaxBackoff:         c.config.API.MaxBackoff,
	}

	// Use stored token string for API manager
	c.p2pManager = p2p.NewManagerWithAPI(p2pConfig, apiConfig, c.authManager, c.tokenString, p2pLogger)

	// Start P2P manager
	if err := c.p2pManager.Start(); err != nil {
		return fmt.Errorf("failed to start P2P manager: %w", err)
	}

	c.logger.Info("P2P manager initialized and started successfully",
		"connection_type", p2pConfig.ConnectionType,
		"tenant_id", p2pConfig.TenantID)

	return nil
}

// GetP2PManager returns the P2P manager
func (c *Client) GetP2PManager() *p2p.Manager {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.p2pManager
}

// GetConnectionType returns the connection type
func (c *Client) GetConnectionType() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connectionType
}

// GetAutoSwitchManager returns the AutoSwitchManager
func (c *Client) GetAutoSwitchManager() *AutoSwitchManager {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.autoSwitchMgr
}

// GetTransportMode returns the current transport mode
func (c *Client) GetTransportMode() TransportMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.autoSwitchMgr != nil {
		return c.autoSwitchMgr.GetCurrentMode()
	}
	return TransportModeQUIC
}

// SetTransportMode sets the transport mode (grpc or json)
func (c *Client) SetTransportMode(mode string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("SetTransportMode called", "mode", mode, "current_useTransportAdapter", c.useTransportAdapter)

	switch mode {
	case "grpc":
		c.useTransportAdapter = true
		if err := c.transportAdapter.SetTransportMode("grpc"); err != nil {
			return fmt.Errorf("failed to set gRPC transport mode: %w", err)
		}
		c.logger.Info("Switched to gRPC transport mode", "useTransportAdapter", c.useTransportAdapter)
	case "json":
		c.useTransportAdapter = false
		c.logger.Info("Switched to JSON transport mode", "useTransportAdapter", c.useTransportAdapter)
	default:
		return fmt.Errorf("unsupported transport mode: %s", mode)
	}

	return nil
}

// GetCurrentTransportMode returns the current transport protocol mode
func (c *Client) GetCurrentTransportMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.useTransportAdapter {
		return c.transportAdapter.GetCurrentMode()
	}
	return "json"
}

// p2pLogger implements the p2p.Logger interface
type p2pLogger struct {
	client *Client
}

func (pl *p2pLogger) Info(msg string, fields ...interface{}) {
	pl.client.logger.Info(msg, fields...)
}

func (pl *p2pLogger) Error(msg string, fields ...interface{}) {
	pl.client.logger.Error(msg, fields...)
}

func (pl *p2pLogger) Debug(msg string, fields ...interface{}) {
	pl.client.logger.Debug(msg, fields...)
}

func (pl *p2pLogger) Warn(msg string, fields ...interface{}) {
	pl.client.logger.Warn(msg, fields...)
}

// relayLogger implements logging for the relay client
type relayLogger struct {
	prefix string
}

// NewRelayLogger creates a new relay logger
func NewRelayLogger(prefix string) *relayLogger {
	return &relayLogger{
		prefix: prefix,
	}
}

func (rl *relayLogger) Info(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		fmt.Printf("[%s] INFO: %s %v\n", rl.prefix, msg, fields)
	} else {
		fmt.Printf("[%s] INFO: %s\n", rl.prefix, msg)
	}
}

func (rl *relayLogger) Error(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		fmt.Printf("[%s] ERROR: %s %v\n", rl.prefix, msg, fields)
	} else {
		fmt.Printf("[%s] ERROR: %s\n", rl.prefix, msg)
	}
}

func (rl *relayLogger) Debug(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		fmt.Printf("[%s] DEBUG: %s %v\n", rl.prefix, msg, fields)
	} else {
		fmt.Printf("[%s] DEBUG: %s\n", rl.prefix, msg)
	}
}

func (rl *relayLogger) Warn(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		fmt.Printf("[%s] WARN: %s %v\n", rl.prefix, msg, fields)
	} else {
		fmt.Printf("[%s] WARN: %s\n", rl.prefix, msg)
	}
}

// handleConfigChange handles configuration changes from the watcher
func (c *Client) handleConfigChange(oldConfig, newConfig *types.Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("Processing configuration changes")

	// Update log level if changed
	if oldConfig.Logging.Level != newConfig.Logging.Level {
		c.logger.Info("Log level changed",
			"old_level", oldConfig.Logging.Level,
			"new_level", newConfig.Logging.Level)
		// In a real implementation, you would update the logger level here
	}

	// Update TLS settings if changed
	if oldConfig.Relay.TLS.VerifyCert != newConfig.Relay.TLS.VerifyCert ||
		oldConfig.Relay.TLS.Enabled != newConfig.Relay.TLS.Enabled ||
		oldConfig.Relay.TLS.CACert != newConfig.Relay.TLS.CACert {
		c.logger.Info("TLS settings changed, reconnecting to apply changes")

		// If connected, trigger reconnection to apply new TLS settings
		if c.connected {
			go func() {
				c.logger.Info("Reconnecting due to TLS configuration changes")

				// Disconnect current connection
				if err := c.Disconnect(); err != nil {
					c.logger.Error("Failed to disconnect for TLS reload", "error", err)
					return
				}

				// Wait a moment for clean disconnect
				time.Sleep(1 * time.Second)

				// Reconnect with new TLS settings
				if err := c.Connect(); err != nil {
					c.logger.Error("Failed to reconnect after TLS reload", "error", err)
				} else {
					c.logger.Info("Successfully reconnected with new TLS settings")
				}
			}()
		}
	}

	// Update endpoints if changed
	if oldConfig.Relay.Host != newConfig.Relay.Host ||
		oldConfig.Relay.Port != newConfig.Relay.Port ||
		oldConfig.API.BaseURL != newConfig.API.BaseURL {
		c.logger.Info("Endpoint settings changed, will take effect on next connection")
	}

	// Handle credential changes (requires re-authentication)
	if oldConfig.Auth.Secret != newConfig.Auth.Secret {
		c.logger.Info("Authentication credentials changed, re-authentication required")

		// Store new token for re-authentication
		c.tokenString = newConfig.Auth.Secret

		// If connected, trigger re-authentication
		if c.connected {
			go func() {
				c.logger.Info("Re-authenticating with new credentials")
				if err := c.Authenticate(newConfig.Auth.Secret); err != nil {
					c.logger.Error("Re-authentication failed", "error", err)
				} else {
					c.logger.Info("Re-authentication successful")
				}
			}()
		}
	}

	// Update the client's config reference
	c.config = newConfig

	c.logger.Info("Configuration changes processed successfully")
	return nil
}

// GetConfigWatcher returns the config watcher
func (c *Client) GetConfigWatcher() *config.ConfigWatcher {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.configWatcher
}

// ForceConfigReload forces a configuration reload
func (c *Client) ForceConfigReload() error {
	if c.configWatcher == nil {
		return fmt.Errorf("config watcher not initialized")
	}
	return c.configWatcher.ForceReload()
}

// UpdateMetrics updates client metrics (called periodically)
func (c *Client) UpdateMetrics() {
	if c.metrics == nil {
		return
	}

	// Update P2P sessions count
	if c.p2pManager != nil {
		// In a real implementation, you would get the actual session count
		// For now, we'll use a placeholder
		c.metrics.SetP2PSessions(1) // Placeholder
	} else {
		c.metrics.SetP2PSessions(0)
	}

	// Update transport mode based on current settings
	if c.useTransportAdapter {
		c.metrics.SetTransportMode(MetricTransportGRPC) // gRPC mode
	} else {
		// Use AutoSwitchManager mode
		mode := c.autoSwitchMgr.GetCurrentMode()
		switch mode {
		case TransportModeQUIC:
			c.metrics.SetTransportMode(MetricTransportQUIC)
		case TransportModeWireGuard:
			c.metrics.SetTransportMode(MetricTransportWireGuard)
		default:
			c.metrics.SetTransportMode(MetricTransportQUIC)
		}
	}
}

// RecordDataTransfer records data transfer for metrics
func (c *Client) RecordDataTransfer(bytesSent, bytesRecv int64) {
	if c.metrics == nil {
		return
	}

	c.metrics.RecordClientBytesSent(bytesSent)
	c.metrics.RecordClientBytesRecv(bytesRecv)
}

// configLoggerAdapter adapts relayLogger to config.Logger interface
type configLoggerAdapter struct {
	logger *relayLogger
}

func (cla *configLoggerAdapter) Info(msg string, fields ...interface{}) {
	cla.logger.Info(msg, fields...)
}

func (cla *configLoggerAdapter) Error(msg string, fields ...interface{}) {
	cla.logger.Error(msg, fields...)
}

func (cla *configLoggerAdapter) Debug(msg string, fields ...interface{}) {
	cla.logger.Debug(msg, fields...)
}

func (cla *configLoggerAdapter) Warn(msg string, fields ...interface{}) {
	cla.logger.Warn(msg, fields...)
}

// initializeEnhancedComponents инициализирует новые компоненты клиента
func (c *Client) initializeEnhancedComponents() {
	c.logger.Info("Initializing enhanced client components...")

	// TODO: Реальная инициализация будет добавлена после создания пакетов
	// Пока что устанавливаем заглушки для совместимости
	c.masqueClient = nil
	c.handoverManager = nil
	c.sloController = nil
	c.probeManager = nil

	c.logger.Info("Enhanced client components initialized successfully")
}

// GetMASQUEClient возвращает MASQUE клиент
func (c *Client) GetMASQUEClient() interface{} {
	return c.masqueClient
}

// GetHandoverManager возвращает Handover Manager
func (c *Client) GetHandoverManager() interface{} {
	return c.handoverManager
}

// GetSLOController возвращает SLO Controller
func (c *Client) GetSLOController() interface{} {
	return c.sloController
}

// GetProbeManager возвращает Synthetic Probe Manager
func (c *Client) GetProbeManager() interface{} {
	return c.probeManager
}

// Start starts the client (implements interfaces.ClientInterface)
func (c *Client) Start() error {
	return c.Connect()
}

// Stop stops the client (implements interfaces.ClientInterface)
func (c *Client) Stop() error {
	return c.Close()
}

// Reconnect reconnects the client (implements interfaces.ClientInterface)
func (c *Client) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		if err := c.Disconnect(); err != nil {
			return fmt.Errorf("failed to disconnect: %w", err)
		}
	}

	return c.Connect()
}

// LastHeartbeat returns the time of the last heartbeat (implements interfaces.ClientInterface)
func (c *Client) LastHeartbeat() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastHeartbeat
}

// SubscribeState returns a channel for state events (implements interfaces.ClientInterface)
func (c *Client) SubscribeState() <-chan interfaces.ClientStateEvent {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stateEvents == nil {
		c.stateEvents = make(chan interfaces.ClientStateEvent, 10)
	}

	return c.stateEvents
}

// emitStateEvent emits a state event
func (c *Client) emitStateEvent(state, reason string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.stateEvents != nil {
		select {
		case c.stateEvents <- interfaces.ClientStateEvent{
			State:   state,
			Reason:  reason,
			Updated: time.Now(),
		}:
		default:
			// Channel is full, skip event
		}
	}
}
