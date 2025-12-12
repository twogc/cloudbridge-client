package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
)

// PushgatewayConfig contains Pushgateway configuration
type PushgatewayConfig struct {
	Enabled      bool
	URL          string
	JobName      string
	Instance     string
	PushInterval time.Duration
}

// Metrics represents the metrics system
type Metrics struct {
	enabled  bool
	running  bool
	port     int
	server   *http.Server
	registry *prometheus.Registry

	// Pushgateway support
	pushgatewayConfig *PushgatewayConfig
	pusher            *push.Pusher
	pushCtx           context.Context
	pushCancel        context.CancelFunc
	pushMutex         sync.RWMutex

	// Required client metrics (объединены в одну метрику)
	clientBytes   *prometheus.CounterVec
	p2pSessions   prometheus.Gauge
	transportMode prometheus.Gauge

	// Prometheus metrics (без tunnel_id для снижения кардинальности)
	bytesTransferred   *prometheus.CounterVec
	connectionsHandled *prometheus.CounterVec
	activeConnections  *prometheus.GaugeVec
	connectionDuration *prometheus.HistogramVec
	bufferPoolSize     *prometheus.GaugeVec
	bufferPoolUsage    *prometheus.GaugeVec
	errorsTotal        *prometheus.CounterVec
	heartbeatLatency   *prometheus.HistogramVec
}

// NewMetrics creates a new metrics system
func NewMetrics(enabled bool, port int) *Metrics {
	m := &Metrics{
		enabled: enabled,
		port:    port,
	}

	if enabled {
		m.initPrometheusMetrics()
	}

	return m
}

// NewMetricsWithPushgateway creates a new metrics system with Pushgateway support
func NewMetricsWithPushgateway(enabled bool, port int, pushConfig *PushgatewayConfig) *Metrics {
	m := &Metrics{
		enabled:           enabled,
		port:              port,
		pushgatewayConfig: pushConfig,
	}

	if enabled {
		m.initPrometheusMetrics()
		if pushConfig != nil && pushConfig.Enabled {
			m.initPushgateway()
		}
	}

	return m
}

// initPrometheusMetrics initializes Prometheus metrics
func (m *Metrics) initPrometheusMetrics() {
	// Создаем собственный registry для избежания паники при повторной инициализации
	m.registry = prometheus.NewRegistry()

	// Required client metrics (объединены в одну метрику)
	m.clientBytes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudbridge_client_bytes_total",
			Help: "Total bytes by client",
		},
		[]string{"direction"}, // sent/recv
	)

	m.p2pSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "p2p_sessions",
			Help: "Number of active P2P sessions",
		},
	)

	m.transportMode = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "transport_mode",
			Help: "Current transport mode (0=QUIC, 1=WireGuard, 2=gRPC)",
		},
	)

	// Bytes transferred counter (без tunnel_id для снижения кардинальности)
	m.bytesTransferred = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudbridge_bytes_transferred_total",
			Help: "Total bytes transferred through tunnels",
		},
		[]string{"tenant_id", "direction"}, // убрали tunnel_id
	)

	// Connections handled counter (без tunnel_id)
	m.connectionsHandled = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudbridge_connections_handled_total",
			Help: "Total connections handled by tunnels",
		},
		[]string{"tenant_id"}, // убрали tunnel_id
	)

	// Active connections gauge (без tunnel_id)
	m.activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudbridge_active_connections",
			Help: "Number of active connections",
		},
		[]string{"tenant_id"}, // убрали tunnel_id
	)

	// Connection duration histogram (без tunnel_id, настроенные buckets)
	m.connectionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cloudbridge_connection_duration_seconds",
			Help:    "Connection duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30}, // сетевые длительности
		},
		[]string{"tenant_id"}, // убрали tunnel_id
	)

	// Buffer pool size gauge (без tunnel_id)
	m.bufferPoolSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudbridge_buffer_pool_size",
			Help: "Buffer pool size",
		},
		[]string{}, // убрали tunnel_id
	)

	// Buffer pool usage gauge (без tunnel_id)
	m.bufferPoolUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudbridge_buffer_pool_usage",
			Help: "Buffer pool usage",
		},
		[]string{}, // убрали tunnel_id
	)

	// Errors total counter (без tunnel_id)
	m.errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudbridge_errors_total",
			Help: "Total number of errors",
		},
		[]string{"error_type", "tenant_id"}, // убрали tunnel_id
	)

	// Heartbeat latency histogram (настроенные buckets)
	m.heartbeatLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cloudbridge_heartbeat_latency_seconds",
			Help:    "Heartbeat latency in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}, // сетевые задержки
		},
		[]string{"tenant_id"},
	)

	// Регистрируем метрики в собственном registry
	m.registry.MustRegister(
		m.clientBytes,
		m.p2pSessions,
		m.transportMode,
		m.bytesTransferred,
		m.connectionsHandled,
		m.activeConnections,
		m.connectionDuration,
		m.bufferPoolSize,
		m.bufferPoolUsage,
		m.errorsTotal,
		m.heartbeatLatency,
	)
}

// initPushgateway initializes Pushgateway pusher
func (m *Metrics) initPushgateway() {
	if m.pushgatewayConfig == nil || !m.pushgatewayConfig.Enabled {
		return
	}

	// Create pusher с собственным registry
	m.pusher = push.New(m.pushgatewayConfig.URL, m.pushgatewayConfig.JobName).
		Grouping("instance", m.pushgatewayConfig.Instance)

	// Добавляем все метрики из registry в pusher
	// TODO: После стабилизации API добавить Gathering(m.registry)

	// Start push context
	m.pushCtx, m.pushCancel = context.WithCancel(context.Background())

	// Start periodic pushing
	go m.pushLoop()

	fmt.Printf("Pushgateway initialized: %s (job: %s, instance: %s)\n",
		m.pushgatewayConfig.URL,
		m.pushgatewayConfig.JobName,
		m.pushgatewayConfig.Instance)
}

// pushLoop runs the periodic push to Pushgateway
func (m *Metrics) pushLoop() {
	ticker := time.NewTicker(m.pushgatewayConfig.PushInterval)
	defer ticker.Stop()

	// Initial push
	m.pushMetrics()

	for {
		select {
		case <-m.pushCtx.Done():
			return
		case <-ticker.C:
			m.pushMetrics()
		}
	}
}

// pushMetrics pushes metrics to Pushgateway with exponential backoff
func (m *Metrics) pushMetrics() {
	m.pushMutex.RLock()
	pusher := m.pusher
	m.pushMutex.RUnlock()

	if pusher == nil {
		return
	}

	// Exponential backoff parameters
	maxRetries := 3
	baseDelay := 1 * time.Second
	maxDelay := 30 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := pusher.Push()
		if err == nil {
			// Success
			return
		}

		// Calculate delay with exponential backoff
		delay := time.Duration(1<<uint(attempt)) * baseDelay
		if delay > maxDelay {
			delay = maxDelay
		}

		fmt.Printf("Failed to push metrics to Pushgateway (attempt %d/%d): %v, retrying in %v\n",
			attempt+1, maxRetries, err, delay)

		// Wait before retry
		select {
		case <-m.pushCtx.Done():
			return
		case <-time.After(delay):
			continue
		}
	}

	fmt.Printf("Failed to push metrics to Pushgateway after %d attempts\n", maxRetries)
}

// Start starts the metrics server
func (m *Metrics) Start() error {
	if !m.enabled {
		return nil
	}

	// Защита от повторного запуска
	if m.running {
		return fmt.Errorf("metrics server already started")
	}

	mux := http.NewServeMux()
	// Используем собственный registry
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))

	m.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", m.port), // Безопасность: только localhost
		Handler: mux,
	}

	m.running = true
	go func() {
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()

	fmt.Printf("Metrics server started on 127.0.0.1:%d\n", m.port)
	return nil
}

// Stop stops the metrics server
func (m *Metrics) Stop() error {
	// Stop Pushgateway pushing
	m.pushMutex.Lock()
	if m.pushCancel != nil {
		m.pushCancel()
		m.pushCancel = nil // Prevent double cancel
	}

	// Удаляем метрики из Pushgateway при остановке
	if m.pusher != nil {
		_ = m.pusher.Delete() // best-effort удаление зомби-инстанса
	}
	m.pushMutex.Unlock()

	// Graceful shutdown HTTP server
	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.running = false
		return m.server.Shutdown(ctx)
	}
	return nil
}

// RecordBytesTransferred records bytes transferred
func (m *Metrics) RecordBytesTransferred(tenantID, direction string, bytes int64) {
	if !m.enabled {
		return
	}

	m.bytesTransferred.WithLabelValues(tenantID, direction).Add(float64(bytes))
}

// RecordConnectionHandled records a handled connection
func (m *Metrics) RecordConnectionHandled(tenantID string) {
	if !m.enabled {
		return
	}

	m.connectionsHandled.WithLabelValues(tenantID).Inc()
}

// SetActiveConnections sets active connections count
func (m *Metrics) SetActiveConnections(tenantID string, count int) {
	if !m.enabled {
		return
	}

	m.activeConnections.WithLabelValues(tenantID).Set(float64(count))
}

// RecordConnectionDuration records connection duration
func (m *Metrics) RecordConnectionDuration(tenantID string, duration time.Duration) {
	if !m.enabled {
		return
	}

	m.connectionDuration.WithLabelValues(tenantID).Observe(duration.Seconds())
}

// SetBufferPoolSize sets buffer pool size
func (m *Metrics) SetBufferPoolSize(size int) {
	if !m.enabled {
		return
	}

	m.bufferPoolSize.WithLabelValues().Set(float64(size))
}

// SetBufferPoolUsage sets buffer pool usage
func (m *Metrics) SetBufferPoolUsage(usage int) {
	if !m.enabled {
		return
	}

	m.bufferPoolUsage.WithLabelValues().Set(float64(usage))
}

// RecordError records an error
func (m *Metrics) RecordError(errorType, tenantID string) {
	if !m.enabled {
		return
	}

	m.errorsTotal.WithLabelValues(errorType, tenantID).Inc()
}

// RecordHeartbeatLatency records heartbeat latency
func (m *Metrics) RecordHeartbeatLatency(tenantID string, latency time.Duration) {
	if !m.enabled {
		return
	}

	m.heartbeatLatency.WithLabelValues(tenantID).Observe(latency.Seconds())
}

// RecordClientBytesSent records bytes sent by client
func (m *Metrics) RecordClientBytesSent(bytes int64) {
	if !m.enabled {
		return
	}
	m.clientBytes.WithLabelValues("sent").Add(float64(bytes))
}

// RecordClientBytesRecv records bytes received by client
func (m *Metrics) RecordClientBytesRecv(bytes int64) {
	if !m.enabled {
		return
	}
	m.clientBytes.WithLabelValues("recv").Add(float64(bytes))
}

// SetP2PSessions sets the number of active P2P sessions
func (m *Metrics) SetP2PSessions(count int) {
	if !m.enabled {
		return
	}
	m.p2pSessions.Set(float64(count))
}

// SetTransportMode sets the current transport mode
// 0=QUIC, 1=WireGuard, 2=gRPC
func (m *Metrics) SetTransportMode(mode int) {
	if !m.enabled {
		return
	}
	m.transportMode.Set(float64(mode))
}

// ForcePush forces an immediate push to Pushgateway
func (m *Metrics) ForcePush() error {
	if !m.enabled || m.pusher == nil {
		return fmt.Errorf("pushgateway not enabled or configured")
	}

	return m.pusher.Push()
}

// GetPushgatewayConfig returns the Pushgateway configuration
func (m *Metrics) GetPushgatewayConfig() *PushgatewayConfig {
	return m.pushgatewayConfig
}

// GetMetrics returns current metrics as a map
func (m *Metrics) GetMetrics() map[string]interface{} {
	if !m.enabled {
		return map[string]interface{}{"enabled": false}
	}

	result := map[string]interface{}{
		"enabled": true,
		"port":    m.port,
	}

	if m.pushgatewayConfig != nil && m.pushgatewayConfig.Enabled {
		result["pushgateway"] = map[string]interface{}{
			"enabled":       m.pushgatewayConfig.Enabled,
			"url":           m.pushgatewayConfig.URL,
			"job_name":      m.pushgatewayConfig.JobName,
			"instance":      m.pushgatewayConfig.Instance,
			"push_interval": m.pushgatewayConfig.PushInterval.String(),
		}
	}

	return result
}
