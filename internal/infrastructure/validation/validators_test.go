package validation

import (
	"testing"
	"time"

	"qwen-go-proxy/internal/domain/entities"

	"github.com/stretchr/testify/assert"
)

func TestNewConfigValidator(t *testing.T) {
	validator := NewConfigValidator()
	assert.NotNil(t, validator)
}

func TestConfigValidator_ValidateConfig_Valid(t *testing.T) {
	config := &entities.Config{
		ServerPort:                 8080,
		ServerHost:                 "localhost",
		QWENOAuthBaseURL:           "https://oauth.example.com",
		QWENOAuthClientID:          "test-client-id",
		QWENOAuthScope:             "test-scope",
		QWENOAuthDeviceAuthURL:     "https://oauth.example.com/device",
		APIBaseURL:                 "https://api.example.com",
		QWENDir:                    ".qwen",
		TokenRefreshBuffer:         5 * time.Minute,
		ShutdownTimeout:            30 * time.Second,
		RateLimitRequestsPerSecond: 10,
		RateLimitBurst:             20,
		LogLevel:                   "info",
		DebugMode:                  false,
		LogFormat:                  "json",
	}

	validator := NewConfigValidator()
	err := validator.ValidateConfig(config)
	assert.NoError(t, err)
}

func TestConfigValidator_ValidateConfig_Nil(t *testing.T) {
	validator := NewConfigValidator()
	err := validator.ValidateConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config is nil")
}

func TestConfigValidator_ValidateConfig_InvalidPort(t *testing.T) {
	config := &entities.Config{
		ServerPort:             0, // Invalid port
		QWENOAuthBaseURL:       "https://oauth.example.com",
		QWENOAuthClientID:      "test-client-id",
		QWENOAuthDeviceAuthURL: "https://oauth.example.com/device",
		APIBaseURL:             "https://api.example.com",
		QWENDir:                ".qwen",
	}

	validator := NewConfigValidator()
	err := validator.ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SERVER_PORT must be a valid port number")
}

func TestConfigValidator_ValidateConfig_InvalidPortTooHigh(t *testing.T) {
	config := &entities.Config{
		ServerPort:             65536, // Invalid port - too high
		QWENOAuthBaseURL:       "https://oauth.example.com",
		QWENOAuthClientID:      "test-client-id",
		QWENOAuthDeviceAuthURL: "https://oauth.example.com/device",
		APIBaseURL:             "https://api.example.com",
		QWENDir:                ".qwen",
	}

	validator := NewConfigValidator()
	err := validator.ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SERVER_PORT must be a valid port number")
}

func TestConfigValidator_ValidateConfig_EmptyOAuthBaseURL(t *testing.T) {
	config := &entities.Config{
		ServerPort:             8080,
		QWENOAuthBaseURL:       "", // Empty
		QWENOAuthClientID:      "test-client-id",
		QWENOAuthDeviceAuthURL: "https://oauth.example.com/device",
		APIBaseURL:             "https://api.example.com",
		QWENDir:                ".qwen",
	}

	validator := NewConfigValidator()
	err := validator.ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "QWEN_OAUTH_BASE_URL cannot be empty")
}

func TestConfigValidator_ValidateConfig_EmptyClientID(t *testing.T) {
	config := &entities.Config{
		ServerPort:             8080,
		QWENOAuthBaseURL:       "https://oauth.example.com",
		QWENOAuthClientID:      "", // Empty
		QWENOAuthDeviceAuthURL: "https://oauth.example.com/device",
		APIBaseURL:             "https://api.example.com",
		QWENDir:                ".qwen",
	}

	validator := NewConfigValidator()
	err := validator.ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "QWEN_OAUTH_CLIENT_ID cannot be empty")
}

func TestConfigValidator_ValidateConfig_EmptyDeviceAuthURL(t *testing.T) {
	config := &entities.Config{
		ServerPort:             8080,
		QWENOAuthBaseURL:       "https://oauth.example.com",
		QWENOAuthClientID:      "test-client-id",
		QWENOAuthDeviceAuthURL: "", // Empty
		APIBaseURL:             "https://api.example.com",
		QWENDir:                ".qwen",
	}

	validator := NewConfigValidator()
	err := validator.ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "QWEN_OAUTH_DEVICE_AUTH_URL cannot be empty")
}

func TestConfigValidator_ValidateConfig_EmptyAPIBaseURL(t *testing.T) {
	config := &entities.Config{
		ServerPort:             8080,
		QWENOAuthBaseURL:       "https://oauth.example.com",
		QWENOAuthClientID:      "test-client-id",
		QWENOAuthDeviceAuthURL: "https://oauth.example.com/device",
		APIBaseURL:             "", // Empty
		QWENDir:                ".qwen",
	}

	validator := NewConfigValidator()
	err := validator.ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API_BASE_URL cannot be empty")
}

func TestConfigValidator_ValidateConfig_EmptyQWENDir(t *testing.T) {
	config := &entities.Config{
		ServerPort:             8080,
		QWENOAuthBaseURL:       "https://oauth.example.com",
		QWENOAuthClientID:      "test-client-id",
		QWENOAuthDeviceAuthURL: "https://oauth.example.com/device",
		APIBaseURL:             "https://api.example.com",
		QWENDir:                "", // Empty
	}

	validator := NewConfigValidator()
	err := validator.ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "QWEN_DIR cannot be empty")
}

func TestNewRequestValidator(t *testing.T) {
	validator := NewRequestValidator()
	assert.NotNil(t, validator)
}

func TestRequestValidator_ValidateChatCompletionRequest_Valid(t *testing.T) {
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		MaxTokens:   100,
		Temperature: 0.5,
		TopP:        0.9,
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatCompletionRequest(req)
	assert.NoError(t, err)
}

func TestRequestValidator_ValidateChatCompletionRequest_Nil(t *testing.T) {
	validator := NewRequestValidator()
	err := validator.ValidateChatCompletionRequest(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chat completion request is nil")
}

func TestRequestValidator_ValidateChatCompletionRequest_NoMessages(t *testing.T) {
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		// No messages
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatCompletionRequest(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "messages are required")
}

func TestRequestValidator_ValidateChatCompletionRequest_InvalidMessage(t *testing.T) {
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "invalid-role",
				Content: "Hello",
			},
		},
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatCompletionRequest(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message 0 is invalid")
}

func TestRequestValidator_ValidateChatCompletionRequest_InvalidMaxTokens(t *testing.T) {
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		MaxTokens: -1, // Invalid
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatCompletionRequest(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_tokens must be non-negative")
}

func TestRequestValidator_ValidateChatCompletionRequest_InvalidTemperature(t *testing.T) {
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		Temperature: 3.0, // Invalid - too high
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatCompletionRequest(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "temperature must be between 0 and 2")
}

func TestRequestValidator_ValidateChatCompletionRequest_InvalidTopP(t *testing.T) {
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		TopP: 1.5, // Invalid - too high
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatCompletionRequest(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "top_p must be between 0 and 1")
}

func TestRequestValidator_ValidateChatMessage_Valid(t *testing.T) {
	msg := &entities.ChatMessage{
		Role:    "user",
		Content: "Hello",
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatMessage(msg)
	assert.NoError(t, err)
}

func TestRequestValidator_ValidateChatMessage_Nil(t *testing.T) {
	validator := NewRequestValidator()
	err := validator.ValidateChatMessage(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chat message is nil")
}

func TestRequestValidator_ValidateChatMessage_NoRole(t *testing.T) {
	msg := &entities.ChatMessage{
		// No role
		Content: "Hello",
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatMessage(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role is required")
}

func TestRequestValidator_ValidateChatMessage_InvalidRole(t *testing.T) {
	msg := &entities.ChatMessage{
		Role:    "invalid-role",
		Content: "Hello",
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatMessage(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role: invalid-role")
}

func TestRequestValidator_ValidateChatMessage_NoContent(t *testing.T) {
	msg := &entities.ChatMessage{
		Role: "user",
		// No content
	}

	validator := NewRequestValidator()
	err := validator.ValidateChatMessage(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content is required")
}

func TestNewExpirationValidator(t *testing.T) {
	validator := NewExpirationValidator()
	assert.NotNil(t, validator)
}

func TestExpirationValidator_IsTokenExpired(t *testing.T) {
	validator := NewExpirationValidator()

	// Test with nil credentials
	expired := validator.IsTokenExpired(nil, 0)
	assert.True(t, expired)

	// Test with zero expiry date
	creds := &entities.Credentials{
		ExpiryDate: 0,
	}
	expired = validator.IsTokenExpired(creds, 0)
	assert.True(t, expired)

	// Test with expired token (past date)
	pastTime := time.Now().Add(-1 * time.Hour).UnixMilli()
	creds.ExpiryDate = pastTime
	expired = validator.IsTokenExpired(creds, 0)
	assert.True(t, expired)

	// Test with future token (not expired)
	futureTime := time.Now().Add(1 * time.Hour).UnixMilli()
	creds.ExpiryDate = futureTime
	expired = validator.IsTokenExpired(creds, 0)
	assert.False(t, expired)

	// Test with future token but within buffer (expired with buffer)
	futureTime = time.Now().Add(5 * time.Minute).UnixMilli()
	creds.ExpiryDate = futureTime
	expired = validator.IsTokenExpired(creds, 10*time.Minute) // 10 minute buffer
	assert.True(t, expired)

	// Test with future token and not within buffer (not expired)
	futureTime = time.Now().Add(15 * time.Minute).UnixMilli()
	creds.ExpiryDate = futureTime
	expired = validator.IsTokenExpired(creds, 10*time.Minute) // 10 minute buffer
	assert.False(t, expired)
}
