package logging

import (
	"log/slog"
	"os"

	"qwen-go-proxy/internal/domain/entities"
)

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
