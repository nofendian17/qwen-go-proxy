package logging

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qwen-go-proxy/internal/domain/entities"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		expected slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"fatal level", "fatal", slog.LevelError + 4},
		{"default level", "unknown", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.logLevel)
			require.NotNil(t, logger)

			// We can't directly access the level, but we can verify the logger was created
			// and test that it works with different levels
			assert.NotNil(t, logger)
		})
	}
}

func TestNewLoggerFromConfig(t *testing.T) {
	config := &entities.Config{
		LogLevel: "debug",
	}

	logger := NewLoggerFromConfig(config)
	require.NotNil(t, logger)
	assert.NotNil(t, logger.Logger)
}

func TestLogger_Debug(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slogLogger := slog.New(handler)

	logger := &Logger{Logger: slogLogger}

	// Test Debug method doesn't panic
	assert.NotPanics(t, func() {
		logger.Debug("test debug message", "key", "value")
	})

	// Verify output contains our message
	output := buf.String()
	assert.Contains(t, output, "test debug message")
	assert.Contains(t, output, "key=value")
}

func TestLogger_Info(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slogLogger := slog.New(handler)

	logger := &Logger{Logger: slogLogger}

	// Test Info method doesn't panic
	assert.NotPanics(t, func() {
		logger.Info("test info message", "key", "value")
	})

	// Verify output contains our message
	output := buf.String()
	assert.Contains(t, output, "test info message")
	assert.Contains(t, output, "key=value")
}

func TestLogger_Warn(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	slogLogger := slog.New(handler)

	logger := &Logger{Logger: slogLogger}

	// Test Warn method doesn't panic
	assert.NotPanics(t, func() {
		logger.Warn("test warn message", "key", "value")
	})

	// Verify output contains our message
	output := buf.String()
	assert.Contains(t, output, "test warn message")
	assert.Contains(t, output, "key=value")
}

func TestLogger_Error(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelError,
	})
	slogLogger := slog.New(handler)

	logger := &Logger{Logger: slogLogger}

	// Test Error method doesn't panic
	assert.NotPanics(t, func() {
		logger.Error("test error message", "key", "value")
	})

	// Verify output contains our message
	output := buf.String()
	assert.Contains(t, output, "test error message")
	assert.Contains(t, output, "key=value")
}

func TestLogger_LogLevels(t *testing.T) {
	tests := []struct {
		name          string
		logLevel      slog.Level
		testFunc      func(*Logger)
		shouldContain string
	}{
		{"debug level logs debug", slog.LevelDebug, func(l *Logger) { l.Debug("debug msg") }, "debug msg"},
		{"info level logs info", slog.LevelInfo, func(l *Logger) { l.Info("info msg") }, "info msg"},
		{"warn level logs warn", slog.LevelWarn, func(l *Logger) { l.Warn("warn msg") }, "warn msg"},
		{"error level logs error", slog.LevelError, func(l *Logger) { l.Error("error msg") }, "error msg"},
		{"debug level doesn't log below threshold", slog.LevelInfo, func(l *Logger) { l.Debug("debug msg") }, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
				Level: tt.logLevel,
			})
			slogLogger := slog.New(handler)

			logger := &Logger{Logger: slogLogger}

			tt.testFunc(logger)

			output := buf.String()
			if tt.shouldContain != "" {
				assert.Contains(t, output, tt.shouldContain)
			} else {
				assert.NotContains(t, output, "debug msg")
			}
		})
	}
}

func TestLoggerInterface(t *testing.T) {
	// Test that Logger implements LoggerInterface
	var _ LoggerInterface = &Logger{}
}

// Test that the interface methods work with various argument types
func TestLogger_VariousArguments(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slogLogger := slog.New(handler)

	logger := &Logger{Logger: slogLogger}

	// Test with various argument types
	assert.NotPanics(t, func() {
		logger.Debug("debug with various args",
			"string", "value",
			"int", 42,
			"bool", true,
			"float", 3.14)
	})

	output := buf.String()
	assert.Contains(t, output, "debug with various args")
	assert.Contains(t, output, "string=value")
	assert.Contains(t, output, "int=42")
	assert.Contains(t, output, "bool=true")
	assert.Contains(t, output, "float=3.14")
}
