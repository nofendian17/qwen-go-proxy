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

// Negative Test Cases

func TestCredentials_IsExpired_NilReceiver(t *testing.T) {
	var c *Credentials
	assert.True(t, c.IsExpired())
}

func TestCredentials_GetToken_NilReceiver(t *testing.T) {
	var c *Credentials
	assert.Equal(t, "", c.GetToken())
}

func TestCredentials_Sanitize_NilReceiver(t *testing.T) {
	var c *Credentials
	assert.Nil(t, c.Sanitize())
}

func TestCredentials_IsExpired_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		credentials *Credentials
		expected    bool
	}{
		{
			name:        "nil credentials",
			credentials: nil,
			expected:    true,
		},
		{
			name: "zero expiry date",
			credentials: &Credentials{
				ExpiryDate: 0,
			},
			expected: true,
		},
		{
			name: "expired credentials",
			credentials: &Credentials{
				ExpiryDate: time.Now().Add(-10 * time.Minute).UnixMilli(),
			},
			expected: true,
		},
		{
			name: "valid credentials",
			credentials: &Credentials{
				ExpiryDate: time.Now().Add(10 * time.Minute).UnixMilli(),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.credentials.IsExpired())
		})
	}
}

func TestConfig_Validate_NilConfig(t *testing.T) {
	var c *Config
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config is nil")
}

func TestConfig_Validate_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		expect string
	}{
		{
			name: "missing oauth base url",
			config: &Config{
				QWENOAuthClientID:      "test-client",
				QWENOAuthDeviceAuthURL: "test-url",
				APIBaseURL:             "test-api",
			},
			expect: "qwen_oauth_base_url is required",
		},
		{
			name: "missing client id",
			config: &Config{
				QWENOAuthBaseURL:       "test-base",
				QWENOAuthDeviceAuthURL: "test-url",
				APIBaseURL:             "test-api",
			},
			expect: "qwen_oauth_client_id is required",
		},
		{
			name: "missing device auth url",
			config: &Config{
				QWENOAuthBaseURL:  "test-base",
				QWENOAuthClientID: "test-client",
				APIBaseURL:        "test-api",
			},
			expect: "qwen_oauth_device_auth_url is required",
		},
		{
			name: "missing api base url",
			config: &Config{
				QWENOAuthBaseURL:       "test-base",
				QWENOAuthClientID:      "test-client",
				QWENOAuthDeviceAuthURL: "test-url",
			},
			expect: "api_base_url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expect)
		})
	}
}

func TestConfig_Validate_InvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		expect string
	}{
		{
			name: "invalid log level",
			config: &Config{
				QWENOAuthBaseURL:       "test-base",
				QWENOAuthClientID:      "test-client",
				QWENOAuthDeviceAuthURL: "test-url",
				APIBaseURL:             "test-api",
				LogLevel:               "invalid",
			},
			expect: "invalid log_level",
		},
		{
			name: "negative rate limit rps",
			config: &Config{
				QWENOAuthBaseURL:           "test-base",
				QWENOAuthClientID:          "test-client",
				QWENOAuthDeviceAuthURL:     "test-url",
				APIBaseURL:                 "test-api",
				LogLevel:                   "info",
				RateLimitRequestsPerSecond: -1,
			},
			expect: "rate_limit_rps must be positive",
		},
		{
			name: "zero rate limit rps",
			config: &Config{
				QWENOAuthBaseURL:           "test-base",
				QWENOAuthClientID:          "test-client",
				QWENOAuthDeviceAuthURL:     "test-url",
				APIBaseURL:                 "test-api",
				LogLevel:                   "info",
				RateLimitRequestsPerSecond: 0,
			},
			expect: "rate_limit_rps must be positive",
		},
		{
			name: "negative rate limit burst",
			config: &Config{
				QWENOAuthBaseURL:           "test-base",
				QWENOAuthClientID:          "test-client",
				QWENOAuthDeviceAuthURL:     "test-url",
				APIBaseURL:                 "test-api",
				LogLevel:                   "info",
				RateLimitRequestsPerSecond: 10,
				RateLimitBurst:             -1,
			},
			expect: "rate_limit_burst must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expect)
		})
	}
}

func TestCompletionRequest_Validate_NilRequest(t *testing.T) {
	var r *CompletionRequest
	err := r.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "completion request is nil")
}

func TestCompletionRequest_Validate_InvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		req    *CompletionRequest
		expect string
	}{
		{
			name: "nil prompt",
			req: &CompletionRequest{
				Prompt: nil,
			},
			expect: "prompt is required",
		},
		{
			name: "empty prompt",
			req: &CompletionRequest{
				Prompt: "",
			},
			expect: "prompt is required",
		},
		{
			name: "negative max tokens",
			req: &CompletionRequest{
				Prompt:    "test prompt",
				MaxTokens: -1,
			},
			expect: "max_tokens must be non-negative",
		},
		{
			name: "temperature too low",
			req: &CompletionRequest{
				Prompt:      "test prompt",
				Temperature: -0.1,
			},
			expect: "temperature must be between 0 and 2",
		},
		{
			name: "temperature too high",
			req: &CompletionRequest{
				Prompt:      "test prompt",
				Temperature: 2.1,
			},
			expect: "temperature must be between 0 and 2",
		},
		{
			name: "top_p too low",
			req: &CompletionRequest{
				Prompt: "test prompt",
				TopP:   -0.1,
			},
			expect: "top_p must be between 0 and 1",
		},
		{
			name: "top_p too high",
			req: &CompletionRequest{
				Prompt: "test prompt",
				TopP:   1.1,
			},
			expect: "top_p must be between 0 and 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expect)
		})
	}
}

func TestChatCompletionRequest_Validate_NilRequest(t *testing.T) {
	var r *ChatCompletionRequest
	err := r.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chat completion request is nil")
}

func TestChatCompletionRequest_Validate_InvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		req    *ChatCompletionRequest
		expect string
	}{
		{
			name: "no messages",
			req: &ChatCompletionRequest{
				Messages: []ChatMessage{},
			},
			expect: "messages are required",
		},
		{
			name: "negative max tokens",
			req: &ChatCompletionRequest{
				Messages:  []ChatMessage{{Role: "user", Content: "test"}},
				MaxTokens: -1,
			},
			expect: "max_tokens must be non-negative",
		},
		{
			name: "temperature too low",
			req: &ChatCompletionRequest{
				Messages:    []ChatMessage{{Role: "user", Content: "test"}},
				Temperature: -0.1,
			},
			expect: "temperature must be between 0 and 2",
		},
		{
			name: "temperature too high",
			req: &ChatCompletionRequest{
				Messages:    []ChatMessage{{Role: "user", Content: "test"}},
				Temperature: 2.1,
			},
			expect: "temperature must be between 0 and 2",
		},
		{
			name: "top_p too low",
			req: &ChatCompletionRequest{
				Messages: []ChatMessage{{Role: "user", Content: "test"}},
				TopP:     -0.1,
			},
			expect: "top_p must be between 0 and 1",
		},
		{
			name: "top_p too high",
			req: &ChatCompletionRequest{
				Messages: []ChatMessage{{Role: "user", Content: "test"}},
				TopP:     1.1,
			},
			expect: "top_p must be between 0 and 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expect)
		})
	}
}

func TestChatMessage_Validate_NilMessage(t *testing.T) {
	var m *ChatMessage
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chat message is nil")
}

func TestChatMessage_Validate_InvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		msg    *ChatMessage
		expect string
	}{
		{
			name: "empty role",
			msg: &ChatMessage{
				Role:    "",
				Content: "test content",
			},
			expect: "role is required",
		},
		{
			name: "invalid role",
			msg: &ChatMessage{
				Role:    "invalid",
				Content: "test content",
			},
			expect: "invalid role",
		},
		{
			name: "nil content",
			msg: &ChatMessage{
				Role:    "user",
				Content: nil,
			},
			expect: "content is required",
		},
		{
			name: "empty content",
			msg: &ChatMessage{
				Role:    "user",
				Content: "",
			},
			expect: "content is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expect)
		})
	}
}

func TestAppError_WithRequestID_NilReceiver(t *testing.T) {
	var err *AppError
	// WithRequestID doesn't handle nil receivers gracefully and will panic
	// This test documents the current behavior
	assert.Panics(t, func() {
		err.WithRequestID("test-id")
	})
}

func TestAppError_WithContext_NilReceiver(t *testing.T) {
	var err *AppError
	// WithContext doesn't handle nil receivers gracefully and will panic
	// This test documents the current behavior
	assert.Panics(t, func() {
		err.WithContext("key", "value")
	})
}

func TestAppError_Error_NilReceiver(t *testing.T) {
	var err *AppError
	// Error method doesn't handle nil receivers gracefully and will panic
	// This test documents the current behavior
	assert.Panics(t, func() {
		err.Error()
	})
}

func TestAppError_Unwrap_NilReceiver(t *testing.T) {
	var err *AppError
	// Unwrap method doesn't handle nil receivers gracefully and will panic
	// This test documents the current behavior
	assert.Panics(t, func() {
		err.Unwrap()
	})
}

func TestErrorConstructors_WithEmptyParameters(t *testing.T) {
	tests := []struct {
		name        string
		constructor func() *AppError
	}{
		{
			name: "NewAuthError with empty message",
			constructor: func() *AppError {
				return NewAuthError("", "", nil)
			},
		},
		{
			name: "NewValidationError with empty message",
			constructor: func() *AppError {
				return NewValidationError("", "", nil)
			},
		},
		{
			name: "NewNetworkError with empty message",
			constructor: func() *AppError {
				return NewNetworkError("", "", nil)
			},
		},
		{
			name: "NewConfigurationError with empty message",
			constructor: func() *AppError {
				return NewConfigurationError("", "", nil)
			},
		},
		{
			name: "NewStreamingError with empty message",
			constructor: func() *AppError {
				return NewStreamingError("", "", nil)
			},
		},
		{
			name: "NewInternalError with empty message",
			constructor: func() *AppError {
				return NewInternalError("", "", nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor()
			assert.NotNil(t, err)
			assert.NotEmpty(t, err.Error())
			assert.NotEqual(t, time.Time{}, err.Timestamp)
		})
	}
}

func TestErrorTypeChecking_WithNilError(t *testing.T) {
	var err error

	assert.False(t, IsAuthError(err))
	assert.False(t, IsRateLimitError(err))
	assert.False(t, IsValidationError(err))
	assert.False(t, IsNetworkError(err))
	assert.False(t, IsStreamingError(err))
	assert.Equal(t, ErrorTypeInternal, GetErrorType(err))
}

func TestErrorTypeChecking_WithNonAppError(t *testing.T) {
	regularErr := errors.New("regular error")

	assert.False(t, IsAuthError(regularErr))
	assert.False(t, IsRateLimitError(regularErr))
	assert.False(t, IsValidationError(regularErr))
	assert.False(t, IsNetworkError(regularErr))
	assert.False(t, IsStreamingError(regularErr))
	assert.Equal(t, ErrorTypeInternal, GetErrorType(regularErr))
}

func TestAppError_WithContext_EdgeCases(t *testing.T) {
	err := NewInternalError("Test error", "", nil)

	// Test with various value types
	err = err.WithContext("string", "value")
	err = err.WithContext("int", 42)
	err = err.WithContext("bool", true)
	err = err.WithContext("nil", nil)

	assert.Equal(t, "value", err.Context["string"])
	assert.Equal(t, 42, err.Context["int"])
	assert.Equal(t, true, err.Context["bool"])
	assert.Equal(t, nil, err.Context["nil"])
}

func TestRateLimitError_Context(t *testing.T) {
	err := NewRateLimitError("Rate limited", 0)
	assert.Equal(t, 0, err.Context["retry_after"])

	err = NewRateLimitError("Rate limited", -1)
	assert.Equal(t, -1, err.Context["retry_after"])

	err = NewRateLimitError("Rate limited", 60)
	assert.Equal(t, 60, err.Context["retry_after"])
}
