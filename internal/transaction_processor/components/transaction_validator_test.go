package components

import (
	"context"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

func TestTransactionValidator_Validate(t *testing.T) {
	mockRepo := &MockLedgerRepo{}
	logger := slog.Default()
	validator := NewTransactionValidator(mockRepo, logger)

	tests := []struct {
		name    string
		request *shared.TransactionRequest
		wantErr bool
	}{
		{
			name: "valid deposit",
			request: &shared.TransactionRequest{
				TransactionID: uuid.New(),
				Type:          shared.TransactionTypeDeposit,
				Amount:        100,
			},
			wantErr: false,
		},
		{
			name: "valid withdrawal",
			request: &shared.TransactionRequest{
				TransactionID: uuid.New(),
				Type:          shared.TransactionTypeWithdrawal,
				Amount:        100,
			},
			wantErr: false,
		},
		{
			name: "invalid amount",
			request: &shared.TransactionRequest{
				TransactionID: uuid.New(),
				Type:          shared.TransactionTypeDeposit,
				Amount:        0,
			},
			wantErr: true,
		},
		{
			name: "invalid transaction type",
			request: &shared.TransactionRequest{
				TransactionID: uuid.New(),
				Type:          "invalid",
				Amount:        100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTransactionValidator_CheckIdempotency(t *testing.T) {
	mockRepo := &MockLedgerRepo{}
	logger := slog.Default()
	validator := NewTransactionValidator(mockRepo, logger)
	ctx := context.Background()

	completedEntry := &ledger.Entry{
		Status: shared.TransactionStatusCompleted,
	}

	pendingEntry := &ledger.Entry{
		Status: shared.TransactionStatusPending,
	}

	tests := []struct {
		name          string
		transactionID uuid.UUID
		setupMock     func()
		wantProcessed bool
		wantErr       bool
	}{
		{
			name:          "transaction not found",
			transactionID: uuid.New(),
			setupMock: func() {
				mockRepo.On("GetByTransactionID", ctx, mock.Anything).Return(nil, ledger.ErrEntryNotFound{}).Once()
			},
			wantProcessed: false,
			wantErr:       false,
		},
		{
			name:          "transaction already completed",
			transactionID: uuid.New(),
			setupMock: func() {
				mockRepo.On("GetByTransactionID", ctx, mock.Anything).Return(completedEntry, nil).Once()
			},
			wantProcessed: true,
			wantErr:       false,
		},
		{
			name:          "transaction pending",
			transactionID: uuid.New(),
			setupMock: func() {
				mockRepo.On("GetByTransactionID", ctx, mock.Anything).Return(pendingEntry, nil).Once()
			},
			wantProcessed: false,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			request := &shared.TransactionRequest{
				TransactionID: tt.transactionID,
			}
			processed, err := validator.CheckIdempotency(ctx, request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantProcessed, processed)
			mockRepo.AssertExpectations(t)
		})
	}
}
