package outbox_poller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/outbox"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
)

// LedgerPublisher publishes outbox messages to ledger
type LedgerPublisher interface {
	PublishToLedger(ctx context.Context, message *outbox.Message) error
}

// LedgerPublisherImpl implements LedgerPublisher
type LedgerPublisherImpl struct {
	outboxRepo outbox.Repository
	ledgerRepo ledger.Repository
	logger     *slog.Logger
}

// NewLedgerPublisher creates a new publisher
func NewLedgerPublisher(
	outboxRepo outbox.Repository,
	ledgerRepo ledger.Repository,
	logger *slog.Logger,
) LedgerPublisher {
	return &LedgerPublisherImpl{
		outboxRepo: outboxRepo,
		ledgerRepo: ledgerRepo,
		logger:     logger,
	}
}

// PublishToLedger processes and publishes a message to ledger
func (p *LedgerPublisherImpl) PublishToLedger(ctx context.Context, message *outbox.Message) error {
	var entryToPublish ledger.Entry
	if err := json.Unmarshal(message.Payload, &entryToPublish); err != nil {
		p.logger.Error("Failed to unmarshal ledger entry from outbox payload",
			"outbox_id", message.ID, "transaction_id", message.TransactionID, "error", err,
		)
		if updateErr := p.outboxRepo.UpdateStatus(ctx, message.ID, shared.OutboxStatusFailedToPublish); updateErr != nil {
			p.logger.Error("Also failed to update outbox status to FAILED_TO_PUBLISH after unmarshal error", "outbox_id", message.ID, "update_error", updateErr)
		}
		return fmt.Errorf("unmarshal payload for outbox %d failed: %w", message.ID, err)
	}

	// Add correlation ID to logger
	logger := p.logger
	if entryToPublish.CorrelationID != "" {
		logger = p.logger.With("correlation_id", entryToPublish.CorrelationID)
	}

	logger.Info("Attempting to publish outbox message to ledger", "outbox_id", message.ID, "transaction_id", message.TransactionID)

	entryToPublish.Status = shared.TransactionStatusCompleted
	now := time.Now().UTC()
	entryToPublish.ProcessedAt = &now

	existingLedger, err := p.ledgerRepo.GetByTransactionID(ctx, entryToPublish.TransactionID)
	if err != nil && !errors.Is(err, ledger.ErrEntryNotFound{}) {
		logger.Error("Failed to check existing ledger entry before publishing", "transaction_id", entryToPublish.TransactionID, "error", err)
		return fmt.Errorf("failed to check existing ledger entry %s: %w", entryToPublish.TransactionID, err)
	}

	if existingLedger != nil {
		if existingLedger.Status == shared.TransactionStatusCompleted {
			logger.Info("Ledger entry already COMPLETED", "transaction_id", entryToPublish.TransactionID)
		} else {
			// Update existing entry status
			err = p.ledgerRepo.UpdateStatus(ctx, entryToPublish.TransactionID, shared.TransactionStatusCompleted, "") // Empty reason for success
			if err != nil {
				logger.Error("Failed to update existing ledger entry to COMPLETED", "transaction_id", entryToPublish.TransactionID, "error", err)
				return fmt.Errorf("failed to update ledger entry %s to COMPLETED: %w", entryToPublish.TransactionID, err)
			}
			logger.Info("Updated existing ledger entry to COMPLETED", "transaction_id", entryToPublish.TransactionID)
		}
	} else {
		// Create new ledger entry
		err = p.ledgerRepo.Create(ctx, &entryToPublish) // entryToPublish already has status=COMPLETED and ProcessedAt set
		if err != nil {
			logger.Error("Failed to create ledger entry in MongoDB", "transaction_id", entryToPublish.TransactionID, "error", err)
			return fmt.Errorf("failed to create ledger entry %s: %w", entryToPublish.TransactionID, err)
		}
		logger.Info("Successfully created ledger entry in MongoDB", "transaction_id", entryToPublish.TransactionID)
	}

	// Mark outbox message as processed
	if err := p.outboxRepo.UpdateStatus(ctx, message.ID, shared.OutboxStatusProcessed); err != nil {
		logger.Error("Failed to update outbox message status to PROCESSED",
			"outbox_id", message.ID, "transaction_id", message.TransactionID, "error", err,
		)
		return fmt.Errorf("ledger write for %s OK, but failed to mark outbox %d as PROCESSED: %w", message.TransactionID, message.ID, err)
	}

	logger.Info("Outbox message successfully processed and marked as PROCESSED", "outbox_id", message.ID, "transaction_id", message.TransactionID)
	return nil
}
