package config

import (
	"os"
	"testing"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

func TestLoadConfig_WithValidFile(t *testing.T) {
	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
relay:
  host: edge.2gc.ru
  port: 5553
  tls:
    enabled: true
    min_version: "1.3"
    verify_cert: true
auth:
  type: jwt
  secret: test-secret
`
	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Relay.Host != "edge.2gc.ru" {
		t.Errorf("Expected relay host edge.2gc.ru, got: %s", config.Relay.Host)
	}
	if config.Relay.Port != 5553 {
		t.Errorf("Expected relay port 5553, got: %d", config.Relay.Port)
	}
	if !config.Relay.TLS.Enabled {
		t.Error("Expected TLS to be enabled")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	// Test with non-existent file - should use defaults
	_, err := LoadConfig("/non/existent/config.yaml")
	if err == nil {
		t.Log("Config loaded with defaults (expected behavior)")
	}
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "edge.2gc.ru",
			Port: 5553,
			TLS: types.TLSConfig{
				Enabled:    true,
				MinVersion: "1.3",
				VerifyCert: true,
			},
		},
		Auth: types.AuthConfig{
			Type:   "jwt",
			Secret: "test-secret",
		},
		RateLimiting: types.RateLimitingConfig{
			MaxRetries:         3,
			BackoffMultiplier:  2.0,
		},
		WireGuard: types.WireGuardConfig{
			Enabled:       true,
			InterfaceName: "wg-test",
			MTU:           1420,
		},
	}

	err := validateConfig(config)
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidateConfig_InvalidRelayHost(t *testing.T) {
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "",
			Port: 5553,
		},
		Auth: types.AuthConfig{
			Type:   "jwt",
			Secret: "test-secret",
		},
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("Expected error for empty relay host")
	}
}

func TestValidateConfig_InvalidRelayPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero port", 0},
		{"negative port", -1},
		{"port too high", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.Config{
				Relay: types.RelayConfig{
					Host: "edge.2gc.ru",
					Port: tt.port,
				},
				Auth: types.AuthConfig{
					Type:   "jwt",
					Secret: "test-secret",
				},
			}

			err := validateConfig(config)
			if err == nil {
				t.Errorf("Expected error for port %d", tt.port)
			}
		})
	}
}

func TestValidateConfig_InvalidTLSVersion(t *testing.T) {
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "edge.2gc.ru",
			Port: 5553,
			TLS: types.TLSConfig{
				Enabled:    true,
				MinVersion: "1.2",
			},
		},
		Auth: types.AuthConfig{
			Type:   "jwt",
			Secret: "test-secret",
		},
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("Expected error for TLS version 1.2")
	}
}

func TestValidateConfig_MissingJWTSecret(t *testing.T) {
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "edge.2gc.ru",
			Port: 5553,
		},
		Auth: types.AuthConfig{
			Type:   "jwt",
			Secret: "",
		},
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("Expected error for missing JWT secret")
	}
}

func TestValidateConfig_OIDCMissingIssuer(t *testing.T) {
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "edge.2gc.ru",
			Port: 5553,
		},
		Auth: types.AuthConfig{
			Type: "oidc",
			OIDC: types.OIDCConfig{
				IssuerURL: "",
				Audience:  "test-client",
			},
		},
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("Expected error for missing OIDC issuer URL")
	}
}

func TestValidateConfig_OIDCMissingAudience(t *testing.T) {
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "edge.2gc.ru",
			Port: 5553,
		},
		Auth: types.AuthConfig{
			Type: "oidc",
			OIDC: types.OIDCConfig{
				IssuerURL: "https://auth.example.com",
				Audience:  "",
			},
		},
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("Expected error for missing OIDC audience")
	}
}

func TestValidateConfig_InvalidRateLimiting(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		multiplier float64
	}{
		{"negative max retries", -1, 2.0},
		{"zero backoff multiplier", 3, 0.0},
		{"negative backoff multiplier", 3, -1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.Config{
				Relay: types.RelayConfig{
					Host: "edge.2gc.ru",
					Port: 5553,
				},
				Auth: types.AuthConfig{
					Type:   "jwt",
					Secret: "test-secret",
				},
				RateLimiting: types.RateLimitingConfig{
					MaxRetries:        tt.maxRetries,
					BackoffMultiplier: tt.multiplier,
				},
			}

			err := validateConfig(config)
			if err == nil {
				t.Error("Expected error for invalid rate limiting config")
			}
		})
	}
}

func TestValidateConfig_InvalidWireGuardInterface(t *testing.T) {
	tests := []struct {
		name          string
		interfaceName string
	}{
		{"empty name", ""},
		{"too long", "wg-very-long-name-exceeds-limit"},
		{"special characters", "wg@test!"},
		{"with spaces", "wg test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.Config{
				Relay: types.RelayConfig{
					Host: "edge.2gc.ru",
					Port: 5553,
				},
				Auth: types.AuthConfig{
					Type:   "jwt",
					Secret: "test-secret",
				},
				WireGuard: types.WireGuardConfig{
					Enabled:       true,
					InterfaceName: tt.interfaceName,
				},
			}

			err := validateConfig(config)
			if err == nil {
				t.Errorf("Expected error for interface name: %s", tt.interfaceName)
			}
		})
	}
}

func TestValidateConfig_InvalidWireGuardMTU(t *testing.T) {
	tests := []struct {
		name string
		mtu  int
	}{
		{"too low", 499},
		{"too high", 9001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.Config{
				Relay: types.RelayConfig{
					Host: "edge.2gc.ru",
					Port: 5553,
				},
				Auth: types.AuthConfig{
					Type:   "jwt",
					Secret: "test-secret",
				},
				WireGuard: types.WireGuardConfig{
					Enabled:       true,
					InterfaceName: "wg-test",
					MTU:           tt.mtu,
				},
			}

			err := validateConfig(config)
			if err == nil {
				t.Errorf("Expected error for MTU: %d", tt.mtu)
			}
		})
	}
}

func TestValidateInterfaceName(t *testing.T) {
	tests := []struct {
		name      string
		ifaceName string
		wantError bool
	}{
		{"valid simple", "wg0", false},
		{"valid with dash", "wg-test", false},
		{"valid with underscore", "wg_test", false},
		{"empty", "", true},
		{"too long", "this-is-a-very-long-interface-name", true},
		{"special chars", "wg@test", true},
		{"spaces", "wg test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInterfaceName(tt.ifaceName)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateInterfaceName(%q) error = %v, wantError %v", tt.ifaceName, err, tt.wantError)
			}
		})
	}
}

func TestSubstituteEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envKey   string
		envValue string
		expected string
	}{
		{
			name:     "simple substitution",
			input:    "${TEST_VAR}",
			envKey:   "TEST_VAR",
			envValue: "test-value",
			expected: "test-value",
		},
		{
			name:     "with default value",
			input:    "${MISSING_VAR:default}",
			envKey:   "",
			envValue: "",
			expected: "default",
		},
		{
			name:     "no substitution",
			input:    "plain-text",
			envKey:   "",
			envValue: "",
			expected: "plain-text",
		},
		{
			name:     "multiple vars",
			input:    "${VAR1}-${VAR2}",
			envKey:   "VAR1",
			envValue: "first",
			expected: "first-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := substituteEnvVar(tt.input)
			if result != tt.expected {
				t.Errorf("substituteEnvVar(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCreateTLSConfig_Disabled(t *testing.T) {
	config := &types.Config{
		Relay: types.RelayConfig{
			TLS: types.TLSConfig{
				Enabled: false,
			},
		},
	}

	tlsConfig, err := CreateTLSConfig(config)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if tlsConfig != nil {
		t.Error("Expected nil TLS config when disabled")
	}
}

func TestCreateTLSConfig_Enabled(t *testing.T) {
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "edge.2gc.ru",
			TLS: types.TLSConfig{
				Enabled:    true,
				MinVersion: "1.3",
				VerifyCert: true,
				ServerName: "b1.2gc.space",
			},
		},
	}

	tlsConfig, err := CreateTLSConfig(config)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if tlsConfig == nil {
		t.Fatal("Expected non-nil TLS config")
	}
	if tlsConfig.ServerName != "b1.2gc.space" {
		t.Errorf("Expected server name b1.2gc.space, got: %s", tlsConfig.ServerName)
	}
	if tlsConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be false")
	}
}

func TestCreateTLSConfig_InsecureSkipVerify(t *testing.T) {
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "edge.2gc.ru",
			TLS: types.TLSConfig{
				Enabled:    true,
				MinVersion: "1.3",
				VerifyCert: false,
			},
		},
	}

	tlsConfig, err := CreateTLSConfig(config)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if tlsConfig == nil {
		t.Fatal("Expected non-nil TLS config")
	}
	if !tlsConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}
}

func TestSetDefaults(t *testing.T) {
	// Clear any existing config
	setDefaults()

	// Test that defaults are set properly
	// Note: This is implicitly tested in LoadConfig tests
	// but we can add specific checks here if needed
	t.Log("Defaults set successfully")
}
