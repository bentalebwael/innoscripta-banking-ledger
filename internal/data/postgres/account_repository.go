// Package postgres provides PostgreSQL implementations of the domain repositories.
// It handles all database operations while maintaining transaction safety and
// proper error handling for the banking ledger system.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/platform/persistence"
	"github.com/jackc/pgx/v5"
)

// AccountRepository implements the account.Repository interface for PostgreSQL
type AccountRepository struct {
	querier persistence.Querier // Can be *pgxpool.Pool or pgx.Tx
	logger  *slog.Logger
}

// NewAccountRepository creates a new PostgreSQL account repository.
// It expects db.Pool() to satisfy persistence.Querier.
func NewAccountRepository(logger *slog.Logger, db *persistence.PostgresDB) account.Repository {
	return &AccountRepository{
		querier: db.Pool(), // Initialize with the pool
		logger:  logger,
	}
}

// WithTx wraps the repository with a transaction, allowing for atomic operations
// across multiple repository calls. The returned repository will use the provided
// transaction for all database operations.
func (r *AccountRepository) WithTx(tx pgx.Tx) account.Repository {
	return &AccountRepository{
		querier: tx, // Use the transaction
		logger:  r.logger,
	}
}

// Create stores a new account in the database. If an account with the same
// national ID already exists, a database constraint error will be returned.
func (r *AccountRepository) Create(ctx context.Context, acc *account.Account) error {
	query := `
		INSERT INTO accounts (id, owner_name, national_id, balance, currency, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.querier.Exec(ctx, query,
		acc.ID,
		acc.OwnerName,
		acc.NationalID,
		acc.Balance,
		acc.Currency,
		acc.Version,
		acc.CreatedAt,
		acc.UpdatedAt,
	)
	if err != nil {
		r.logger.Error("Failed to create account", "error", err)
		return fmt.Errorf("failed to create account: %w", err)
	}

	return nil
}

// GetByID retrieves an account by its ID
func (r *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	query := `
		SELECT id, owner_name, national_id, balance, currency, version, created_at, updated_at
		FROM accounts
		WHERE id = $1
	`

	var acc account.Account
	err := r.querier.QueryRow(ctx, query, id).Scan(
		&acc.ID,
		&acc.OwnerName,
		&acc.NationalID,
		&acc.Balance,
		&acc.Currency,
		&acc.Version,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, account.ErrAccountNotFound{AccountID: id}
		}
		r.logger.Error("Failed to get account", "id", id.String(), "error", err)
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &acc, nil
}

// GetByNationalID retrieves an account by its National ID
func (r *AccountRepository) GetByNationalID(ctx context.Context, nationalID string) (*account.Account, error) {
	query := `
		SELECT id, owner_name, national_id, balance, currency, version, created_at, updated_at
		FROM accounts
		WHERE national_id = $1
	`

	var acc account.Account
	err := r.querier.QueryRow(ctx, query, nationalID).Scan(
		&acc.ID,
		&acc.OwnerName,
		&acc.NationalID,
		&acc.Balance,
		&acc.Currency,
		&acc.Version,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Return nil, nil when no account is found with the given National ID
		}
		r.logger.Error("Failed to get account by national ID", "nationalID", nationalID, "error", err)
		return nil, fmt.Errorf("failed to get account by national ID: %w", err)
	}

	return &acc, nil
}

// Update updates an existing account in the database
func (r *AccountRepository) Update(ctx context.Context, acc *account.Account) error {
	query := `
		UPDATE accounts
		SET owner_name = $1, national_id = $2, balance = $3, currency = $4, version = $5, updated_at = $6
		WHERE id = $7 AND version = $8
	`

	result, err := r.querier.Exec(ctx, query,
		acc.OwnerName,
		acc.NationalID,
		acc.Balance,
		acc.Currency,
		acc.Version,
		acc.UpdatedAt,
		acc.ID,
		acc.Version-1, // Check previous version for optimistic locking
	)
	if err != nil {
		r.logger.Error("Failed to update account", "id", acc.ID.String(), "error", err)
		return fmt.Errorf("failed to update account: %w", err)
	}

	if result.RowsAffected() == 0 {
		return account.ErrConcurrentModification{AccountID: acc.ID}
	}

	return nil
}

// UpdateBalance atomically updates the account balance using optimistic locking.
// Returns ErrConcurrentModification if the account was modified between read and update.
func (r *AccountRepository) UpdateBalance(ctx context.Context, id uuid.UUID, amount int64, version int) error {
	query := `
		UPDATE accounts
		SET balance = balance + $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND version = $3
	`

	result, err := r.querier.Exec(ctx, query, amount, id, version)
	if err != nil {
		r.logger.Error("Failed to update account balance", "id", id.String(), "error", err)
		return fmt.Errorf("failed to update account balance: %w", err)
	}

	if result.RowsAffected() == 0 {
		return account.ErrConcurrentModification{AccountID: id}
	}

	return nil
}

// LockForUpdate obtains a pessimistic lock on the account and returns its current state.
// This should be used within a transaction when strong consistency is required.
func (r *AccountRepository) LockForUpdate(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	query := `
		SELECT id, owner_name, national_id, balance, currency, version, created_at, updated_at
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`

	var acc account.Account
	err := r.querier.QueryRow(ctx, query, id).Scan(
		&acc.ID,
		&acc.OwnerName,
		&acc.NationalID,
		&acc.Balance,
		&acc.Currency,
		&acc.Version,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, account.ErrAccountNotFound{AccountID: id}
		}
		r.logger.Error("Failed to lock account for update", "id", id.String(), "error", err)
		return nil, fmt.Errorf("failed to lock account for update: %w", err)
	}

	return &acc, nil
}
