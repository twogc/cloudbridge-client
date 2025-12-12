package probes

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SyntheticProbeManager управляет synthetic probes
type SyntheticProbeManager struct {
	probes   map[string]*ProbeConfig
	results  map[string]*ProbeResult
	interval time.Duration
	timeout  time.Duration
	mu       sync.RWMutex
	logger   Logger
	stopCh   chan struct{}
	running  bool
}

// Logger интерфейс для логирования
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// ProbeConfig конфигурация probe
type ProbeConfig struct {
	Name     string        `json:"name"`     // Имя probe
	URL      string        `json:"url"`      // URL для проверки
	Method   string        `json:"method"`   // HTTP метод
	Timeout  time.Duration `json:"timeout"`  // Таймаут probe
	Interval time.Duration `json:"interval"` // Интервал между probe
	Enabled  bool          `json:"enabled"`  // Включен ли probe
	Mode     string        `json:"mode"`     // Режим транспорта (h3, h2, ws, tcp)
	POP      string        `json:"pop"`      // ID POP/сервера
}

// ProbeResult результат probe
type ProbeResult struct {
	Success      bool          `json:"success"`       // Успешен ли probe
	Latency      time.Duration `json:"latency"`       // Латентность
	ErrorRate    float64       `json:"error_rate"`    // Rate ошибок
	LastCheck    time.Time     `json:"last_check"`    // Время последней проверки
	CheckCount   int64         `json:"check_count"`   // Количество проверок
	SuccessCount int64         `json:"success_count"` // Количество успешных проверок
	Mode         string        `json:"mode"`          // Режим транспорта
	POP          string        `json:"pop"`           // ID POP/сервера
}

// SyntheticProbeConfig конфигурация synthetic probe manager
type SyntheticProbeConfig struct {
	Interval time.Duration `json:"interval"` // Интервал между probe
	Timeout  time.Duration `json:"timeout"`  // Таймаут probe
}

// DefaultSyntheticProbeConfig возвращает конфигурацию по умолчанию
func DefaultSyntheticProbeConfig() *SyntheticProbeConfig {
	return &SyntheticProbeConfig{
		Interval: 5 * time.Second,
		Timeout:  1 * time.Second,
	}
}

// NewSyntheticProbeManager создает новый synthetic probe manager
func NewSyntheticProbeManager(config *SyntheticProbeConfig, logger Logger) *SyntheticProbeManager {
	if config == nil {
		config = DefaultSyntheticProbeConfig()
	}

	return &SyntheticProbeManager{
		probes:   make(map[string]*ProbeConfig),
		results:  make(map[string]*ProbeResult),
		interval: config.Interval,
		timeout:  config.Timeout,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// AddProbe добавляет новый probe
func (spm *SyntheticProbeManager) AddProbe(config *ProbeConfig) {
	spm.mu.Lock()
	defer spm.mu.Unlock()

	spm.probes[config.Name] = config
	spm.results[config.Name] = &ProbeResult{
		Mode: config.Mode,
		POP:  config.POP,
	}

	spm.logger.Info("Added synthetic probe",
		"name", config.Name,
		"url", config.URL,
		"mode", config.Mode,
		"pop", config.POP)
}

// Start запускает synthetic probes
func (spm *SyntheticProbeManager) Start() error {
	spm.mu.Lock()
	defer spm.mu.Unlock()

	if spm.running {
		return fmt.Errorf("synthetic probes already running")
	}

	spm.running = true
	spm.stopCh = make(chan struct{})

	go spm.runProbes()

	spm.logger.Info("Started synthetic probes",
		"interval", spm.interval,
		"timeout", spm.timeout)

	return nil
}

// Stop останавливает synthetic probes
func (spm *SyntheticProbeManager) Stop() {
	spm.mu.Lock()
	defer spm.mu.Unlock()

	if !spm.running {
		return
	}

	close(spm.stopCh)
	spm.running = false

	spm.logger.Info("Stopped synthetic probes")
}

// runProbes запускает цикл probe
func (spm *SyntheticProbeManager) runProbes() {
	ticker := time.NewTicker(spm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			spm.runAllProbes()
		case <-spm.stopCh:
			return
		}
	}
}

// runAllProbes запускает все probe
func (spm *SyntheticProbeManager) runAllProbes() {
	spm.mu.RLock()
	probes := make([]*ProbeConfig, 0, len(spm.probes))
	for _, probe := range spm.probes {
		if probe.Enabled {
			probes = append(probes, probe)
		}
	}
	spm.mu.RUnlock()

	// Запускаем probe параллельно
	var wg sync.WaitGroup
	for _, probe := range probes {
		wg.Add(1)
		go func(p *ProbeConfig) {
			defer wg.Done()
			spm.runProbe(p)
		}(probe)
	}

	wg.Wait()
}

// runProbe запускает один probe
func (spm *SyntheticProbeManager) runProbe(probe *ProbeConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), probe.Timeout)
	defer cancel()

	start := time.Now()
	success := spm.executeProbe(ctx, probe)
	latency := time.Since(start)

	spm.updateResult(probe.Name, success, latency)
}

// executeProbe выполняет probe
func (spm *SyntheticProbeManager) executeProbe(ctx context.Context, probe *ProbeConfig) bool {
	req, err := http.NewRequestWithContext(ctx, probe.Method, probe.URL, nil)
	if err != nil {
		spm.logger.Error("Failed to create probe request",
			"probe", probe.Name,
			"error", err)
		return false
	}

	client := &http.Client{Timeout: probe.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		spm.logger.Debug("Probe failed",
			"probe", probe.Name,
			"error", err)
		return false
	}
	defer resp.Body.Close()

	success := resp.StatusCode == http.StatusOK
	spm.logger.Debug("Probe completed",
		"probe", probe.Name,
		"status", resp.StatusCode,
		"success", success)

	return success
}

// updateResult обновляет результат probe
func (spm *SyntheticProbeManager) updateResult(name string, success bool, latency time.Duration) {
	spm.mu.Lock()
	defer spm.mu.Unlock()

	result, exists := spm.results[name]
	if !exists {
		spm.logger.Warn("Probe result not found", "name", name)
		return
	}

	result.LastCheck = time.Now()
	result.CheckCount++

	if success {
		result.SuccessCount++
		result.Latency = latency
	}

	// Вычисляем error rate
	if result.CheckCount > 0 {
		result.ErrorRate = 1.0 - float64(result.SuccessCount)/float64(result.CheckCount)
	}

	result.Success = success

	spm.logger.Debug("Updated probe result",
		"probe", name,
		"success", success,
		"latency", latency,
		"error_rate", result.ErrorRate)
}

// GetResults возвращает результаты всех probe
func (spm *SyntheticProbeManager) GetResults() map[string]*ProbeResult {
	spm.mu.RLock()
	defer spm.mu.RUnlock()

	results := make(map[string]*ProbeResult)
	for name, result := range spm.results {
		results[name] = result
	}

	return results
}

// GetResult возвращает результат конкретного probe
func (spm *SyntheticProbeManager) GetResult(name string) (*ProbeResult, bool) {
	spm.mu.RLock()
	defer spm.mu.RUnlock()

	result, exists := spm.results[name]
	return result, exists
}

// GetMetrics возвращает метрики synthetic probes
func (spm *SyntheticProbeManager) GetMetrics() map[string]interface{} {
	spm.mu.RLock()
	defer spm.mu.RUnlock()

	metrics := map[string]interface{}{
		"total_probes":   len(spm.probes),
		"enabled_probes": 0,
		"interval":       spm.interval.String(),
		"timeout":        spm.timeout.String(),
		"running":        spm.running,
	}

	enabledCount := 0
	for _, probe := range spm.probes {
		if probe.Enabled {
			enabledCount++
		}
	}
	metrics["enabled_probes"] = enabledCount

	return metrics
}

// GetProbeMetrics возвращает метрики для Prometheus
func (spm *SyntheticProbeManager) GetProbeMetrics() map[string]interface{} {
	spm.mu.RLock()
	defer spm.mu.RUnlock()

	metrics := make(map[string]interface{})

	for name, result := range spm.results {
		prefix := fmt.Sprintf("probe_%s", name)
		metrics[prefix+"_success"] = result.Success
		metrics[prefix+"_latency_ms"] = result.Latency.Milliseconds()
		metrics[prefix+"_error_rate"] = result.ErrorRate
		metrics[prefix+"_check_count"] = result.CheckCount
		metrics[prefix+"_success_count"] = result.SuccessCount
		metrics[prefix+"_mode"] = result.Mode
		metrics[prefix+"_pop"] = result.POP
	}

	return metrics
}

// RemoveProbe удаляет probe
func (spm *SyntheticProbeManager) RemoveProbe(name string) {
	spm.mu.Lock()
	defer spm.mu.Unlock()

	delete(spm.probes, name)
	delete(spm.results, name)

	spm.logger.Info("Removed synthetic probe", "name", name)
}

// EnableProbe включает probe
func (spm *SyntheticProbeManager) EnableProbe(name string) {
	spm.mu.Lock()
	defer spm.mu.Unlock()

	if probe, exists := spm.probes[name]; exists {
		probe.Enabled = true
		spm.logger.Info("Enabled synthetic probe", "name", name)
	}
}

// DisableProbe отключает probe
func (spm *SyntheticProbeManager) DisableProbe(name string) {
	spm.mu.Lock()
	defer spm.mu.Unlock()

	if probe, exists := spm.probes[name]; exists {
		probe.Enabled = false
		spm.logger.Info("Disabled synthetic probe", "name", name)
	}
}
