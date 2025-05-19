package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
)

// AccountServiceImpl implements the AccountService interface
type AccountServiceImpl struct {
	accountRepo account.Repository
}

// NewAccountService creates a new account service
func NewAccountService(accountRepo account.Repository) AccountService {
	return &AccountServiceImpl{
		accountRepo: accountRepo,
	}
}

// CreateAccount creates a new account with the given details, checking for duplicate national IDs
func (s *AccountServiceImpl) CreateAccount(ctx context.Context, ownerName string, nationalID string, initialBalance int64, currency string) (*account.Account, error) {
	existingAccount, err := s.accountRepo.GetByNationalID(ctx, nationalID)
	if err != nil {
		return nil, err
	}
	if existingAccount != nil {
		return nil, account.ErrDuplicateNationalID{NationalID: nationalID}
	}

	acc, err := account.NewAccount(ownerName, nationalID, initialBalance, currency)
	if err != nil {
		return nil, err
	}

	if err := s.accountRepo.Create(ctx, acc); err != nil {
		return nil, err
	}

	return acc, nil
}

// GetAccountByID retrieves an account by its ID, returns ErrAccountNotFound if not found
func (s *AccountServiceImpl) GetAccountByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	return s.accountRepo.GetByID(ctx, id)
}
