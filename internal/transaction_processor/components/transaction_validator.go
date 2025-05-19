package components

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/service"
)

type TransactionValidatorImpl struct {
	ledgerRepo ledger.Repository
	logger     *slog.Logger
}

func NewTransactionValidator(ledgerRepo ledger.Repository, logger *slog.Logger) service.TransactionValidator {
	return &TransactionValidatorImpl{
		ledgerRepo: ledgerRepo,
		logger:     logger,
	}
}

// Validate checks transaction request validity
func (v *TransactionValidatorImpl) Validate(ctx context.Context, request *shared.TransactionRequest) error {
	logger := v.logger
	if request.CorrelationID != "" {
		logger = v.logger.With("correlation_id", request.CorrelationID)
	}

	if request.Type != shared.TransactionTypeDeposit && request.Type != shared.TransactionTypeWithdrawal {
		logger.Error("Unknown transaction type", "req_id", request.TransactionID.String(), "type", request.Type)
		return shared.ErrInvalidTransactionType
	}

	if request.Amount <= 0 {
		logger.Error("Invalid amount", "req_id", request.TransactionID.String(), "amount", request.Amount)
		return fmt.Errorf("amount must be positive: %d", request.Amount)
	}

	return nil
}

// CheckIdempotency checks if transaction was already processed
func (v *TransactionValidatorImpl) CheckIdempotency(ctx context.Context, request *shared.TransactionRequest) (bool, error) {
	logger := v.logger
	if request.CorrelationID != "" {
		logger = v.logger.With("correlation_id", request.CorrelationID)
	}

	existingLedgerEntry, err := v.ledgerRepo.GetByTransactionID(ctx, request.TransactionID)
	if err != nil && !errors.Is(err, ledger.ErrEntryNotFound{}) {
		logger.Error("Failed to check ledger for idempotency", "transaction_id", request.TransactionID.String(), "error", err)
		return false, fmt.Errorf("idempotency check failed for transaction %s: %w", request.TransactionID.String(), err)
	}

	if existingLedgerEntry != nil {
		if existingLedgerEntry.Status == shared.TransactionStatusCompleted || existingLedgerEntry.Status == shared.TransactionStatusFailed {
			logger.Info("Transaction already processed (idempotency)", "transaction_id", request.TransactionID.String(), "status", existingLedgerEntry.Status)
			return true, nil // Skip processing
		}
		logger.Info("Transaction found in ledger with non-terminal status, proceeding", "transaction_id", request.TransactionID.String(), "status", existingLedgerEntry.Status)
	}

	return false, nil // Continue processing
}
