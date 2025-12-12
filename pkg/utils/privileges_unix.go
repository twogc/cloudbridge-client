//go:build !windows

package utils

import "os"

// isUnixRoot checks if running as root on Unix-like systems
func isUnixRoot() bool {
	return os.Geteuid() == 0
}

// isWindowsAdmin is a stub for Unix (not used)
func isWindowsAdmin() bool {
	return false
}
