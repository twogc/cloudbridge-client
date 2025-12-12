package errors

import (
	"fmt"
	"time"
)

// Error codes as defined in the requirements
const (
	ErrInvalidToken           = "invalid_token"
	ErrRateLimitExceeded      = "rate_limit_exceeded"
	ErrConnectionLimitReached = "connection_limit_reached"
	ErrServerUnavailable      = "server_unavailable"
	ErrInvalidTunnelInfo      = "invalid_tunnel_info"
	ErrUnknownMessageType     = "unknown_message_type"
	ErrTLSHandshakeFailed     = "tls_handshake_failed"
	ErrAuthenticationFailed   = "authentication_failed"
	ErrTunnelCreationFailed   = "tunnel_creation_failed"
	ErrHeartbeatFailed        = "heartbeat_failed"
	ErrTenantLimitExceeded    = "tenant_limit_exceeded"
	ErrTenantNotFound         = "tenant_not_found"
	ErrIPNotAllowed           = "ip_not_allowed"
	ErrBufferPoolExhausted    = "buffer_pool_exhausted"
	ErrConnectionTimeout      = "connection_timeout"
	ErrDataTransferFailed     = "data_transfer_failed"
)

// RelayError represents a relay-specific error
type RelayError struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Retry   bool          `json:"retry,omitempty"`
	Delay   time.Duration `json:"delay,omitempty"`
}

// Error implements the error interface
func (e *RelayError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// IsRetryable returns true if the error can be retried
func (e *RelayError) IsRetryable() bool {
	return e.Retry
}

// GetDelay returns the delay before retry
func (e *RelayError) GetDelay() time.Duration {
	return e.Delay
}

// NewRelayError creates a new relay error
func NewRelayError(code, message string) *RelayError {
	return &RelayError{
		Code:    code,
		Message: message,
		Retry:   isRetryable(code),
		Delay:   getRetryDelay(code),
	}
}

// isRetryable determines if an error code is retryable
func isRetryable(code string) bool {
	switch code {
	case ErrRateLimitExceeded, ErrServerUnavailable, ErrHeartbeatFailed,
		ErrConnectionTimeout, ErrDataTransferFailed:
		return true
	default:
		return false
	}
}

// getRetryDelay returns the appropriate delay for retry
func getRetryDelay(code string) time.Duration {
	switch code {
	case ErrRateLimitExceeded:
		return time.Second * 5
	case ErrServerUnavailable:
		return time.Second * 10
	case ErrHeartbeatFailed:
		return time.Second * 2
	case ErrConnectionTimeout:
		return time.Second * 3
	case ErrDataTransferFailed:
		return time.Second * 1
	default:
		return time.Second
	}
}

// HandleError handles relay-specific errors and returns appropriate actions
func HandleError(err error) (*RelayError, error) {
	if relayErr, ok := err.(*RelayError); ok {
		return relayErr, nil
	}

	// Try to parse error message for known patterns
	msg := err.Error()
	switch {
	case contains(msg, "invalid token"):
		return NewRelayError(ErrInvalidToken, "Invalid or expired JWT token"), nil
	case contains(msg, "rate limit"):
		return NewRelayError(ErrRateLimitExceeded, "Rate limit exceeded"), nil
	case contains(msg, "connection limit"):
		return NewRelayError(ErrConnectionLimitReached, "Connection limit reached"), nil
	case contains(msg, "server unavailable"):
		return NewRelayError(ErrServerUnavailable, "Server unavailable"), nil
	case contains(msg, "tls"):
		return NewRelayError(ErrTLSHandshakeFailed, "TLS handshake failed"), nil
	default:
		return NewRelayError(ErrUnknownMessageType, fmt.Sprintf("Unknown error: %s", msg)), nil
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 1; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}

// RetryStrategy defines retry behavior
type RetryStrategy struct {
	MaxRetries        int
	BackoffMultiplier float64
	MaxBackoff        time.Duration
	CurrentRetry      int
}

// NewRetryStrategy creates a new retry strategy
func NewRetryStrategy(maxRetries int, backoffMultiplier float64, maxBackoff time.Duration) *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:        maxRetries,
		BackoffMultiplier: backoffMultiplier,
		MaxBackoff:        maxBackoff,
		CurrentRetry:      0,
	}
}

// ShouldRetry determines if an operation should be retried
func (rs *RetryStrategy) ShouldRetry(err error) bool {
	if rs.CurrentRetry >= rs.MaxRetries {
		return false
	}

	relayErr, handleErr := HandleError(err)
	if handleErr != nil {
		// Log error but continue with retry logic
		return false
	}
	return relayErr != nil && relayErr.IsRetryable()
}

// GetNextDelay calculates the delay for the next retry
func (rs *RetryStrategy) GetNextDelay(err error) time.Duration {
	relayErr, handleErr := HandleError(err)
	if handleErr != nil {
		// Log error but continue with default delay
		return time.Second
	}
	if relayErr == nil {
		return time.Second
	}

	baseDelay := relayErr.GetDelay()
	delay := time.Duration(float64(baseDelay) * rs.BackoffMultiplier * float64(rs.CurrentRetry+1))

	if delay > rs.MaxBackoff {
		delay = rs.MaxBackoff
	}

	rs.CurrentRetry++
	return delay
}

// Reset resets the retry counter
func (rs *RetryStrategy) Reset() {
	rs.CurrentRetry = 0
}
