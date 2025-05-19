package ledger

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
)

// Repository manages ledger entry persistence with pagination support
type Repository interface {
	Create(ctx context.Context, entry *Entry) error
	GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*Entry, error)
	GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*Entry, error)
	GetByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*Entry, error)
	CountByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error)
	UpdateStatus(ctx context.Context, transactionID uuid.UUID, status shared.TransactionStatus, reason string) error
	GetByTimeRange(ctx context.Context, startTime, endTime time.Time, limit, offset int) ([]*Entry, error)
}

// ErrEntryNotFound indicates missing ledger entry
type ErrEntryNotFound struct {
	TransactionID uuid.UUID
}

func (e ErrEntryNotFound) Error() string {
	return "ledger entry not found: " + e.TransactionID.String()
}

// Is implements the errors.Is interface for ErrEntryNotFound
func (e ErrEntryNotFound) Is(target error) bool {
	t, ok := target.(ErrEntryNotFound)
	if !ok {
		return false
	}
	// If the target TransactionID is empty, consider it a match for any ErrEntryNotFound
	if t.TransactionID == uuid.Nil {
		return true
	}
	// Otherwise, match on TransactionID
	return e.TransactionID == t.TransactionID
}

// ErrDuplicateEntry indicates transaction uniqueness violation
type ErrDuplicateEntry struct {
	TransactionID uuid.UUID
}

func (e ErrDuplicateEntry) Error() string {
	return "duplicate ledger entry: " + e.TransactionID.String()
}

// Is implements the errors.Is interface for ErrDuplicateEntry
func (e ErrDuplicateEntry) Is(target error) bool {
	t, ok := target.(ErrDuplicateEntry)
	if !ok {
		return false
	}
	// If the target TransactionID is empty, consider it a match for any ErrDuplicateEntry
	if t.TransactionID == uuid.Nil {
		return true
	}
	// Otherwise, match on TransactionID
	return e.TransactionID == t.TransactionID
}
