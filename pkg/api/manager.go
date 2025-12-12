package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/auth"
)

// Manager manages HTTP API operations for P2P mesh
type Manager struct {
	client            *Client
	authManager       *auth.AuthManager
	config            *ManagerConfig
	token             string
	tenantID          string
	peerID            string
	relaySessionID    string
	heartbeatInterval time.Duration
	logger            Logger
	ctx               context.Context
	cancel            context.CancelFunc
	mu                sync.RWMutex
	running           bool
	sessionID         string
	hbStopCh          chan struct{}
	startTime         time.Time
}

// ManagerConfig represents the API manager configuration
type ManagerConfig struct {
	BaseURL            string
	HeartbeatURL       string
	InsecureSkipVerify bool
	Timeout            time.Duration
	MaxRetries         int
	BackoffMultiplier  float64
	MaxBackoff         time.Duration
	Token              string
	TenantID           string
	HeartbeatInterval  time.Duration
}

// NewManager creates a new HTTP API manager
func NewManager(cfg *ManagerConfig, authManager *auth.AuthManager, logger Logger) *Manager {
	// Валидируем конфигурацию
	if err := validateManagerConfig(cfg); err != nil {
		logger.Warn("Invalid API manager configuration", "error", err)
	}

	apiConfig := &ClientConfig{
		BaseURL:            cfg.BaseURL,
		HeartbeatURL:       cfg.HeartbeatURL,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		Timeout:            cfg.Timeout,
		MaxRetries:         cfg.MaxRetries,
		BackoffMultiplier:  cfg.BackoffMultiplier,
		MaxBackoff:         cfg.MaxBackoff,
	}

	client := NewClient(apiConfig, logger)

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		client:            client,
		authManager:       authManager,
		config:            cfg,
		token:             cfg.Token,
		tenantID:          cfg.TenantID,
		heartbeatInterval: cfg.HeartbeatInterval,
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
		hbStopCh:          make(chan struct{}),
	}
}

// Start starts the API manager
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("manager is already running")
	}

	// Initialize start time for uptime calculation
	m.startTime = time.Now()

	// Register peer first
	if err := m.registerPeer(); err != nil {
		return fmt.Errorf("failed to register peer: %w", err)
	}

	// Notify relay about opened logical connection (monitoring)
	m.sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
	m.logger.Debug("Opening relay connection for monitoring", "tenant_id", m.tenantID, "peer_id", m.peerID, "session_id", m.sessionID)
	openReq := &ConnectionRequest{
		TenantID:  m.tenantID,
		PeerID:    m.peerID, // This is now set by registerPeer()
		SessionID: m.sessionID,
		Protocol:  "tcp",
		Meta: map[string]interface{}{
			"remote_host": "", // unknown at this layer
			"remote_port": 0,
		},
	}
	if err := m.client.OpenConnection(m.ctx, m.token, openReq); err != nil {
		m.logger.Warn("Failed to open relay connection for monitoring", "error", err)
	} else {
		m.logger.Info("Successfully opened relay connection for monitoring", "session_id", m.sessionID)
	}

	// Start heartbeat
	go m.startHeartbeat()

	// Start connection heartbeat to relay status (every 20-30s)
	// Protect against artifacts from previous runs
	if m.hbStopCh != nil {
		close(m.hbStopCh)
		m.hbStopCh = nil
	}
	m.hbStopCh = make(chan struct{})
	go func(sessionID string) {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-m.hbStopCh:
				return
			case <-ticker.C:
				req := &HeartbeatRequest{
					Status:         "active",
					RelaySessionID: sessionID,
				}
				if err := m.client.HeartbeatConnection(m.ctx, m.token, req); err != nil {
					m.logger.Warn("Heartbeat to relay failed", "error", err)
				} else {
					m.logger.Debug("Heartbeat to relay sent")
				}
			}
		}
	}(m.sessionID)

	m.running = true
	m.logger.Info("API manager started", "tenant_id", m.tenantID, "peer_id", m.peerID)

	return nil
}

// Stop stops the API manager
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	// Notify relay about closed logical connection (monitoring)
	if m.sessionID != "" {
		if m.hbStopCh != nil {
			close(m.hbStopCh)
			m.hbStopCh = nil
		}
		closeReq := &ConnectionRequest{
			TenantID:  m.tenantID,
			PeerID:    m.peerID,
			SessionID: m.sessionID,
			Protocol:  "tcp",
		}
		if err := m.client.CloseConnection(m.ctx, m.token, closeReq); err != nil {
			m.logger.Warn("Failed to close relay connection for monitoring", "error", err)
		}
	}

	m.cancel()
	m.running = false

	m.logger.Info("API manager stopped")
}

// IsRunning returns true if the manager is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetPeerID returns the peer ID
func (m *Manager) GetPeerID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.peerID
}

// GetRelaySessionID returns the relay session ID
func (m *Manager) GetRelaySessionID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.relaySessionID
}

// GetToken returns the authentication token
func (m *Manager) GetToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token
}

// GetClient returns the underlying HTTP client
func (m *Manager) GetClient() *Client {
	return m.client
}

// DiscoverPeers discovers peers in the tenant
func (m *Manager) DiscoverPeers() (*DiscoverResponse, error) {
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	discoverResp, err := m.client.DiscoverPeers(ctx, m.tenantID, m.token)
	if err != nil {
		return nil, fmt.Errorf("failed to discover peers: %w", err)
	}

	m.logger.Info("Discovered peers", "count", len(discoverResp.Peers), "tenant_id", discoverResp.TenantID)

	return discoverResp, nil
}

// registerPeer registers the peer with the API
func (m *Manager) registerPeer() error {
	// Extract public key and allowed IPs from token
	validatedToken, err := m.authManager.ValidateToken(m.token)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	// Extract QUIC config from token
	quicConfig, err := m.authManager.ExtractQUICConfig(validatedToken)
	if err != nil {
		return fmt.Errorf("failed to extract QUIC config: %w", err)
	}

	// Use public key from config or generate one
	publicKey := quicConfig.PublicKey
	if publicKey == "" {
		// Generate a unique public key for this session
		// This ensures each client instance gets a unique peer ID
		publicKey = fmt.Sprintf("generated-quic-key-%d", time.Now().UnixNano())
	}

	// Use allowed IPs from token or default
	allowedIPs := quicConfig.AllowedIPs
	if len(allowedIPs) == 0 {
		allowedIPs = []string{"10.0.0.0/24"}
	}

	// Create simplified registration request
	req := &PeerRegistrationRequest{
		PublicKey:  publicKey,
		AllowedIPs: allowedIPs,
	}

	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	resp, err := m.client.RegisterPeer(ctx, m.tenantID, m.token, req)
	if err != nil {
		return fmt.Errorf("failed to register peer: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("peer registration failed: %s", resp.Error)
	}

	// Store peer ID and relay session ID
	m.mu.Lock()
	m.peerID = resp.PeerID
	m.relaySessionID = resp.RelaySessionID
	m.mu.Unlock()

	m.logger.Info("Peer registered successfully",
		"peer_id", resp.PeerID,
		"relay_session_id", resp.RelaySessionID,
		"registered_at", resp.RegisteredAt)

	return nil
}

// RegisterPeerWith re-registers the peer using an explicit public key and allowed IPs
func (m *Manager) RegisterPeerWith(publicKey string, allowedIPs []string) error {
	if strings.TrimSpace(publicKey) == "" {
		return fmt.Errorf("public key is required")
	}
	if len(allowedIPs) == 0 {
		allowedIPs = []string{"10.100.0.0/16"}
	}

	req := &PeerRegistrationRequest{
		PublicKey:  publicKey,
		AllowedIPs: allowedIPs,
	}

	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	resp, err := m.client.RegisterPeer(ctx, m.tenantID, m.token, req)
	if err != nil {
		return fmt.Errorf("failed to register peer with override: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("peer registration failed: %s", resp.Error)
	}

	m.mu.Lock()
	m.peerID = resp.PeerID
	m.relaySessionID = resp.RelaySessionID
	m.mu.Unlock()

	m.logger.Info("Peer re-registered with explicit public key",
		"peer_id", resp.PeerID,
		"relay_session_id", resp.RelaySessionID,
		"registered_at", resp.RegisteredAt)
	return nil
}

// startHeartbeat starts the heartbeat mechanism
func (m *Manager) startHeartbeat() {
	// Set default heartbeat interval if not configured
	interval := m.heartbeatInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	consecutiveFailures := 0

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.sendHeartbeat(); err != nil {
				consecutiveFailures++
				m.logger.Error("Failed to send heartbeat", "error", err, "failures", consecutiveFailures)

				// After 3 consecutive failures, attempt to re-register and reopen monitoring session
				if consecutiveFailures >= 3 {
					m.logger.Warn("Heartbeat failing consecutively; attempting re-registration and session reopen")

					// Close old session if exists
					oldSession := m.sessionID
					if oldSession != "" {
						closeReq := &ConnectionRequest{
							TenantID:  m.tenantID,
							PeerID:    m.peerID,
							SessionID: oldSession,
							Protocol:  "tcp",
						}
						_ = m.client.CloseConnection(m.ctx, m.token, closeReq)
						m.logger.Debug("Closed old session", "old_session", oldSession)
					}

					if err := m.registerPeer(); err != nil {
						m.logger.Error("Re-registration failed", "error", err)
					} else {
						// Reopen monitoring session with a new session ID
						m.sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
						openReq := &ConnectionRequest{
							TenantID:  m.tenantID,
							PeerID:    m.peerID,
							SessionID: m.sessionID,
							Protocol:  "tcp",
							Meta: map[string]interface{}{
								"reopened_after": oldSession,
							},
						}
						if err := m.client.OpenConnection(m.ctx, m.token, openReq); err != nil {
							m.logger.Warn("Failed to reopen relay connection after re-registration", "error", err)
						} else {
							m.logger.Info("Monitoring session reopened after re-registration", "session_id", m.sessionID)
						}
					}

					// Reset failure counter regardless of outcome to avoid tight loops
					consecutiveFailures = 0
				}
			} else {
				// Success resets failure counter
				consecutiveFailures = 0
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to the API
func (m *Manager) sendHeartbeat() error {
	m.mu.RLock()
	peerID := m.peerID
	relaySessionID := m.relaySessionID
	m.mu.RUnlock()

	if peerID == "" || relaySessionID == "" {
		return fmt.Errorf("peer not registered")
	}

	// Create status request
	req := &PeerStatusRequest{
		RelaySessionID: relaySessionID,
		Status:         "active",
		Metrics: map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"uptime":    time.Since(m.startTime).Seconds(), // Correct uptime calculation
		},
	}

	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	defer cancel()

	statusResp, err := m.client.UpdatePeerStatus(ctx, m.tenantID, peerID, m.token, req)
	if err != nil {
		return fmt.Errorf("failed to update peer status: %w", err)
	}

	if !statusResp.Success {
		return fmt.Errorf("heartbeat failed: %s", statusResp.Error)
	}

	m.logger.Debug("Heartbeat sent successfully", "peer_id", peerID)

	return nil
}

// postJSONRequest makes a POST request with JSON body and expects a response with Success field
func (m *Manager) postJSONRequest(endpoint string, request, response interface{}, requestType string) error {
	base := strings.TrimSuffix(m.client.baseURL, "/")
	url := base + endpoint

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal %s request: %w", requestType, err)
	}

	httpReq, err := http.NewRequestWithContext(m.ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", requestType, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.token)
	httpReq.Header.Set("X-Request-ID", fmt.Sprintf("%d", time.Now().UnixNano()))

	resp, err := m.client.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send %s request: %w", requestType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s request failed with status: %d", requestType, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return fmt.Errorf("failed to decode %s response: %w", requestType, err)
	}

	return nil
}

// SendICESignaling sends ICE candidates to the relay server
func (m *Manager) SendICESignaling(req *ICESignalingRequest) error {
	var response ICESignalingResponse
	if err := m.postJSONRequest("/api/v1/ice/candidates", req, &response, "ICE signaling"); err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("ICE signaling failed: %s", response.Error)
	}

	m.logger.Debug("ICE signaling sent successfully", "session_id", req.SessionID)
	return nil
}

// GetRemoteICECandidates gets remote ICE candidates from the relay server
func (m *Manager) GetRemoteICECandidates(req *ICECandidateRequest) (*ICECandidateResponse, error) {
	base := strings.TrimSuffix(m.client.baseURL, "/")
	url := fmt.Sprintf("%s/api/v1/ice/candidates/%s", base, req.TargetPeerID)

	httpReq, err := http.NewRequestWithContext(m.ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create ICE candidate request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+m.token)
	httpReq.Header.Set("X-Session-ID", req.SessionID)

	resp, err := m.client.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote ICE candidates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ICE candidate request failed with status: %d", resp.StatusCode)
	}

	var response ICECandidateResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode ICE candidate response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("failed to get remote ICE candidates: %s", response.Error)
	}

	m.logger.Debug("Remote ICE candidates retrieved", "target_peer_id", req.TargetPeerID, "count", len(response.Candidates))
	return &response, nil
}

// RequestP2PConnection requests a P2P connection to another peer
func (m *Manager) RequestP2PConnection(req *P2PConnectionRequest) error {
	var response P2PConnectionResponse
	if err := m.postJSONRequest("/api/v1/p2p/connect", req, &response, "P2P connection"); err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("P2P connection failed: %s", response.Error)
	}

	m.logger.Debug("P2P connection requested successfully", "target_peer_id", req.TargetPeerID)
	return nil
}

// SendHeartbeat sends a heartbeat to maintain peer connection
func (m *Manager) SendHeartbeat(ctx context.Context, tenantID, peerID, token string, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	// Use heartbeat URL from config
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/heartbeat", m.config.HeartbeatURL, tenantID, peerID)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal heartbeat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{
		Timeout: m.config.Timeout,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send heartbeat request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read heartbeat response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("heartbeat request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var heartbeatResp HeartbeatResponse
	if err := json.Unmarshal(body, &heartbeatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal heartbeat response: %w", err)
	}

	return &heartbeatResp, nil
}

// validateManagerConfig валидирует конфигурацию API Manager
func validateManagerConfig(cfg *ManagerConfig) error {
	if cfg.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}
	if cfg.TenantID == "" {
		return fmt.Errorf("tenant ID is required")
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive: %v", cfg.Timeout)
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative: %d", cfg.MaxRetries)
	}
	if cfg.BackoffMultiplier <= 0 {
		return fmt.Errorf("backoff multiplier must be positive: %v", cfg.BackoffMultiplier)
	}
	if cfg.MaxBackoff <= 0 {
		return fmt.Errorf("max backoff must be positive: %v", cfg.MaxBackoff)
	}
	return nil
}
