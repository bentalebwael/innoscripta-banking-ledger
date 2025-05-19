package components

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/service"
	"github.com/jackc/pgx/v5"
)

// AccountManagerImpl implements the AccountManager interface
type AccountManagerImpl struct {
	accountRepo account.Repository
	logger      *slog.Logger
}

// NewAccountManager creates a new AccountManagerImpl
func NewAccountManager(accountRepo account.Repository, logger *slog.Logger) service.AccountManager {
	return &AccountManagerImpl{
		accountRepo: accountRepo,
		logger:      logger,
	}
}

// LockAndUpdateAccount locks an account, validates the transaction against it,
// and updates the account balance if valid
func (m *AccountManagerImpl) LockAndUpdateAccount(ctx context.Context, tx pgx.Tx, request *shared.TransactionRequest) (*account.Account, error) {
	logger := m.logger
	if request.CorrelationID != "" {
		logger = m.logger.With("correlation_id", request.CorrelationID)
	}

	// Use the repository with the transaction
	accountRepoTx := m.accountRepo.WithTx(tx)

	// Lock the account for update
	lockedAccount, err := accountRepoTx.LockForUpdate(ctx, request.AccountID)
	if err != nil {
		if errors.Is(err, account.ErrAccountNotFound{AccountID: request.AccountID}) {
			logger.Warn("Account not found for lock", "req_id", request.TransactionID.String(), "acc_id", request.AccountID.String(), "original_error", err)
			return nil, err
		}
		logger.Error("Failed to lock account", "req_id", request.TransactionID.String(), "acc_id", request.AccountID.String(), "error", err)
		return nil, fmt.Errorf("failed to lock account %s: %w", request.AccountID.String(), err)
	}
	logger.Info("Account locked", "req_id", request.TransactionID.String(), "acc_id", lockedAccount.ID.String(), "bal", lockedAccount.Balance, "ver", lockedAccount.Version)

	// Validate currency
	if lockedAccount.Currency != request.Currency {
		logger.Error("Currency mismatch", "req_id", request.TransactionID.String(), "req_curr", request.Currency, "acc_curr", lockedAccount.Currency)
		return nil, shared.ErrInvalidCurrency
	}

	// Apply transaction to account
	if request.Type == shared.TransactionTypeDeposit {
		if depositErr := lockedAccount.Deposit(request.Amount); depositErr != nil {
			logger.Error("Failed to apply deposit to account model", "req_id", request.TransactionID.String(), "error", depositErr)
			return nil, depositErr
		}
	} else if request.Type == shared.TransactionTypeWithdrawal {
		if withdrawErr := lockedAccount.Withdraw(request.Amount); withdrawErr != nil {
			logger.Warn("Failed to apply withdrawal to account model", "req_id", request.TransactionID.String(), "error", withdrawErr, "bal", lockedAccount.Balance, "amt", request.Amount)
			return nil, withdrawErr
		}
	}
	logger.Info("Account balance updated in memory", "req_id", request.TransactionID.String(), "new_bal", lockedAccount.Balance, "new_ver", lockedAccount.Version)

	// Persist account changes
	if err = accountRepoTx.Update(ctx, lockedAccount); err != nil {
		if errors.Is(err, account.ErrConcurrentModification{AccountID: lockedAccount.ID}) {
			logger.Warn("Concurrent modification on account update", "req_id", request.TransactionID.String(), "acc_id", lockedAccount.ID.String())
		} else {
			logger.Error("Failed to update account in DB", "req_id", request.TransactionID.String(), "acc_id", lockedAccount.ID.String(), "error", err)
		}
		return nil, err
	}
	logger.Info("Account updated in DB", "req_id", request.TransactionID.String(), "acc_id", lockedAccount.ID.String())

	return lockedAccount, nil
}
