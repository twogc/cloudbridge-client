package api

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WireGuardClient расширяет базовый Client для WireGuard операций
type WireGuardClient struct {
	*Client
}

// NewWireGuardClient создает новый WireGuard клиент
func NewWireGuardClient(baseClient *Client) *WireGuardClient {
	return &WireGuardClient{
		Client: baseClient,
	}
}

// WireGuardTunnelRequest запрос на создание WireGuard туннеля
type WireGuardTunnelRequest struct {
	TenantID  string            `json:"tenant_id"`
	RequestID string            `json:"request_id"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// WireGuardTunnelResponse ответ на создание WireGuard туннеля
type WireGuardTunnelResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	RequestID    string `json:"request_id"`
	PublicKey    string `json:"public_key"`
	Endpoint     string `json:"endpoint"`
	Port         int    `json:"port"`
	ClientConfig string `json:"client_config"`
}

// WireGuardTunnelInfo информация о WireGuard туннеле
type WireGuardTunnelInfo struct {
	TenantID      string `json:"tenant_id"`
	PublicKey     string `json:"public_key"`
	Endpoint      string `json:"endpoint"`
	Port          int    `json:"port"`
	IsActive      bool   `json:"is_active"`
	BytesReceived int64  `json:"bytes_received"`
	BytesSent     int64  `json:"bytes_sent"`
	LastConnected int64  `json:"last_connected"`
}

// WireGuardStatusResponse ответ со статусом WireGuard
type WireGuardStatusResponse struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	RequestID string                 `json:"request_id"`
	Status    map[string]interface{} `json:"status"`
}

// WireGuardMetricsResponse ответ с метриками WireGuard
type WireGuardMetricsResponse struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	RequestID string                 `json:"request_id"`
	Metrics   map[string]interface{} `json:"metrics"`
}

// CreateWireGuardTunnel создает WireGuard туннель для tenant'а
func (wgc *WireGuardClient) CreateWireGuardTunnel(ctx context.Context, token, tenantID string) (*WireGuardTunnelResponse, error) {
	requestID := fmt.Sprintf("wg-create-%d", time.Now().Unix())

	req := &WireGuardTunnelRequest{
		TenantID:  tenantID,
		RequestID: requestID,
		Metadata: map[string]string{
			"client_version": "1.0.0",
			"platform":       "linux",
		},
	}

	// Используем существующий путь для создания peer'а
	url := fmt.Sprintf("%s/api/v1/wireguard/peers", wgc.baseURL)

	var resp WireGuardTunnelResponse
	_, err := wgc.doRequestWithRetry(ctx, "POST", url, token, req, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to create WireGuard tunnel: %w", err)
	}

	return &resp, nil
}

// DeleteWireGuardTunnel удаляет WireGuard туннель
func (wgc *WireGuardClient) DeleteWireGuardTunnel(ctx context.Context, token, tenantID string) error {
	requestID := fmt.Sprintf("wg-delete-%d", time.Now().Unix())

	// Используем существующий путь для удаления peer'а
	url := fmt.Sprintf("%s/api/v1/wireguard/peers?request_id=%s", wgc.baseURL, requestID)

	var resp map[string]interface{}
	_, err := wgc.doRequestWithRetry(ctx, "DELETE", url, token, nil, &resp)
	if err != nil {
		return fmt.Errorf("failed to delete WireGuard tunnel: %w", err)
	}

	return nil
}

// GetWireGuardTunnel получает информацию о WireGuard туннеле
func (wgc *WireGuardClient) GetWireGuardTunnel(ctx context.Context, token, tenantID string) (*WireGuardTunnelInfo, error) {
	// Используем существующий путь для получения списка peer'ов
	url := fmt.Sprintf("%s/api/v1/wireguard/peers", wgc.baseURL)

	var resp struct {
		Success   bool                   `json:"success"`
		Message   string                 `json:"message"`
		RequestID string                 `json:"request_id"`
		Peers     []*WireGuardTunnelInfo `json:"peers"`
	}

	_, err := wgc.doRequestWithRetry(ctx, "GET", url, token, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get WireGuard tunnel: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to get tunnel: %s", resp.Message)
	}

	// Возвращаем первый peer для данного tenant'а
	if len(resp.Peers) > 0 {
		return resp.Peers[0], nil
	}

	return nil, fmt.Errorf("no WireGuard tunnel found for tenant %s", tenantID)
}

// ListWireGuardTunnels получает список всех WireGuard туннелей
func (wgc *WireGuardClient) ListWireGuardTunnels(ctx context.Context, token string) ([]*WireGuardTunnelInfo, error) {
	// Используем существующий путь для получения списка peer'ов
	url := fmt.Sprintf("%s/api/v1/wireguard/peers", wgc.baseURL)

	var resp struct {
		Success   bool                   `json:"success"`
		Message   string                 `json:"message"`
		RequestID string                 `json:"request_id"`
		Peers     []*WireGuardTunnelInfo `json:"peers"`
	}

	_, err := wgc.doRequestWithRetry(ctx, "GET", url, token, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to list WireGuard tunnels: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to list tunnels: %s", resp.Message)
	}

	return resp.Peers, nil
}

// GetWireGuardStatus получает статус WireGuard сервера
func (wgc *WireGuardClient) GetWireGuardStatus(ctx context.Context, token string) (*WireGuardStatusResponse, error) {
	url := fmt.Sprintf("%s/api/v1/wireguard/status", wgc.baseURL)

	var resp WireGuardStatusResponse
	_, err := wgc.doRequestWithRetry(ctx, "GET", url, token, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get WireGuard status: %w", err)
	}

	return &resp, nil
}

// GetWireGuardMetrics получает метрики WireGuard
// ВНИМАНИЕ: Этот endpoint не существует на сервере
func (wgc *WireGuardClient) GetWireGuardMetrics(ctx context.Context, token string) (*WireGuardMetricsResponse, error) {
	return nil, fmt.Errorf("WireGuard metrics endpoint not implemented on server")
}

// UpdateWireGuardConfig обновляет конфигурацию WireGuard сервера
// ВНИМАНИЕ: Этот endpoint не существует на сервере
func (wgc *WireGuardClient) UpdateWireGuardConfig(ctx context.Context, token string, forceReload bool) error {
	return fmt.Errorf("WireGuard config update endpoint not implemented on server")
}

// WireGuardConfigRequest запрос на получение WireGuard конфигурации
type WireGuardConfigRequest struct {
	TenantID string `json:"tenant_id"`
	PeerID   string `json:"peer_id"`
}

// WireGuardConfigResponse ответ с WireGuard конфигурацией
type WireGuardConfigResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	ClientConfig string `json:"client_config"`
	PeerIP       string `json:"peer_ip,omitempty"`
	TenantCIDR   string `json:"tenant_cidr,omitempty"`
	Error        string `json:"error,omitempty"`
}

// GetWireGuardConfig получает WireGuard конфигурацию для peer'а
func (wgc *WireGuardClient) GetWireGuardConfig(ctx context.Context, token, tenantID, peerID string) (*WireGuardConfigResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/peers/%s/wireguard-config", wgc.baseURL, tenantID, peerID)

	var resp WireGuardConfigResponse
	_, err := wgc.doRequestWithRetry(ctx, "GET", url, token, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get WireGuard config: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to get WireGuard config: %s", resp.Error)
	}

	return &resp, nil
}

// SaveWireGuardConfig сохраняет конфигурацию клиента в файл
func (wgc *WireGuardClient) SaveWireGuardConfig(config, filename string) error {
	return wgc.SaveWireGuardConfigWithBackup(config, filename, false)
}

// SaveWireGuardConfigWithBackup сохраняет конфигурацию с опциональным бэкапом
func (wgc *WireGuardClient) SaveWireGuardConfigWithBackup(config, filename string, createBackup bool) error {
	if strings.TrimSpace(config) == "" {
		return fmt.Errorf("empty config")
	}
	if filename == "" {
		return fmt.Errorf("filename is required")
	}

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	// Создаем бэкап если файл существует и запрошен бэкап
	if createBackup {
		if _, err := os.Stat(filename); err == nil {
			backupPath := filename + ".bak"
			if err := os.Rename(filename, backupPath); err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}
		}
	}

	if err := os.WriteFile(filename, []byte(config), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// ApplyWireGuardConfig применяет WireGuard конфигурацию через wg-quick
// Возвращает путь к созданному конфигурационному файлу
func (wgc *WireGuardClient) ApplyWireGuardConfig(config string) (string, error) {
	if strings.TrimSpace(config) == "" {
		return "", fmt.Errorf("empty config")
	}

	// Проверяем права root для wg-quick
	if os.Geteuid() != 0 {
		return "", fmt.Errorf("wg-quick requires root privileges. Please run with sudo")
	}

	tmpDir := filepath.Join(os.TempDir(), "cloudbridge-wg")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to prepare temp dir: %w", err)
	}
	cfgPath := filepath.Join(tmpDir, fmt.Sprintf("wg-%d.conf", time.Now().UnixNano()))
	if err := os.WriteFile(cfgPath, []byte(config), 0o600); err != nil {
		return "", fmt.Errorf("failed to write temp config: %w", err)
	}

	// Проверяем, не поднят ли уже интерфейс
	interfaceName := wgc.extractInterfaceName(config)
	if interfaceName != "" {
		if wgc.isInterfaceUp(interfaceName) {
			return cfgPath, fmt.Errorf("interface %s is already up", interfaceName)
		}
	}

	cmd := exec.Command("wg-quick", "up", cfgPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return cfgPath, fmt.Errorf("wg-quick up failed: %w: %s", err, string(out))
	}
	return cfgPath, nil
}

// DownWireGuardConfig останавливает интерфейс по конфигу (опционально для тестов)
func (wgc *WireGuardClient) DownWireGuardConfig(configPath string) error {
	if strings.TrimSpace(configPath) == "" {
		return fmt.Errorf("config path required")
	}

	// Если путь не найден, попробуем найти интерфейс по имени
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Попробуем найти интерфейс по имени из файла
		interfaceName := wgc.extractInterfaceNameFromFile(configPath)
		if interfaceName != "" && wgc.isInterfaceUp(interfaceName) {
			cmd := exec.Command("wg-quick", "down", interfaceName)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("wg-quick down %s failed: %w: %s", interfaceName, err, string(out))
			}
			return nil
		}
		return fmt.Errorf("config file not found: %s", configPath)
	}

	cmd := exec.Command("wg-quick", "down", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick down failed: %w: %s", err, string(out))
	}
	return nil
}

// extractInterfaceName извлекает имя интерфейса из конфигурации
func (wgc *WireGuardClient) extractInterfaceName(config string) string {
	lines := strings.Split(config, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[Interface]") {
			// Ищем следующую строку с Address
			for i, l := range lines {
				if i > 0 && strings.HasPrefix(strings.TrimSpace(l), "Address") {
					// Имя интерфейса обычно в комментарии или можно извлечь из Address
					// Для простоты возвращаем "wg0" как дефолт
					return "wg0"
				}
			}
		}
	}
	return ""
}

// extractInterfaceNameFromFile извлекает имя интерфейса из файла конфигурации
func (wgc *WireGuardClient) extractInterfaceNameFromFile(configPath string) string {
	// Имя интерфейса обычно в базовом имени файла без расширения
	baseName := filepath.Base(configPath)
	if strings.HasSuffix(baseName, ".conf") {
		return strings.TrimSuffix(baseName, ".conf")
	}
	return baseName
}

// isInterfaceUp проверяет, поднят ли интерфейс
func (wgc *WireGuardClient) isInterfaceUp(interfaceName string) bool {
	cmd := exec.Command("wg", "show", interfaceName)
	err := cmd.Run()
	return err == nil
}
