package components

import (
	"context"
	"testing"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAccountRepo struct {
	mock.Mock
}

func (m *MockAccountRepo) Create(ctx context.Context, account *account.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *MockAccountRepo) GetByNationalID(ctx context.Context, nationalID string) (*account.Account, error) {
	args := m.Called(ctx, nationalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *MockAccountRepo) Update(ctx context.Context, account *account.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepo) UpdateBalance(ctx context.Context, id uuid.UUID, amount int64, version int) error {
	args := m.Called(ctx, id, amount, version)
	return args.Error(0)
}

func (m *MockAccountRepo) LockForUpdate(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *MockAccountRepo) WithTx(tx pgx.Tx) account.Repository {
	args := m.Called(tx)
	return args.Get(0).(account.Repository)
}

// TestAccountManager_LockAndUpdateAccount tests account transactions with mocked dependencies
func TestAccountManager_LockAndUpdateAccount(t *testing.T) {
	mockRepo := &MockAccountRepo{}
	logger := slog.Default()
	manager := NewAccountManager(mockRepo, logger)

	tests := []struct {
		name          string
		request       *shared.TransactionRequest
		setupMocks    func()
		expectedError error
		expectAccount *account.Account
	}{
		{
			name: "successful deposit",
			request: &shared.TransactionRequest{
				TransactionID: uuid.New(),
				AccountID:     uuid.New(),
				Type:          shared.TransactionTypeDeposit,
				Amount:        100,
				Currency:      "USD",
			},
			setupMocks: func() {
				acc := &account.Account{
					ID:       uuid.New(),
					Balance:  500,
					Currency: "USD",
					Version:  1,
				}

				mockRepo.On("WithTx", mock.Anything).Return(mockRepo)
				mockRepo.On("LockForUpdate", mock.Anything, mock.Anything).Return(acc, nil)
				mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(a *account.Account) bool {
					return a.Balance == 600 && a.Version == 2
				})).Return(nil)
			},
			expectedError: nil,
			expectAccount: &account.Account{
				Balance:  600,
				Currency: "USD",
			},
		},
		{
			name: "successful withdrawal",
			request: &shared.TransactionRequest{
				TransactionID: uuid.New(),
				AccountID:     uuid.New(),
				Type:          shared.TransactionTypeWithdrawal,
				Amount:        100,
				Currency:      "USD",
			},
			setupMocks: func() {
				acc := &account.Account{
					ID:       uuid.New(),
					Balance:  500,
					Currency: "USD",
					Version:  1,
				}

				mockRepo.On("WithTx", mock.Anything).Return(mockRepo)
				mockRepo.On("LockForUpdate", mock.Anything, mock.Anything).Return(acc, nil)
				mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(a *account.Account) bool {
					return a.Balance == 400 && a.Version == 2
				})).Return(nil)
			},
			expectedError: nil,
			expectAccount: &account.Account{
				Balance:  400,
				Currency: "USD",
			},
		},
		{
			name: "currency mismatch",
			request: &shared.TransactionRequest{
				TransactionID: uuid.New(),
				AccountID:     uuid.New(),
				Type:          shared.TransactionTypeDeposit,
				Amount:        100,
				Currency:      "USD",
			},
			setupMocks: func() {
				acc := &account.Account{
					ID:       uuid.New(),
					Balance:  500,
					Currency: "EUR", // Different currency
					Version:  1,
				}
				mockRepo.On("WithTx", mock.Anything).Return(mockRepo)
				mockRepo.On("LockForUpdate", mock.Anything, mock.Anything).Return(acc, nil)
			},
			expectedError: shared.ErrInvalidCurrency,
			expectAccount: nil,
		},
		{
			name: "account not found",
			request: &shared.TransactionRequest{
				TransactionID: uuid.New(),
				AccountID:     uuid.New(),
				Type:          shared.TransactionTypeDeposit,
				Amount:        100,
				Currency:      "USD",
			},
			setupMocks: func() {
				mockRepo.On("WithTx", mock.Anything).Return(mockRepo)
				mockRepo.On("LockForUpdate", mock.Anything, mock.Anything).Return(nil, account.ErrAccountNotFound{})
			},
			expectedError: account.ErrAccountNotFound{},
			expectAccount: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo = &MockAccountRepo{}
			manager = NewAccountManager(mockRepo, logger)

			tt.setupMocks()
			ctx := context.Background()

			account, err := manager.LockAndUpdateAccount(ctx, nil, tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Nil(t, account)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, account)
				if tt.expectAccount != nil {
					assert.Equal(t, tt.expectAccount.Balance, account.Balance)
					assert.Equal(t, tt.expectAccount.Currency, account.Currency)
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}
