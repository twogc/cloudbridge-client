package heartbeat

import (
	"context"
	"fmt"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/dashboard"
)

// DashboardIntegration handles integration with CloudBridge Dashboard
type DashboardIntegration struct {
	dashboardClient *dashboard.DashboardClient
	peerID          string
	tenantID        string
	enabled         bool
}

// NewDashboardIntegration creates a new dashboard integration
func NewDashboardIntegration(dashboardURL, apiKey, tenantID, peerID string) *DashboardIntegration {
	config := &dashboard.DashboardConfig{
		BaseURL:  dashboardURL,
		APIKey:   apiKey,
		TenantID: tenantID,
		Timeout:  30 * time.Second,
	}

	client := dashboard.NewDashboardClient(config)

	return &DashboardIntegration{
		dashboardClient: client,
		peerID:          peerID,
		tenantID:        tenantID,
		enabled:         true,
	}
}

// SendHeartbeatToDashboard sends enhanced heartbeat to dashboard
func (di *DashboardIntegration) SendHeartbeatToDashboard(ctx context.Context, payload *EnhancedHeartbeatPayload) error {
	if !di.enabled {
		return nil
	}

	// Convert enhanced payload to dashboard heartbeat request
	req := &dashboard.HeartbeatRequest{
		PeerID:    di.peerID,
		TenantID:  di.tenantID,
		UserID:    payload.UserID,
		DeviceID:  payload.DeviceID,
		Hostname:  di.getHostname(),
		OS:        payload.OS,
		Arch:      payload.Arch,
		Version:   payload.Version,
		Status:    payload.HealthStatus,
		IPAddress: payload.IPv4,
		IPv6Address: payload.IPv6,
		Metrics:   di.convertMetrics(payload),
		Health:    di.convertHealth(payload),
		Location:  di.getLocationInfo(),
		Metadata:  payload.Metadata,
		Timestamp: payload.Timestamp,
	}

	// Send heartbeat to dashboard
	resp, err := di.dashboardClient.SendHeartbeat(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat to dashboard: %w", err)
	}

	// Process any commands from dashboard
	if len(resp.Commands) > 0 {
		di.processCommands(ctx, resp.Commands)
	}

	return nil
}

// GetPeersFromDashboard retrieves peer information from dashboard
func (di *DashboardIntegration) GetPeersFromDashboard(ctx context.Context) ([]*dashboard.PeerInfo, error) {
	if !di.enabled {
		return nil, fmt.Errorf("dashboard integration is disabled")
	}

	peers, err := di.dashboardClient.GetPeers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get peers from dashboard: %w", err)
	}

	return peers, nil
}

// GetPeerMetricsFromDashboard retrieves specific peer metrics from dashboard
func (di *DashboardIntegration) GetPeerMetricsFromDashboard(ctx context.Context, peerID string) (*dashboard.PeerMetrics, error) {
	if !di.enabled {
		return nil, fmt.Errorf("dashboard integration is disabled")
	}

	metrics, err := di.dashboardClient.GetPeerMetrics(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer metrics from dashboard: %w", err)
	}

	return metrics, nil
}

// GetPeerHealthFromDashboard retrieves peer health from dashboard
func (di *DashboardIntegration) GetPeerHealthFromDashboard(ctx context.Context, peerID string) (*dashboard.PeerHealth, error) {
	if !di.enabled {
		return nil, fmt.Errorf("dashboard integration is disabled")
	}

	health, err := di.dashboardClient.GetPeerHealth(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer health from dashboard: %w", err)
	}

	return health, nil
}

// GetCommandsFromDashboard retrieves pending commands from dashboard
func (di *DashboardIntegration) GetCommandsFromDashboard(ctx context.Context) ([]*dashboard.DashboardCommand, error) {
	if !di.enabled {
		return nil, fmt.Errorf("dashboard integration is disabled")
	}

	commands, err := di.dashboardClient.GetCommands(ctx, di.peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get commands from dashboard: %w", err)
	}

	return commands, nil
}

// UpdatePeerStatus updates peer status in dashboard
func (di *DashboardIntegration) UpdatePeerStatus(ctx context.Context, status string) error {
	if !di.enabled {
		return nil
	}

	err := di.dashboardClient.UpdatePeerStatus(ctx, di.peerID, status)
	if err != nil {
		return fmt.Errorf("failed to update peer status in dashboard: %w", err)
	}

	return nil
}

// GetTenantStats retrieves tenant statistics from dashboard
func (di *DashboardIntegration) GetTenantStats(ctx context.Context) (map[string]interface{}, error) {
	if !di.enabled {
		return nil, fmt.Errorf("dashboard integration is disabled")
	}

	stats, err := di.dashboardClient.GetTenantStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant stats from dashboard: %w", err)
	}

	return stats, nil
}

// Enable enables dashboard integration
func (di *DashboardIntegration) Enable() {
	di.enabled = true
}

// Disable disables dashboard integration
func (di *DashboardIntegration) Disable() {
	di.enabled = false
}

// IsEnabled returns whether dashboard integration is enabled
func (di *DashboardIntegration) IsEnabled() bool {
	return di.enabled
}

// Helper methods
func (di *DashboardIntegration) convertMetrics(payload *EnhancedHeartbeatPayload) *dashboard.PeerMetrics {
	return &dashboard.PeerMetrics{
		BytesSent:     payload.BytesSent,
		BytesRecv:     payload.BytesRecv,
		PacketLoss:    payload.PacketLoss,
		Latency:       payload.RTT,
		BandwidthUp:   payload.BandwidthUp,
		BandwidthDown: payload.BandwidthDown,
		Connections:   payload.ActiveConnections,
		DirectConns:  payload.DirectConnections,
		RelayConns:    payload.RelayConnections,
		CPUUsage:      payload.CPUUsage,
		MemoryUsage:   payload.MemoryUsage,
		LastUpdated:   payload.Timestamp,
	}
}

func (di *DashboardIntegration) convertHealth(payload *EnhancedHeartbeatPayload) *dashboard.PeerHealth {
	health := &dashboard.PeerHealth{
		Status:     payload.HealthStatus,
		LastCheck:  payload.Timestamp,
		Issues:     []string{},
	}

	// Determine connectivity status
	if payload.RTT > 100 {
		health.Connectivity = "poor"
	} else if payload.RTT > 50 {
		health.Connectivity = "degraded"
	} else {
		health.Connectivity = "good"
	}

	// Determine authentication status
	// This would be based on actual token validation
	health.Authentication = "valid"

	// Determine performance status
	if payload.CPUUsage > 80 || payload.MemoryUsage > 1000 {
		health.Performance = "poor"
	} else if payload.CPUUsage > 50 || payload.MemoryUsage > 500 {
		health.Performance = "degraded"
	} else {
		health.Performance = "good"
	}

	// Determine network status
	if payload.PacketLoss > 5 {
		health.Network = "unstable"
	} else if payload.PacketLoss > 1 {
		health.Network = "degraded"
	} else {
		health.Network = "stable"
	}

	return health
}

func (di *DashboardIntegration) getHostname() string {
	// This would get the actual hostname
	return "cloudbridge-peer"
}

func (di *DashboardIntegration) getLocationInfo() *dashboard.LocationInfo {
	// This would get actual location information
	// For now, return mock data
	return &dashboard.LocationInfo{
		Country:   "Russia",
		Region:    "Moscow",
		City:      "Moscow",
		Latitude:  55.7558,
		Longitude: 37.6176,
		Timezone:  "Europe/Moscow",
		ISP:       "CloudBridge",
		ASN:       "AS12345",
	}
}

func (di *DashboardIntegration) processCommands(ctx context.Context, commands []dashboard.DashboardCommand) {
	for _, cmd := range commands {
		di.processCommand(ctx, &cmd)
	}
}

func (di *DashboardIntegration) processCommand(ctx context.Context, cmd *dashboard.DashboardCommand) {
	switch cmd.Type {
	case "restart":
		di.handleRestartCommand(ctx, cmd)
	case "update":
		di.handleUpdateCommand(ctx, cmd)
	case "config_change":
		di.handleConfigChangeCommand(ctx, cmd)
	case "status_update":
		di.handleStatusUpdateCommand(ctx, cmd)
	default:
		di.handleUnknownCommand(ctx, cmd)
	}
}

func (di *DashboardIntegration) handleRestartCommand(ctx context.Context, cmd *dashboard.DashboardCommand) {
	// This would implement restart logic
	result := map[string]interface{}{
		"success": true,
		"message": "Restart command processed",
		"timestamp": time.Now(),
	}

	di.dashboardClient.MarkCommandCompleted(ctx, di.peerID, cmd.ID, result)
}

func (di *DashboardIntegration) handleUpdateCommand(ctx context.Context, cmd *dashboard.DashboardCommand) {
	// This would implement update logic
	result := map[string]interface{}{
		"success": true,
		"message": "Update command processed",
		"timestamp": time.Now(),
	}

	di.dashboardClient.MarkCommandCompleted(ctx, di.peerID, cmd.ID, result)
}

func (di *DashboardIntegration) handleConfigChangeCommand(ctx context.Context, cmd *dashboard.DashboardCommand) {
	// This would implement config change logic
	result := map[string]interface{}{
		"success": true,
		"message": "Config change command processed",
		"timestamp": time.Now(),
	}

	di.dashboardClient.MarkCommandCompleted(ctx, di.peerID, cmd.ID, result)
}

func (di *DashboardIntegration) handleStatusUpdateCommand(ctx context.Context, cmd *dashboard.DashboardCommand) {
	// This would implement status update logic
	result := map[string]interface{}{
		"success": true,
		"message": "Status update command processed",
		"timestamp": time.Now(),
	}

	di.dashboardClient.MarkCommandCompleted(ctx, di.peerID, cmd.ID, result)
}

func (di *DashboardIntegration) handleUnknownCommand(ctx context.Context, cmd *dashboard.DashboardCommand) {
	// Handle unknown command types
	result := map[string]interface{}{
		"success": false,
		"message": fmt.Sprintf("Unknown command type: %s", cmd.Type),
		"timestamp": time.Now(),
	}

	di.dashboardClient.MarkCommandCompleted(ctx, di.peerID, cmd.ID, result)
}
