package utils

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// CommandValidator provides safe command execution with input validation
type CommandValidator struct {
	allowedCommands map[string]bool
	interfaceRegex  *regexp.Regexp
}

// NewCommandValidator creates a new command validator
func NewCommandValidator() *CommandValidator {
	return &CommandValidator{
		allowedCommands: map[string]bool{
			"wg":       true,
			"wg-quick": true,
			"ip":       true,
			"ifconfig": true,
			"nssm":     true,
		},
		interfaceRegex: regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`),
	}
}

// ValidateInterfaceName validates interface name to prevent command injection
func (cv *CommandValidator) ValidateInterfaceName(name string) error {
	const maxInterfaceNameLen = 15 // Linux interface name limit

	if len(name) == 0 || len(name) > maxInterfaceNameLen {
		return fmt.Errorf("invalid interface name length: %d (must be 1-%d chars)", len(name), maxInterfaceNameLen)
	}

	if !cv.interfaceRegex.MatchString(name) {
		return fmt.Errorf("interface name contains invalid characters: %s (allowed: alphanumeric, _, -)", name)
	}

	return nil
}

// ValidateCommand checks if a command is in the allowed list
func (cv *CommandValidator) ValidateCommand(cmd string) error {
	if !cv.allowedCommands[cmd] {
		return fmt.Errorf("command not allowed: %s", cmd)
	}
	return nil
}

// ValidateFilePath validates file path to prevent path traversal
func (cv *CommandValidator) ValidateFilePath(path string) error {
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains directory traversal: %s", path)
	}

	// Check for null bytes (can be used to bypass checks)
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains null bytes: %s", path)
	}

	return nil
}

// SafeExecWG safely executes a WireGuard command with validation
func (cv *CommandValidator) SafeExecWG(subcommand string, interfaceName string, args ...string) (*exec.Cmd, error) {
	// Validate interface name
	if interfaceName != "" {
		if err := cv.ValidateInterfaceName(interfaceName); err != nil {
			return nil, fmt.Errorf("invalid interface name: %w", err)
		}
	}

	// Build command
	cmdArgs := []string{subcommand}
	if interfaceName != "" {
		cmdArgs = append(cmdArgs, interfaceName)
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("wg", cmdArgs...)
	return cmd, nil
}

// SafeExecWGQuick safely executes wg-quick with validation
func (cv *CommandValidator) SafeExecWGQuick(action string, configPath string) (*exec.Cmd, error) {
	// Validate action
	validActions := map[string]bool{"up": true, "down": true}
	if !validActions[action] {
		return nil, fmt.Errorf("invalid wg-quick action: %s (allowed: up, down)", action)
	}

	// Validate config path
	if err := cv.ValidateFilePath(configPath); err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	cmd := exec.Command("wg-quick", action, configPath)
	return cmd, nil
}

// SafeExecIP safely executes ip command with validation
func (cv *CommandValidator) SafeExecIP(subcommand string, interfaceName string, args ...string) (*exec.Cmd, error) {
	// Validate interface name
	if interfaceName != "" {
		if err := cv.ValidateInterfaceName(interfaceName); err != nil {
			return nil, fmt.Errorf("invalid interface name: %w", err)
		}
	}

	// Validate subcommand
	validSubcommands := map[string]bool{
		"link": true,
		"addr": true,
		"route": true,
	}
	if !validSubcommands[subcommand] {
		return nil, fmt.Errorf("invalid ip subcommand: %s", subcommand)
	}

	// Build command
	cmdArgs := []string{subcommand}
	cmdArgs = append(cmdArgs, args...)
	if interfaceName != "" {
		cmdArgs = append(cmdArgs, "dev", interfaceName)
	}

	cmd := exec.Command("ip", cmdArgs...)
	return cmd, nil
}

// SanitizeArgument sanitizes a command argument
func (cv *CommandValidator) SanitizeArgument(arg string) (string, error) {
	// Check for shell metacharacters
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return "", fmt.Errorf("argument contains dangerous character: %s", char)
		}
	}

	return arg, nil
}
