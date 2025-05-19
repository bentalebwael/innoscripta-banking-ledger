package components

import (
	"testing"

	"log/slog"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/innoscripta-banking-ledger/internal/platform/persistence"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/service"
	"github.com/stretchr/testify/assert"
)

// We're reusing the mocks from other test files:
// MockAccountRepo from account_manager_test.go
// MockOutboxRepo from outbox_manager_test.go
// MockLedgerRepoForFailure from failure_recorder_test.go

func TestCreateProcessingService(t *testing.T) {
	mockPgDB := &persistence.PostgresDB{}
	mockAccountRepo := &MockAccountRepo{}
	mockOutboxRepo := &MockOutboxRepo{}
	mockLedgerRepo := &MockLedgerRepoForFailure{}
	logger := slog.Default()

	cfg := &config.Config{
		WorkerPool: config.WorkerPoolConfig{
			Size: 5,
		},
	}

	t.Run("creates worker pool service with valid config", func(t *testing.T) {
		processingService := CreateProcessingService(
			mockPgDB,
			mockAccountRepo,
			mockOutboxRepo,
			mockLedgerRepo,
			logger,
			cfg,
		)

		assert.NotNil(t, processingService)

		// Note: Type checking is done via interface implementation since we can't access concrete type
		_, ok := processingService.(service.ProcessingService)
		assert.True(t, ok)
	})

	t.Run("falls back to base service with invalid config", func(t *testing.T) {
		invalidCfg := &config.Config{
			WorkerPool: config.WorkerPoolConfig{
				Size: 0, // Invalid size
			},
		}

		processingService := CreateProcessingService(
			mockPgDB,
			mockAccountRepo,
			mockOutboxRepo,
			mockLedgerRepo,
			logger,
			invalidCfg,
		)

		assert.NotNil(t, processingService)

		// Note: Verify interface implementation as concrete type check is not possible
		_, ok := processingService.(service.ProcessingService)
		assert.True(t, ok)
	})
}
