package logging

import (
	"log/slog"
	"os"

	"qwen-go-proxy/internal/domain/entities"
)

// LoggerInterface defines the logging interface for dependency injection
type LoggerInterface interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// NewLogger initializes a structured logger with the specified level
func NewLogger(logLevel string) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "fatal":
		level = slog.LevelError + 4 // Custom level for fatal
	default:
		level = slog.LevelInfo // Default to info
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler)
}

// Logger is a wrapper around slog.Logger for dependency injection
type Logger struct {
	*slog.Logger
}

// NewLoggerFromConfig creates a logger from config
func NewLoggerFromConfig(config *entities.Config) *Logger {
	return &Logger{
		Logger: NewLogger(config.LogLevel),
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}
