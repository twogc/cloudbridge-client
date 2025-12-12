package onboarding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateToken_Success(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != validateEndpoint {
			t.Errorf("Expected path %s, got %s", validateEndpoint, r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		resp := ValidateResponse{
			Valid: true,
			Invite: &Invite{
				ID:        "invite-123",
				TenantID:  "tenant-001",
				Role:      "device",
				ExpiresAt: time.Now().Add(24 * time.Hour),
				Scopes:    []string{"network:read", "network:write"},
				MaxUses:   1,
				UsedCount: 0,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mgr := NewOnboardingManager(server.URL)

	resp, err := mgr.ValidateToken(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if !resp.Valid {
		t.Error("Expected valid=true")
	}

	if resp.Invite.ID != "invite-123" {
		t.Errorf("Expected invite ID 'invite-123', got '%s'", resp.Invite.ID)
	}

	if resp.Invite.TenantID != "tenant-001" {
		t.Errorf("Expected tenant ID 'tenant-001', got '%s'", resp.Invite.TenantID)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ValidateResponse{
			Valid: false,
			Error: "token expired",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mgr := NewOnboardingManager(server.URL)

	resp, err := mgr.ValidateToken(context.Background(), "invalid-token")
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if resp.Valid {
		t.Error("Expected valid=false for invalid token")
	}

	if resp.Error != "token expired" {
		t.Errorf("Expected error 'token expired', got '%s'", resp.Error)
	}
}

func TestJoin_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == validateEndpoint {
			resp := ValidateResponse{
				Valid: true,
				Invite: &Invite{
					ID:        "invite-123",
					TenantID:  "tenant-001",
					Role:      "device",
					ExpiresAt: time.Now().Add(24 * time.Hour),
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == redeemEndpoint {
			resp := JoinResponse{
				DeviceID: "device-abc",
				TenantID: "tenant-001",
				RelayHosts: []string{"edge.2gc.ru"},
			}
			resp.Credentials.Token = "jwt-token-here"
			resp.Credentials.ExpiresAt = time.Now().Add(24 * time.Hour)

			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr := NewOnboardingManager(server.URL)

	cfg, joinResp, err := mgr.Join(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	if joinResp.DeviceID != "device-abc" {
		t.Errorf("Expected device ID 'device-abc', got '%s'", joinResp.DeviceID)
	}

	if joinResp.TenantID != "tenant-001" {
		t.Errorf("Expected tenant ID 'tenant-001', got '%s'", joinResp.TenantID)
	}

	// Check generated config
	if cfg.Relay.Host != "edge.2gc.ru" {
		t.Errorf("Expected relay host 'edge.2gc.ru', got '%s'", cfg.Relay.Host)
	}

	if cfg.Auth.Token != "jwt-token-here" {
		t.Errorf("Expected auth token 'jwt-token-here', got '%s'", cfg.Auth.Token)
	}

	// Device ID and Tenant ID should be in the join response
	// The config structure doesn't have ServerID and TenantID in P2P config
	// These are returned in the join response
}

func TestJoin_InvalidToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		resp := ValidateResponse{
			Valid: false,
			Error: "token not found",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mgr := NewOnboardingManager(server.URL)

	_, _, err := mgr.Join(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("Expected error for invalid token, got nil")
	}

	if !contains(err.Error(), "invalid invite token") {
		t.Errorf("Expected 'invalid invite token' in error, got: %v", err)
	}
}

func TestGatherDeviceInfo(t *testing.T) {
	mgr := NewOnboardingManager("")

	info := mgr.gatherDeviceInfo()

	if info["os"] == nil {
		t.Error("Expected 'os' field in device info")
	}

	if info["arch"] == nil {
		t.Error("Expected 'arch' field in device info")
	}

	if info["hostname"] == nil {
		t.Error("Expected 'hostname' field in device info")
	}

	if info["version"] == nil {
		t.Error("Expected 'version' field in device info")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
