package components

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/domain/outbox"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockOutboxRepo struct {
	mock.Mock
}

func (m *MockOutboxRepo) Create(ctx context.Context, message *outbox.Message) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockOutboxRepo) GetPending(ctx context.Context, limit int) ([]*outbox.Message, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*outbox.Message), args.Error(1)
}

func (m *MockOutboxRepo) UpdateStatus(ctx context.Context, id int64, status shared.OutboxStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockOutboxRepo) IncrementAttempts(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOutboxRepo) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOutboxRepo) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*outbox.Message, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbox.Message), args.Error(1)
}

func (m *MockOutboxRepo) WithTx(tx pgx.Tx) outbox.Repository {
	args := m.Called(tx)
	return args.Get(0).(outbox.Repository)
}

func TestOutboxManager_CreateOutboxEntry(t *testing.T) {
	txID := uuid.New()
	accountID := uuid.New()
	now := time.Now()
	dbError := errors.New("db error")

	tests := []struct {
		name          string
		request       *shared.TransactionRequest
		account       *account.Account
		setupMocks    func(mockRepo *MockOutboxRepo)
		expectedError error
		errorContains string
	}{
		{
			name: "successful outbox entry creation",
			request: &shared.TransactionRequest{
				TransactionID:  txID,
				AccountID:      accountID,
				Type:           shared.TransactionTypeDeposit,
				Amount:         100,
				Currency:       "USD",
				IdempotencyKey: "key1",
				CorrelationID:  "corr1",
				Timestamp:      now,
			},
			account: &account.Account{
				ID:       accountID,
				Balance:  600,
				Currency: "USD",
			},
			setupMocks: func(mockRepo *MockOutboxRepo) {
				mockRepo.On("WithTx", mock.Anything).Return(mockRepo)
				mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(msg *outbox.Message) bool {
					return msg.Payload != nil && msg.Status == shared.OutboxStatusPending
				})).Return(nil)
			},
			expectedError: nil,
			errorContains: "",
		},
		{
			name: "error creating outbox entry",
			request: &shared.TransactionRequest{
				TransactionID:  txID,
				AccountID:      accountID,
				Type:           shared.TransactionTypeDeposit,
				Amount:         100,
				Currency:       "USD",
				IdempotencyKey: "key1",
				CorrelationID:  "corr1",
				Timestamp:      now,
			},
			account: &account.Account{
				ID:       accountID,
				Balance:  600,
				Currency: "USD",
			},
			setupMocks: func(mockRepo *MockOutboxRepo) {
				mockRepo.On("WithTx", mock.Anything).Return(mockRepo)
				mockRepo.On("Create", mock.Anything, mock.Anything).Return(dbError)
			},
			expectedError: nil,
			errorContains: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockOutboxRepo{}
			logger := slog.Default()
			manager := NewOutboxManager(mockRepo, logger)

			tt.setupMocks(mockRepo)
			ctx := context.Background()

			err := manager.CreateOutboxEntry(ctx, nil, tt.request, tt.account)

			if tt.errorContains != "" {
				assert.Error(t, err)
				assert.True(t, strings.Contains(err.Error(), tt.errorContains),
					"Expected error to contain '%s', got '%s'", tt.errorContains, err.Error())
			} else if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}
