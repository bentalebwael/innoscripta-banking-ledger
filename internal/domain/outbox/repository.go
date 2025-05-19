package outbox

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/jackc/pgx/v5"
)

// Repository manages transactional outbox message persistence
type Repository interface {
	Create(ctx context.Context, message *Message) error
	GetPending(ctx context.Context, limit int) ([]*Message, error)
	UpdateStatus(ctx context.Context, id int64, status shared.OutboxStatus) error
	IncrementAttempts(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
	GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*Message, error)
	WithTx(tx pgx.Tx) Repository
}

// ErrMessageNotFound indicates missing outbox message
type ErrMessageNotFound struct {
	ID int64
}

func (e ErrMessageNotFound) Error() string {
	return "outbox message not found: " + strconv.FormatInt(e.ID, 10)
}

// ErrDuplicateMessage indicates transaction uniqueness violation
type ErrDuplicateMessage struct {
	TransactionID uuid.UUID
}

func (e ErrDuplicateMessage) Error() string {
	return "duplicate outbox message: " + e.TransactionID.String()
}
