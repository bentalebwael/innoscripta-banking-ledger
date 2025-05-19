package service

import (
	"context"

	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/jackc/pgx/v5"
)

// ProcessingService defines the interface for processing transaction requests.
type ProcessingService interface {
	ProcessTransaction(ctx context.Context, request *shared.TransactionRequest) error
}

// TransactionValidator validates transaction requests before processing
type TransactionValidator interface {
	Validate(ctx context.Context, request *shared.TransactionRequest) error
	CheckIdempotency(ctx context.Context, request *shared.TransactionRequest) (bool, error)
}

// AccountManager handles account-related operations during transaction processing
type AccountManager interface {
	LockAndUpdateAccount(ctx context.Context, tx pgx.Tx, request *shared.TransactionRequest) (*account.Account, error)
}

// OutboxManager handles the creation of outbox entries for processed transactions
type OutboxManager interface {
	CreateOutboxEntry(ctx context.Context, tx pgx.Tx, request *shared.TransactionRequest, updatedAccount *account.Account) error
}

// FailureRecorder handles recording failed transactions
type FailureRecorder interface {
	RecordFailure(ctx context.Context, request *shared.TransactionRequest, failureReason string) error
}
