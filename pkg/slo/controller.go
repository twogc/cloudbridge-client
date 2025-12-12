package slo

import (
	"sync"
	"time"
)

// SLOController управляет SLO с гистерезисом и анти-флаппингом
type SLOController struct {
	// EMA фильтры для сглаживания метрик
	rttEMA    float64
	lossEMA   float64
	jitterEMA float64

	// Анти-флаппинг настройки
	switchPenalty time.Duration
	lastSwitch    time.Time
	minDwell      time.Duration

	// Гистерезис настройки
	hysteresisThreshold float64
	alpha               float64 // EMA alpha для новых данных

	mu     sync.RWMutex
	logger Logger
}

// Logger интерфейс для логирования
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// SLOConfig конфигурация SLO контроллера
type SLOConfig struct {
	// EMA настройки
	Alpha float64 `json:"alpha"` // EMA alpha (0.0-1.0)

	// Анти-флаппинг настройки
	SwitchPenalty time.Duration `json:"switch_penalty"` // Penalty после переключения
	MinDwell      time.Duration `json:"min_dwell"`      // Минимальное время на пути

	// Гистерезис настройки
	HysteresisThreshold float64 `json:"hysteresis_threshold"` // Порог гистерезиса (%)

	// SLO targets
	TargetRTT    time.Duration `json:"target_rtt"`    // Целевой RTT
	TargetLoss   float64       `json:"target_loss"`   // Целевой loss rate
	TargetJitter time.Duration `json:"target_jitter"` // Целевой jitter
}

// DefaultSLOConfig возвращает конфигурацию по умолчанию
func DefaultSLOConfig() *SLOConfig {
	return &SLOConfig{
		Alpha:               0.2,                    // 20% новых данных, 80% старых
		SwitchPenalty:       15 * time.Second,       // 15 секунд penalty
		MinDwell:            20 * time.Second,       // 20 секунд минимальное время
		HysteresisThreshold: 10.0,                   // 10% гистерезис
		TargetRTT:           120 * time.Millisecond, // 120ms целевой RTT
		TargetLoss:          0.01,                   // 1% целевой loss
		TargetJitter:        25 * time.Millisecond,  // 25ms целевой jitter
	}
}

// NewSLOController создает новый SLO контроллер
func NewSLOController(config *SLOConfig, logger Logger) *SLOController {
	if config == nil {
		config = DefaultSLOConfig()
	}

	return &SLOController{
		switchPenalty:       config.SwitchPenalty,
		minDwell:            config.MinDwell,
		hysteresisThreshold: config.HysteresisThreshold,
		alpha:               config.Alpha,
		logger:              logger,
	}
}

// Update обновляет SLO метрики с EMA фильтрацией
func (sc *SLOController) Update(rtt, loss, jitter float64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Обновляем EMA фильтры
	sc.rttEMA = sc.alpha*rtt + (1-sc.alpha)*sc.rttEMA
	sc.lossEMA = sc.alpha*loss + (1-sc.alpha)*sc.lossEMA
	sc.jitterEMA = sc.alpha*jitter + (1-sc.alpha)*sc.jitterEMA

	sc.logger.Debug("SLO metrics updated",
		"rtt_raw", rtt,
		"rtt_ema", sc.rttEMA,
		"loss_raw", loss,
		"loss_ema", sc.lossEMA,
		"jitter_raw", jitter,
		"jitter_ema", sc.jitterEMA)
}

// ShouldSwitch определяет, нужно ли переключение с учетом гистерезиса
func (sc *SLOController) ShouldSwitch(currentPath, candidatePath *PathMetrics) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	// Проверяем минимальное время на текущем пути
	if time.Since(sc.lastSwitch) < sc.minDwell {
		sc.logger.Debug("Switch blocked by minimum dwell time")
		return false
	}

	// Вычисляем улучшение с учетом гистерезиса
	improvement := sc.calculateImprovement(currentPath, candidatePath)

	// Применяем гистерезис
	threshold := sc.hysteresisThreshold / 100.0
	if improvement < threshold {
		sc.logger.Debug("Switch blocked by hysteresis",
			"improvement", improvement,
			"threshold", threshold)
		return false
	}

	// Проверяем penalty
	penalty := sc.getCurrentPenalty()
	if penalty > 0 {
		sc.logger.Debug("Switch blocked by penalty",
			"penalty", penalty)
		return false
	}

	sc.logger.Info("Switch recommended",
		"improvement", improvement,
		"threshold", threshold,
		"current_rtt", currentPath.RTT,
		"candidate_rtt", candidatePath.RTT)

	return true
}

// RecordSwitch записывает переключение
func (sc *SLOController) RecordSwitch() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.lastSwitch = time.Now()
	sc.logger.Info("Recorded SLO switch",
		"switch_time", sc.lastSwitch)
}

// calculateImprovement вычисляет улучшение с учетом гистерезиса
func (sc *SLOController) calculateImprovement(current, candidate *PathMetrics) float64 {
	// Вычисляем стоимость текущего пути
	currentCost := sc.calculateCost(current)

	// Вычисляем стоимость кандидата с penalty
	candidateCost := sc.calculateCost(candidate) + sc.getCurrentPenalty()

	// Улучшение в процентах
	if currentCost == 0 {
		return 0
	}

	improvement := (currentCost - candidateCost) / currentCost * 100
	return improvement
}

// calculateCost вычисляет стоимость пути
func (sc *SLOController) calculateCost(path *PathMetrics) float64 {
	// Простая метрика стоимости: RTT + loss*1000 + jitter
	return float64(path.RTT.Milliseconds()) + path.Loss*1000 + float64(path.Jitter.Milliseconds())
}

// getCurrentPenalty возвращает текущий penalty
func (sc *SLOController) getCurrentPenalty() float64 {
	elapsed := time.Since(sc.lastSwitch)
	if elapsed < sc.switchPenalty {
		// Линейное уменьшение penalty
		penaltyRatio := 1.0 - float64(elapsed)/float64(sc.switchPenalty)
		return penaltyRatio * 50.0 // Максимальный penalty 50ms
	}
	return 0.0
}

// GetCurrentMetrics возвращает текущие EMA метрики
func (sc *SLOController) GetCurrentMetrics() *PathMetrics {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	return &PathMetrics{
		RTT:    time.Duration(sc.rttEMA) * time.Millisecond,
		Loss:   sc.lossEMA,
		Jitter: time.Duration(sc.jitterEMA) * time.Millisecond,
	}
}

// GetSLIStatus возвращает SLI статус
func (sc *SLOController) GetSLIStatus() *SLIStatus {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	metrics := &PathMetrics{
		RTT:    time.Duration(sc.rttEMA) * time.Millisecond,
		Loss:   sc.lossEMA,
		Jitter: time.Duration(sc.jitterEMA) * time.Millisecond,
	}

	// Вычисляем SLI score (0.0-1.0)
	score := sc.calculateSLIScore(metrics)

	return &SLIStatus{
		Metrics: metrics,
		Score:   score,
		Healthy: score > 0.8, // 80% threshold
	}
}

// calculateSLIScore вычисляет SLI score
func (sc *SLOController) calculateSLIScore(metrics *PathMetrics) float64 {
	score := 1.0

	// Проверяем RTT
	if metrics.RTT > 120*time.Millisecond {
		score -= 0.3
	}

	// Проверяем loss
	if metrics.Loss > 0.01 {
		score -= 0.3
	}

	// Проверяем jitter
	if metrics.Jitter > 25*time.Millisecond {
		score -= 0.2
	}

	if score < 0 {
		score = 0
	}

	return score
}

// GetMetrics возвращает метрики SLO контроллера
func (sc *SLOController) GetMetrics() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	return map[string]interface{}{
		"rtt_ema":              sc.rttEMA,
		"loss_ema":             sc.lossEMA,
		"jitter_ema":           sc.jitterEMA,
		"last_switch":          sc.lastSwitch,
		"switch_penalty":       sc.switchPenalty.String(),
		"min_dwell":            sc.minDwell.String(),
		"hysteresis_threshold": sc.hysteresisThreshold,
		"alpha":                sc.alpha,
		"current_penalty":      sc.getCurrentPenalty(),
	}
}

// PathMetrics представляет метрики пути
type PathMetrics struct {
	RTT    time.Duration `json:"rtt"`    // RTT в миллисекундах
	Loss   float64       `json:"loss"`   // Loss rate (0.0-1.0)
	Jitter time.Duration `json:"jitter"` // Jitter в миллисекундах
}

// SLIStatus представляет SLI статус
type SLIStatus struct {
	Metrics *PathMetrics `json:"metrics"`
	Score   float64      `json:"score"`   // SLI score (0.0-1.0)
	Healthy bool         `json:"healthy"` // Здоров ли путь
}
