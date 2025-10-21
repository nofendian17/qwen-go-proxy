// Package validation provides validation utilities for domain entities.
// This package contains validation logic that depends on infrastructure concerns
// like URL parsing, separating them from pure domain logic.
package validation

import (
	"fmt"
	"net/url"
	"time"

	"qwen-go-proxy/internal/domain/entities"
)

// ConfigValidator provides validation for configuration entities.
// This handles validation that requires infrastructure dependencies like URL parsing.
type ConfigValidator struct{}

// NewConfigValidator creates a new configuration validator.
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

// ValidateConfig validates a configuration entity.
// It performs comprehensive validation including infrastructure-dependent checks.
func (v *ConfigValidator) ValidateConfig(config *entities.Config) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate server port is in valid range (1-65535)
	if config.ServerPort < 1 || config.ServerPort > 65535 {
		return fmt.Errorf("SERVER_PORT must be a valid port number (1-65535), got: %d", config.ServerPort)
	}

	if config.QWENOAuthBaseURL == "" {
		return fmt.Errorf("QWEN_OAUTH_BASE_URL cannot be empty")
	}

	if config.QWENOAuthClientID == "" {
		return fmt.Errorf("QWEN_OAUTH_CLIENT_ID cannot be empty")
	}

	if config.QWENOAuthDeviceAuthURL == "" {
		return fmt.Errorf("QWEN_OAUTH_DEVICE_AUTH_URL cannot be empty")
	}

	if config.APIBaseURL == "" {
		return fmt.Errorf("API_BASE_URL cannot be empty")
	}

	if config.QWENDir == "" {
		return fmt.Errorf("QWEN_DIR cannot be empty")
	}

	if config.TokenRefreshBuffer < 0 {
		return fmt.Errorf("TOKEN_REFRESH_BUFFER must be non-negative")
	}

	if config.ShutdownTimeout < 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be non-negative")
	}

	if config.RateLimitRequestsPerSecond <= 0 {
		return fmt.Errorf("RATE_LIMIT_REQUESTS_PER_SECOND must be positive")
	}

	if config.RateLimitBurst <= 0 {
		return fmt.Errorf("RATE_LIMIT_BURST must be positive")
	}

	if config.LogLevel == "" {
		return fmt.Errorf("LOG_LEVEL cannot be empty")
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLogLevels, config.LogLevel) {
		return fmt.Errorf("LOG_LEVEL must be one of: %v, got: %s", validLogLevels, config.LogLevel)
	}

	// Infrastructure-dependent URL validation
	if err := v.validateURL(config.QWENOAuthBaseURL, "QWEN_OAUTH_BASE_URL"); err != nil {
		return err
	}

	if parsed, err := url.Parse(config.QWENOAuthBaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("QWEN_OAUTH_BASE_URL must be a valid absolute URL with scheme and host")
	}

	if err := v.validateURL(config.APIBaseURL, "API_BASE_URL"); err != nil {
		return err
	}

	if parsed, err := url.Parse(config.APIBaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("API_BASE_URL must be a valid absolute URL with scheme and host")
	}

	return nil
}

// validateURL validates a URL string.
func (v *ConfigValidator) validateURL(rawURL, fieldName string) error {
	if _, err := url.Parse(rawURL); err != nil {
		return fmt.Errorf("%s is not a valid URL: %w", fieldName, err)
	}
	return nil
}

// contains checks if a slice contains a specific string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RequestValidator provides validation for request entities.
type RequestValidator struct{}

// NewRequestValidator creates a new request validator.
func NewRequestValidator() *RequestValidator {
	return &RequestValidator{}
}

// ValidateChatCompletionRequest validates a chat completion request.
func (v *RequestValidator) ValidateChatCompletionRequest(req *entities.ChatCompletionRequest) error {
	if req == nil {
		return fmt.Errorf("chat completion request is nil")
	}

	if len(req.Messages) == 0 {
		return fmt.Errorf("messages are required")
	}

	// Validate each message
	for i, msg := range req.Messages {
		if err := v.ValidateChatMessage(&msg); err != nil {
			return fmt.Errorf("message %d is invalid: %w", i, err)
		}
	}

	if req.MaxTokens < 0 {
		return fmt.Errorf("max_tokens must be non-negative")
	}

	if req.Temperature < 0 || req.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	if req.TopP < 0 || req.TopP > 1 {
		return fmt.Errorf("top_p must be between 0 and 1")
	}

	return nil
}

// ValidateChatMessage validates a chat message.
func (v *RequestValidator) ValidateChatMessage(msg *entities.ChatMessage) error {
	if msg == nil {
		return fmt.Errorf("chat message is nil")
	}

	if msg.Role == "" {
		return fmt.Errorf("role is required")
	}

	validRoles := map[string]bool{
		"system":    true,
		"user":      true,
		"assistant": true,
		"tool":      true,
	}

	if !validRoles[msg.Role] {
		return fmt.Errorf("invalid role: %s (must be system, user, assistant, or tool)", msg.Role)
	}

	if msg.Content == nil || msg.Content == "" {
		return fmt.Errorf("content is required")
	}

	return nil
}

// ExpirationValidator provides validation for credential entities.
type ExpirationValidator struct{}

// NewExpirationValidator creates a new expiration validator.
func NewExpirationValidator() *ExpirationValidator {
	return &ExpirationValidator{}
}

// IsTokenExpired checks if the credentials have expired with a buffer for token refresh.
func (v *ExpirationValidator) IsTokenExpired(credentials *entities.Credentials, buffer time.Duration) bool {
	if credentials == nil || credentials.ExpiryDate == 0 {
		return true
	}
	bufferTime := time.Now().Add(buffer).UnixMilli()
	return credentials.ExpiryDate <= bufferTime
}