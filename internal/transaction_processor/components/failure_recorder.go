package components

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/service"
)

type FailureRecorderImpl struct {
	ledgerRepo ledger.Repository
	logger     *slog.Logger
}

func NewFailureRecorder(ledgerRepo ledger.Repository, logger *slog.Logger) service.FailureRecorder {
	return &FailureRecorderImpl{
		ledgerRepo: ledgerRepo,
		logger:     logger,
	}
}

// RecordFailure records a failed transaction in the ledger
func (r *FailureRecorderImpl) RecordFailure(ctx context.Context, request *shared.TransactionRequest, failureReason string) error {
	logger := r.logger
	if request.CorrelationID != "" {
		logger = r.logger.With("correlation_id", request.CorrelationID)
	}

	logger.Info("Recording failed transaction", "transaction_id", request.TransactionID.String(), "reason", failureReason)

	now := time.Now()
	entry := &ledger.Entry{
		TransactionID:  request.TransactionID,
		AccountID:      request.AccountID,
		Type:           request.Type,
		Amount:         request.Amount,
		Currency:       request.Currency,
		IdempotencyKey: request.IdempotencyKey,
		CorrelationID:  request.CorrelationID,
		Status:         shared.TransactionStatusFailed,
		FailureReason:  failureReason,
		CreatedAt:      request.Timestamp,
		ProcessedAt:    &now,
	}

	existingEntry, err := r.ledgerRepo.GetByTransactionID(ctx, request.TransactionID)
	if err != nil && !errors.Is(err, ledger.ErrEntryNotFound{}) {
		logger.Error("Failed to get existing ledger entry for failed transaction", "transaction_id", request.TransactionID.String(), "error", err)
	}

	if existingEntry != nil {
		if existingEntry.Status != shared.TransactionStatusFailed {
			logger.Info("Updating existing ledger entry to FAILED", "transaction_id", request.TransactionID.String(), "ledger_tx_id", existingEntry.TransactionID)
			updateErr := r.ledgerRepo.UpdateStatus(ctx, request.TransactionID, shared.TransactionStatusFailed, failureReason)
			if updateErr != nil {
				logger.Error("Failed to update ledger entry to FAILED", "transaction_id", request.TransactionID.String(), "error", updateErr)
				return updateErr
			}
			logger.Info("Successfully updated ledger entry to FAILED", "transaction_id", request.TransactionID.String())
			return nil
		}
		logger.Info("Ledger entry already marked as FAILED", "transaction_id", request.TransactionID.String())
		return nil
	}

	logger.Info("Creating new FAILED ledger entry", "transaction_id", request.TransactionID.String())
	createErr := r.ledgerRepo.Create(ctx, entry)
	if createErr != nil {
		logger.Error("Failed to create FAILED ledger entry", "transaction_id", request.TransactionID.String(), "error", createErr)
		return createErr
	}
	logger.Info("Successfully created FAILED ledger entry", "transaction_id", request.TransactionID.String())
	return nil
}
