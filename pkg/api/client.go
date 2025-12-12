package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client represents an HTTP API client for P2P mesh operations
type Client struct {
	baseURL            string
	httpClient         *http.Client
	insecureSkipVerify bool
	logger             Logger
}

// Logger interface for logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// PeerRegistrationRequest represents a peer registration request (simplified)
type PeerRegistrationRequest struct {
	PublicKey  string   `json:"public_key"`
	AllowedIPs []string `json:"allowed_ips"`
}

// PeerRegistrationResponse represents a peer registration response (simplified)
type PeerRegistrationResponse struct {
	Success        bool   `json:"success"`
	PeerID         string `json:"peer_id"`
	RelaySessionID string `json:"relay_session_id"`
	RegisteredAt   string `json:"registered_at"`
	Error          string `json:"error,omitempty"`
	Code           int    `json:"code,omitempty"`
	Message        string `json:"message,omitempty"`
}

// ICECredentialsRequest represents ICE credentials exchange request
type ICECredentialsRequest struct {
	Ufrag string `json:"ufrag"`
	Pwd   string `json:"pwd"`
}

// ICECredentialsResponse represents ICE credentials exchange response
type ICECredentialsResponse struct {
	Success bool   `json:"success"`
	Ufrag   string `json:"ufrag"`
	Pwd     string `json:"pwd"`
	Error   string `json:"error,omitempty"`
}

// PeerStatusRequest represents a peer status update request
type PeerStatusRequest struct {
	RelaySessionID string                 `json:"relay_session_id"`
	Status         string                 `json:"status"`
	Metrics        map[string]interface{} `json:"metrics,omitempty"`
}

// PeerStatusResponse represents a peer status update response
type PeerStatusResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Code    int    `json:"code,omitempty"`
}

// Peer represents a discovered peer
type Peer struct {
	PeerID         string         `json:"peer_id"`
	RelaySessionID string         `json:"relay_session_id"`
	PublicKey      string         `json:"public_key,omitempty"`
	AllowedIPs     []string       `json:"allowed_ips,omitempty"`
	Endpoint       string         `json:"endpoint,omitempty"`
	Keepalive      int            `json:"keepalive,omitempty"`
	MTU            int            `json:"mtu,omitempty"`
	IsOnline       bool           `json:"is_online,omitempty"`
	LastSeen       string         `json:"last_seen,omitempty"`
	QUICConfig     *QUICConfig    `json:"quic_config,omitempty"`
	NetworkConfig  *NetworkConfig `json:"network_config,omitempty"`
}

// QUICConfig for discover payloads
type QUICConfig struct {
	PublicKey  string   `json:"public_key,omitempty"`
	AllowedIPs []string `json:"allowed_ips,omitempty"`
}

// NetworkConfig for discover payloads
type NetworkConfig struct {
	MTU int `json:"mtu,omitempty"`
}

// DiscoverResponse represents a peer discovery response
type DiscoverResponse struct {
	Success  bool    `json:"success"`
	TenantID string  `json:"tenant_id"`
	Peers    []*Peer `json:"peers"`
	Error    string  `json:"error,omitempty"`
	Code     int     `json:"code,omitempty"`
}

// ConnectionRequest represents an open/close connection request for relay monitoring
type ConnectionRequest struct {
	TenantID  string                 `json:"tenant_id"`
	PeerID    string                 `json:"peer_id"`
	SessionID string                 `json:"session_id"`
	Protocol  string                 `json:"protocol"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

// HeartbeatRequest represents a heartbeat for an open logical connection
type HeartbeatRequest struct {
	Status         string `json:"status"`
	RelaySessionID string `json:"relay_session_id"`
}

// HeartbeatResponse represents a heartbeat response
type HeartbeatResponse struct {
	Success  bool   `json:"success"`
	PeerID   string `json:"peer_id"`
	Status   string `json:"status"`
	LastSeen string `json:"last_seen"`
	Error    string `json:"error,omitempty"`
	Code     int    `json:"code,omitempty"`
}

// ClientConfig represents the API client configuration
type ClientConfig struct {
	BaseURL            string
	HeartbeatURL       string
	InsecureSkipVerify bool
	Timeout            time.Duration
	MaxRetries         int
	BackoffMultiplier  float64
	MaxBackoff         time.Duration
}

// NewClient creates a new HTTP API client
func NewClient(cfg *ClientConfig, logger Logger) *Client {
	// Create HTTP client with TLS configuration
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.InsecureSkipVerify,
			},
		},
	}

	return &Client{
		baseURL:            strings.TrimSuffix(cfg.BaseURL, "/"),
		httpClient:         httpClient,
		insecureSkipVerify: cfg.InsecureSkipVerify,
		logger:             logger,
	}
}

// RegisterPeer registers a peer without endpoint
func (c *Client) RegisterPeer(ctx context.Context, tenantID, token string,
	req *PeerRegistrationRequest) (*PeerRegistrationResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/register", c.baseURL, tenantID)

	resp, err := c.doRequestWithRetry(ctx, "POST", url, token, req, &PeerRegistrationResponse{})
	if err != nil {
		return nil, err
	}
	if result, ok := resp.(*PeerRegistrationResponse); ok {
		return result, nil
	}
	return nil, fmt.Errorf("unexpected response type")
}

// UpdatePeerStatus updates peer status with heartbeat
func (c *Client) UpdatePeerStatus(ctx context.Context, tenantID, peerID, token string,
	req *PeerStatusRequest) (*PeerStatusResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/status", c.baseURL, tenantID, peerID)

	resp, err := c.doRequestWithRetry(ctx, "PUT", url, token, req, &PeerStatusResponse{})
	if err != nil {
		return nil, err
	}
	if result, ok := resp.(*PeerStatusResponse); ok {
		return result, nil
	}
	return nil, fmt.Errorf("unexpected response type")
}

// DiscoverPeers discovers peers in the tenant
func (c *Client) DiscoverPeers(ctx context.Context, tenantID, token string) (*DiscoverResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/discover", c.baseURL, tenantID)

	resp, err := c.doRequestWithRetry(ctx, "GET", url, token, nil, &DiscoverResponse{})
	if err != nil {
		return nil, err
	}
	if result, ok := resp.(*DiscoverResponse); ok {
		return result, nil
	}
	return nil, fmt.Errorf("unexpected response type")
}

// OpenConnection notifies relay about an opened logical connection (increments gauge)
func (c *Client) OpenConnection(ctx context.Context, token string, req *ConnectionRequest) error {
	url := fmt.Sprintf("%s/api/v1/relay/connections/open", c.baseURL)
	_, err := c.doRequestWithRetry(ctx, "POST", url, token, req, nil)
	return err
}

// CloseConnection notifies relay about a closed logical connection (decrements gauge)
func (c *Client) CloseConnection(ctx context.Context, token string, req *ConnectionRequest) error {
	url := fmt.Sprintf("%s/api/v1/relay/connections/close", c.baseURL)
	_, err := c.doRequestWithRetry(ctx, "POST", url, token, req, nil)
	return err
}

// HeartbeatConnection sends a heartbeat for an open logical connection
func (c *Client) HeartbeatConnection(ctx context.Context, token string, req *HeartbeatRequest) error {
	url := fmt.Sprintf("%s/api/v1/relay/connections/heartbeat", c.baseURL)
	_, err := c.doRequestWithRetry(ctx, "POST", url, token, req, nil)
	return err
}

// doRequestWithRetry performs HTTP request with retry logic
func (c *Client) doRequestWithRetry(ctx context.Context, method, url, token string, reqBody, respBody interface{}) (interface{}, error) {
	var lastErr error
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Add jitter to backoff
			jitter := time.Duration(rand.Float64() * float64(backoff) * 0.1)
			sleepTime := backoff + jitter

			c.logger.Debug("Retrying request", "attempt", attempt, "sleep", sleepTime)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(sleepTime):
			}

			backoff = time.Duration(float64(backoff) * 1.5)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		resp, err := c.doRequest(ctx, method, url, token, reqBody, respBody)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		c.logger.Warn("Request failed, will retry", "attempt", attempt+1, "error", err)
	}

	return nil, fmt.Errorf("request failed after retries: %w", lastErr)
}

// doRequest performs a single HTTP request
func (c *Client) doRequest(ctx context.Context, method, url, token string, reqBody, respBody interface{}) (interface{}, error) {
	var body io.Reader
	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", fmt.Sprintf("%d", time.Now().UnixNano()))
	req.Header.Set("User-Agent", "CloudBridge-Relay-Client/1.0.0")

	c.logger.Debug("Making HTTP request", "method", method, "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debug("Received response", "status", resp.StatusCode, "body_length", len(respData))

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		var errorResp struct {
			Error   string `json:"error"`
			Code    int    `json:"code"`
			Message string `json:"message"`
		}

		if err := json.Unmarshal(respData, &errorResp); err == nil {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errorResp.Error)
		}

		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respData))
	}

	// Parse successful response
	if respBody != nil {
		if err := json.Unmarshal(respData, respBody); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		return respBody, nil
	}

	return nil, nil
}

// ParseHeartbeatInterval parses heartbeat interval from mesh config
func ParseHeartbeatInterval(interval interface{}) time.Duration {
	switch v := interval.(type) {
	case string:
		if duration, err := time.ParseDuration(v); err == nil {
			return duration
		}
		// Try parsing as seconds
		if seconds, err := strconv.Atoi(v); err == nil {
			return time.Duration(seconds) * time.Second
		}
	case int:
		return time.Duration(v) * time.Second
	case float64:
		return time.Duration(v) * time.Second
	}

	// Default to 30 seconds
	return 30 * time.Second
}

// ICECandidate represents an ICE candidate
type ICECandidate struct {
	Foundation string `json:"foundation"`
	Component  int    `json:"component"`
	Transport  string `json:"transport"`
	Priority   int    `json:"priority"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Type       string `json:"type"`
}

// ICESignalingRequest represents an ICE signaling request
type ICESignalingRequest struct {
	SessionID  string          `json:"session_id"`
	PeerID     string          `json:"peer_id"`
	Candidates []*ICECandidate `json:"candidates"`
}

// ICESignalingResponse represents an ICE signaling response
type ICESignalingResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ICECandidateRequest represents a request for remote ICE candidates
type ICECandidateRequest struct {
	SessionID    string `json:"session_id"`
	TargetPeerID string `json:"target_peer_id"`
}

// ICECandidateResponse represents a response with remote ICE candidates
type ICECandidateResponse struct {
	Success    bool            `json:"success"`
	Candidates []*ICECandidate `json:"candidates"`
	Error      string          `json:"error,omitempty"`
}

// P2PConnectionRequest represents a P2P connection request
type P2PConnectionRequest struct {
	SessionID    string `json:"session_id"`
	TargetPeerID string `json:"target_peer_id"`
	Protocol     string `json:"protocol"`
}

// P2PConnectionResponse represents a P2P connection response
type P2PConnectionResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// SendHeartbeat sends a heartbeat to maintain peer connection
func (c *Client) SendHeartbeat(ctx context.Context, tenantID, peerID, token string, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/heartbeat", c.baseURL, tenantID, peerID)

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

	resp, err := c.httpClient.Do(httpReq)
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

// ExchangeICECredentials обменивается ICE credentials с сервером
func (c *Client) ExchangeICECredentials(ctx context.Context, token, tenantID, peerID string, req *ICECredentialsRequest) (*ICECredentialsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/ice-credentials", c.baseURL, tenantID, peerID)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ICE credentials request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create ICE credentials request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("X-Request-ID", fmt.Sprintf("%d", time.Now().UnixNano()))
	httpReq.Header.Set("User-Agent", "CloudBridge-Relay-Client/1.0.0")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send ICE credentials request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ICE credentials response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ICE credentials request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var iceResp ICECredentialsResponse
	if err := json.Unmarshal(body, &iceResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ICE credentials response: %w", err)
	}

	return &iceResp, nil
}

// GetICECredentials получает ICE credentials от другого пира
func (c *Client) GetICECredentials(ctx context.Context, token, tenantID, targetPeerID string) (*ICECredentialsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/ice-credentials", c.baseURL, tenantID, targetPeerID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get ICE credentials request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("X-Request-ID", fmt.Sprintf("%d", time.Now().UnixNano()))
	httpReq.Header.Set("User-Agent", "CloudBridge-Relay-Client/1.0.0")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send get ICE credentials request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read get ICE credentials response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get ICE credentials request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var iceResp ICECredentialsResponse
	if err := json.Unmarshal(body, &iceResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal get ICE credentials response: %w", err)
	}

	return &iceResp, nil
}
