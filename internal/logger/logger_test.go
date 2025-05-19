package logger

import (
	"context"
	"log/slog"
	"testing"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	testCases := []struct {
		name              string
		logLevel          string
		expectedSlogLevel slog.Level
		expectAddSource   bool
	}{
		{"DebugLevel", "debug", slog.LevelDebug, true},
		{"InfoLevel", "info", slog.LevelInfo, false},
		{"WarnLevel", "warn", slog.LevelWarn, false},
		{"ErrorLevel", "error", slog.LevelError, false},
		{"DefaultToInfo", "unknown", slog.LevelInfo, false},
		{"EmptyToInfo", "", slog.LevelInfo, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Logging: config.LoggingConfig{
					Level: tc.logLevel,
				},
			}

			// Test logger properties since we can't capture initialization output

			logger := NewLogger(cfg)
			require.NotNil(t, logger)

			assert.True(t, logger.Enabled(context.Background(), tc.expectedSlogLevel), "Logger should be enabled for level "+tc.expectedSlogLevel.String())

			// Verify level cascade behavior
			if tc.expectedSlogLevel == slog.LevelDebug {
				assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo), "Logger set to Debug should also enable Info")
			}

			// AddSource behavior can only be verified indirectly through debug log output
			// We rely on NewLogger setting it based on level == slog.LevelDebug
		})
	}
}
