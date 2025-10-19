package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appError *AppError
		expected string
	}{
		{
			name: "error with details",
			appError: &AppError{
				Type:    ErrorTypeAuthentication,
				Message: "Authentication failed",
				Details: "Invalid credentials",
			},
			expected: "[authentication] Authentication failed: Invalid credentials",
		},
		{
			name: "error without details",
			appError: &AppError{
				Type:    ErrorTypeNetwork,
				Message: "Connection timeout",
			},
			expected: "[network] Connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.appError.Error())
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	appErr := &AppError{
		Type:       ErrorTypeInternal,
		Message:    "Internal error",
		Underlying: underlyingErr,
	}

	assert.Equal(t, underlyingErr, appErr.Unwrap())
}

func TestNewAuthError(t *testing.T) {
	underlyingErr := errors.New("oauth failed")
	err := NewAuthError("Authentication failed", "Invalid token", underlyingErr)

	assert.Equal(t, ErrorTypeAuthentication, err.Type)
	assert.Equal(t, "AUTH_FAILED", err.Code)
	assert.Equal(t, "Authentication failed", err.Message)
	assert.Equal(t, "Invalid token", err.Details)
	assert.Equal(t, underlyingErr, err.Underlying)
	assert.WithinDuration(t, time.Now(), err.Timestamp, time.Second)
}

func TestNewValidationError(t *testing.T) {
	underlyingErr := errors.New("validation failed")
	err := NewValidationError("Validation failed", "Invalid input", underlyingErr)

	assert.Equal(t, ErrorTypeValidation, err.Type)
	assert.Equal(t, "VALIDATION_FAILED", err.Code)
	assert.Equal(t, "Validation failed", err.Message)
	assert.Equal(t, "Invalid input", err.Details)
	assert.Equal(t, underlyingErr, err.Underlying)
}

func TestNewNetworkError(t *testing.T) {
	underlyingErr := errors.New("connection failed")
	err := NewNetworkError("Network error", "Connection timeout", underlyingErr)

	assert.Equal(t, ErrorTypeNetwork, err.Type)
	assert.Equal(t, "NETWORK_ERROR", err.Code)
	assert.Equal(t, "Network error", err.Message)
	assert.Equal(t, "Connection timeout", err.Details)
	assert.Equal(t, underlyingErr, err.Underlying)
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError("Rate limit exceeded", 60)

	assert.Equal(t, ErrorTypeRateLimit, err.Type)
	assert.Equal(t, "RATE_LIMIT_EXCEEDED", err.Code)
	assert.Equal(t, "Rate limit exceeded", err.Message)
	assert.Equal(t, 60, err.Context["retry_after"])
}

func TestNewConfigurationError(t *testing.T) {
	underlyingErr := errors.New("config parse failed")
	err := NewConfigurationError("Configuration error", "Invalid config file", underlyingErr)

	assert.Equal(t, ErrorTypeConfiguration, err.Type)
	assert.Equal(t, "CONFIG_ERROR", err.Code)
	assert.Equal(t, "Configuration error", err.Message)
	assert.Equal(t, "Invalid config file", err.Details)
	assert.Equal(t, underlyingErr, err.Underlying)
}

func TestNewStreamingError(t *testing.T) {
	underlyingErr := errors.New("stream failed")
	err := NewStreamingError("Streaming error", "Connection lost", underlyingErr)

	assert.Equal(t, ErrorTypeStreaming, err.Type)
	assert.Equal(t, "STREAMING_ERROR", err.Code)
	assert.Equal(t, "Streaming error", err.Message)
	assert.Equal(t, "Connection lost", err.Details)
	assert.Equal(t, underlyingErr, err.Underlying)
}

func TestNewInternalError(t *testing.T) {
	underlyingErr := errors.New("internal failure")
	err := NewInternalError("Internal error", "Unexpected condition", underlyingErr)

	assert.Equal(t, ErrorTypeInternal, err.Type)
	assert.Equal(t, "INTERNAL_ERROR", err.Code)
	assert.Equal(t, "Internal error", err.Message)
	assert.Equal(t, "Unexpected condition", err.Details)
	assert.Equal(t, underlyingErr, err.Underlying)
}

func TestAppError_WithRequestID(t *testing.T) {
	err := NewInternalError("Test error", "", nil)
	err = err.WithRequestID("req-123")

	assert.Equal(t, "req-123", err.RequestID)
}

func TestAppError_WithContext(t *testing.T) {
	err := NewInternalError("Test error", "", nil)
	err = err.WithContext("user_id", 123)
	err = err.WithContext("action", "login")

	assert.Equal(t, 123, err.Context["user_id"])
	assert.Equal(t, "login", err.Context["action"])
}

func TestIsAuthError(t *testing.T) {
	authErr := NewAuthError("Auth failed", "", nil)
	networkErr := NewNetworkError("Network failed", "", nil)
	regularErr := errors.New("regular error")

	assert.True(t, IsAuthError(authErr))
	assert.False(t, IsAuthError(networkErr))
	assert.False(t, IsAuthError(regularErr))
}

func TestIsRateLimitError(t *testing.T) {
	rateLimitErr := NewRateLimitError("Rate limited", 30)
	networkErr := NewNetworkError("Network failed", "", nil)
	regularErr := errors.New("regular error")

	assert.True(t, IsRateLimitError(rateLimitErr))
	assert.False(t, IsRateLimitError(networkErr))
	assert.False(t, IsRateLimitError(regularErr))
}

func TestIsValidationError(t *testing.T) {
	validationErr := NewValidationError("Validation failed", "", nil)
	networkErr := NewNetworkError("Network failed", "", nil)
	regularErr := errors.New("regular error")

	assert.True(t, IsValidationError(validationErr))
	assert.False(t, IsValidationError(networkErr))
	assert.False(t, IsValidationError(regularErr))
}

func TestIsNetworkError(t *testing.T) {
	networkErr := NewNetworkError("Network failed", "", nil)
	authErr := NewAuthError("Auth failed", "", nil)
	regularErr := errors.New("regular error")

	assert.True(t, IsNetworkError(networkErr))
	assert.False(t, IsNetworkError(authErr))
	assert.False(t, IsNetworkError(regularErr))
}

func TestIsStreamingError(t *testing.T) {
	streamingErr := NewStreamingError("Streaming failed", "", nil)
	networkErr := NewNetworkError("Network failed", "", nil)
	regularErr := errors.New("regular error")

	assert.True(t, IsStreamingError(streamingErr))
	assert.False(t, IsStreamingError(networkErr))
	assert.False(t, IsStreamingError(regularErr))
}

func TestGetErrorType(t *testing.T) {
	authErr := NewAuthError("Auth failed", "", nil)
	networkErr := NewNetworkError("Network failed", "", nil)
	regularErr := errors.New("regular error")

	assert.Equal(t, ErrorTypeAuthentication, GetErrorType(authErr))
	assert.Equal(t, ErrorTypeNetwork, GetErrorType(networkErr))
	assert.Equal(t, ErrorTypeInternal, GetErrorType(regularErr))
}

func TestErrorsAsCompatibility(t *testing.T) {
	underlyingErr := errors.New("underlying")
	appErr := NewInternalError("Internal error", "", underlyingErr)

	// Test that errors.As works with our AppError
	var target *AppError
	assert.True(t, errors.As(appErr, &target))
	assert.Equal(t, appErr, target)

	// Test unwrapping
	assert.True(t, errors.Is(appErr, underlyingErr))
}

func TestErrorTypeString(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		expected  string
	}{
		{ErrorTypeAuthentication, "authentication"},
		{ErrorTypeValidation, "validation"},
		{ErrorTypeNetwork, "network"},
		{ErrorTypeRateLimit, "rate_limit"},
		{ErrorTypeConfiguration, "configuration"},
		{ErrorTypeInternal, "internal"},
		{ErrorTypeStreaming, "streaming"},
	}

	for _, tt := range tests {
		t.Run(string(tt.errorType), func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.errorType))
		})
	}
}
