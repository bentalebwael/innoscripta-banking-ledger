package mongo

import (
	"context"
	"errors"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/mongo"
)

type MockLedgerRepository struct {
	mock.Mock
}

func (m *MockLedgerRepository) Create(ctx context.Context, entry *ledger.Entry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockLedgerRepository) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*ledger.Entry, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepository) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*ledger.Entry, error) {
	args := m.Called(ctx, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepository) GetByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*ledger.Entry, error) {
	args := m.Called(ctx, accountID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepository) CountByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockLedgerRepository) UpdateStatus(ctx context.Context, transactionID uuid.UUID, status shared.TransactionStatus, reason string) error {
	args := m.Called(ctx, transactionID, status, reason)
	return args.Error(0)
}

func (m *MockLedgerRepository) GetByTimeRange(ctx context.Context, startTime, endTime time.Time, limit, offset int) ([]*ledger.Entry, error) {
	args := m.Called(ctx, startTime, endTime, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ledger.Entry), args.Error(1)
}

func TestNewLedgerRepository(t *testing.T) {
	db := &mongo.Database{}
	logger := slog.Default()

	repo := NewLedgerRepository(logger, db)

	assert.NotNil(t, repo)
	assert.IsType(t, &LedgerRepository{}, repo)
}

func TestLedgerRepository_Create(t *testing.T) {
	mockRepo := &MockLedgerRepository{}

	txID := uuid.New()
	accountID := uuid.New()
	entry := &ledger.Entry{
		TransactionID:  txID,
		AccountID:      accountID,
		Type:           shared.TransactionTypeDeposit,
		Amount:         100,
		Currency:       "USD",
		IdempotencyKey: "key1",
		CorrelationID:  "corr1",
		Status:         shared.TransactionStatusPending,
		CreatedAt:      time.Now(),
	}

	tests := []struct {
		name          string
		setupMocks    func()
		expectedError error
	}{
		{
			name: "successful creation",
			setupMocks: func() {
				mockRepo.On("Create", mock.Anything, entry).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "duplicate entry",
			setupMocks: func() {
				mockRepo.On("Create", mock.Anything, entry).Return(ledger.ErrDuplicateEntry{TransactionID: txID})
			},
			expectedError: ledger.ErrDuplicateEntry{TransactionID: txID},
		},
		{
			name: "database error",
			setupMocks: func() {
				mockRepo.On("Create", mock.Anything, entry).Return(errors.New("db error"))
			},
			expectedError: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo = &MockLedgerRepository{}
			tt.setupMocks()

			ctx := context.Background()
			err := mockRepo.Create(ctx, entry)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestLedgerRepository_GetByTransactionID(t *testing.T) {
	mockRepo := &MockLedgerRepository{}

	txID := uuid.New()
	accountID := uuid.New()
	entry := &ledger.Entry{
		TransactionID:  txID,
		AccountID:      accountID,
		Type:           shared.TransactionTypeDeposit,
		Amount:         100,
		Currency:       "USD",
		IdempotencyKey: "key1",
		CorrelationID:  "corr1",
		Status:         shared.TransactionStatusPending,
		CreatedAt:      time.Now(),
	}

	tests := []struct {
		name          string
		setupMocks    func()
		expectedEntry *ledger.Entry
		expectedError error
	}{
		{
			name: "entry found",
			setupMocks: func() {
				mockRepo.On("GetByTransactionID", mock.Anything, txID).Return(entry, nil)
			},
			expectedEntry: entry,
			expectedError: nil,
		},
		{
			name: "entry not found",
			setupMocks: func() {
				mockRepo.On("GetByTransactionID", mock.Anything, txID).Return(nil, ledger.ErrEntryNotFound{TransactionID: txID})
			},
			expectedEntry: nil,
			expectedError: ledger.ErrEntryNotFound{TransactionID: txID},
		},
		{
			name: "database error",
			setupMocks: func() {
				mockRepo.On("GetByTransactionID", mock.Anything, txID).Return(nil, errors.New("db error"))
			},
			expectedEntry: nil,
			expectedError: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo = &MockLedgerRepository{}
			tt.setupMocks()

			ctx := context.Background()
			result, err := mockRepo.GetByTransactionID(ctx, txID)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedEntry, result)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestLedgerRepository_UpdateStatus(t *testing.T) {
	mockRepo := &MockLedgerRepository{}

	txID := uuid.New()
	status := shared.TransactionStatusCompleted
	reason := "test reason"

	tests := []struct {
		name          string
		setupMocks    func()
		expectedError error
	}{
		{
			name: "successful update",
			setupMocks: func() {
				mockRepo.On("UpdateStatus", mock.Anything, txID, status, reason).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "entry not found",
			setupMocks: func() {
				mockRepo.On("UpdateStatus", mock.Anything, txID, status, reason).Return(ledger.ErrEntryNotFound{TransactionID: txID})
			},
			expectedError: ledger.ErrEntryNotFound{TransactionID: txID},
		},
		{
			name: "database error",
			setupMocks: func() {
				mockRepo.On("UpdateStatus", mock.Anything, txID, status, reason).Return(errors.New("db error"))
			},
			expectedError: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo = &MockLedgerRepository{}
			tt.setupMocks()

			ctx := context.Background()
			err := mockRepo.UpdateStatus(ctx, txID, status, reason)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}
