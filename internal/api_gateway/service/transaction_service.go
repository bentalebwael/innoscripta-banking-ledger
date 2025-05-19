package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/innoscripta-banking-ledger/internal/platform/messaging/producers"
)

// TransactionServiceImpl implements the TransactionService interface
type TransactionServiceImpl struct {
	ledgerRepo ledger.Repository
	producer   producers.MessagePublisher // Changed to specific interface
	logger     *slog.Logger
}

// NewTransactionService creates a new transaction service
func NewTransactionService(logger *slog.Logger, ledgerRepo ledger.Repository, producer producers.MessagePublisher) TransactionService { // Changed to specific interface
	return &TransactionServiceImpl{
		ledgerRepo: ledgerRepo,
		producer:   producer,
		logger:     logger,
	}
}

// CreateTransaction initiates a new transaction, supporting idempotency via idempotencyKey.
// Returns transaction ID, existing ledger entry (if found via idempotencyKey), and any error
func (s *TransactionServiceImpl) CreateTransaction(ctx context.Context, transactionRequest *shared.TransactionRequest) (string, *ledger.Entry, error) {
	idempotencyKey := transactionRequest.IdempotencyKey

	if idempotencyKey != "" {
		existingEntry, err := s.ledgerRepo.GetByIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			s.logger.Error("Failed to check for existing transaction with idempotency key",
				"idempotency_key", idempotencyKey,
				"error", err,
			)
			return "", nil, err
		}

		if existingEntry != nil {
			s.logger.Info("Found existing transaction with idempotency key",
				"idempotency_key", idempotencyKey,
				"transaction_id", existingEntry.TransactionID,
				"status", string(existingEntry.Status),
			)
			return existingEntry.TransactionID.String(), existingEntry, nil
		}
	}

	key := transactionRequest.TransactionID.String()
	if err := s.producer.Publish(ctx, key, transactionRequest); err != nil {
		s.logger.Error("Failed to publish transaction request",
			"account_id", transactionRequest.AccountID,
			"transaction_type", string(transactionRequest.Type),
			"amount", transactionRequest.Amount,
			"error", err,
		)
		return "", nil, err
	}

	s.logger.Info("Transaction request published",
		"transaction_id", transactionRequest.TransactionID,
		"account_id", transactionRequest.AccountID,
		"transaction_type", string(transactionRequest.Type),
		"amount", transactionRequest.Amount,
	)

	return transactionRequest.TransactionID.String(), nil, nil
}

// GetTransactionByID retrieves a transaction by its ID. Returns nil if not found
func (s *TransactionServiceImpl) GetTransactionByID(ctx context.Context, transactionID uuid.UUID) (*ledger.Entry, error) {
	res, err := s.ledgerRepo.GetByTransactionID(ctx, transactionID)
	if err != nil {
		var errEntryNotFound ledger.ErrEntryNotFound
		if errors.As(err, &errEntryNotFound) {
			s.logger.Info("Transaction not found", "transaction_id", transactionID.String())
			return nil, nil
		}
		s.logger.Error("Failed to get transaction by ID", "transaction_id", transactionID.String(), "error", err)
		return nil, err
	}
	return res, nil
}

// GetTransactionsByAccountID retrieves paginated list of transactions for an account
// Returns entries, total count, and any error
func (s *TransactionServiceImpl) GetTransactionsByAccountID(ctx context.Context, accountID uuid.UUID, page, perPage int) ([]*ledger.Entry, int64, error) {
	offset := (page - 1) * perPage

	entries, err := s.ledgerRepo.GetByAccountID(ctx, accountID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.ledgerRepo.CountByAccountID(ctx, accountID)
	if err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}
