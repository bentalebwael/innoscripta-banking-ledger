package postgres

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestAccountRepository_Create(t *testing.T) {
	ctx := context.Background()
	logger := newTestLogger()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &AccountRepository{querier: mock, logger: logger}

	acc := &account.Account{
		ID:         uuid.New(),
		OwnerName:  "Test User",
		NationalID: "AB123456789",
		Balance:    1000,
		Currency:   "USD",
		Version:    1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	query := `
		INSERT INTO accounts \(id, owner_name, national_id, balance, currency, version, created_at, updated_at\)
		VALUES \(\$1, \$2, \$3, \$4, \$5, \$6, \$7, \$8\)
	`

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(query).
			WithArgs(acc.ID, acc.OwnerName, acc.NationalID, acc.Balance, acc.Currency, acc.Version, acc.CreatedAt, acc.UpdatedAt).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := repo.Create(ctx, acc)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("failure", func(t *testing.T) {
		expectedErr := errors.New("db error")
		mock.ExpectExec(query).
			WithArgs(acc.ID, acc.OwnerName, acc.NationalID, acc.Balance, acc.Currency, acc.Version, acc.CreatedAt, acc.UpdatedAt).
			WillReturnError(expectedErr)

		err := repo.Create(ctx, acc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create account")
		assert.ErrorIs(t, err, expectedErr) // Check underlying error if wrapped
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAccountRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	logger := newTestLogger()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &AccountRepository{querier: mock, logger: logger}
	accID := uuid.New()
	now := time.Now()

	expectedAccount := &account.Account{
		ID:         accID,
		OwnerName:  "Test User",
		NationalID: "AB123456789",
		Balance:    1000,
		Currency:   "USD",
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	query := `
		SELECT id, owner_name, national_id, balance, currency, version, created_at, updated_at
		FROM accounts
		WHERE id = \$1
	`
	rows := pgxmock.NewRows([]string{"id", "owner_name", "national_id", "balance", "currency", "version", "created_at", "updated_at"}).
		AddRow(expectedAccount.ID, expectedAccount.OwnerName, expectedAccount.NationalID, expectedAccount.Balance, expectedAccount.Currency, expectedAccount.Version, expectedAccount.CreatedAt, expectedAccount.UpdatedAt)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(accID).WillReturnRows(rows)

		acc, err := repo.GetByID(ctx, accID)
		assert.NoError(t, err)
		assert.Equal(t, expectedAccount, acc)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(accID).WillReturnError(pgx.ErrNoRows)

		acc, err := repo.GetByID(ctx, accID)
		assert.Error(t, err)
		assert.Nil(t, acc)
		var accNotFoundErr account.ErrAccountNotFound
		assert.ErrorAs(t, err, &accNotFoundErr)
		assert.Equal(t, accID, accNotFoundErr.AccountID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("some db error")
		mock.ExpectQuery(query).WithArgs(accID).WillReturnError(dbErr)

		acc, err := repo.GetByID(ctx, accID)
		assert.Error(t, err)
		assert.Nil(t, acc)
		assert.Contains(t, err.Error(), "failed to get account")
		assert.ErrorIs(t, err, dbErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAccountRepository_GetByNationalID(t *testing.T) {
	ctx := context.Background()
	logger := newTestLogger()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &AccountRepository{querier: mock, logger: logger}
	nationalID := "AB123456789"
	now := time.Now()

	expectedAccount := &account.Account{
		ID:         uuid.New(),
		OwnerName:  "Test User",
		NationalID: nationalID,
		Balance:    1000,
		Currency:   "USD",
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	query := `
		SELECT id, owner_name, national_id, balance, currency, version, created_at, updated_at
		FROM accounts
		WHERE national_id = \$1
	`
	rows := pgxmock.NewRows([]string{"id", "owner_name", "national_id", "balance", "currency", "version", "created_at", "updated_at"}).
		AddRow(expectedAccount.ID, expectedAccount.OwnerName, expectedAccount.NationalID, expectedAccount.Balance, expectedAccount.Currency, expectedAccount.Version, expectedAccount.CreatedAt, expectedAccount.UpdatedAt)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(nationalID).WillReturnRows(rows)

		acc, err := repo.GetByNationalID(ctx, nationalID)
		assert.NoError(t, err)
		assert.Equal(t, expectedAccount, acc)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(nationalID).WillReturnError(pgx.ErrNoRows)

		acc, err := repo.GetByNationalID(ctx, nationalID)
		assert.NoError(t, err) // No error, just nil account
		assert.Nil(t, acc)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("some db error")
		mock.ExpectQuery(query).WithArgs(nationalID).WillReturnError(dbErr)

		acc, err := repo.GetByNationalID(ctx, nationalID)
		assert.Error(t, err)
		assert.Nil(t, acc)
		assert.Contains(t, err.Error(), "failed to get account by national ID")
		assert.ErrorIs(t, err, dbErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAccountRepository_Update(t *testing.T) {
	ctx := context.Background()
	logger := newTestLogger()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &AccountRepository{querier: mock, logger: logger}
	now := time.Now()
	accToUpdate := &account.Account{
		ID:         uuid.New(),
		OwnerName:  "Updated User",
		NationalID: "XY987654321",
		Balance:    1500,
		Currency:   "EUR",
		Version:    2, // New version after update
		UpdatedAt:  now,
	}
	previousVersion := accToUpdate.Version - 1

	query := `
		UPDATE accounts
		SET owner_name = \$1, national_id = \$2, balance = \$3, currency = \$4, version = \$5, updated_at = \$6
		WHERE id = \$7 AND version = \$8
	`

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(query).
			WithArgs(accToUpdate.OwnerName, accToUpdate.NationalID, accToUpdate.Balance, accToUpdate.Currency, accToUpdate.Version, accToUpdate.UpdatedAt, accToUpdate.ID, previousVersion).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := repo.Update(ctx, accToUpdate)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("concurrent modification", func(t *testing.T) {
		mock.ExpectExec(query).
			WithArgs(accToUpdate.OwnerName, accToUpdate.NationalID, accToUpdate.Balance, accToUpdate.Currency, accToUpdate.Version, accToUpdate.UpdatedAt, accToUpdate.ID, previousVersion).
			WillReturnResult(pgxmock.NewResult("UPDATE", 0)) // 0 rows affected

		err := repo.Update(ctx, accToUpdate)
		assert.Error(t, err)
		var concurrentModErr account.ErrConcurrentModification
		assert.ErrorAs(t, err, &concurrentModErr)
		assert.Equal(t, accToUpdate.ID, concurrentModErr.AccountID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("update db error")
		mock.ExpectExec(query).
			WithArgs(accToUpdate.OwnerName, accToUpdate.NationalID, accToUpdate.Balance, accToUpdate.Currency, accToUpdate.Version, accToUpdate.UpdatedAt, accToUpdate.ID, previousVersion).
			WillReturnError(dbErr)

		err := repo.Update(ctx, accToUpdate)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update account")
		assert.ErrorIs(t, err, dbErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAccountRepository_UpdateBalance(t *testing.T) {
	ctx := context.Background()
	logger := newTestLogger()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &AccountRepository{querier: mock, logger: logger}
	accID := uuid.New()
	amount := int64(500)
	currentVersion := 1

	query := `
		UPDATE accounts
		SET balance = balance \+ \$1, version = version \+ 1, updated_at = NOW\(\)
		WHERE id = \$2 AND version = \$3
	`

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(query).
			WithArgs(amount, accID, currentVersion).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := repo.UpdateBalance(ctx, accID, amount, currentVersion)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("concurrent modification", func(t *testing.T) {
		mock.ExpectExec(query).
			WithArgs(amount, accID, currentVersion).
			WillReturnResult(pgxmock.NewResult("UPDATE", 0)) // 0 rows affected

		err := repo.UpdateBalance(ctx, accID, amount, currentVersion)
		assert.Error(t, err)
		var concurrentModErr account.ErrConcurrentModification
		assert.ErrorAs(t, err, &concurrentModErr)
		assert.Equal(t, accID, concurrentModErr.AccountID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("update balance db error")
		mock.ExpectExec(query).
			WithArgs(amount, accID, currentVersion).
			WillReturnError(dbErr)

		err := repo.UpdateBalance(ctx, accID, amount, currentVersion)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update account balance")
		assert.ErrorIs(t, err, dbErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAccountRepository_LockForUpdate(t *testing.T) {
	ctx := context.Background()
	logger := newTestLogger()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &AccountRepository{querier: mock, logger: logger}
	accID := uuid.New()
	now := time.Now()

	expectedAccount := &account.Account{
		ID:         accID,
		OwnerName:  "Lock User",
		NationalID: "CD456789012",
		Balance:    2000,
		Currency:   "GBP",
		Version:    3,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	query := `
		SELECT id, owner_name, national_id, balance, currency, version, created_at, updated_at
		FROM accounts
		WHERE id = \$1
		FOR UPDATE
	`
	rows := pgxmock.NewRows([]string{"id", "owner_name", "national_id", "balance", "currency", "version", "created_at", "updated_at"}).
		AddRow(expectedAccount.ID, expectedAccount.OwnerName, expectedAccount.NationalID, expectedAccount.Balance, expectedAccount.Currency, expectedAccount.Version, expectedAccount.CreatedAt, expectedAccount.UpdatedAt)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(accID).WillReturnRows(rows)

		acc, err := repo.LockForUpdate(ctx, accID)
		assert.NoError(t, err)
		assert.Equal(t, expectedAccount, acc)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(accID).WillReturnError(pgx.ErrNoRows)

		acc, err := repo.LockForUpdate(ctx, accID)
		assert.Error(t, err)
		assert.Nil(t, acc)
		var accNotFoundErr account.ErrAccountNotFound
		assert.ErrorAs(t, err, &accNotFoundErr)
		assert.Equal(t, accID, accNotFoundErr.AccountID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("lock db error")
		mock.ExpectQuery(query).WithArgs(accID).WillReturnError(dbErr)

		acc, err := repo.LockForUpdate(ctx, accID)
		assert.Error(t, err)
		assert.Nil(t, acc)
		assert.Contains(t, err.Error(), "failed to lock account for update")
		assert.ErrorIs(t, err, dbErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAccountRepository_WithTx(t *testing.T) {
	logger := newTestLogger()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	// Original repository with a pool
	originalRepo := &AccountRepository{querier: mockPool, logger: logger}

	// Create a mock transaction
	mockTx, err := pgxmock.NewConn() // NewConn can simulate a Tx for mocking purposes
	require.NoError(t, err)
	defer mockTx.Close(context.Background())

	mockPool.ExpectBegin()
	pgxTx, err := mockPool.Begin(context.Background())
	require.NoError(t, err)

	txRepo := originalRepo.WithTx(pgxTx)

	assert.NotNil(t, txRepo)
	assert.Equal(t, originalRepo.logger, txRepo.(*AccountRepository).logger)
	assert.Equal(t, pgxTx, txRepo.(*AccountRepository).querier, "Querier in new repo should be the transaction")

	assert.NoError(t, mockPool.ExpectationsWereMet())
}
