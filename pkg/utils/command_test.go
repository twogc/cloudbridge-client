package utils

import (
	"strings"
	"testing"
)

func TestCommandValidator_ValidateInterfaceName(t *testing.T) {
	cv := NewCommandValidator()

	tests := []struct {
		name      string
		ifaceName string
		wantError bool
	}{
		{"valid simple", "wg0", false},
		{"valid with dash", "wg-test", false},
		{"valid with underscore", "wg_test", false},
		{"valid max length", "wg-12345678901", false},
		{"empty", "", true},
		{"too long", "this-is-a-very-long-interface-name", true},
		{"special chars", "wg@test", true},
		{"spaces", "wg test", true},
		{"semicolon", "wg;rm -rf /", true},
		{"pipe", "wg|cat", true},
		{"dollar", "wg$test", true},
		{"backtick", "wg`whoami`", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cv.ValidateInterfaceName(tt.ifaceName)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateInterfaceName(%q) error = %v, wantError %v", tt.ifaceName, err, tt.wantError)
			}
		})
	}
}

func TestCommandValidator_ValidateCommand(t *testing.T) {
	cv := NewCommandValidator()

	tests := []struct {
		name      string
		cmd       string
		wantError bool
	}{
		{"allowed wg", "wg", false},
		{"allowed wg-quick", "wg-quick", false},
		{"allowed ip", "ip", false},
		{"not allowed rm", "rm", true},
		{"not allowed sh", "sh", true},
		{"not allowed bash", "bash", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cv.ValidateCommand(tt.cmd)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateCommand(%q) error = %v, wantError %v", tt.cmd, err, tt.wantError)
			}
		})
	}
}

func TestCommandValidator_ValidateFilePath(t *testing.T) {
	cv := NewCommandValidator()

	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{"valid absolute path", "/etc/wireguard/wg0.conf", false},
		{"valid relative path", "config/wg0.conf", false},
		{"path traversal dots", "../../../etc/passwd", true},
		{"path traversal in middle", "/etc/../../../passwd", true},
		{"null byte", "/etc/wireguard/\x00wg0.conf", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cv.ValidateFilePath(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateFilePath(%q) error = %v, wantError %v", tt.path, err, tt.wantError)
			}
		})
	}
}

func TestCommandValidator_SafeExecWG(t *testing.T) {
	cv := NewCommandValidator()

	tests := []struct {
		name          string
		subcommand    string
		interfaceName string
		args          []string
		wantError     bool
	}{
		{
			name:          "valid show command",
			subcommand:    "show",
			interfaceName: "wg0",
			args:          []string{},
			wantError:     false,
		},
		{
			name:          "valid set command",
			subcommand:    "set",
			interfaceName: "wg-test",
			args:          []string{"peer", "ABC123", "remove"},
			wantError:     false,
		},
		{
			name:          "invalid interface name",
			subcommand:    "show",
			interfaceName: "wg;rm -rf /",
			args:          []string{},
			wantError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := cv.SafeExecWG(tt.subcommand, tt.interfaceName, tt.args...)
			if (err != nil) != tt.wantError {
				t.Errorf("SafeExecWG() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && cmd == nil {
				t.Error("Expected non-nil command")
			}
			if !tt.wantError {
				// Verify command structure
				if cmd.Path != "wg" && !strings.HasSuffix(cmd.Path, "/wg") {
					t.Errorf("Expected wg command, got %s", cmd.Path)
				}
			}
		})
	}
}

func TestCommandValidator_SafeExecWGQuick(t *testing.T) {
	cv := NewCommandValidator()

	tests := []struct {
		name       string
		action     string
		configPath string
		wantError  bool
	}{
		{
			name:       "valid up action",
			action:     "up",
			configPath: "/etc/wireguard/wg0.conf",
			wantError:  false,
		},
		{
			name:       "valid down action",
			action:     "down",
			configPath: "/etc/wireguard/wg0.conf",
			wantError:  false,
		},
		{
			name:       "invalid action",
			action:     "restart",
			configPath: "/etc/wireguard/wg0.conf",
			wantError:  true,
		},
		{
			name:       "path traversal",
			action:     "up",
			configPath: "../../../etc/passwd",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := cv.SafeExecWGQuick(tt.action, tt.configPath)
			if (err != nil) != tt.wantError {
				t.Errorf("SafeExecWGQuick() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && cmd == nil {
				t.Error("Expected non-nil command")
			}
		})
	}
}

func TestCommandValidator_SafeExecIP(t *testing.T) {
	cv := NewCommandValidator()

	tests := []struct {
		name          string
		subcommand    string
		interfaceName string
		args          []string
		wantError     bool
	}{
		{
			name:          "valid link show",
			subcommand:    "link",
			interfaceName: "wg0",
			args:          []string{"show"},
			wantError:     false,
		},
		{
			name:          "valid link set down",
			subcommand:    "link",
			interfaceName: "wg-test",
			args:          []string{"set", "down"},
			wantError:     false,
		},
		{
			name:          "invalid subcommand",
			subcommand:    "netns",
			interfaceName: "wg0",
			args:          []string{},
			wantError:     true,
		},
		{
			name:          "invalid interface name",
			subcommand:    "link",
			interfaceName: "wg;ls",
			args:          []string{"show"},
			wantError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := cv.SafeExecIP(tt.subcommand, tt.interfaceName, tt.args...)
			if (err != nil) != tt.wantError {
				t.Errorf("SafeExecIP() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && cmd == nil {
				t.Error("Expected non-nil command")
			}
		})
	}
}

func TestCommandValidator_SanitizeArgument(t *testing.T) {
	cv := NewCommandValidator()

	tests := []struct {
		name      string
		arg       string
		wantError bool
	}{
		{"valid simple", "wg0", false},
		{"valid with dash", "wg-test", false},
		{"valid number", "123", false},
		{"semicolon", "test;rm -rf /", true},
		{"ampersand", "test&whoami", true},
		{"pipe", "test|cat", true},
		{"dollar", "test$HOME", true},
		{"backtick", "test`whoami`", true},
		{"parenthesis", "test()", true},
		{"redirect", "test>file", true},
		{"newline", "test\nrm", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cv.SanitizeArgument(tt.arg)
			if (err != nil) != tt.wantError {
				t.Errorf("SanitizeArgument(%q) error = %v, wantError %v", tt.arg, err, tt.wantError)
				return
			}
			if !tt.wantError && result != tt.arg {
				t.Errorf("SanitizeArgument(%q) = %q, want %q", tt.arg, result, tt.arg)
			}
		})
	}
}

func TestCommandValidator_ConcurrentValidation(t *testing.T) {
	cv := NewCommandValidator()

	// Test concurrent access to validator
	done := make(chan bool)
	iterations := 100

	for i := 0; i < 3; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				_ = cv.ValidateInterfaceName("wg0")
				_ = cv.ValidateCommand("wg")
				_ = cv.ValidateFilePath("/etc/wireguard/wg0.conf")
			}
			done <- true
		}()
	}

	for i := 0; i < 3; i++ {
		<-done
	}
}
