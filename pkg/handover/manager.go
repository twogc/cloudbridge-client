package handover

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ContinuationToken представляет токен для seamless handover
type ContinuationToken struct {
	ConnID    string    `json:"conn_id"`    // Стабильный CID/логический ID
	Epoch     uint64    `json:"epoch"`      // Версия ключей/параметров
	ExpiresAt time.Time `json:"expires_at"` // Время истечения
	MAC       []byte    `json:"mac"`        // Подпись сервера (HMAC)
	ServerID  string    `json:"server_id"`  // ID сервера
	TenantID  string    `json:"tenant_id"`  // ID тенанта
}

// PathSLI представляет SLI метрики для пути
type PathSLI struct {
	RTT    time.Duration `json:"rtt"`    // RTT в миллисекундах
	Loss   float64       `json:"loss"`   // Loss rate (0.0-1.0)
	Jitter time.Duration `json:"jitter"` // Jitter в миллисекундах
	Mode   string        `json:"mode"`   // Режим транспорта
	Server string        `json:"server"` // ID сервера
}

// Cost вычисляет стоимость пути
func (ps *PathSLI) Cost(alpha, beta, gamma float64) float64 {
	// cost = α*RTT_p95 + β*loss_p95 + γ*jitter_p95
	return alpha*float64(ps.RTT.Milliseconds()) + beta*ps.Loss + gamma*float64(ps.Jitter.Milliseconds())
}

// tokenPayload — детерминированный набор полей для подписи
type tokenPayload struct {
	ConnID    string `json:"conn_id"`
	Epoch     uint64 `json:"epoch"`
	ExpiresAt string `json:"expires_at"` // RFC3339Nano
	ServerID  string `json:"server_id"`
	TenantID  string `json:"tenant_id"`
}

// HandoverManager управляет seamless handover
type HandoverManager struct {
	token       *ContinuationToken
	penalty     time.Duration // Penalty/cooldown
	lastSwitch  time.Time     // Время последнего переключения
	secretKey   []byte        // Секретный ключ для подписи
	tokenTTL    time.Duration // TTL для токенов из конфига
	minInterval time.Duration // Минимальный интервал между переключениями
	mu          sync.RWMutex
	logger      Logger
}

// Logger интерфейс для логирования
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// makePayload создает детерминированный payload для подписи
func (hm *HandoverManager) makePayload(t *ContinuationToken) tokenPayload {
	return tokenPayload{
		ConnID:    t.ConnID,
		Epoch:     t.Epoch,
		ExpiresAt: t.ExpiresAt.UTC().Format(time.RFC3339Nano),
		ServerID:  t.ServerID,
		TenantID:  t.TenantID,
	}
}

// HandoverConfig конфигурация handover
type HandoverConfig struct {
	Penalty     time.Duration `json:"penalty"`      // Penalty после переключения
	TokenTTL    time.Duration `json:"token_ttl"`    // TTL для токенов
	SecretKey   string        `json:"secret_key"`   // Секретный ключ
	MinInterval time.Duration `json:"min_interval"` // Минимальный интервал между переключениями
}

// DefaultHandoverConfig возвращает конфигурацию по умолчанию
func DefaultHandoverConfig() *HandoverConfig {
	return &HandoverConfig{
		Penalty:     15 * time.Second,
		TokenTTL:    5 * time.Minute,
		SecretKey:   "default-secret-key", // В production должен быть из конфига
		MinInterval: 10 * time.Second,
	}
}

// NewHandoverManager создает новый handover менеджер
func NewHandoverManager(config *HandoverConfig, logger Logger) *HandoverManager {
	if config == nil {
		config = DefaultHandoverConfig()
	}

	return &HandoverManager{
		penalty:     config.Penalty,
		secretKey:   []byte(config.SecretKey),
		tokenTTL:    config.TokenTTL,
		minInterval: config.MinInterval,
		logger:      logger,
	}
}

// CreateToken создает новый continuation token
func (hm *HandoverManager) CreateToken(connID, serverID, tenantID string) (*ContinuationToken, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Генерируем случайный epoch с binary.BigEndian
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, fmt.Errorf("failed to generate epoch: %w", err)
	}
	epoch := binary.BigEndian.Uint64(b[:])

	// Используем TTL из конфига
	exp := time.Now().UTC().Add(hm.tokenTTL)
	exp = exp.Round(0) // Нормализуем время

	token := &ContinuationToken{
		ConnID:    connID,
		Epoch:     epoch,
		ExpiresAt: exp,
		ServerID:  serverID,
		TenantID:  tenantID,
	}

	// Подписываем токен
	if err := hm.signToken(token); err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	hm.token = token
	hm.logger.Info("Created continuation token",
		"conn_id", connID,
		"server_id", serverID,
		"tenant_id", tenantID,
		"expires_at", token.ExpiresAt)

	return token, nil
}

// ValidateToken валидирует continuation token
func (hm *HandoverManager) ValidateToken(token *ContinuationToken) error {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	// Проверяем срок действия
	if time.Now().After(token.ExpiresAt) {
		return fmt.Errorf("token expired")
	}

	// Проверяем подпись
	if !hm.verifyToken(token) {
		return fmt.Errorf("invalid token signature")
	}

	return nil
}

// ShouldSwitch определяет, нужно ли переключение
func (hm *HandoverManager) ShouldSwitch(oldPath, newPath *PathSLI) bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	// Минимальный интервал
	if hm.minInterval > 0 && time.Since(hm.lastSwitch) < hm.minInterval {
		hm.logger.Debug("Handover blocked by minimum interval",
			"elapsed", time.Since(hm.lastSwitch).String(),
			"min_interval", hm.minInterval.String())
		return false
	}

	// Стоимость путей (единицы: ms для rtt/jitter + beta для потерь)
	alpha, beta, gamma := 1.0, 100.0, 1.0
	costOld := oldPath.Cost(alpha, beta, gamma)
	costNew := newPath.Cost(alpha, beta, gamma) + hm.currentPenalty() // <— исправлено: используем newPath

	// Требуем относительное улучшение
	relDelta := 0.10 // 10%
	shouldSwitch := costNew < costOld*(1.0-relDelta)

	hm.logger.Debug("Handover decision",
		"old_cost", costOld,
		"new_cost", costNew,
		"penalty_ms", hm.currentPenalty(),
		"rel_delta", relDelta,
		"should_switch", shouldSwitch)

	return shouldSwitch
}

// RecordSwitch записывает переключение
func (hm *HandoverManager) RecordSwitch(newServerID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.lastSwitch = time.Now()
	hm.logger.Info("Recorded handover switch",
		"new_server", newServerID,
		"switch_time", hm.lastSwitch)
}

// currentPenalty возвращает текущий пенальти в миллисекундах (float64)
func (hm *HandoverManager) currentPenalty() float64 {
	elapsed := time.Since(hm.lastSwitch)
	if elapsed < hm.penalty {
		// Линейное уменьшение penalty
		penaltyRatio := 1.0 - float64(elapsed)/float64(hm.penalty)
		return penaltyRatio * 50.0 // Максимальный penalty 50ms
	}
	return 0.0
}

// signToken подписывает токен
func (hm *HandoverManager) signToken(token *ContinuationToken) error {
	pl := hm.makePayload(token)
	data, err := json.Marshal(pl)
	if err != nil {
		return fmt.Errorf("failed to marshal token payload: %w", err)
	}
	h := hmac.New(sha256.New, hm.secretKey)
	if _, err := h.Write(data); err != nil {
		return fmt.Errorf("failed to write hmac: %w", err)
	}
	token.MAC = h.Sum(nil)
	return nil
}

// verifyToken проверяет подпись токена
func (hm *HandoverManager) verifyToken(token *ContinuationToken) bool {
	pl := hm.makePayload(token)
	data, err := json.Marshal(pl)
	if err != nil {
		return false
	}
	h := hmac.New(sha256.New, hm.secretKey)
	if _, err := h.Write(data); err != nil {
		return false
	}
	expected := h.Sum(nil)
	return hmac.Equal(token.MAC, expected)
}

// GetToken возвращает текущий токен
func (hm *HandoverManager) GetToken() *ContinuationToken {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.token
}

// SetToken устанавливает токен
func (hm *HandoverManager) SetToken(token *ContinuationToken) error {
	if err := hm.ValidateToken(token); err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	hm.mu.Lock()
	hm.token = token
	hm.mu.Unlock()

	hm.logger.Info("Set continuation token",
		"conn_id", token.ConnID,
		"server_id", token.ServerID,
		"tenant_id", token.TenantID)

	return nil
}

// GetMetrics возвращает метрики handover
func (hm *HandoverManager) GetMetrics() map[string]interface{} {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	metrics := map[string]interface{}{
		"last_switch":     hm.lastSwitch,
		"penalty":         hm.penalty.String(),
		"current_penalty": hm.currentPenalty(),
		"has_token":       hm.token != nil,
	}

	if hm.token != nil {
		metrics["token_expires_at"] = hm.token.ExpiresAt
		metrics["token_conn_id"] = hm.token.ConnID
		metrics["token_server_id"] = hm.token.ServerID
	}

	return metrics
}

// SerializeToken сериализует токен в строку
func (hm *HandoverManager) SerializeToken(token *ContinuationToken) (string, error) {
	data, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DeserializeToken десериализует токен из строки
func (hm *HandoverManager) DeserializeToken(tokenStr string) (*ContinuationToken, error) {
	data, err := base64.RawURLEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	var token ContinuationToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// SetSecretKey безопасно обновляет секрет (без смены epoch)
func (hm *HandoverManager) SetSecretKey(newKey []byte) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.secretKey = append([]byte(nil), newKey...)
}

// RotateSecretKey меняет секрет и (опционально) бампает epoch
func (hm *HandoverManager) RotateSecretKey(newKey []byte, bumpEpoch bool) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.secretKey = append([]byte(nil), newKey...)
	if hm.token != nil && bumpEpoch {
		hm.token.Epoch++
		_ = hm.signToken(hm.token) // пересчитываем MAC
	}
}
