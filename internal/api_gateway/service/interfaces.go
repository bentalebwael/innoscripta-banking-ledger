package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
)

// AccountService defines the interface for account operations
type AccountService interface {
	// CreateAccount creates a new account with the given details
	// Returns ErrDuplicateNationalID if an account with the same national ID exists
	CreateAccount(ctx context.Context, ownerName string, nationalID string, initialBalance int64, currency string) (*account.Account, error)

	// GetAccountByID retrieves an account by its ID
	// Returns ErrAccountNotFound if the account doesn't exist
	GetAccountByID(ctx context.Context, id uuid.UUID) (*account.Account, error)
}

// TransactionService defines the interface for transaction operations
type TransactionService interface {
	// CreateTransaction initiates a new transaction with idempotency support
	// Returns transaction ID, existing ledger entry (if found via idempotencyKey), and any error
	CreateTransaction(ctx context.Context, transactionRequest *shared.TransactionRequest) (string, *ledger.Entry, error)

	// GetTransactionByID retrieves a transaction by its ID
	// Returns nil if the transaction is not found
	GetTransactionByID(ctx context.Context, transactionID uuid.UUID) (*ledger.Entry, error)

	// GetTransactionsByAccountID retrieves paginated list of transactions for an account
	// Returns entries, total count of all transactions, and any error
	GetTransactionsByAccountID(ctx context.Context, accountID uuid.UUID, page, perPage int) ([]*ledger.Entry, int64, error)
}
