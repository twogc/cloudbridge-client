//go:build windows

package utils

import (
	"os"
	"os/user"
)

// isWindowsAdmin checks if running as Administrator on Windows
func isWindowsAdmin() bool {
	// Method 1: Check if we can write to a system directory
	if canWriteToSystem() {
		return true
	}

	// Method 2: Check user groups (requires user package)
	if currentUser, err := user.Current(); err == nil {
		// Check if user is in Administrators group
		// This is a simplified check - in production you might want to use Windows APIs
		if currentUser.Username == "Administrator" || currentUser.Name == "Administrator" {
			return true
		}
	}

	// Method 3: Try to access a privileged registry key or system resource
	// For now, we'll assume non-admin if other checks fail
	return false
}

// canWriteToSystem tries to write to a system directory to check admin privileges
func canWriteToSystem() bool {
	// Try to create a temporary file in Windows system directory
	tempFile := os.Getenv("WINDIR") + "\\Temp\\cloudbridge_admin_test.tmp"

	file, err := os.Create(tempFile)
	if err != nil {
		return false
	}

	file.Close()
	os.Remove(tempFile) // Clean up
	return true
}

// isUnixRoot is a stub for Windows (not used)
func isUnixRoot() bool {
	return false
}
