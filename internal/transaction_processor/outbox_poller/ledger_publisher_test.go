package outbox_poller

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/outbox"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockOutboxRepo for testing
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

// MockLedgerRepo for testing
type MockLedgerRepo struct {
	mock.Mock
}

func (m *MockLedgerRepo) Create(ctx context.Context, entry *ledger.Entry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockLedgerRepo) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*ledger.Entry, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepo) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*ledger.Entry, error) {
	args := m.Called(ctx, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepo) GetByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*ledger.Entry, error) {
	args := m.Called(ctx, accountID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepo) CountByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockLedgerRepo) UpdateStatus(ctx context.Context, transactionID uuid.UUID, status shared.TransactionStatus, reason string) error {
	args := m.Called(ctx, transactionID, status, reason)
	return args.Error(0)
}

func (m *MockLedgerRepo) GetByTimeRange(ctx context.Context, startTime, endTime time.Time, limit, offset int) ([]*ledger.Entry, error) {
	args := m.Called(ctx, startTime, endTime, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ledger.Entry), args.Error(1)
}

func TestLedgerPublisher_PublishToLedger(t *testing.T) {
	mockOutboxRepo := &MockOutboxRepo{}
	mockLedgerRepo := &MockLedgerRepo{}
	logger := slog.Default()

	publisher := NewLedgerPublisher(mockOutboxRepo, mockLedgerRepo, logger)

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
		Status:         shared.TransactionStatusProcessing,
	}

	entryJSON, err := json.Marshal(entry)
	assert.NoError(t, err)

	message := &outbox.Message{
		ID:            1,
		TransactionID: txID,
		Status:        shared.OutboxStatusPending,
		Payload:       entryJSON,
		Attempts:      0,
		CreatedAt:     time.Now(),
	}

	tests := []struct {
		name          string
		message       *outbox.Message
		setupMocks    func()
		expectedError error
	}{
		{
			name:    "successful publish - no existing entry",
			message: message,
			setupMocks: func() {
				mockLedgerRepo.On("GetByTransactionID", mock.Anything, txID).Return(nil, ledger.ErrEntryNotFound{}).Once()

				mockLedgerRepo.On("Create", mock.Anything, mock.MatchedBy(func(e *ledger.Entry) bool {
					return e.TransactionID == txID && e.Status == shared.TransactionStatusCompleted
				})).Return(nil).Once()

				mockOutboxRepo.On("UpdateStatus", mock.Anything, int64(1), shared.OutboxStatusProcessed).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name:    "successful publish - existing entry with non-completed status",
			message: message,
			setupMocks: func() {
				existingEntry := &ledger.Entry{
					TransactionID: txID,
					Status:        shared.TransactionStatusPending,
				}
				mockLedgerRepo.On("GetByTransactionID", mock.Anything, txID).Return(existingEntry, nil).Once()

				mockLedgerRepo.On("UpdateStatus", mock.Anything, txID, shared.TransactionStatusCompleted, "").Return(nil).Once()

				mockOutboxRepo.On("UpdateStatus", mock.Anything, int64(1), shared.OutboxStatusProcessed).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name:    "successful publish - existing entry with completed status",
			message: message,
			setupMocks: func() {
				existingEntry := &ledger.Entry{
					TransactionID: txID,
					Status:        shared.TransactionStatusCompleted,
				}
				mockLedgerRepo.On("GetByTransactionID", mock.Anything, txID).Return(existingEntry, nil).Once()

				mockOutboxRepo.On("UpdateStatus", mock.Anything, int64(1), shared.OutboxStatusProcessed).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name: "error unmarshalling payload",
			message: &outbox.Message{
				ID:            1,
				TransactionID: txID,
				Status:        shared.OutboxStatusPending,
				Payload:       []byte("invalid json"),
				Attempts:      0,
				CreatedAt:     time.Now(),
			},
			setupMocks: func() {
				mockOutboxRepo.On("UpdateStatus", mock.Anything, int64(1), shared.OutboxStatusFailedToPublish).Return(nil).Once()
			},
			expectedError: errors.New("unmarshal payload"),
		},
		{
			name:    "error creating ledger entry",
			message: message,
			setupMocks: func() {
				mockLedgerRepo.On("GetByTransactionID", mock.Anything, txID).Return(nil, ledger.ErrEntryNotFound{}).Once()

				mockLedgerRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db error")).Once()
			},
			expectedError: errors.New("failed to create ledger entry"),
		},
		{
			name:    "error updating outbox status",
			message: message,
			setupMocks: func() {
				mockLedgerRepo.On("GetByTransactionID", mock.Anything, txID).Return(nil, ledger.ErrEntryNotFound{}).Once()

				mockLedgerRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()

				mockOutboxRepo.On("UpdateStatus", mock.Anything, int64(1), shared.OutboxStatusProcessed).Return(errors.New("db error")).Once()
			},
			expectedError: errors.New("failed to mark outbox"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOutboxRepo = &MockOutboxRepo{}
			mockLedgerRepo = &MockLedgerRepo{}
			publisher = NewLedgerPublisher(mockOutboxRepo, mockLedgerRepo, logger)

			tt.setupMocks()
			ctx := context.Background()

			err := publisher.PublishToLedger(ctx, tt.message)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			mockOutboxRepo.AssertExpectations(t)
			mockLedgerRepo.AssertExpectations(t)
		})
	}
}
