package outbox_poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/outbox"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
)

// Poller processes pending outbox messages
type Poller struct {
	outboxRepo       outbox.Repository
	ledgerPublisher  LedgerPublisher
	logger           *slog.Logger
	pollInterval     time.Duration
	batchSize        int
	maxRetryAttempts int
}

func NewPoller(
	cfg *config.OutboxConfig,
	outboxRepo outbox.Repository,
	ledgerPublisher LedgerPublisher,
	logger *slog.Logger,
) *Poller {
	return &Poller{
		outboxRepo:       outboxRepo,
		ledgerPublisher:  ledgerPublisher,
		logger:           logger,
		pollInterval:     cfg.PollingInterval,
		batchSize:        cfg.BatchSize,
		maxRetryAttempts: cfg.MaxRetryAttempts,
	}
}

// Start begins polling until context is canceled
func (p *Poller) Start(ctx context.Context) {
	p.logger.Info("Starting Outbox Poller",
		"poll_interval", p.pollInterval.String(),
		"batch_size", p.batchSize,
		"max_retry_attempts", p.maxRetryAttempts,
	)
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Outbox Poller stopping due to context cancellation.")
			return
		case <-ticker.C:
			p.logger.Debug("Outbox Poller tick: processing pending messages")
			if err := p.processPendingMessages(ctx); err != nil {
				p.logger.Error("Error during batch processing of pending outbox messages", "error", err)
			}
		}
	}
}

func (p *Poller) processPendingMessages(ctx context.Context) error {
	messages, err := p.outboxRepo.GetPending(ctx, p.batchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending outbox messages: %w", err)
	}

	if len(messages) == 0 {
		p.logger.Debug("No pending outbox messages found.")
		return nil
	}

	p.logger.Info("Fetched pending outbox messages", "count", len(messages))

	for _, msg := range messages {
		correlationID := ""
		var entry ledger.Entry
		if err := json.Unmarshal(msg.Payload, &entry); err == nil && entry.CorrelationID != "" {
			correlationID = entry.CorrelationID
		}

		logger := p.logger
		if correlationID != "" {
			logger = p.logger.With("correlation_id", correlationID)
		}

		err := p.ledgerPublisher.PublishToLedger(ctx, msg)
		if err != nil {
			logger.Error("Failed to publish outbox message to ledger",
				"outbox_id", msg.ID, "transaction_id", msg.TransactionID, "current_attempts", msg.Attempts, "error", err,
			)

			// Increment attempt count
			if errInc := p.outboxRepo.IncrementAttempts(ctx, msg.ID); errInc != nil {
				logger.Error("Failed to increment attempts for outbox message", "outbox_id", msg.ID, "error", errInc)
				// Continue to next message if increment fails
				continue
			}

			if msg.Attempts+1 >= p.maxRetryAttempts {
				logger.Warn("Max retry attempts reached for outbox message, marking as FAILED_TO_PUBLISH",
					"outbox_id", msg.ID, "transaction_id", msg.TransactionID, "attempts_made", msg.Attempts+1,
				)
				if errUpdate := p.outboxRepo.UpdateStatus(ctx, msg.ID, shared.OutboxStatusFailedToPublish); errUpdate != nil {
					logger.Error("Failed to update outbox status to FAILED_TO_PUBLISH after max retries", "outbox_id", msg.ID, "error", errUpdate)
				}
			}
			continue
		}
		logger.Info("Successfully processed and published outbox message via LedgerPublisher", "outbox_id", msg.ID, "transaction_id", msg.TransactionID)
	}
	return nil
}
