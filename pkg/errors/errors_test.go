package errors

import (
	"errors"
	"testing"
	"time"
)

func TestNewRelayError(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		message       string
		expectRetry   bool
		expectDelay   time.Duration
	}{
		{
			name:          "rate limit error",
			code:          ErrRateLimitExceeded,
			message:       "Too many requests",
			expectRetry:   true,
			expectDelay:   time.Second * 5,
		},
		{
			name:          "server unavailable error",
			code:          ErrServerUnavailable,
			message:       "Server is down",
			expectRetry:   true,
			expectDelay:   time.Second * 10,
		},
		{
			name:          "invalid token error",
			code:          ErrInvalidToken,
			message:       "Token is invalid",
			expectRetry:   false,
			expectDelay:   time.Second,
		},
		{
			name:          "authentication failed",
			code:          ErrAuthenticationFailed,
			message:       "Authentication failed",
			expectRetry:   false,
			expectDelay:   time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRelayError(tt.code, tt.message)

			if err.Code != tt.code {
				t.Errorf("Expected code %s, got %s", tt.code, err.Code)
			}

			if err.Message != tt.message {
				t.Errorf("Expected message %s, got %s", tt.message, err.Message)
			}

			if err.IsRetryable() != tt.expectRetry {
				t.Errorf("Expected retryable %v, got %v", tt.expectRetry, err.IsRetryable())
			}

			if err.GetDelay() != tt.expectDelay {
				t.Errorf("Expected delay %v, got %v", tt.expectDelay, err.GetDelay())
			}
		})
	}
}

func TestRelayError_Error(t *testing.T) {
	err := NewRelayError(ErrInvalidToken, "Token expired")
	expected := "invalid_token: Token expired"

	if err.Error() != expected {
		t.Errorf("Expected error string %q, got %q", expected, err.Error())
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{"rate limit", ErrRateLimitExceeded, true},
		{"server unavailable", ErrServerUnavailable, true},
		{"heartbeat failed", ErrHeartbeatFailed, true},
		{"connection timeout", ErrConnectionTimeout, true},
		{"data transfer failed", ErrDataTransferFailed, true},
		{"invalid token", ErrInvalidToken, false},
		{"authentication failed", ErrAuthenticationFailed, false},
		{"tunnel creation failed", ErrTunnelCreationFailed, false},
		{"tenant not found", ErrTenantNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryable(tt.code)
			if result != tt.expected {
				t.Errorf("isRetryable(%s) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

func TestGetRetryDelay(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected time.Duration
	}{
		{"rate limit", ErrRateLimitExceeded, time.Second * 5},
		{"server unavailable", ErrServerUnavailable, time.Second * 10},
		{"heartbeat failed", ErrHeartbeatFailed, time.Second * 2},
		{"connection timeout", ErrConnectionTimeout, time.Second * 3},
		{"data transfer failed", ErrDataTransferFailed, time.Second * 1},
		{"unknown", "unknown_error", time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRetryDelay(tt.code)
			if result != tt.expected {
				t.Errorf("getRetryDelay(%s) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

func TestHandleError_RelayError(t *testing.T) {
	originalErr := NewRelayError(ErrInvalidToken, "Token is invalid")

	relayErr, err := HandleError(originalErr)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if relayErr == nil {
		t.Fatal("Expected non-nil relay error")
	}

	if relayErr.Code != ErrInvalidToken {
		t.Errorf("Expected code %s, got %s", ErrInvalidToken, relayErr.Code)
	}
}

func TestHandleError_GenericError(t *testing.T) {
	tests := []struct {
		name         string
		errorMessage string
		expectedCode string
	}{
		{
			name:         "invalid token",
			errorMessage: "invalid token provided",
			expectedCode: ErrInvalidToken,
		},
		{
			name:         "rate limit",
			errorMessage: "rate limit exceeded",
			expectedCode: ErrRateLimitExceeded,
		},
		{
			name:         "connection limit",
			errorMessage: "connection limit reached",
			expectedCode: ErrConnectionLimitReached,
		},
		{
			name:         "server unavailable",
			errorMessage: "server unavailable",
			expectedCode: ErrServerUnavailable,
		},
		{
			name:         "tls error",
			errorMessage: "tls handshake failed",
			expectedCode: ErrTLSHandshakeFailed,
		},
		{
			name:         "unknown error",
			errorMessage: "something went wrong",
			expectedCode: ErrUnknownMessageType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errorMessage)
			relayErr, handleErr := HandleError(err)

			if handleErr != nil {
				t.Errorf("Expected no handle error, got: %v", handleErr)
			}

			if relayErr == nil {
				t.Fatal("Expected non-nil relay error")
			}

			if relayErr.Code != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, relayErr.Code)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"exact match", "hello", "hello", true},
		{"substring at start", "hello world", "hello", true},
		{"substring at end", "hello world", "world", true},
		{"substring in middle", "hello world", "lo wo", true},
		{"not found", "hello world", "xyz", false},
		{"empty substring", "hello", "", true},
		{"empty string", "", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestNewRetryStrategy(t *testing.T) {
	maxRetries := 5
	backoffMultiplier := 2.0
	maxBackoff := time.Second * 30

	strategy := NewRetryStrategy(maxRetries, backoffMultiplier, maxBackoff)

	if strategy.MaxRetries != maxRetries {
		t.Errorf("Expected MaxRetries %d, got %d", maxRetries, strategy.MaxRetries)
	}

	if strategy.BackoffMultiplier != backoffMultiplier {
		t.Errorf("Expected BackoffMultiplier %f, got %f", backoffMultiplier, strategy.BackoffMultiplier)
	}

	if strategy.MaxBackoff != maxBackoff {
		t.Errorf("Expected MaxBackoff %v, got %v", maxBackoff, strategy.MaxBackoff)
	}

	if strategy.CurrentRetry != 0 {
		t.Errorf("Expected CurrentRetry 0, got %d", strategy.CurrentRetry)
	}
}

func TestRetryStrategy_ShouldRetry(t *testing.T) {
	tests := []struct {
		name          string
		maxRetries    int
		currentRetry  int
		error         error
		expected      bool
	}{
		{
			name:          "retryable error within limit",
			maxRetries:    3,
			currentRetry:  1,
			error:         NewRelayError(ErrRateLimitExceeded, "Too many requests"),
			expected:      true,
		},
		{
			name:          "retryable error at limit",
			maxRetries:    3,
			currentRetry:  3,
			error:         NewRelayError(ErrRateLimitExceeded, "Too many requests"),
			expected:      false,
		},
		{
			name:          "non-retryable error",
			maxRetries:    3,
			currentRetry:  1,
			error:         NewRelayError(ErrInvalidToken, "Invalid token"),
			expected:      false,
		},
		{
			name:          "generic error",
			maxRetries:    3,
			currentRetry:  1,
			error:         errors.New("some error"),
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewRetryStrategy(tt.maxRetries, 2.0, time.Second*30)
			strategy.CurrentRetry = tt.currentRetry

			result := strategy.ShouldRetry(tt.error)
			if result != tt.expected {
				t.Errorf("ShouldRetry() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRetryStrategy_GetNextDelay(t *testing.T) {
	strategy := NewRetryStrategy(5, 2.0, time.Second*30)

	tests := []struct {
		name          string
		error         error
		currentRetry  int
		expectedMin   time.Duration
		expectedMax   time.Duration
	}{
		{
			name:          "first retry",
			error:         NewRelayError(ErrRateLimitExceeded, "Too many requests"),
			currentRetry:  0,
			expectedMin:   time.Second * 5,
			expectedMax:   time.Second * 15,
		},
		{
			name:          "second retry",
			error:         NewRelayError(ErrServerUnavailable, "Server down"),
			currentRetry:  1,
			expectedMin:   time.Second * 10,
			expectedMax:   time.Second * 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy.CurrentRetry = tt.currentRetry
			delay := strategy.GetNextDelay(tt.error)

			if delay < tt.expectedMin {
				t.Errorf("Delay %v is less than minimum %v", delay, tt.expectedMin)
			}

			if delay > tt.expectedMax && delay > strategy.MaxBackoff {
				t.Errorf("Delay %v exceeds maximum %v", delay, tt.expectedMax)
			}
		})
	}
}

func TestRetryStrategy_GetNextDelay_CapsAtMaxBackoff(t *testing.T) {
	maxBackoff := time.Second * 10
	strategy := NewRetryStrategy(10, 2.0, maxBackoff)
	strategy.CurrentRetry = 5

	err := NewRelayError(ErrServerUnavailable, "Server unavailable")
	delay := strategy.GetNextDelay(err)

	if delay > maxBackoff {
		t.Errorf("Delay %v exceeds MaxBackoff %v", delay, maxBackoff)
	}
}

func TestRetryStrategy_Reset(t *testing.T) {
	strategy := NewRetryStrategy(5, 2.0, time.Second*30)
	strategy.CurrentRetry = 3

	strategy.Reset()

	if strategy.CurrentRetry != 0 {
		t.Errorf("Expected CurrentRetry to be 0 after reset, got %d", strategy.CurrentRetry)
	}
}

func TestRetryStrategy_IncrementRetry(t *testing.T) {
	strategy := NewRetryStrategy(5, 2.0, time.Second*30)

	initialRetry := strategy.CurrentRetry
	err := NewRelayError(ErrServerUnavailable, "Server unavailable")
	_ = strategy.GetNextDelay(err)

	if strategy.CurrentRetry != initialRetry+1 {
		t.Errorf("Expected CurrentRetry to increment from %d to %d, got %d",
			initialRetry, initialRetry+1, strategy.CurrentRetry)
	}
}

func TestErrorCodes(t *testing.T) {
	// Test that all error codes are defined
	codes := []string{
		ErrInvalidToken,
		ErrRateLimitExceeded,
		ErrConnectionLimitReached,
		ErrServerUnavailable,
		ErrInvalidTunnelInfo,
		ErrUnknownMessageType,
		ErrTLSHandshakeFailed,
		ErrAuthenticationFailed,
		ErrTunnelCreationFailed,
		ErrHeartbeatFailed,
		ErrTenantLimitExceeded,
		ErrTenantNotFound,
		ErrIPNotAllowed,
		ErrBufferPoolExhausted,
		ErrConnectionTimeout,
		ErrDataTransferFailed,
	}

	for _, code := range codes {
		if code == "" {
			t.Errorf("Error code is empty")
		}
	}
}
