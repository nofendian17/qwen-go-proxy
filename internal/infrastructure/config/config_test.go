package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qwen-go-proxy/internal/domain/entities"
)

func TestLoadConfig_DefaultValues(t *testing.T) {
	// Clear environment variables to test defaults
	clearEnvVars()

	config, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Test default values
	assert.Equal(t, 8080, config.ServerPort)
	assert.Equal(t, "0.0.0.0", config.ServerHost)
	assert.Equal(t, 30*time.Second, config.ReadTimeout)
	assert.Equal(t, 30*time.Second, config.WriteTimeout)
	assert.Equal(t, "https://chat.qwen.ai", config.QWENOAuthBaseURL)
	assert.Equal(t, "f0304373b74a44d2b584a3fb70ca9e56", config.QWENOAuthClientID)
	assert.Equal(t, ".qwen", config.QWENDir)
	assert.Equal(t, 5*time.Minute, config.TokenRefreshBuffer)
	assert.Equal(t, 30*time.Second, config.ShutdownTimeout)
	assert.False(t, config.DebugMode)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, "json", config.LogFormat)
	assert.Equal(t, 10, config.RateLimitRequestsPerSecond)
	assert.Equal(t, 20, config.RateLimitBurst)
	assert.Equal(t, "https://portal.qwen.ai/v1", config.APIBaseURL)
	assert.Empty(t, config.TrustedProxies)
	assert.False(t, config.EnableTLS)
	assert.Empty(t, config.TLSCertFile)
	assert.Empty(t, config.TLSKeyFile)
}

func TestLoadConfig_WithEnvVars(t *testing.T) {
	// Clear environment variables first
	clearEnvVars()

	// Set custom environment variables
	envVars := map[string]string{
		"SERVER_PORT":                    "9090",
		"SERVER_HOST":                    "127.0.0.1",
		"READ_TIMEOUT":                   "60s",
		"WRITE_TIMEOUT":                  "60s",
		"QWEN_OAUTH_BASE_URL":            "https://custom.qwen.ai",
		"QWEN_OAUTH_CLIENT_ID":           "custom-client-id",
		"QWEN_DIR":                       "/custom/qwen",
		"TOKEN_REFRESH_BUFFER":           "10m",
		"SHUTDOWN_TIMEOUT":               "60s",
		"DEBUG_MODE":                     "true",
		"LOG_LEVEL":                      "debug",
		"LOG_FORMAT":                     "text",
		"RATE_LIMIT_RPS":                 "20",
		"RATE_LIMIT_BURST":               "40",
		"STREAMING_MAX_ERRORS":           "20",
		"STREAMING_BUFFER_SIZE":          "8192",
		"STREAMING_TIMEOUT_SECONDS":      "1800",
		"STREAMING_WINDOW_SIZE":          "10",
		"STREAMING_SIMILARITY_THRESHOLD": "0.9",
		"STREAMING_TIME_WINDOW":          "5s",
		"STREAMING_MIN_CONFIDENCE":       "0.8",
		"API_BASE_URL":                   "https://custom.api.com/v1",
		"TRUSTED_PROXIES":                "127.0.0.1,192.168.1.1",
		"ENABLE_TLS":                     "true",
		"TLS_CERT_FILE":                  "/path/to/cert.pem",
		"TLS_KEY_FILE":                   "/path/to/key.pem",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer clearEnvVars()

	config, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Test custom values
	assert.Equal(t, 9090, config.ServerPort)
	assert.Equal(t, "127.0.0.1", config.ServerHost)
	assert.Equal(t, 60*time.Second, config.ReadTimeout)
	assert.Equal(t, 60*time.Second, config.WriteTimeout)
	assert.Equal(t, "https://custom.qwen.ai", config.QWENOAuthBaseURL)
	assert.Equal(t, "custom-client-id", config.QWENOAuthClientID)
	assert.Equal(t, "/custom/qwen", config.QWENDir)
	assert.Equal(t, 10*time.Minute, config.TokenRefreshBuffer)
	assert.Equal(t, 60*time.Second, config.ShutdownTimeout)
	assert.True(t, config.DebugMode)
	assert.Equal(t, "debug", config.LogLevel)
	assert.Equal(t, "text", config.LogFormat)
	assert.Equal(t, 20, config.RateLimitRequestsPerSecond)
	assert.Equal(t, 40, config.RateLimitBurst)
	assert.Equal(t, "https://custom.api.com/v1", config.APIBaseURL)
	assert.Equal(t, []string{"127.0.0.1", "192.168.1.1"}, config.TrustedProxies)
	assert.True(t, config.EnableTLS)
	assert.Equal(t, "/path/to/cert.pem", config.TLSCertFile)
	assert.Equal(t, "/path/to/key.pem", config.TLSKeyFile)
}

func TestValidateConfig_Valid(t *testing.T) {
	config := &entities.Config{
		ServerPort:                 8080,
		QWENOAuthBaseURL:           "https://chat.qwen.ai",
		QWENOAuthClientID:          "test-client-id",
		QWENOAuthDeviceAuthURL:     "https://chat.qwen.ai/api/v1/oauth2/device/code",
		QWENDir:                    ".qwen",
		TokenRefreshBuffer:         5 * time.Minute,
		ShutdownTimeout:            30 * time.Second,
		LogLevel:                   "info",
		RateLimitRequestsPerSecond: 10,
		RateLimitBurst:             20,
		APIBaseURL:                 "https://portal.qwen.ai/v1",
	}

	err := config.Validate()
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidPort(t *testing.T) {
	tests := []struct {
		name  string
		port  int
		error string
	}{
		{"port too low", 0, "SERVER_PORT must be a valid port number (1-65535), got: 0"},
		{"port too high", 70000, "SERVER_PORT must be a valid port number (1-65535), got: 70000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &entities.Config{
				ServerPort:                 tt.port,
				QWENOAuthBaseURL:           "https://chat.qwen.ai",
				QWENOAuthClientID:          "test-client-id",
				QWENOAuthDeviceAuthURL:     "https://chat.qwen.ai/api/v1/oauth2/device/code",
				QWENDir:                    ".qwen",
				TokenRefreshBuffer:         5 * time.Minute,
				ShutdownTimeout:            30 * time.Second,
				LogLevel:                   "info",
				RateLimitRequestsPerSecond: 10,
				RateLimitBurst:             20,
				APIBaseURL:                 "https://portal.qwen.ai/v1",
			}

			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.error)
		})
	}
}

func TestValidateConfig_InvalidURLs(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		field string
	}{
		{"invalid oauth base url", "not-a-url", "QWEN_OAUTH_BASE_URL"},
		{"invalid api base url", "also-not-a-url", "API_BASE_URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &entities.Config{
				ServerPort:                 8080,
				QWENOAuthBaseURL:           "https://chat.qwen.ai",
				QWENOAuthClientID:          "test-client-id",
				QWENOAuthDeviceAuthURL:     "https://chat.qwen.ai/api/v1/oauth2/device/code",
				QWENDir:                    ".qwen",
				TokenRefreshBuffer:         5 * time.Minute,
				ShutdownTimeout:            30 * time.Second,
				LogLevel:                   "info",
				RateLimitRequestsPerSecond: 10,
				RateLimitBurst:             20,
				APIBaseURL:                 "https://portal.qwen.ai/v1",
			}

			if tt.field == "QWEN_OAUTH_BASE_URL" {
				config.QWENOAuthBaseURL = tt.url
			} else {
				config.APIBaseURL = tt.url
			}

			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.field)
		})
	}
}

func TestValidateConfig_InvalidLogLevel(t *testing.T) {
	config := &entities.Config{
		ServerPort:                 8080,
		QWENOAuthBaseURL:           "https://chat.qwen.ai",
		QWENOAuthClientID:          "test-client-id",
		QWENOAuthDeviceAuthURL:     "https://chat.qwen.ai/api/v1/oauth2/device/code",
		QWENDir:                    ".qwen",
		TokenRefreshBuffer:         5 * time.Minute,
		ShutdownTimeout:            30 * time.Second,
		LogLevel:                   "invalid",
		RateLimitRequestsPerSecond: 10,
		RateLimitBurst:             20,
		APIBaseURL:                 "https://portal.qwen.ai/v1",
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LOG_LEVEL must be one of:")
}

func TestValidateConfig_EmptyFields(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*entities.Config)
		error string
	}{
		{"empty oauth base url", func(c *entities.Config) { c.QWENOAuthBaseURL = "" }, "QWEN_OAUTH_BASE_URL cannot be empty"},
		{"empty client id", func(c *entities.Config) { c.QWENOAuthClientID = "" }, "QWEN_OAUTH_CLIENT_ID cannot be empty"},
		{"empty qwen dir", func(c *entities.Config) { c.QWENDir = "" }, "QWEN_DIR cannot be empty"},
		{"empty log level", func(c *entities.Config) { c.LogLevel = "" }, "LOG_LEVEL cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &entities.Config{
				ServerPort:                 8080,
				QWENOAuthBaseURL:           "https://chat.qwen.ai",
				QWENOAuthClientID:          "test-client-id",
				QWENOAuthDeviceAuthURL:     "https://chat.qwen.ai/api/v1/oauth2/device/code",
				QWENDir:                    ".qwen",
				TokenRefreshBuffer:         5 * time.Minute,
				ShutdownTimeout:            30 * time.Second,
				LogLevel:                   "info",
				RateLimitRequestsPerSecond: 10,
				RateLimitBurst:             20,
				APIBaseURL:                 "https://portal.qwen.ai/v1",
			}

			tt.setup(config)

			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.error)
		})
	}
}

func TestValidateConfig_InvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*entities.Config)
		error string
	}{
		{"negative token refresh buffer", func(c *entities.Config) { c.TokenRefreshBuffer = -1 * time.Minute }, "TOKEN_REFRESH_BUFFER must be non-negative"},
		{"negative shutdown timeout", func(c *entities.Config) { c.ShutdownTimeout = -1 * time.Second }, "SHUTDOWN_TIMEOUT must be non-negative"},
		{"zero rate limit rps", func(c *entities.Config) { c.RateLimitRequestsPerSecond = 0 }, "RATE_LIMIT_REQUESTS_PER_SECOND must be positive"},
		{"negative rate limit burst", func(c *entities.Config) { c.RateLimitBurst = -1 }, "RATE_LIMIT_BURST must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &entities.Config{
				ServerPort:                 8080,
				QWENOAuthBaseURL:           "https://chat.qwen.ai",
				QWENOAuthClientID:          "test-client-id",
				QWENOAuthDeviceAuthURL:     "https://chat.qwen.ai/api/v1/oauth2/device/code",
				QWENDir:                    ".qwen",
				TokenRefreshBuffer:         5 * time.Minute,
				ShutdownTimeout:            30 * time.Second,
				LogLevel:                   "info",
				RateLimitRequestsPerSecond: 10,
				RateLimitBurst:             20,
				APIBaseURL:                 "https://portal.qwen.ai/v1",
			}

			tt.setup(config)

			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.error)
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	// Clear environment
	clearEnvVars()
	defer clearEnvVars()

	// Test getEnvWithDefault
	assert.Equal(t, "default", getEnvWithDefault("NONEXISTENT", "default"))
	os.Setenv("TEST_VAR", "value")
	assert.Equal(t, "value", getEnvWithDefault("TEST_VAR", "default"))

	// Test getEnvIntWithDefault
	assert.Equal(t, 42, getEnvIntWithDefault("NONEXISTENT", 42))
	os.Setenv("TEST_INT", "123")
	assert.Equal(t, 123, getEnvIntWithDefault("TEST_INT", 42))
	os.Setenv("TEST_INT", "invalid")
	assert.Equal(t, 42, getEnvIntWithDefault("TEST_INT", 42))

	// Test getEnvDurationWithDefault
	assert.Equal(t, 30*time.Second, getEnvDurationWithDefault("NONEXISTENT", 30*time.Second))
	os.Setenv("TEST_DURATION", "60s")
	assert.Equal(t, 60*time.Second, getEnvDurationWithDefault("TEST_DURATION", 30*time.Second))
	os.Setenv("TEST_DURATION", "invalid")
	assert.Equal(t, 30*time.Second, getEnvDurationWithDefault("TEST_DURATION", 30*time.Second))

	// Test getEnvBoolWithDefault
	assert.True(t, getEnvBoolWithDefault("NONEXISTENT", true))
	assert.False(t, getEnvBoolWithDefault("NONEXISTENT", false))
	os.Setenv("TEST_BOOL", "true")
	assert.True(t, getEnvBoolWithDefault("TEST_BOOL", false))
	os.Setenv("TEST_BOOL", "false")
	assert.False(t, getEnvBoolWithDefault("TEST_BOOL", true))
	os.Setenv("TEST_BOOL", "invalid")
	assert.True(t, getEnvBoolWithDefault("TEST_BOOL", true))

	// Test getEnvSliceWithDefault
	assert.Equal(t, []string{"default"}, getEnvSliceWithDefault("NONEXISTENT", []string{"default"}))
	os.Setenv("TEST_SLICE", "value")
	assert.Equal(t, []string{"value"}, getEnvSliceWithDefault("TEST_SLICE", []string{"default"}))
	os.Setenv("TEST_SLICE", "a,b,c")
	assert.Equal(t, []string{"a", "b", "c"}, getEnvSliceWithDefault("TEST_SLICE", []string{"default"}))
	os.Setenv("TEST_SLICE", "  a  ,  b  ,  c  ")
	assert.Equal(t, []string{"a", "b", "c"}, getEnvSliceWithDefault("TEST_SLICE", []string{"default"}))
	os.Setenv("TEST_SLICE", ",a,,b,")
	assert.Equal(t, []string{"a", "b"}, getEnvSliceWithDefault("TEST_SLICE", []string{"default"}))
}

// clearEnvVars clears all test environment variables
func clearEnvVars() {
	envVars := []string{
		"SERVER_PORT", "SERVER_HOST", "READ_TIMEOUT", "WRITE_TIMEOUT",
		"QWEN_OAUTH_BASE_URL", "QWEN_OAUTH_CLIENT_ID", "QWEN_DIR",
		"TOKEN_REFRESH_BUFFER", "SHUTDOWN_TIMEOUT", "DEBUG_MODE",
		"LOG_LEVEL", "LOG_FORMAT", "RATE_LIMIT_RPS", "RATE_LIMIT_BURST",
		"API_BASE_URL", "TRUSTED_PROXIES",
		"ENABLE_TLS", "TLS_CERT_FILE", "TLS_KEY_FILE",
		"TEST_VAR", "TEST_INT", "TEST_DURATION", "TEST_BOOL", "TEST_SLICE",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}
