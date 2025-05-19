package components

import (
	"log/slog"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/outbox"
	"github.com/innoscripta-banking-ledger/internal/platform/persistence"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/service"
)

// CreateProcessingService creates a new ProcessingService with all its dependencies.
func CreateProcessingService(
	pgDB *persistence.PostgresDB,
	accountRepo account.Repository,
	outboxRepo outbox.Repository,
	ledgerRepo ledger.Repository,
	logger *slog.Logger,
	cfg *config.Config,
) service.ProcessingService {
	validator := NewTransactionValidator(ledgerRepo, logger)
	accountManager := NewAccountManager(accountRepo, logger)
	outboxManager := NewOutboxManager(outboxRepo, logger)
	failureRecorder := NewFailureRecorder(ledgerRepo, logger)

	baseService := service.NewProcessingService(
		pgDB,
		validator,
		accountManager,
		outboxManager,
		failureRecorder,
		logger,
	)

	workerPoolService, err := service.NewWorkerPoolProcessingService(
		baseService,
		service.WorkerPoolConfig{
			Size: cfg.WorkerPool.Size,
		},
		logger.With("component", "worker_pool"),
	)

	if err != nil {
		logger.Error("Failed to create worker pool service, falling back to base service", "error", err)
		return baseService
	}

	logger.Info("Created worker pool processing service", "pool_size", cfg.WorkerPool.Size)
	return workerPoolService
}
