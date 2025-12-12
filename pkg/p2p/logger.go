package p2p

import (
	"fmt"
	"log"
)

// SimpleLogger implements the Logger interface using Go's standard log package
type SimpleLogger struct {
	prefix string
}

// NewSimpleLogger creates a new simple logger
func NewSimpleLogger(prefix string) *SimpleLogger {
	return &SimpleLogger{
		prefix: prefix,
	}
}

// Info logs an info message
func (l *SimpleLogger) Info(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		log.Printf("[%s] INFO: %s %v", l.prefix, msg, fields)
	} else {
		log.Printf("[%s] INFO: %s", l.prefix, msg)
	}
}

// Error logs an error message
func (l *SimpleLogger) Error(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		log.Printf("[%s] ERROR: %s %v", l.prefix, msg, fields)
	} else {
		log.Printf("[%s] ERROR: %s", l.prefix, msg)
	}
}

// Debug logs a debug message
func (l *SimpleLogger) Debug(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		log.Printf("[%s] DEBUG: %s %v", l.prefix, msg, fields)
	} else {
		log.Printf("[%s] DEBUG: %s", l.prefix, msg)
	}
}

// Warn logs a warning message
func (l *SimpleLogger) Warn(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		log.Printf("[%s] WARN: %s %v", l.prefix, msg, fields)
	} else {
		log.Printf("[%s] WARN: %s", l.prefix, msg)
	}
}

// NoOpLogger implements the Logger interface but does nothing (for testing)
type NoOpLogger struct{}

// NewNoOpLogger creates a new no-op logger
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

// Info does nothing
func (l *NoOpLogger) Info(msg string, fields ...interface{}) {}

// Error does nothing
func (l *NoOpLogger) Error(msg string, fields ...interface{}) {}

// Debug does nothing
func (l *NoOpLogger) Debug(msg string, fields ...interface{}) {}

// Warn does nothing
func (l *NoOpLogger) Warn(msg string, fields ...interface{}) {}

// FormatFields formats fields for logging
func FormatFields(fields ...interface{}) string {
	if len(fields) == 0 {
		return ""
	}

	result := ""
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			if result != "" {
				result += ", "
			}
			result += fmt.Sprintf("%v=%v", fields[i], fields[i+1])
		}
	}
	return result
}
