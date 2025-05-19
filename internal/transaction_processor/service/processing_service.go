package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/innoscripta-banking-ledger/internal/platform/persistence"
	"github.com/jackc/pgx/v5"
)

type ProcessingServiceImpl struct {
	pgDB            *persistence.PostgresDB
	validator       TransactionValidator
	accountManager  AccountManager
	outboxManager   OutboxManager
	failureRecorder FailureRecorder
	logger          *slog.Logger
}

func NewProcessingService(
	pgDB *persistence.PostgresDB,
	validator TransactionValidator,
	accountManager AccountManager,
	outboxManager OutboxManager,
	failureRecorder FailureRecorder,
	logger *slog.Logger,
) ProcessingService {
	return &ProcessingServiceImpl{
		pgDB:            pgDB,
		validator:       validator,
		accountManager:  accountManager,
		outboxManager:   outboxManager,
		failureRecorder: failureRecorder,
		logger:          logger,
	}
}

// ProcessTransaction handles the core logic for processing a transaction.
func (s *ProcessingServiceImpl) ProcessTransaction(ctx context.Context, request *shared.TransactionRequest) error {
	logger := s.logger
	if request.CorrelationID != "" {
		logger = s.logger.With("correlation_id", request.CorrelationID)
	}

	logger.Info("Processing transaction", "transaction_id", request.TransactionID.String(), "account_id", request.AccountID.String())

	// 1. Validate the transaction
	if err := s.validator.Validate(ctx, request); err != nil {
		logger.Error("Transaction validation failed", "transaction_id", request.TransactionID.String(), "error", err)

		// Record the failure based on the specific error
		var failureReason string
		if errors.Is(err, shared.ErrInvalidTransactionType) {
			failureReason = string(shared.FailureReasonUnknownError)
		} else {
			failureReason = string(shared.FailureReasonInvalidAmount)
		}

		if recordErr := s.failureRecorder.RecordFailure(ctx, request, failureReason); recordErr != nil {
			logger.Error("Failed to record transaction failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
		}

		return nil // Return nil to Kafka consumer to acknowledge the message
	}

	// 2. Check idempotency
	skip, err := s.validator.CheckIdempotency(ctx, request)
	if err != nil {
		return err // Let Kafka retry
	}
	if skip {
		return nil // Already processed, return success
	}

	// 3. Begin database transaction
	var tx pgx.Tx
	tx, err = s.pgDB.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin database transaction", "transaction_id", request.TransactionID.String(), "error", err)
		return fmt.Errorf("failed to begin DB transaction for %s: %w", request.TransactionID.String(), err)
	}
	defer func() {
		if p := recover(); p != nil {
			logger.Error("Panic recovered, rolling back transaction", "panic", p, "transaction_id", request.TransactionID.String())
			_ = tx.Rollback(ctx)
			panic(p) // Re-panic
		} else if err != nil {
			logger.Error("Error occurred, rolling back transaction", "error", err, "transaction_id", request.TransactionID.String())
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				logger.Error("Failed to rollback transaction after error", "rollback_error", rbErr, "original_error", err, "transaction_id", request.TransactionID.String())
			}
		}
	}()

	// 4. Lock and update account
	updatedAccount, err := s.accountManager.LockAndUpdateAccount(ctx, tx, request)
	if err != nil {
		// Handle specific business errors
		if errors.Is(err, account.ErrAccountNotFound{AccountID: request.AccountID}) {
			if recordErr := s.failureRecorder.RecordFailure(ctx, request, string(shared.FailureReasonAccountNotFound)); recordErr != nil {
				logger.Error("Failed to record account not found failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
			}
			return nil // Return nil to Kafka consumer
		} else if errors.Is(err, shared.ErrInvalidCurrency) {
			failureReasonStr := fmt.Sprintf(string(shared.FailureReasonCurrencyMismatchFormat), request.Currency, "account_currency")
			if recordErr := s.failureRecorder.RecordFailure(ctx, request, failureReasonStr); recordErr != nil {
				logger.Error("Failed to record currency mismatch failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
			}
			return nil // Return nil to Kafka consumer
		} else if errors.Is(err, account.ErrInvalidAmount) {
			if recordErr := s.failureRecorder.RecordFailure(ctx, request, string(shared.FailureReasonInvalidAmount)); recordErr != nil {
				logger.Error("Failed to record invalid amount failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
			}
			return nil // Return nil to Kafka consumer
		} else if errors.Is(err, account.ErrInsufficientFunds) {
			if recordErr := s.failureRecorder.RecordFailure(ctx, request, string(shared.FailureReasonInsufficientFunds)); recordErr != nil {
				logger.Error("Failed to record insufficient funds failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
			}
			return nil // Return nil to Kafka consumer
		}

		// For other errors, let them propagate for retry
		return err
	}

	// 5. Create outbox entry
	if err = s.outboxManager.CreateOutboxEntry(ctx, tx, request, updatedAccount); err != nil {
		return err // Let the defer handle rollback
	}

	// 6. Commit transaction
	if err = tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit database transaction",
			"req_id", request.TransactionID.String(),
			"acc_id", request.AccountID.String(),
			"error", err,
		)
		return fmt.Errorf("failed to commit DB transaction for tx %s: %w", request.TransactionID.String(), err)
	}

	logger.Info("Database transaction committed successfully", "req_id", request.TransactionID.String(), "acc_id", request.AccountID.String())
	return nil // SUCCESS!
}
