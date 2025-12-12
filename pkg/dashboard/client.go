package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DashboardClient handles communication with the CloudBridge Dashboard API
type DashboardClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	tenantID   string
}

// DashboardConfig contains configuration for dashboard client
type DashboardConfig struct {
	BaseURL  string
	APIKey   string
	TenantID string
	Timeout  time.Duration
}

// PeerInfo represents peer information from dashboard
type PeerInfo struct {
	ID              string                 `json:"id"`
	TenantID        string                 `json:"tenant_id"`
	UserID          string                 `json:"user_id,omitempty"`
	DeviceID        string                 `json:"device_id,omitempty"`
	Hostname        string                 `json:"hostname"`
	OS              string                 `json:"os"`
	Arch            string                 `json:"arch"`
	Version         string                 `json:"version"`
	Status          string                 `json:"status"` // online, offline, degraded
	LastSeen        time.Time              `json:"last_seen"`
	IPAddress       string                 `json:"ip_address,omitempty"`
	IPv6Address     string                 `json:"ipv6_address,omitempty"`
	Location        *LocationInfo          `json:"location,omitempty"`
	Metrics         *PeerMetrics           `json:"metrics,omitempty"`
	Health          *PeerHealth            `json:"health,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// LocationInfo represents peer location information
type LocationInfo struct {
	Country     string  `json:"country,omitempty"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
	ISP         string  `json:"isp,omitempty"`
	ASN         string  `json:"asn,omitempty"`
}

// PeerMetrics represents peer performance metrics
type PeerMetrics struct {
	BytesSent        uint64    `json:"bytes_sent"`
	BytesRecv        uint64    `json:"bytes_recv"`
	PacketsSent      uint64    `json:"packets_sent"`
	PacketsRecv      uint64    `json:"packets_recv"`
	PacketLoss       float64   `json:"packet_loss_pct"`
	Latency          float64   `json:"latency_ms"`
	BandwidthUp      float64   `json:"bandwidth_up_mbps"`
	BandwidthDown    float64   `json:"bandwidth_down_mbps"`
	Connections      int       `json:"active_connections"`
	DirectConns      int       `json:"direct_connections"`
	RelayConns       int       `json:"relay_connections"`
	CPUUsage         float64   `json:"cpu_usage_pct"`
	MemoryUsage      float64   `json:"memory_usage_mb"`
	DiskUsage        float64   `json:"disk_usage_pct"`
	LastUpdated      time.Time `json:"last_updated"`
}

// PeerHealth represents peer health status
type PeerHealth struct {
	Status          string    `json:"status"` // healthy, degraded, unhealthy
	LastCheck       time.Time `json:"last_check"`
	Issues          []string  `json:"issues,omitempty"`
	Connectivity    string    `json:"connectivity"` // good, poor, failed
	Authentication  string    `json:"authentication"` // valid, expired, invalid
	Performance     string    `json:"performance"` // good, degraded, poor
	Network         string    `json:"network"` // stable, unstable, down
}

// HeartbeatRequest represents a heartbeat request to dashboard
type HeartbeatRequest struct {
	PeerID          string                 `json:"peer_id"`
	TenantID        string                 `json:"tenant_id"`
	UserID          string                 `json:"user_id,omitempty"`
	DeviceID        string                 `json:"device_id,omitempty"`
	Hostname        string                 `json:"hostname"`
	OS              string                 `json:"os"`
	Arch            string                 `json:"arch"`
	Version         string                 `json:"version"`
	Status          string                 `json:"status"`
	IPAddress       string                 `json:"ip_address,omitempty"`
	IPv6Address     string                 `json:"ipv6_address,omitempty"`
	Metrics         *PeerMetrics           `json:"metrics,omitempty"`
	Health          *PeerHealth            `json:"health,omitempty"`
	Location        *LocationInfo           `json:"location,omitempty"`
	Metadata        map[string]interface{}  `json:"metadata,omitempty"`
	Timestamp       time.Time               `json:"timestamp"`
}

// HeartbeatResponse represents the response from dashboard heartbeat
type HeartbeatResponse struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message,omitempty"`
	PeerID    string                 `json:"peer_id"`
	TenantID  string                 `json:"tenant_id"`
	Config    map[string]interface{} `json:"config,omitempty"`
	Commands  []DashboardCommand     `json:"commands,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// DashboardCommand represents a command from dashboard
type DashboardCommand struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // restart, update, config_change, etc.
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Priority  string                 `json:"priority"` // low, medium, high, critical
	ExpiresAt time.Time              `json:"expires_at"`
	CreatedAt time.Time              `json:"created_at"`
}

// NewDashboardClient creates a new dashboard client
func NewDashboardClient(config *DashboardConfig) *DashboardClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &DashboardClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		apiKey:   config.APIKey,
		tenantID: config.TenantID,
	}
}

// SendHeartbeat sends a heartbeat to the dashboard
func (dc *DashboardClient) SendHeartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/heartbeat", dc.baseURL, dc.tenantID)
	
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal heartbeat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+dc.apiKey)
	httpReq.Header.Set("X-Tenant-ID", dc.tenantID)

	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("heartbeat failed with status %d: %s", resp.StatusCode, string(body))
	}

	var heartbeatResp HeartbeatResponse
	if err := json.Unmarshal(body, &heartbeatResp); err != nil {
		return nil, fmt.Errorf("failed to parse heartbeat response: %w", err)
	}

	return &heartbeatResp, nil
}

// GetPeers retrieves all peers for the tenant
func (dc *DashboardClient) GetPeers(ctx context.Context) ([]*PeerInfo, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers", dc.baseURL, dc.tenantID)
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+dc.apiKey)
	httpReq.Header.Set("X-Tenant-ID", dc.tenantID)

	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get peers: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get peers failed with status %d: %s", resp.StatusCode, string(body))
	}

	var peers []*PeerInfo
	if err := json.Unmarshal(body, &peers); err != nil {
		return nil, fmt.Errorf("failed to parse peers response: %w", err)
	}

	return peers, nil
}

// GetPeer retrieves a specific peer by ID
func (dc *DashboardClient) GetPeer(ctx context.Context, peerID string) (*PeerInfo, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s", dc.baseURL, dc.tenantID, peerID)
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+dc.apiKey)
	httpReq.Header.Set("X-Tenant-ID", dc.tenantID)

	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get peer failed with status %d: %s", resp.StatusCode, string(body))
	}

	var peer PeerInfo
	if err := json.Unmarshal(body, &peer); err != nil {
		return nil, fmt.Errorf("failed to parse peer response: %w", err)
	}

	return &peer, nil
}

// GetPeerMetrics retrieves metrics for a specific peer
func (dc *DashboardClient) GetPeerMetrics(ctx context.Context, peerID string) (*PeerMetrics, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/metrics", dc.baseURL, dc.tenantID, peerID)
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+dc.apiKey)
	httpReq.Header.Set("X-Tenant-ID", dc.tenantID)

	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer metrics: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get peer metrics failed with status %d: %s", resp.StatusCode, string(body))
	}

	var metrics PeerMetrics
	if err := json.Unmarshal(body, &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse peer metrics response: %w", err)
	}

	return &metrics, nil
}

// GetPeerHealth retrieves health status for a specific peer
func (dc *DashboardClient) GetPeerHealth(ctx context.Context, peerID string) (*PeerHealth, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/health", dc.baseURL, dc.tenantID, peerID)
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+dc.apiKey)
	httpReq.Header.Set("X-Tenant-ID", dc.tenantID)

	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer health: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get peer health failed with status %d: %s", resp.StatusCode, string(body))
	}

	var health PeerHealth
	if err := json.Unmarshal(body, &health); err != nil {
		return nil, fmt.Errorf("failed to parse peer health response: %w", err)
	}

	return &health, nil
}

// GetCommands retrieves pending commands for the peer
func (dc *DashboardClient) GetCommands(ctx context.Context, peerID string) ([]*DashboardCommand, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/commands", dc.baseURL, dc.tenantID, peerID)
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+dc.apiKey)
	httpReq.Header.Set("X-Tenant-ID", dc.tenantID)

	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get commands: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get commands failed with status %d: %s", resp.StatusCode, string(body))
	}

	var commands []*DashboardCommand
	if err := json.Unmarshal(body, &commands); err != nil {
		return nil, fmt.Errorf("failed to parse commands response: %w", err)
	}

	return commands, nil
}

// MarkCommandCompleted marks a command as completed
func (dc *DashboardClient) MarkCommandCompleted(ctx context.Context, peerID, commandID string, result map[string]interface{}) error {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/commands/%s/complete", dc.baseURL, dc.tenantID, peerID, commandID)
	
	jsonData, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal command result: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+dc.apiKey)
	httpReq.Header.Set("X-Tenant-ID", dc.tenantID)

	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to mark command completed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mark command completed failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UpdatePeerStatus updates the peer status
func (dc *DashboardClient) UpdatePeerStatus(ctx context.Context, peerID, status string) error {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/status", dc.baseURL, dc.tenantID, peerID)
	
	statusData := map[string]string{"status": status}
	jsonData, err := json.Marshal(statusData)
	if err != nil {
		return fmt.Errorf("failed to marshal status data: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+dc.apiKey)
	httpReq.Header.Set("X-Tenant-ID", dc.tenantID)

	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to update peer status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update peer status failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetTenantStats retrieves tenant-level statistics
func (dc *DashboardClient) GetTenantStats(ctx context.Context) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/stats", dc.baseURL, dc.tenantID)
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+dc.apiKey)
	httpReq.Header.Set("X-Tenant-ID", dc.tenantID)

	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant stats: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get tenant stats failed with status %d: %s", resp.StatusCode, string(body))
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse tenant stats response: %w", err)
	}

	return stats, nil
}
