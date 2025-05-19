package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAccountRepository struct {
	mock.Mock
}

func (m *MockAccountRepository) Create(ctx context.Context, acc *account.Account) error {
	args := m.Called(ctx, acc)
	return args.Error(0)
}

func (m *MockAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *MockAccountRepository) GetByNationalID(ctx context.Context, nationalID string) (*account.Account, error) {
	args := m.Called(ctx, nationalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *MockAccountRepository) Update(ctx context.Context, acc *account.Account) error {
	args := m.Called(ctx, acc)
	return args.Error(0)
}

func (m *MockAccountRepository) WithTx(tx pgx.Tx) account.Repository {
	args := m.Called(tx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(account.Repository)
}

func (m *MockAccountRepository) LockForUpdate(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *MockAccountRepository) UpdateBalance(ctx context.Context, id uuid.UUID, amount int64, version int) error {
	args := m.Called(ctx, id, amount, version)
	return args.Error(0)
}

func TestAccountServiceImpl_CreateAccount(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockAccountRepository)
		service := NewAccountService(mockRepo)
		ownerName := "Test User"
		nationalID := "AB123456789"
		initialBalance := int64(10000) // 100.00
		currency := "USD"

		mockRepo.On("GetByNationalID", ctx, nationalID).Return(nil, nil).Once()
		mockRepo.On("Create", ctx, mock.AnythingOfType("*account.Account")).Return(nil).Once()

		acc, err := service.CreateAccount(ctx, ownerName, nationalID, initialBalance, currency)

		assert.NoError(t, err)
		assert.NotNil(t, acc)
		assert.Equal(t, ownerName, acc.OwnerName)
		assert.Equal(t, nationalID, acc.NationalID)
		assert.Equal(t, initialBalance, acc.Balance)
		assert.Equal(t, currency, acc.Currency)
		assert.NotEqual(t, uuid.Nil, acc.ID)
		mockRepo.AssertExpectations(t)
	})

	t.Run("InvalidAccountData", func(t *testing.T) {
		mockRepo := new(MockAccountRepository)
		service := NewAccountService(mockRepo)
		nationalIDForTest := "AB123456789"
		mockRepo.On("GetByNationalID", ctx, nationalIDForTest).Return(nil, nil).Once()
		_, err := service.CreateAccount(ctx, "", nationalIDForTest, 10000, "USD")
		assert.Error(t, err) // Expecting an error from account.NewAccount
		mockRepo.AssertNotCalled(t, "Create", ctx, mock.AnythingOfType("*account.Account"))
		mockRepo.AssertExpectations(t)
	})

	t.Run("RepositoryCreateError", func(t *testing.T) {
		mockRepo := new(MockAccountRepository)
		service := NewAccountService(mockRepo)
		ownerName := "Test User Fail"
		nationalID := "XY987654321"
		initialBalance := int64(5000)
		currency := "EUR"
		repoError := errors.New("database error")

		mockRepo.On("GetByNationalID", ctx, nationalID).Return(nil, nil).Once()
		mockRepo.On("Create", ctx, mock.AnythingOfType("*account.Account")).Return(repoError).Once()

		acc, err := service.CreateAccount(ctx, ownerName, nationalID, initialBalance, currency)

		assert.Error(t, err)
		assert.Nil(t, acc)
		assert.Equal(t, repoError, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DuplicateNationalID", func(t *testing.T) {
		mockRepo := new(MockAccountRepository)
		service := NewAccountService(mockRepo)
		ownerName := "Test User"
		nationalID := "AB123456789"
		initialBalance := int64(10000)
		currency := "USD"

		existingAccount := &account.Account{
			ID:         uuid.New(),
			OwnerName:  "Existing User",
			NationalID: nationalID,
			Balance:    5000,
			Currency:   "USD",
			Version:    1,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		mockRepo.On("GetByNationalID", ctx, nationalID).Return(existingAccount, nil).Once()

		acc, err := service.CreateAccount(ctx, ownerName, nationalID, initialBalance, currency)

		assert.Error(t, err)
		assert.Nil(t, acc)
		var duplicateNationalIDErr account.ErrDuplicateNationalID
		assert.ErrorAs(t, err, &duplicateNationalIDErr)
		assert.Equal(t, nationalID, duplicateNationalIDErr.NationalID)
		mockRepo.AssertExpectations(t)
	})
}

func TestAccountServiceImpl_GetAccountByID(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockAccountRepository)
		service := NewAccountService(mockRepo)
		accountID := uuid.New()
		expectedAccount := &account.Account{
			ID:         accountID,
			OwnerName:  "Found User",
			NationalID: "CD456789012",
			Balance:    20000,
			Currency:   "GBP",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		mockRepo.On("GetByID", ctx, accountID).Return(expectedAccount, nil).Once()

		acc, err := service.GetAccountByID(ctx, accountID)

		assert.NoError(t, err)
		assert.NotNil(t, acc)
		assert.Equal(t, expectedAccount, acc)
		mockRepo.AssertExpectations(t)
	})

	t.Run("AccountNotFound", func(t *testing.T) {
		mockRepo := new(MockAccountRepository)
		service := NewAccountService(mockRepo)
		accountID := uuid.New()
		var notFoundError error = account.ErrAccountNotFound{AccountID: accountID}

		mockRepo.On("GetByID", ctx, accountID).Return(nil, notFoundError).Once()

		acc, err := service.GetAccountByID(ctx, accountID)

		assert.Error(t, err)
		assert.Nil(t, acc)
		if _, ok := notFoundError.(account.ErrAccountNotFound); ok {
			assert.True(t, errors.Is(err, notFoundError))
		} else {
			assert.Equal(t, notFoundError, err)
		}
		mockRepo.AssertExpectations(t)
	})

	t.Run("RepositoryGetError", func(t *testing.T) {
		mockRepo := new(MockAccountRepository)
		service := NewAccountService(mockRepo)
		accountID := uuid.New()
		repoError := errors.New("some other db error")

		mockRepo.On("GetByID", ctx, accountID).Return(nil, repoError).Once()

		acc, err := service.GetAccountByID(ctx, accountID)

		assert.Error(t, err)
		assert.Nil(t, acc)
		assert.Equal(t, repoError, err)
		mockRepo.AssertExpectations(t)
	})
}

var _ account.Repository = (*MockAccountRepository)(nil)
