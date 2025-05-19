package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/innoscripta-banking-ledger/internal/config"
)

// NewLogger creates and configures a new slog.Logger
func NewLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
		// Add source code location to log output
		AddSource: level == slog.LevelDebug,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	logger.Info("logger initialized", "level", level)

	return logger
}
