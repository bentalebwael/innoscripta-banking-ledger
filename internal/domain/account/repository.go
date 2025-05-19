package account

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository defines account persistence operations
type Repository interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id uuid.UUID) (*Account, error)
	GetByNationalID(ctx context.Context, nationalID string) (*Account, error)
	Update(ctx context.Context, account *Account) error

	// UpdateBalance uses optimistic locking to update account balance
	UpdateBalance(ctx context.Context, id uuid.UUID, amount int64, version int) error

	// LockForUpdate acquires a pessimistic lock for transaction processing
	LockForUpdate(ctx context.Context, id uuid.UUID) (*Account, error)
	WithTx(tx pgx.Tx) Repository
}

// ErrConcurrentModification indicates optimistic lock failure
type ErrConcurrentModification struct {
	AccountID uuid.UUID
}

func (e ErrConcurrentModification) Error() string {
	return "concurrent modification detected for account: " + e.AccountID.String()
}

// ErrAccountNotFound indicates missing account
type ErrAccountNotFound struct {
	AccountID uuid.UUID
}

func (e ErrAccountNotFound) Error() string {
	return "account not found: " + e.AccountID.String()
}

// ErrDuplicateNationalID indicates national ID uniqueness violation
type ErrDuplicateNationalID struct {
	NationalID string
}

func (e ErrDuplicateNationalID) Error() string {
	return "account with national ID already exists: " + e.NationalID
}
