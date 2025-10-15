package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"qwen-go-proxy/internal/domain/entities"
)

// LoadConfig loads configuration from environment variables with defaults and validation
func LoadConfig() (*entities.Config, error) {
	// Load .env file if it exists (ignore error if file doesn't exist)
	godotenv.Load()

	config := &entities.Config{
		ServerPort:                 getEnvIntWithDefault("SERVER_PORT", 8080),
		ServerHost:                 getEnvWithDefault("SERVER_HOST", "0.0.0.0"),
		ReadTimeout:                getEnvDurationWithDefault("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:               getEnvDurationWithDefault("WRITE_TIMEOUT", 30*time.Second),
		QWENOAuthBaseURL:           getEnvWithDefault("QWEN_OAUTH_BASE_URL", "https://chat.qwen.ai"),
		QWENOAuthClientID:          getEnvWithDefault("QWEN_OAUTH_CLIENT_ID", "f0304373b74a44d2b584a3fb70ca9e56"),
		QWENOAuthScope:             getEnvWithDefault("QWEN_OAUTH_SCOPE", "openid profile email model.completion"),
		QWENOAuthDeviceAuthURL:     getEnvWithDefault("QWEN_OAUTH_DEVICE_AUTH_URL", "https://chat.qwen.ai/api/v1/oauth2/device/code"),
		QWENDir:                    getEnvWithDefault("QWEN_DIR", ".qwen"),
		TokenRefreshBuffer:         getEnvDurationWithDefault("TOKEN_REFRESH_BUFFER", 5*time.Minute),
		ShutdownTimeout:            getEnvDurationWithDefault("SHUTDOWN_TIMEOUT", 30*time.Second),
		DebugMode:                  getEnvBoolWithDefault("DEBUG_MODE", false),
		LogLevel:                   getEnvWithDefault("LOG_LEVEL", "info"),
		LogFormat:                  getEnvWithDefault("LOG_FORMAT", "json"),
		RateLimitRequestsPerSecond: getEnvIntWithDefault("RATE_LIMIT_RPS", 10),
		RateLimitBurst:             getEnvIntWithDefault("RATE_LIMIT_BURST", 20),
		APIBaseURL:                 getEnvWithDefault("API_BASE_URL", "https://portal.qwen.ai/v1"),
		TrustedProxies:             getEnvSliceWithDefault("TRUSTED_PROXIES", []string{}),
		EnableTLS:                  getEnvBoolWithDefault("ENABLE_TLS", false),
		TLSCertFile:                getEnvWithDefault("TLS_CERT_FILE", ""),
		TLSKeyFile:                 getEnvWithDefault("TLS_KEY_FILE", ""),
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// validateConfig checks if the configuration is valid
func validateConfig(c *entities.Config) error {
	// Validate server port is in valid range (1-65535)
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		return fmt.Errorf("SERVER_PORT must be a valid port number (1-65535), got: %d", c.ServerPort)
	}

	if c.QWENOAuthBaseURL == "" {
		return fmt.Errorf("QWEN_OAUTH_BASE_URL cannot be empty")
	}

	if c.QWENOAuthClientID == "" {
		return fmt.Errorf("QWEN_OAUTH_CLIENT_ID cannot be empty")
	}

	if c.QWENDir == "" {
		return fmt.Errorf("QWEN_DIR cannot be empty")
	}

	if c.TokenRefreshBuffer < 0 {
		return fmt.Errorf("TOKEN_REFRESH_BUFFER must be non-negative")
	}

	if c.ShutdownTimeout < 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT_SECONDS must be non-negative")
	}

	if c.RateLimitRequestsPerSecond <= 0 {
		return fmt.Errorf("RATE_LIMIT_REQUESTS_PER_SECOND must be positive")
	}

	if c.RateLimitBurst <= 0 {
		return fmt.Errorf("RATE_LIMIT_BURST must be positive")
	}

	if c.LogLevel == "" {
		return fmt.Errorf("LOG_LEVEL cannot be empty")
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLogLevels, c.LogLevel) {
		return fmt.Errorf("LOG_LEVEL must be one of: %v, got: %s", validLogLevels, c.LogLevel)
	}

	// Validate URLs
	if _, err := url.Parse(c.QWENOAuthBaseURL); err != nil {
		return fmt.Errorf("QWEN_OAUTH_BASE_URL is not a valid URL: %w", err)
	}

	if _, err := url.Parse(c.APIBaseURL); err != nil {
		return fmt.Errorf("API_BASE_URL is not a valid URL: %w", err)
	}

	return nil
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getEnvWithDefault gets an environment variable with a default fallback
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt64WithDefault gets an environment variable as int64 with a default fallback
func getEnvInt64WithDefault(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvDurationWithDefault gets an environment variable as time.Duration with a default fallback
func getEnvDurationWithDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvBoolWithDefault gets an environment variable as bool with a default fallback
func getEnvBoolWithDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvIntWithDefault gets an environment variable as int with a default fallback
func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 32); err == nil {
			return int(parsed)
		}
	}
	return defaultValue
}

// getEnvSliceWithDefault gets an environment variable as string slice with a default fallback
func getEnvSliceWithDefault(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple comma-separated parsing for now
		// In production, consider using a more robust parsing library
		if value == "" {
			return defaultValue
		}
		// For now, return a slice with the single value
		// TODO: Implement proper comma-separated parsing if needed
		return []string{value}
	}
	return defaultValue
}
