// Package onboarding handles invite-based client onboarding
package onboarding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

const (
	// DefaultControlPlaneURL is the default control plane URL
	DefaultControlPlaneURL = "https://edge.2gc.ru"

	// API endpoints
	validateEndpoint = "/api/v1/invites/validate"
	redeemEndpoint   = "/api/v1/invites/redeem"
)

// OnboardingManager handles invite-based onboarding
type OnboardingManager struct {
	apiClient *http.Client
	baseURL   string
}

// NewOnboardingManager creates a new onboarding manager
func NewOnboardingManager(baseURL string) *OnboardingManager {
	if baseURL == "" {
		baseURL = DefaultControlPlaneURL
	}

	return &OnboardingManager{
		apiClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

// ValidateRequest represents token validation request
type ValidateRequest struct {
	Token string `json:"token"`
}

// ValidateResponse represents token validation response
type ValidateResponse struct {
	Valid  bool   `json:"valid"`
	Invite *Invite `json:"invite,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Invite represents an invite
type Invite struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenant_id"`
	Role      string            `json:"role"`
	ExpiresAt time.Time         `json:"expires_at"`
	Scopes    []string          `json:"scopes"`
	MaxUses   int               `json:"max_uses"`
	UsedCount int               `json:"used_count"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// JoinRequest represents join via invite token
type JoinRequest struct {
	Token      string                 `json:"token"`
	DeviceInfo map[string]interface{} `json:"device_info"`
}

// JoinResponse from control plane
type JoinResponse struct {
	DeviceID    string   `json:"device_id,omitempty"`
	UserID      string   `json:"user_id,omitempty"`
	TenantID    string   `json:"tenant_id"`
	RelayHosts  []string `json:"relay_hosts,omitempty"`
	Credentials struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	} `json:"credentials"`
	Config struct {
		ServerID     string `json:"server_id,omitempty"`
		WireGuardIP  string `json:"wireguard_ip,omitempty"`
		WireGuardKey string `json:"wireguard_key,omitempty"`
	} `json:"config,omitempty"`
}

// Join redeems invite token and configures client
func (m *OnboardingManager) Join(ctx context.Context, token string) (*types.Config, *JoinResponse, error) {
	// 1. Validate token
	validateResp, err := m.validateToken(ctx, token)
	if err != nil {
		return nil, nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !validateResp.Valid {
		return nil, nil, fmt.Errorf("invalid invite token: %s", validateResp.Error)
	}

	// 2. Gather device info
	deviceInfo := m.gatherDeviceInfo()

	// 3. Redeem invite
	joinResp, err := m.redeemInvite(ctx, token, deviceInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("invite redemption failed: %w", err)
	}

	// 4. Generate config
	cfg, err := m.generateConfig(joinResp)
	if err != nil {
		return nil, nil, fmt.Errorf("config generation failed: %w", err)
	}

	return cfg, joinResp, nil
}

// ValidateToken validates an invite token without redeeming it
func (m *OnboardingManager) ValidateToken(ctx context.Context, token string) (*ValidateResponse, error) {
	return m.validateToken(ctx, token)
}

func (m *OnboardingManager) validateToken(ctx context.Context, token string) (*ValidateResponse, error) {
	url := m.baseURL + validateEndpoint

	reqBody := ValidateRequest{
		Token: token,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("validation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var validateResp ValidateResponse
	if err := json.Unmarshal(body, &validateResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &validateResp, nil
}

func (m *OnboardingManager) redeemInvite(ctx context.Context, token string, deviceInfo map[string]interface{}) (*JoinResponse, error) {
	url := m.baseURL + redeemEndpoint

	reqBody := JoinRequest{
		Token:      token,
		DeviceInfo: deviceInfo,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("redemption failed with status %d: %s", resp.StatusCode, string(body))
	}

	var joinResp JoinResponse
	if err := json.Unmarshal(body, &joinResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &joinResp, nil
}

func (m *OnboardingManager) gatherDeviceInfo() map[string]interface{} {
	hostname := "unknown"
	if h, err := os.Hostname(); err == nil {
		hostname = h
	}

	return map[string]interface{}{
		"hostname": hostname,
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"version":  runtime.Version(),
	}
}

func (m *OnboardingManager) generateConfig(joinResp *JoinResponse) (*types.Config, error) {
	// Determine relay host
	relayHost := "edge.2gc.ru"
	if len(joinResp.RelayHosts) > 0 {
		relayHost = joinResp.RelayHosts[0]
	}

	// Determine server ID
	serverID := joinResp.DeviceID
	if serverID == "" {
		serverID = joinResp.UserID
	}
	if serverID == "" {
		return nil, fmt.Errorf("no device_id or user_id in response")
	}

	cfg := &types.Config{
		Relay: types.RelayConfig{
			Host: relayHost,
			Port: 5552,
			TLS: types.TLSConfig{
				Enabled:    true,
				VerifyCert: true,
			},
		},
		Auth: types.AuthConfig{
			Type:  "jwt",
			Token: joinResp.Credentials.Token,
		},
		P2P: types.P2PConfig{
			MaxConnections: 100,
			HeartbeatInterval: 30 * time.Second,
		},
		ICE: types.ICEConfig{
			STUNServers: []string{
				"stun.l.google.com:19302",
				"stun1.l.google.com:19302",
			},
		},
		WebSocket: types.WebSocketConfig{
			Enabled: true,
		},
		API: types.APIConfig{
			BaseURL: fmt.Sprintf("https://%s:5552", relayHost),
			Timeout: 30 * time.Second,
		},
		Logging: types.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Performance: types.PerformanceConfig{
			Enabled: true,
		},
	}

	return cfg, nil
}

// PrintJoinSummary prints a summary of the join operation
func PrintJoinSummary(joinResp *JoinResponse, configPath string) {
	fmt.Println("\n✅ Successfully joined CloudBridge network!")
	fmt.Println("\nSummary:")

	if joinResp.DeviceID != "" {
		fmt.Printf("   Device ID:  %s\n", joinResp.DeviceID)
	}
	if joinResp.UserID != "" {
		fmt.Printf("   User ID:    %s\n", joinResp.UserID)
	}
	fmt.Printf("   Tenant ID:  %s\n", joinResp.TenantID)

	if len(joinResp.RelayHosts) > 0 {
		fmt.Printf("   Relay Host: %s\n", joinResp.RelayHosts[0])
	}

	if !joinResp.Credentials.ExpiresAt.IsZero() {
		fmt.Printf("   Token Expires: %s\n", joinResp.Credentials.ExpiresAt.Format(time.RFC3339))
	}

	fmt.Printf("\nConfiguration saved to: %s\n", configPath)

	fmt.Println("\nNext steps:")
	fmt.Println("   1. Start service:  sudo systemctl start cloudbridge-client")
	fmt.Println("   2. Enable on boot: sudo systemctl enable cloudbridge-client")
	fmt.Println("   3. Check status:   sudo systemctl status cloudbridge-client")
	fmt.Println("   4. View logs:      sudo journalctl -u cloudbridge-client -f")
	fmt.Println()
}
