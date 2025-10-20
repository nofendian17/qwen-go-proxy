package config

import (
	"os"
	"strconv"
	"strings"
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

	return config, nil
}

// getEnvWithDefault gets an environment variable with a default fallback
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
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
		// Simple comma-separated parsing
		if value == "" {
			return defaultValue
		}
		// Split by comma and trim whitespace
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return defaultValue
}
