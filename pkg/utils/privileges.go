package utils

import (
	"runtime"
)

// IsRunningAsAdmin checks if the current process is running with administrative privileges
func IsRunningAsAdmin() bool {
	switch runtime.GOOS {
	case "windows":
		return isWindowsAdmin()
	default:
		return isUnixRoot()
	}
}

// GetPrivilegeWarning returns a platform-specific warning message about privileges
func GetPrivilegeWarning() string {
	switch runtime.GOOS {
	case "windows":
		return "Warning: WireGuard fallback may require Administrator privileges"
	case "linux", "darwin", "freebsd", "openbsd", "netbsd":
		return "Warning: WireGuard fallback may require root privileges or CAP_NET_ADMIN capability"
	default:
		return "Warning: WireGuard fallback may require elevated privileges"
	}
}
