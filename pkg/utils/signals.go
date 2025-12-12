package utils

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// SetupSignalHandler sets up cross-platform signal handling
func SetupSignalHandler() chan os.Signal {
	sigChan := make(chan os.Signal, 1)

	switch runtime.GOOS {
	case "windows":
		// Windows supports SIGINT and SIGTERM
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	default:
		// Unix-like systems support more signals
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	}

	return sigChan
}

// GetSupportedSignals returns the list of signals supported on the current platform
func GetSupportedSignals() []os.Signal {
	switch runtime.GOOS {
	case "windows":
		return []os.Signal{os.Interrupt, syscall.SIGTERM}
	default:
		return []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}
	}
}
