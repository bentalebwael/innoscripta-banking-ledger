package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/outbox"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/innoscripta-banking-ledger/internal/platform/persistence"
	"github.com/jackc/pgx/v5"
)

// OutboxRepository implements the outbox.Repository interface for PostgreSQL
type OutboxRepository struct {
	querier persistence.Querier
	logger  *slog.Logger
}

// NewOutboxRepository creates a new PostgreSQL outbox repository
func NewOutboxRepository(logger *slog.Logger, db *persistence.PostgresDB) outbox.Repository {
	return &OutboxRepository{
		querier: db.Pool(),
		logger:  logger,
	}
}

// WithTx wraps the repository with a transaction for atomic operations.
// This ensures message creation is atomic with other database operations.
func (r *OutboxRepository) WithTx(tx pgx.Tx) outbox.Repository {
	return &OutboxRepository{
		querier: tx,
		logger:  r.logger,
	}
}

// Create stores a new outbox message in pending status.
// The message will be picked up by the outbox poller for processing.
func (r *OutboxRepository) Create(ctx context.Context, message *outbox.Message) error {
	query := `
		INSERT INTO transaction_outbox (transaction_id, account_id, payload, status, attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	err := r.querier.QueryRow(ctx, query,
		message.TransactionID,
		message.AccountID,
		message.Payload,
		message.Status,
		message.Attempts,
		message.CreatedAt,
	).Scan(&message.ID)

	if err != nil {
		r.logger.Error("Failed to create outbox message",
			"transaction_id", message.TransactionID.String(),
			"error", err,
		)
		return fmt.Errorf("failed to create outbox message: %w", err)
	}

	return nil
}

// GetPending retrieves a batch of pending outbox messages ordered by creation time.
// This is used by the outbox poller to process messages in FIFO order.
func (r *OutboxRepository) GetPending(ctx context.Context, limit int) ([]*outbox.Message, error) {
	query := `
		SELECT id, transaction_id, account_id, payload, status, attempts, created_at, last_attempt_at
		FROM transaction_outbox
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2
	`

	rows, err := r.querier.Query(ctx, query, shared.OutboxStatusPending, limit)
	if err != nil {
		r.logger.Error("Failed to get pending outbox messages", "error", err)
		return nil, fmt.Errorf("failed to get pending outbox messages: %w", err)
	}
	defer rows.Close()

	var messages []*outbox.Message
	for rows.Next() {
		var message outbox.Message
		err := rows.Scan(
			&message.ID,
			&message.TransactionID,
			&message.AccountID,
			&message.Payload,
			&message.Status,
			&message.Attempts,
			&message.CreatedAt,
			&message.LastAttemptAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan outbox message", "error", err)
			return nil, fmt.Errorf("failed to scan outbox message: %w", err)
		}
		messages = append(messages, &message)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating over outbox messages", "error", err)
		return nil, fmt.Errorf("error iterating over outbox messages: %w", err)
	}

	return messages, nil
}

// UpdateStatus updates the message status and last attempt timestamp.
// Returns ErrMessageNotFound if the message doesn't exist.
func (r *OutboxRepository) UpdateStatus(ctx context.Context, id int64, status shared.OutboxStatus) error {
	query := `
		UPDATE transaction_outbox
		SET status = $1, last_attempt_at = $2
		WHERE id = $3
	`

	result, err := r.querier.Exec(ctx, query, status, time.Now(), id)
	if err != nil {
		r.logger.Error("Failed to update outbox message status",
			"id", id,
			"status", string(status),
			"error", err,
		)
		return fmt.Errorf("failed to update outbox message status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return outbox.ErrMessageNotFound{ID: id}
	}

	return nil
}

// IncrementAttempts increments the retry counter and updates last attempt time.
// This is used for tracking failed processing attempts and implementing retry logic.
func (r *OutboxRepository) IncrementAttempts(ctx context.Context, id int64) error {
	query := `
		UPDATE transaction_outbox
		SET attempts = attempts + 1, last_attempt_at = $1
		WHERE id = $2
	`

	result, err := r.querier.Exec(ctx, query, time.Now(), id)
	if err != nil {
		r.logger.Error("Failed to increment outbox message attempts",
			"id", id,
			"error", err,
		)
		return fmt.Errorf("failed to increment outbox message attempts: %w", err)
	}

	if result.RowsAffected() == 0 {
		return outbox.ErrMessageNotFound{ID: id}
	}

	return nil
}

// Delete permanently removes a message from the outbox.
// This is typically called after successful message processing.
func (r *OutboxRepository) Delete(ctx context.Context, id int64) error {
	query := `
		DELETE FROM transaction_outbox
		WHERE id = $1
	`

	result, err := r.querier.Exec(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete outbox message",
			"id", id,
			"error", err,
		)
		return fmt.Errorf("failed to delete outbox message: %w", err)
	}

	if result.RowsAffected() == 0 {
		return outbox.ErrMessageNotFound{ID: id}
	}

	return nil
}

// GetByTransactionID retrieves a message by transaction ID for idempotency checking.
// Returns ErrMessageNotFound if no message exists for the given transaction.
func (r *OutboxRepository) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*outbox.Message, error) {
	query := `
		SELECT id, transaction_id, account_id, payload, status, attempts, created_at, last_attempt_at
		FROM transaction_outbox
		WHERE transaction_id = $1
	`

	var message outbox.Message
	err := r.querier.QueryRow(ctx, query, transactionID).Scan(
		&message.ID,
		&message.TransactionID,
		&message.AccountID,
		&message.Payload,
		&message.Status,
		&message.Attempts,
		&message.CreatedAt,
		&message.LastAttemptAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, outbox.ErrMessageNotFound{ID: 0}
		}
		r.logger.Error("Failed to get outbox message by transaction ID",
			"transaction_id", transactionID.String(),
			"error", err,
		)
		return nil, fmt.Errorf("failed to get outbox message by transaction ID: %w", err)
	}

	return &message, nil
}
