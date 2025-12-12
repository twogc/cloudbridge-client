package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/dashboard"
	"github.com/twogc/cloudbridge-client/pkg/interfaces"
)

// EnhancedHeartbeatPayload represents the enhanced heartbeat payload with metrics
type EnhancedHeartbeatPayload struct {
	// Basic identification
	ServerID   string `json:"server_id"`
	TenantID   string `json:"tenant_id"`
	UserID     string `json:"user_id,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`

	// System information
	Version    string `json:"version"`
	Uptime     int64  `json:"uptime_seconds"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	GoVersion  string `json:"go_version"`

	// Network metrics
	Peers         int     `json:"peer_count"`
	BytesSent     uint64  `json:"bytes_sent"`
	BytesRecv     uint64  `json:"bytes_recv"`
	RTT           float64 `json:"rtt_ms"`
	PacketLoss    float64 `json:"packet_loss_pct"`
	BandwidthUp   float64 `json:"bandwidth_up_mbps"`
	BandwidthDown float64 `json:"bandwidth_down_mbps"`

	// Connection metrics
	DirectConnections  int `json:"direct_connections"`
	RelayConnections   int `json:"relay_connections"`
	ActiveConnections  int `json:"active_connections"`

	// Health status
	HealthStatus string `json:"health_status"` // healthy, degraded, unhealthy
	LastError    string `json:"last_error,omitempty"`

	// Performance metrics
	CPUUsage    float64 `json:"cpu_usage_pct"`
	MemoryUsage float64 `json:"memory_usage_mb"`
	Latency     float64 `json:"latency_ms"`

	// Network interface information
	InterfaceName string `json:"interface_name"`
	IPv4          string `json:"ipv4,omitempty"`
	IPv6          string `json:"ipv6,omitempty"`
	MTU           int    `json:"mtu"`

	// Timestamps
	Timestamp     time.Time `json:"timestamp"`
	LastHeartbeat time.Time `json:"last_heartbeat,omitempty"`

	// Additional metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// EnhancedManager extends the basic heartbeat manager with enhanced metrics
type EnhancedManager struct {
	*Manager
	metricsCollector     *MetricsCollector
	lastPayload          *EnhancedHeartbeatPayload
	payloadMutex         sync.RWMutex
	dashboardIntegration *DashboardIntegration
}

// MetricsCollector collects various metrics for heartbeat
type MetricsCollector struct {
	startTime        time.Time
	bytesSent        uint64
	bytesRecv        uint64
	lastBytesSent    uint64
	lastBytesRecv    uint64
	lastCollectTime  time.Time
	peerCount        int
	directConns      int
	relayConns       int
	activeConns      int
	lastError        string
	healthStatus     string
	interfaceName   string
	ipv4             string
	ipv6             string
	mtu              int
	mutex            sync.RWMutex
}

// NewEnhancedManager creates a new enhanced heartbeat manager
func NewEnhancedManager(client interfaces.ClientInterface) *EnhancedManager {
	baseManager := NewManager(client)
	collector := &MetricsCollector{
		startTime:       time.Now(),
		lastCollectTime: time.Now(),
		healthStatus:    "healthy",
	}

	return &EnhancedManager{
		Manager:          baseManager,
		metricsCollector: collector,
	}
}

// NewEnhancedManagerWithDashboard creates a new enhanced heartbeat manager with dashboard integration
func NewEnhancedManagerWithDashboard(client interfaces.ClientInterface, dashboardURL, apiKey, tenantID, peerID string) *EnhancedManager {
	manager := NewEnhancedManager(client)
	manager.dashboardIntegration = NewDashboardIntegration(dashboardURL, apiKey, tenantID, peerID)
	return manager
}

// Start starts the enhanced heartbeat mechanism
func (em *EnhancedManager) Start() error {
	if err := em.Manager.Start(); err != nil {
		return err
	}

	// Start metrics collection
	go em.metricsCollectionLoop()

	return nil
}

// Stop stops the enhanced heartbeat mechanism
func (em *EnhancedManager) Stop() {
	em.Manager.Stop()
}

// SendEnhancedHeartbeat sends an enhanced heartbeat with metrics
func (em *EnhancedManager) SendEnhancedHeartbeat() error {
	payload := em.buildEnhancedPayload()
	
	em.payloadMutex.Lock()
	em.lastPayload = payload
	em.payloadMutex.Unlock()

	// Send the enhanced heartbeat
	return em.sendEnhancedHeartbeat(payload)
}

// GetLastPayload returns the last sent heartbeat payload
func (em *EnhancedManager) GetLastPayload() *EnhancedHeartbeatPayload {
	em.payloadMutex.RLock()
	defer em.payloadMutex.RUnlock()
	return em.lastPayload
}

// GetMetrics returns current metrics
func (em *EnhancedManager) GetMetrics() map[string]interface{} {
	em.metricsCollector.mutex.RLock()
	defer em.metricsCollector.mutex.RUnlock()

	uptime := time.Since(em.metricsCollector.startTime)
	
	return map[string]interface{}{
		"uptime_seconds":     int64(uptime.Seconds()),
		"bytes_sent":        em.metricsCollector.bytesSent,
		"bytes_recv":        em.metricsCollector.bytesRecv,
		"peer_count":        em.metricsCollector.peerCount,
		"direct_connections": em.metricsCollector.directConns,
		"relay_connections": em.metricsCollector.relayConns,
		"active_connections": em.metricsCollector.activeConns,
		"health_status":     em.metricsCollector.healthStatus,
		"last_error":        em.metricsCollector.lastError,
		"interface_name":    em.metricsCollector.interfaceName,
		"ipv4":              em.metricsCollector.ipv4,
		"ipv6":              em.metricsCollector.ipv6,
		"mtu":               em.metricsCollector.mtu,
	}
}

// UpdateMetrics updates specific metrics
func (em *EnhancedManager) UpdateMetrics(updates map[string]interface{}) {
	em.metricsCollector.mutex.Lock()
	defer em.metricsCollector.mutex.Unlock()

	for key, value := range updates {
		switch key {
		case "bytes_sent":
			if v, ok := value.(uint64); ok {
				em.metricsCollector.bytesSent = v
			}
		case "bytes_recv":
			if v, ok := value.(uint64); ok {
				em.metricsCollector.bytesRecv = v
			}
		case "peer_count":
			if v, ok := value.(int); ok {
				em.metricsCollector.peerCount = v
			}
		case "direct_connections":
			if v, ok := value.(int); ok {
				em.metricsCollector.directConns = v
			}
		case "relay_connections":
			if v, ok := value.(int); ok {
				em.metricsCollector.relayConns = v
			}
		case "active_connections":
			if v, ok := value.(int); ok {
				em.metricsCollector.activeConns = v
			}
		case "health_status":
			if v, ok := value.(string); ok {
				em.metricsCollector.healthStatus = v
			}
		case "last_error":
			if v, ok := value.(string); ok {
				em.metricsCollector.lastError = v
			}
		}
	}
}

// buildEnhancedPayload builds the enhanced heartbeat payload
func (em *EnhancedManager) buildEnhancedPayload() *EnhancedHeartbeatPayload {
	em.metricsCollector.mutex.RLock()
	defer em.metricsCollector.mutex.RUnlock()

	uptime := time.Since(em.metricsCollector.startTime)
	
	// Calculate bandwidth (simplified)
	now := time.Now()
	timeDiff := now.Sub(em.metricsCollector.lastCollectTime).Seconds()
	bytesSentDiff := em.metricsCollector.bytesSent - em.metricsCollector.lastBytesSent
	bytesRecvDiff := em.metricsCollector.bytesRecv - em.metricsCollector.lastBytesRecv
	
	var bandwidthUp, bandwidthDown float64
	if timeDiff > 0 {
		bandwidthUp = float64(bytesSentDiff) * 8 / (timeDiff * 1000000) // Mbps
		bandwidthDown = float64(bytesRecvDiff) * 8 / (timeDiff * 1000000) // Mbps
	}

	// Get system metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryUsage := float64(m.Alloc) / 1024 / 1024 // MB

	// Get network interface info
	ipv4, ipv6 := em.getNetworkInterfaceInfo()

	payload := &EnhancedHeartbeatPayload{
		// Basic identification
		ServerID: em.getServerID(),
		TenantID: em.getTenantID(),
		UserID:   em.getUserID(),
		DeviceID: em.getDeviceID(),

		// System information
		Version:    em.getVersion(),
		Uptime:     int64(uptime.Seconds()),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		GoVersion:  runtime.Version(),

		// Network metrics
		Peers:         em.metricsCollector.peerCount,
		BytesSent:    em.metricsCollector.bytesSent,
		BytesRecv:    em.metricsCollector.bytesRecv,
		RTT:          em.estimateRTT(),
		PacketLoss:   em.estimatePacketLoss(),
		BandwidthUp:   bandwidthUp,
		BandwidthDown: bandwidthDown,

		// Connection metrics
		DirectConnections: em.metricsCollector.directConns,
		RelayConnections:  em.metricsCollector.relayConns,
		ActiveConnections: em.metricsCollector.activeConns,

		// Health status
		HealthStatus: em.metricsCollector.healthStatus,
		LastError:    em.metricsCollector.lastError,

		// Performance metrics
		CPUUsage:    em.estimateCPUUsage(),
		MemoryUsage: memoryUsage,
		Latency:     em.estimateLatency(),

		// Network interface information
		InterfaceName: em.metricsCollector.interfaceName,
		IPv4:          ipv4,
		IPv6:          ipv6,
		MTU:           em.metricsCollector.mtu,

		// Timestamps
		Timestamp:     time.Now(),
		LastHeartbeat: em.Manager.GetLastBeat(),

		// Additional metadata
		Metadata: em.buildMetadata(),
	}

	// Update last collection time and values
	em.metricsCollector.lastCollectTime = now
	em.metricsCollector.lastBytesSent = em.metricsCollector.bytesSent
	em.metricsCollector.lastBytesRecv = em.metricsCollector.bytesRecv

	return payload
}

// sendEnhancedHeartbeat sends the enhanced heartbeat to the server
func (em *EnhancedManager) sendEnhancedHeartbeat(payload *EnhancedHeartbeatPayload) error {
	// Send to dashboard if integration is enabled
	if em.dashboardIntegration != nil && em.dashboardIntegration.IsEnabled() {
		ctx := context.Background()
		if err := em.dashboardIntegration.SendHeartbeatToDashboard(ctx, payload); err != nil {
			// Log error but don't fail the heartbeat
			// This ensures the basic heartbeat still works even if dashboard is down
			fmt.Printf("Warning: Dashboard heartbeat failed: %v\n", err)
		}
	}

	// Convert payload to JSON (for future use)
	_, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat payload: %w", err)
	}

	// Send via client interface
	// This would need to be implemented in the client interface
	// For now, we'll use the basic heartbeat mechanism
	return em.Manager.SendManualHeartbeat()
}

// metricsCollectionLoop continuously collects metrics
func (em *EnhancedManager) metricsCollectionLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-em.stopChan:
			return
		case <-ticker.C:
			em.collectMetrics()
		}
	}
}

// collectMetrics collects various system and network metrics
func (em *EnhancedManager) collectMetrics() {
	em.metricsCollector.mutex.Lock()
	defer em.metricsCollector.mutex.Unlock()

	// Update network interface information
	em.updateNetworkInterfaceInfo()

	// Update connection metrics
	em.updateConnectionMetrics()

	// Update health status
	em.updateHealthStatus()
}

// updateNetworkInterfaceInfo updates network interface information
func (em *EnhancedManager) updateNetworkInterfaceInfo() {
	// This would integrate with the actual network interface
	// For now, use default values
	em.metricsCollector.interfaceName = "cloudbridge0"
	em.metricsCollector.mtu = 1420
}

// updateConnectionMetrics updates connection-related metrics
func (em *EnhancedManager) updateConnectionMetrics() {
	// This would integrate with the actual P2P manager
	// For now, use mock data
	em.metricsCollector.peerCount = 12
	em.metricsCollector.directConns = 8
	em.metricsCollector.relayConns = 4
	em.metricsCollector.activeConns = 5
}

// updateHealthStatus updates the health status
func (em *EnhancedManager) updateHealthStatus() {
	// Simple health status logic
	if em.Manager.GetFailCount() > 0 {
		em.metricsCollector.healthStatus = "degraded"
	} else {
		em.metricsCollector.healthStatus = "healthy"
	}
}

// Helper methods for getting various identifiers and metrics
func (em *EnhancedManager) getServerID() string {
	// This would get the actual server ID from config or client
	return "server-001"
}

func (em *EnhancedManager) getTenantID() string {
	// This would get the actual tenant ID from config or client
	return "tenant-001"
}

func (em *EnhancedManager) getUserID() string {
	// This would get the actual user ID from config or client
	return "user-001"
}

func (em *EnhancedManager) getDeviceID() string {
	// This would get the actual device ID from config or client
	return "device-001"
}

func (em *EnhancedManager) getVersion() string {
	// This would get the actual version from build info
	return "1.4.20"
}

func (em *EnhancedManager) getNetworkInterfaceInfo() (string, string) {
	// This would get actual network interface information
	return "10.16.1.42", "fd00::42"
}

func (em *EnhancedManager) estimateRTT() float64 {
	// This would measure actual RTT to relay server
	return 15.5
}

func (em *EnhancedManager) estimatePacketLoss() float64 {
	// This would measure actual packet loss
	return 0.02
}

func (em *EnhancedManager) estimateCPUUsage() float64 {
	// This would measure actual CPU usage
	return 2.5
}

func (em *EnhancedManager) estimateLatency() float64 {
	// This would measure actual latency
	return 15.5
}

func (em *EnhancedManager) buildMetadata() map[string]interface{} {
	return map[string]interface{}{
		"client_type":    "cloudbridge-client",
		"build_type":     "production",
		"platform":       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		"go_routines":    runtime.NumGoroutine(),
		"gc_runs":        runtime.NumGoroutine(), // Simplified
		"last_gc":        time.Now().Format(time.RFC3339),
	}
}

// Override the basic heartbeat loop to use enhanced payload

// API endpoints for dashboard integration
const (
	HeartbeatEndpoint = "/api/v1/tenants/{tenant_id}/peers/heartbeat"
	PeersEndpoint     = "/api/v1/tenants/{tenant_id}/peers"
	MetricsEndpoint  = "/api/v1/tenants/{tenant_id}/peers/{peer_id}/metrics"
)

// SendHeartbeatToDashboard sends heartbeat to dashboard API
func (em *EnhancedManager) SendHeartbeatToDashboard(ctx context.Context, dashboardURL string) error {
	_ = em.buildEnhancedPayload()
	
	// This would make an HTTP POST request to the dashboard API
	// Implementation would depend on the specific API requirements
	
	return nil
}


// GetPeerMetricsFromDashboard retrieves specific peer metrics from dashboard
func (em *EnhancedManager) GetPeerMetricsFromDashboard(ctx context.Context, peerID string) (map[string]interface{}, error) {
	if em.dashboardIntegration == nil {
		return nil, fmt.Errorf("dashboard integration not configured")
	}

	metrics, err := em.dashboardIntegration.GetPeerMetricsFromDashboard(ctx, peerID)
	if err != nil {
		return nil, err
	}

	// Convert to generic map
	result := map[string]interface{}{
		"bytes_sent":         metrics.BytesSent,
		"bytes_recv":         metrics.BytesRecv,
		"packet_loss":        metrics.PacketLoss,
		"latency":            metrics.Latency,
		"bandwidth_up":       metrics.BandwidthUp,
		"bandwidth_down":     metrics.BandwidthDown,
		"connections":        metrics.Connections,
		"direct_connections": metrics.DirectConns,
		"relay_connections":  metrics.RelayConns,
		"cpu_usage":          metrics.CPUUsage,
		"memory_usage":       metrics.MemoryUsage,
		"last_updated":       metrics.LastUpdated,
	}

	return result, nil
}

// GetPeersFromDashboard retrieves all peers from dashboard
func (em *EnhancedManager) GetPeersFromDashboard(ctx context.Context) ([]*dashboard.PeerInfo, error) {
	if em.dashboardIntegration == nil {
		return nil, fmt.Errorf("dashboard integration not configured")
	}

	return em.dashboardIntegration.GetPeersFromDashboard(ctx)
}

// GetPeerHealthFromDashboard retrieves peer health from dashboard
func (em *EnhancedManager) GetPeerHealthFromDashboard(ctx context.Context, peerID string) (*dashboard.PeerHealth, error) {
	if em.dashboardIntegration == nil {
		return nil, fmt.Errorf("dashboard integration not configured")
	}

	return em.dashboardIntegration.GetPeerHealthFromDashboard(ctx, peerID)
}

// GetCommandsFromDashboard retrieves pending commands from dashboard
func (em *EnhancedManager) GetCommandsFromDashboard(ctx context.Context) ([]*dashboard.DashboardCommand, error) {
	if em.dashboardIntegration == nil {
		return nil, fmt.Errorf("dashboard integration not configured")
	}

	return em.dashboardIntegration.GetCommandsFromDashboard(ctx)
}

// UpdatePeerStatus updates peer status in dashboard
func (em *EnhancedManager) UpdatePeerStatus(ctx context.Context, status string) error {
	if em.dashboardIntegration == nil {
		return fmt.Errorf("dashboard integration not configured")
	}

	return em.dashboardIntegration.UpdatePeerStatus(ctx, status)
}

// GetTenantStats retrieves tenant statistics from dashboard
func (em *EnhancedManager) GetTenantStats(ctx context.Context) (map[string]interface{}, error) {
	if em.dashboardIntegration == nil {
		return nil, fmt.Errorf("dashboard integration not configured")
	}

	return em.dashboardIntegration.GetTenantStats(ctx)
}
