package components

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/outbox"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/service"
	"github.com/jackc/pgx/v5"
)

type OutboxManagerImpl struct {
	outboxRepo outbox.Repository
	logger     *slog.Logger
}

func NewOutboxManager(outboxRepo outbox.Repository, logger *slog.Logger) service.OutboxManager {
	return &OutboxManagerImpl{
		outboxRepo: outboxRepo,
		logger:     logger,
	}
}

// CreateOutboxEntry creates an outbox entry for a processed transaction
func (m *OutboxManagerImpl) CreateOutboxEntry(ctx context.Context, tx pgx.Tx, request *shared.TransactionRequest, updatedAccount *account.Account) error {
	logger := m.logger
	if request.CorrelationID != "" {
		logger = m.logger.With("correlation_id", request.CorrelationID)
	}

	outboxRepoTx := m.outboxRepo.WithTx(tx)

	ledgerEntryForOutbox := &ledger.Entry{
		TransactionID:  request.TransactionID,
		AccountID:      request.AccountID,
		Type:           request.Type,
		Amount:         request.Amount,
		Currency:       request.Currency,
		IdempotencyKey: request.IdempotencyKey,
		CorrelationID:  request.CorrelationID,
		Status:         shared.TransactionStatusProcessing,
		CreatedAt:      request.Timestamp,
		// ProcessedAt is set by the poller
	}

	outboxMessage, err := outbox.NewMessage(ledgerEntryForOutbox)
	if err != nil {
		logger.Error("Failed to create new outbox message (marshal payload)",
			"req_id", request.TransactionID.String(),
			"error", err,
		)
		return fmt.Errorf("failed to create outbox message payload for tx %s: %w", request.TransactionID.String(), err)
	}

	if err = outboxRepoTx.Create(ctx, outboxMessage); err != nil {
		logger.Error("Failed to create outbox message",
			"req_id", request.TransactionID.String(),
			"acc_id", request.AccountID.String(),
			"error", err,
		)
		return fmt.Errorf("failed to create outbox message for tx %s: %w", request.TransactionID.String(), err)
	}
	logger.Info("Outbox message created successfully",
		"req_id", request.TransactionID.String(),
		"outbox_id", outboxMessage.ID,
	)

	return nil
}
