package transport

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Mode представляет режим транспорта
type Mode string

const (
	ModeH3        Mode = "h3"          // HTTP/3 + MASQUE
	ModeH2Connect Mode = "h2_connect"  // HTTP/2 CONNECT
	ModeWS        Mode = "ws"          // WebSocket
	ModeTCP       Mode = "tcp_connect" // TCP CONNECT
)

// HealthChecker интерфейс для проверки здоровья транспорта
type HealthChecker interface {
	OK(ctx context.Context) bool
	Latency(ctx context.Context) time.Duration
	ErrorRate(ctx context.Context) float64
}

// AutoSwitchManager управляет автоматическим переключением транспортов
type AutoSwitchManager struct {
	order          []Mode                 // Порядок fallback
	healthCheckers map[Mode]HealthChecker // Health checkers для каждого режима
	currentMode    Mode                   // Текущий режим
	lastSwitch     time.Time              // Время последнего переключения
	cooldown       time.Duration          // Cooldown после переключения
	minDwell       time.Duration          // Минимальное время на режиме
	mu             sync.RWMutex
	logger         Logger
}

// Logger интерфейс для логирования
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// AutoSwitchConfig конфигурация auto-switch
type AutoSwitchConfig struct {
	Order    []Mode        `json:"order"`     // Порядок fallback
	Cooldown time.Duration `json:"cooldown"`  // Cooldown после переключения
	MinDwell time.Duration `json:"min_dwell"` // Минимальное время на режиме
}

// DefaultAutoSwitchConfig возвращает конфигурацию по умолчанию
func DefaultAutoSwitchConfig() *AutoSwitchConfig {
	return &AutoSwitchConfig{
		Order:    []Mode{ModeH3, ModeH2Connect, ModeWS, ModeTCP},
		Cooldown: 15 * time.Second,
		MinDwell: 20 * time.Second,
	}
}

// NewAutoSwitchManager создает новый auto-switch менеджер
func NewAutoSwitchManager(config *AutoSwitchConfig, logger Logger) *AutoSwitchManager {
	if config == nil {
		config = DefaultAutoSwitchConfig()
	}

	return &AutoSwitchManager{
		order:          config.Order,
		healthCheckers: make(map[Mode]HealthChecker),
		currentMode:    config.Order[0], // Начинаем с первого в списке
		cooldown:       config.Cooldown,
		minDwell:       config.MinDwell,
		lastSwitch:     time.Now(), // CRITICAL: инициализируем время
		logger:         logger,
	}
}

// RegisterHealthChecker регистрирует health checker для режима
func (asm *AutoSwitchManager) RegisterHealthChecker(mode Mode, checker HealthChecker) {
	asm.mu.Lock()
	defer asm.mu.Unlock()
	asm.healthCheckers[mode] = checker
}

// GetCurrentMode возвращает текущий режим
func (asm *AutoSwitchManager) GetCurrentMode() Mode {
	asm.mu.RLock()
	defer asm.mu.RUnlock()
	return asm.currentMode
}

// NextHealthy находит следующий здоровый режим
func (asm *AutoSwitchManager) NextHealthy(ctx context.Context) Mode {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	// Проверяем, можно ли переключаться
	if !asm.canSwitch() && asm.isHealthyNoLock(ctx, asm.currentMode) {
		return asm.currentMode
	}

	// Ищем лучший режим
	for _, mode := range asm.order {
		if mode == asm.currentMode {
			continue // Пропускаем текущий
		}
		if asm.isHealthyNoLock(ctx, mode) && asm.betterThanCurrent(ctx, mode) {
			asm.logger.Info("Switching transport mode",
				"from", asm.currentMode,
				"to", mode)
			asm.currentMode = mode
			asm.lastSwitch = time.Now()
			return mode
		}
	}

	// Если ничего не найдено, возвращаем текущий режим
	asm.logger.Warn("No better healthy transport mode found")
	return asm.currentMode
}

// canSwitch проверяет, можно ли переключаться
func (asm *AutoSwitchManager) canSwitch() bool {
	if time.Since(asm.lastSwitch) < asm.cooldown {
		return false
	}
	if time.Since(asm.lastSwitch) < asm.minDwell {
		return false
	}
	return true
}

// isHealthyNoLock проверяет здоровье режима без лока
func (asm *AutoSwitchManager) isHealthyNoLock(ctx context.Context, mode Mode) bool {
	checker, exists := asm.healthCheckers[mode]
	if !exists {
		asm.logger.Warn("No health checker for mode", "mode", mode)
		return false
	}
	return checker.OK(ctx)
}

// betterThanCurrent проверяет, лучше ли кандидат чем текущий режим
func (asm *AutoSwitchManager) betterThanCurrent(ctx context.Context, candidate Mode) bool {
	hcCand, okCand := asm.healthCheckers[candidate]
	hcCurr, okCurr := asm.healthCheckers[asm.currentMode]
	if !okCand || !okCurr {
		return okCand && !okCurr
	}

	// Простая гистерезис-логика: требуем 10% выигрыша по latency
	latCand := hcCand.Latency(ctx)
	latCurr := hcCurr.Latency(ctx)
	return latCand < time.Duration(float64(latCurr)*0.9)
}

// isHealthy проверяет здоровье режима (deprecated, используйте isHealthyNoLock)
func (asm *AutoSwitchManager) isHealthy(ctx context.Context, mode Mode) bool {
	return asm.isHealthyNoLock(ctx, mode)
}

// ForceSwitch принудительно переключает на режим
func (asm *AutoSwitchManager) ForceSwitch(mode Mode) {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	asm.logger.Info("Force switching transport mode",
		"from", asm.currentMode,
		"to", mode)

	asm.currentMode = mode
	asm.lastSwitch = time.Now()
}

// GetHealthStatus возвращает статус здоровья всех режимов
func (asm *AutoSwitchManager) GetHealthStatus(ctx context.Context) map[Mode]bool {
	asm.mu.RLock()
	defer asm.mu.RUnlock()

	status := make(map[Mode]bool)
	for mode := range asm.healthCheckers {
		status[mode] = asm.isHealthy(ctx, mode)
	}

	return status
}

// GetMetrics возвращает метрики auto-switch
func (asm *AutoSwitchManager) GetMetrics() map[string]interface{} {
	asm.mu.RLock()
	defer asm.mu.RUnlock()

	return map[string]interface{}{
		"current_mode":    asm.currentMode,
		"last_switch":     asm.lastSwitch,
		"cooldown":        asm.cooldown.String(),
		"min_dwell":       asm.minDwell.String(),
		"available_modes": len(asm.healthCheckers),
	}
}

// H3HealthChecker health checker для HTTP/3
type H3HealthChecker struct {
	client *http.Client
	url    string
}

// NewH3HealthChecker создает health checker для HTTP/3
func NewH3HealthChecker(client *http.Client, url string) *H3HealthChecker {
	return &H3HealthChecker{
		client: client,
		url:    url,
	}
}

// OK проверяет здоровье HTTP/3
func (h3hc *H3HealthChecker) OK(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", h3hc.url+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := h3hc.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Latency возвращает латентность HTTP/3
func (h3hc *H3HealthChecker) Latency(ctx context.Context) time.Duration {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", h3hc.url+"/health", nil)
	if err != nil {
		return 0
	}

	resp, err := h3hc.client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	return time.Since(start)
}

// ErrorRate возвращает rate ошибок HTTP/3
func (h3hc *H3HealthChecker) ErrorRate(ctx context.Context) float64 {
	// TODO: Реализовать подсчет error rate
	return 0.0
}

// H2HealthChecker health checker для HTTP/2 CONNECT
type H2HealthChecker struct {
	client *http.Client
	url    string
}

// NewH2HealthChecker создает health checker для HTTP/2
func NewH2HealthChecker(client *http.Client, url string) *H2HealthChecker {
	return &H2HealthChecker{
		client: client,
		url:    url,
	}
}

// OK проверяет здоровье HTTP/2
func (h2hc *H2HealthChecker) OK(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", h2hc.url+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := h2hc.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Latency возвращает латентность HTTP/2
func (h2hc *H2HealthChecker) Latency(ctx context.Context) time.Duration {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", h2hc.url+"/health", nil)
	if err != nil {
		return 0
	}

	resp, err := h2hc.client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	return time.Since(start)
}

// ErrorRate возвращает rate ошибок HTTP/2
func (h2hc *H2HealthChecker) ErrorRate(ctx context.Context) float64 {
	// TODO: Реализовать подсчет error rate
	return 0.0
}
