package heartbeat

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/interfaces"
)

// Logger — опциональный интерфейс логгера (совместим с вашим стилем в других пакетах)
type Logger interface {
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	Debug(msg string, fields ...any)
}

// Manager handles heartbeat operations
type Manager struct {
	client   interfaces.ClientInterface
	interval time.Duration

	// runtime
	ticker   *time.Ticker
	stopChan chan struct{}
	running  bool

	// state
	mu        sync.RWMutex
	lastBeat  time.Time
	failCount int
	maxFails  int

	// backoff
	baseInterval time.Duration
	maxBackoff   time.Duration
	jitterFrac   float64

	// logging
	logger Logger
}

// NewManager creates a new heartbeat manager
func NewManager(client interfaces.ClientInterface) *Manager {
	base := 30 * time.Second
	return &Manager{
		client:       client,
		interval:     base, // current interval (may grow with backoff)
		baseInterval: base, // baseline to restore on success
		maxBackoff:   2 * time.Minute,
		jitterFrac:   0.15, // ±15% джиттер
		maxFails:     3,
	}
}

// WithLogger plugs a logger (optional)
func (m *Manager) WithLogger(l Logger) *Manager {
	m.logger = l
	return m
}

// SetIntervalFromConfig устанавливает интервал из гибкого конфига
func (m *Manager) SetIntervalFromConfig(configInterval interface{}) {
	if duration, ok := parseFlexibleDuration(configInterval); ok {
		m.SetInterval(duration)
		m.logDebug("heartbeat interval set from config", "interval", duration.String())
	}
}

// parseFlexibleDuration парсит duration из разных форматов (локальная копия)
func parseFlexibleDuration(v interface{}) (time.Duration, bool) {
	if v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case time.Duration:
		return t, true
	case string:
		if t == "" {
			return 0, false
		}
		if d, err := time.ParseDuration(t); err == nil {
			return d, true
		}
	case float64:
		return time.Duration(int64(t)) * time.Millisecond, true
	case int:
		return time.Duration(t) * time.Millisecond, true
	case int64:
		return time.Duration(t) * time.Millisecond, true
	case uint64:
		return time.Duration(int64(t)) * time.Millisecond, true
	}
	return 0, false
}

// Start starts the heartbeat mechanism
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("heartbeat manager is already running")
	}

	// (re)create control chan & ticker
	m.stopChan = make(chan struct{})
	m.ticker = time.NewTicker(m.interval)
	m.failCount = 0
	m.running = true

	// стартуем, даже если клиент не подключен — будем ретраить с backoff
	go m.heartbeatLoop()

	// подписываемся на события состояния клиента для авто-рестарта
	go m.stateEventLoop()

	m.logDebug("heartbeat started", "interval", m.interval.String())
	return nil
}

// Stop stops the heartbeat mechanism
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	m.running = false

	// закрываем тикер и сигналим горутине
	if m.ticker != nil {
		m.ticker.Stop()
		m.ticker = nil
	}
	if m.stopChan != nil {
		close(m.stopChan)
		m.stopChan = nil
	}

	m.logDebug("heartbeat stopped")
}

// SetInterval sets the baseline heartbeat interval (also resets current interval)
func (m *Manager) SetInterval(interval time.Duration) {
	if interval <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.baseInterval = interval
	m.interval = interval
	if m.ticker != nil {
		// безопасно пересоздать тикер, чтобы исключить гонки Reset
		m.ticker.Stop()
		m.ticker = time.NewTicker(m.interval)
	}

	m.logDebug("heartbeat interval set", "interval", m.interval.String())
}

// GetInterval returns the current heartbeat interval
func (m *Manager) GetInterval() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.interval
}

// IsRunning returns true if the heartbeat manager is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetLastBeat returns the time of the last successful heartbeat
func (m *Manager) GetLastBeat() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastBeat
}

// GetFailCount returns the number of consecutive failures
func (m *Manager) GetFailCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failCount
}

// SendManualHeartbeat sends a manual heartbeat (for testing)
func (m *Manager) SendManualHeartbeat() error {
	m.mu.RLock()
	running := m.running
	m.mu.RUnlock()
	if !running {
		return fmt.Errorf("heartbeat manager is not running")
	}
	return m.sendHeartbeat()
}

// GetStats returns heartbeat statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"running":     m.running,
		"interval":    m.interval.String(),
		"last_beat":   m.lastBeat,
		"fail_count":  m.failCount,
		"max_fails":   m.maxFails,
		"base_intvl":  m.baseInterval.String(),
		"max_backoff": m.maxBackoff.String(),
	}
	if !m.lastBeat.IsZero() {
		stats["time_since_last_beat"] = time.Since(m.lastBeat).String()
	}
	return stats
}

// ResetFailCount resets the failure counter
func (m *Manager) ResetFailCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failCount = 0
}

// SetMaxFails sets the maximum number of consecutive failures
func (m *Manager) SetMaxFails(maxFails int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if maxFails > 0 {
		m.maxFails = maxFails
	}
}

// === internal ===

func (m *Manager) heartbeatLoop() {
	for {
		select {
		case <-m.stopChan:
			return
		case <-m.ticker.C:
			if err := m.sendHeartbeat(); err != nil {
				m.handleHeartbeatFailure(err)
			} else {
				m.handleHeartbeatSuccess()
			}
		}
	}
}

func (m *Manager) sendHeartbeat() error {
	if !m.client.IsConnected() {
		return fmt.Errorf("client is not connected")
	}
	return m.client.SendHeartbeat()
}

func (m *Manager) handleHeartbeatSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastBeat = time.Now()
	m.failCount = 0

	// вернуть интервал к базовому при успехе
	if m.interval != m.baseInterval {
		m.interval = m.baseInterval
		if m.ticker != nil {
			m.ticker.Stop()
			m.ticker = time.NewTicker(m.interval)
		}
	}

	m.logDebug("heartbeat OK", "at", m.lastBeat.Format(time.RFC3339), "interval", m.interval.String())
}

func (m *Manager) handleHeartbeatFailure(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failCount++
	m.logWarn("heartbeat failed", "attempt", m.failCount, "max", m.maxFails, "error", err)

	// backoff: экспонента по failCount с верхней границей + джиттер
	next := m.backoffIntervalLocked()
	if next != m.interval {
		m.interval = next
		if m.ticker != nil {
			m.ticker.Stop()
			m.ticker = time.NewTicker(m.interval)
		}
		m.logDebug("heartbeat backoff applied", "next_interval", m.interval.String())
	}

	if m.failCount >= m.maxFails {
		m.logWarn("too many heartbeat failures, stopping")
		m.running = false
		if m.ticker != nil {
			m.ticker.Stop()
			m.ticker = nil
		}
		if m.stopChan != nil {
			close(m.stopChan)
			m.stopChan = nil
		}
	}
}

func (m *Manager) backoffIntervalLocked() time.Duration {
	// экспоненциальный: base * 2^(failCount-1), clamp to maxBackoff; плюс джиттер ±jitterFrac
	exp := m.baseInterval
	if m.failCount > 0 {
		for i := 1; i < m.failCount; i++ {
			exp *= 2
			if exp >= m.maxBackoff {
				exp = m.maxBackoff
				break
			}
		}
	}
	// джиттер
	j := 1.0 + (rand.Float64()*2-1)*m.jitterFrac
	next := time.Duration(float64(exp) * j)
	if next < time.Second {
		next = time.Second
	}
	if next > m.maxBackoff {
		next = m.maxBackoff
	}
	return next
}

// stateEventLoop listens for client state changes and handles auto-restart
func (m *Manager) stateEventLoop() {
	stateChan := m.client.SubscribeState()

	for {
		select {
		case <-m.stopChan:
			return
		case event, ok := <-stateChan:
			if !ok {
				m.logDebug("client state channel closed")
				return
			}

			m.handleStateEvent(event)
		}
	}
}

// handleStateEvent processes client state changes
func (m *Manager) handleStateEvent(event interfaces.ClientStateEvent) {
	m.logDebug("client state changed",
		"state", event.State,
		"reason", event.Reason,
		"updated", event.Updated.Format(time.RFC3339))

	switch event.State {
	case "connected":
		// клиент подключился - сбрасываем счетчик сбоев и возвращаем базовый интервал
		m.mu.Lock()
		m.failCount = 0
		if m.interval != m.baseInterval {
			m.interval = m.baseInterval
			if m.ticker != nil {
				m.ticker.Stop()
				m.ticker = time.NewTicker(m.interval)
			}
		}
		m.mu.Unlock()
		m.logDebug("heartbeat reset to base interval after reconnection")

	case "disconnected", "reconnecting":
		// клиент отключился - увеличиваем backoff
		m.mu.Lock()
		m.failCount++
		next := m.backoffIntervalLocked()
		if next != m.interval {
			m.interval = next
			if m.ticker != nil {
				m.ticker.Stop()
				m.ticker = time.NewTicker(m.interval)
			}
		}
		m.mu.Unlock()
		m.logDebug("heartbeat backoff applied due to disconnection", "next_interval", m.interval.String())
	}
}

func (m *Manager) logDebug(msg string, fields ...any) {
	if m.logger != nil {
		m.logger.Debug(msg, fields...)
	} else {
		// no-op to keep lib quiet by default
	}
}
func (m *Manager) logWarn(msg string, fields ...any) {
	if m.logger != nil {
		m.logger.Warn(msg, fields...)
	}
}
