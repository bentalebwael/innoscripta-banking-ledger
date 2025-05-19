package components

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
)

// MockLedgerRepo is already defined in transaction_validator_test.go, but we'll define it here for clarity
type MockLedgerRepoForFailure struct {
	mock.Mock
}

func (m *MockLedgerRepoForFailure) Create(ctx context.Context, entry *ledger.Entry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockLedgerRepoForFailure) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*ledger.Entry, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepoForFailure) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*ledger.Entry, error) {
	args := m.Called(ctx, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepoForFailure) GetByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*ledger.Entry, error) {
	args := m.Called(ctx, accountID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepoForFailure) CountByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockLedgerRepoForFailure) UpdateStatus(ctx context.Context, transactionID uuid.UUID, status shared.TransactionStatus, reason string) error {
	args := m.Called(ctx, transactionID, status, reason)
	return args.Error(0)
}

func (m *MockLedgerRepoForFailure) GetByTimeRange(ctx context.Context, startTime, endTime time.Time, limit, offset int) ([]*ledger.Entry, error) {
	args := m.Called(ctx, startTime, endTime, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ledger.Entry), args.Error(1)
}

func TestFailureRecorder_RecordFailure(t *testing.T) {
	mockRepo := &MockLedgerRepoForFailure{}
	logger := slog.Default()
	recorder := NewFailureRecorder(mockRepo, logger)

	txID := uuid.New()
	accountID := uuid.New()
	failureReason := "insufficient funds"

	tests := []struct {
		name          string
		request       *shared.TransactionRequest
		setupMocks    func()
		expectedError error
	}{
		{
			name: "create new failed entry",
			request: &shared.TransactionRequest{
				TransactionID:  txID,
				AccountID:      accountID,
				Type:           shared.TransactionTypeWithdrawal,
				Amount:         100,
				Currency:       "USD",
				IdempotencyKey: "key1",
				CorrelationID:  "corr1",
				Timestamp:      time.Now(),
			},
			setupMocks: func() {
				mockRepo.On("GetByTransactionID", mock.Anything, txID).Return(nil, ledger.ErrEntryNotFound{}).Once()

				mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(entry *ledger.Entry) bool {
					return entry.TransactionID == txID &&
						entry.Status == shared.TransactionStatusFailed &&
						entry.FailureReason == failureReason
				})).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name: "update existing entry to failed",
			request: &shared.TransactionRequest{
				TransactionID:  txID,
				AccountID:      accountID,
				Type:           shared.TransactionTypeWithdrawal,
				Amount:         100,
				Currency:       "USD",
				IdempotencyKey: "key1",
				CorrelationID:  "corr1",
				Timestamp:      time.Now(),
			},
			setupMocks: func() {
				existingEntry := &ledger.Entry{
					TransactionID: txID,
					Status:        shared.TransactionStatusPending,
				}
				mockRepo.On("GetByTransactionID", mock.Anything, txID).Return(existingEntry, nil).Once()

				mockRepo.On("UpdateStatus", mock.Anything, txID, shared.TransactionStatusFailed, failureReason).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name: "entry already failed",
			request: &shared.TransactionRequest{
				TransactionID:  txID,
				AccountID:      accountID,
				Type:           shared.TransactionTypeWithdrawal,
				Amount:         100,
				Currency:       "USD",
				IdempotencyKey: "key1",
				CorrelationID:  "corr1",
				Timestamp:      time.Now(),
			},
			setupMocks: func() {
				existingEntry := &ledger.Entry{
					TransactionID: txID,
					Status:        shared.TransactionStatusFailed,
				}
				mockRepo.On("GetByTransactionID", mock.Anything, txID).Return(existingEntry, nil).Once()

			},
			expectedError: nil,
		},
		{
			name: "error creating entry",
			request: &shared.TransactionRequest{
				TransactionID:  txID,
				AccountID:      accountID,
				Type:           shared.TransactionTypeWithdrawal,
				Amount:         100,
				Currency:       "USD",
				IdempotencyKey: "key1",
				CorrelationID:  "corr1",
				Timestamp:      time.Now(),
			},
			setupMocks: func() {
				mockRepo.On("GetByTransactionID", mock.Anything, txID).Return(nil, ledger.ErrEntryNotFound{}).Once()

				mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db error")).Once()
			},
			expectedError: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			ctx := context.Background()

			err := recorder.RecordFailure(ctx, tt.request, failureReason)

			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}
