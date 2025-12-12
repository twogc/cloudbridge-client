package heartbeat

import (
	"runtime"
	"time"
)

// HeartbeatPayload represents extended heartbeat data sent to control plane
type HeartbeatPayload struct {
	// Basic identification
	ServerID string `json:"server_id"`
	TenantID string `json:"tenant_id"`

	// Client information
	Version   string `json:"version"`
	BuildTime string `json:"build_time,omitempty"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Hostname  string `json:"hostname,omitempty"`

	// Uptime and health
	UptimeSeconds   int64     `json:"uptime_seconds"`
	LastStartTime   time.Time `json:"last_start_time"`
	HealthStatus    string    `json:"health_status"` // healthy, degraded, unhealthy
	HeartbeatSeqNum uint64    `json:"heartbeat_seq_num"`

	// Connectivity
	PeerCount           int     `json:"peer_count"`
	DirectPeerCount     int     `json:"direct_peer_count"`
	RelayedPeerCount    int     `json:"relayed_peer_count"`
	ConnectionQuality   string  `json:"connection_quality"` // excellent, good, fair, poor
	AverageRTT          float64 `json:"average_rtt_ms"`
	P95RTT              float64 `json:"p95_rtt_ms"`
	PacketLossPercent   float64 `json:"packet_loss_pct"`
	JitterMS            float64 `json:"jitter_ms"`

	// Network statistics
	BytesSent        uint64  `json:"bytes_sent"`
	BytesReceived    uint64  `json:"bytes_recv"`
	PacketsSent      uint64  `json:"packets_sent"`
	PacketsReceived  uint64  `json:"packets_recv"`
	ThroughputMbps   float64 `json:"throughput_mbps"`

	// System resources
	CPUUsagePercent   float64 `json:"cpu_usage_pct"`
	MemoryUsedMB      uint64  `json:"memory_used_mb"`
	MemoryTotalMB     uint64  `json:"memory_total_mb"`
	GoRoutines        int     `json:"goroutines"`
	ConnectionsActive int     `json:"connections_active"`

	// Features enabled
	Features FeatureFlags `json:"features"`

	// Error tracking
	RecentErrors []ErrorInfo `json:"recent_errors,omitempty"`
	ErrorCount   int         `json:"error_count_last_minute"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// FeatureFlags represents enabled features
type FeatureFlags struct {
	P2PEnabled       bool `json:"p2p_enabled"`
	WireGuardEnabled bool `json:"wireguard_enabled"`
	QUICEnabled      bool `json:"quic_enabled"`
	WebSocketEnabled bool `json:"websocket_enabled"`
	MetricsEnabled   bool `json:"metrics_enabled"`
}

// ErrorInfo represents recent error information
type ErrorInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Count     int       `json:"count"` // Number of times this error occurred
}

// PayloadBuilder helps build heartbeat payloads
type PayloadBuilder struct {
	payload      *HeartbeatPayload
	startTime    time.Time
	sequenceNum  uint64
	errorTracker *ErrorTracker
}

// NewPayloadBuilder creates a new payload builder
func NewPayloadBuilder(serverID, tenantID string) *PayloadBuilder {
	return &PayloadBuilder{
		payload: &HeartbeatPayload{
			ServerID:     serverID,
			TenantID:     tenantID,
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			HealthStatus: "healthy",
		},
		startTime:    time.Now(),
		errorTracker: NewErrorTracker(),
	}
}

// WithVersion sets version information
func (b *PayloadBuilder) WithVersion(version, buildTime string) *PayloadBuilder {
	b.payload.Version = version
	b.payload.BuildTime = buildTime
	return b
}

// WithHostname sets hostname
func (b *PayloadBuilder) WithHostname(hostname string) *PayloadBuilder {
	b.payload.Hostname = hostname
	return b
}

// WithUptime calculates uptime
func (b *PayloadBuilder) WithUptime() *PayloadBuilder {
	b.payload.UptimeSeconds = int64(time.Since(b.startTime).Seconds())
	b.payload.LastStartTime = b.startTime
	return b
}

// WithPeerStats sets peer statistics
func (b *PayloadBuilder) WithPeerStats(total, direct, relayed int) *PayloadBuilder {
	b.payload.PeerCount = total
	b.payload.DirectPeerCount = direct
	b.payload.RelayedPeerCount = relayed
	return b
}

// WithNetworkQuality sets network quality metrics
func (b *PayloadBuilder) WithNetworkQuality(avgRTT, p95RTT, packetLoss, jitter float64) *PayloadBuilder {
	b.payload.AverageRTT = avgRTT
	b.payload.P95RTT = p95RTT
	b.payload.PacketLossPercent = packetLoss
	b.payload.JitterMS = jitter

	// Determine connection quality
	if avgRTT < 50 && packetLoss < 0.5 {
		b.payload.ConnectionQuality = "excellent"
	} else if avgRTT < 100 && packetLoss < 1.0 {
		b.payload.ConnectionQuality = "good"
	} else if avgRTT < 200 && packetLoss < 2.0 {
		b.payload.ConnectionQuality = "fair"
	} else {
		b.payload.ConnectionQuality = "poor"
	}

	return b
}

// WithTrafficStats sets traffic statistics
func (b *PayloadBuilder) WithTrafficStats(bytesSent, bytesRecv, packetsSent, packetsRecv uint64) *PayloadBuilder {
	b.payload.BytesSent = bytesSent
	b.payload.BytesReceived = bytesRecv
	b.payload.PacketsSent = packetsSent
	b.payload.PacketsReceived = packetsRecv

	// Calculate throughput (simple approximation)
	// Use uptime duration in seconds (as float64 for precision)
	var uptimeDuration float64
	if b.payload.UptimeSeconds > 0 {
		uptimeDuration = float64(b.payload.UptimeSeconds)
	} else {
		// Calculate from startTime, preserving sub-second precision
		uptimeDuration = time.Since(b.startTime).Seconds()
	}

	if uptimeDuration > 0 {
		totalBytes := float64(bytesSent + bytesRecv)
		totalBits := totalBytes * 8
		throughputBps := totalBits / uptimeDuration
		b.payload.ThroughputMbps = throughputBps / 1_000_000
	}

	return b
}

// WithSystemStats sets system resource statistics
func (b *PayloadBuilder) WithSystemStats(cpuUsage float64, memUsedMB, memTotalMB uint64) *PayloadBuilder {
	b.payload.CPUUsagePercent = cpuUsage
	b.payload.MemoryUsedMB = memUsedMB
	b.payload.MemoryTotalMB = memTotalMB
	b.payload.GoRoutines = runtime.NumGoroutine()
	return b
}

// WithConnectionCount sets active connections count
func (b *PayloadBuilder) WithConnectionCount(count int) *PayloadBuilder {
	b.payload.ConnectionsActive = count
	return b
}

// WithFeatures sets enabled features
func (b *PayloadBuilder) WithFeatures(p2p, wireguard, quic, websocket, metrics bool) *PayloadBuilder {
	b.payload.Features = FeatureFlags{
		P2PEnabled:       p2p,
		WireGuardEnabled: wireguard,
		QUICEnabled:      quic,
		WebSocketEnabled: websocket,
		MetricsEnabled:   metrics,
	}
	return b
}

// WithHealthStatus sets health status
func (b *PayloadBuilder) WithHealthStatus(status string) *PayloadBuilder {
	b.payload.HealthStatus = status
	return b
}

// AddError adds an error to tracking
func (b *PayloadBuilder) AddError(errorType, message string) *PayloadBuilder {
	b.errorTracker.AddError(errorType, message)
	return b
}

// Build builds the final payload
func (b *PayloadBuilder) Build() *HeartbeatPayload {
	b.sequenceNum++
	b.payload.HeartbeatSeqNum = b.sequenceNum
	b.payload.Timestamp = time.Now()

	// Add recent errors
	b.payload.RecentErrors = b.errorTracker.GetRecentErrors(5)
	b.payload.ErrorCount = b.errorTracker.GetErrorCountLastMinute()

	// Create a copy of the payload to return
	// This ensures each Build() call returns a distinct payload
	payloadCopy := *b.payload
	return &payloadCopy
}

// ErrorTracker tracks errors for heartbeat reporting
type ErrorTracker struct {
	errors     []ErrorInfo
	maxErrors  int
	errorCounts map[string]int
}

// NewErrorTracker creates a new error tracker
func NewErrorTracker() *ErrorTracker {
	return &ErrorTracker{
		errors:      make([]ErrorInfo, 0, 100),
		maxErrors:   100,
		errorCounts: make(map[string]int),
	}
}

// AddError adds an error to the tracker
func (t *ErrorTracker) AddError(errorType, message string) {
	now := time.Now()

	// Check if this error already exists (deduplication)
	key := errorType + ":" + message
	t.errorCounts[key]++

	// Find existing error or add new one
	found := false
	for i := range t.errors {
		if t.errors[i].Type == errorType && t.errors[i].Message == message {
			t.errors[i].Count = t.errorCounts[key]
			t.errors[i].Timestamp = now
			found = true
			break
		}
	}

	if !found {
		t.errors = append(t.errors, ErrorInfo{
			Timestamp: now,
			Type:      errorType,
			Message:   message,
			Count:     1,
		})
	}

	// Keep only recent errors
	if len(t.errors) > t.maxErrors {
		t.errors = t.errors[len(t.errors)-t.maxErrors:]
	}
}

// GetRecentErrors returns N most recent unique errors
func (t *ErrorTracker) GetRecentErrors(n int) []ErrorInfo {
	if len(t.errors) == 0 {
		return nil
	}

	// Sort by timestamp (most recent first)
	start := len(t.errors) - n
	if start < 0 {
		start = 0
	}

	result := make([]ErrorInfo, len(t.errors)-start)
	copy(result, t.errors[start:])

	// Reverse to get most recent first
	for i := 0; i < len(result)/2; i++ {
		result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
	}

	return result
}

// GetErrorCountLastMinute returns error count in last minute
func (t *ErrorTracker) GetErrorCountLastMinute() int {
	now := time.Now()
	oneMinuteAgo := now.Add(-time.Minute)

	count := 0
	for _, err := range t.errors {
		if err.Timestamp.After(oneMinuteAgo) {
			count += err.Count
		}
	}

	return count
}

// Clear clears all tracked errors
func (t *ErrorTracker) Clear() {
	t.errors = make([]ErrorInfo, 0, 100)
	t.errorCounts = make(map[string]int)
}
