package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/innoscripta-banking-ledger/internal/platform/messaging/producers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func (m *MockLedgerRepository) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*ledger.Entry, error) {
	args := m.Called(ctx, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ledger.Entry), args.Error(1)
}

func (m *MockLedgerRepository) GetByTimeRange(ctx context.Context, startTime, endTime time.Time, limit, offset int) ([]*ledger.Entry, error) {
	args := m.Called(ctx, startTime, endTime, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ledger.Entry), args.Error(1)
}

type MockMessagingProducer struct {
	mock.Mock
}

func (m *MockMessagingProducer) Publish(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockMessagingProducer) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestTransactionServiceImpl_CreateTransaction(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockLedgerRepo := new(MockLedgerRepository)
		mockProducer := new(MockMessagingProducer)
		service := NewTransactionService(logger, mockLedgerRepo, mockProducer)
		transactionRequest := &shared.TransactionRequest{
			AccountID:      uuid.New(),
			Type:           shared.TransactionTypeDeposit,
			Amount:         10000,
			Currency:       "USD",
			IdempotencyKey: uuid.New().String(),
		}

		mockProducer.On("Publish", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("*shared.TransactionRequest")).Return(nil).Once()
		mockLedgerRepo.On("GetByIdempotencyKey", ctx, transactionRequest.IdempotencyKey).Return(nil, nil).Once()

		requestIDStr, actualLedgerEntry, err := service.CreateTransaction(ctx, transactionRequest)

		assert.NoError(t, err)
		assert.NotEmpty(t, requestIDStr)
		_, parseErr := uuid.Parse(requestIDStr)
		assert.NoError(t, parseErr, "Returned request ID should be a valid UUID")
		assert.Nil(t, actualLedgerEntry)

		mockProducer.AssertExpectations(t)
		mockLedgerRepo.AssertExpectations(t)
	})

	t.Run("IdempotencyHit", func(t *testing.T) {
		mockLedgerRepo := new(MockLedgerRepository)
		mockProducer := new(MockMessagingProducer)
		service := NewTransactionService(logger, mockLedgerRepo, mockProducer)
		idempotencyKey := uuid.New().String()
		existingEntry := &ledger.Entry{
			TransactionID:  uuid.New(),
			AccountID:      uuid.New(),
			IdempotencyKey: idempotencyKey,
			Status:         shared.TransactionStatusCompleted,
		}
		transactionRequest := &shared.TransactionRequest{
			AccountID:      existingEntry.AccountID,
			Type:           shared.TransactionTypeDeposit,
			Amount:         10000,
			Currency:       "USD",
			IdempotencyKey: idempotencyKey,
		}

		mockLedgerRepo.On("GetByIdempotencyKey", ctx, idempotencyKey).Return(existingEntry, nil).Once()

		requestIDStr, actualLedgerEntry, err := service.CreateTransaction(ctx, transactionRequest)

		assert.NoError(t, err)
		assert.Equal(t, existingEntry.TransactionID.String(), requestIDStr)
		assert.Equal(t, existingEntry, actualLedgerEntry)
		mockProducer.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
		mockLedgerRepo.AssertExpectations(t)
	})

	t.Run("ValidationError", func(t *testing.T) {
		mockLedgerRepo := new(MockLedgerRepository)
		mockProducer := new(MockMessagingProducer)
		service := NewTransactionService(logger, mockLedgerRepo, mockProducer)
		transactionRequest := &shared.TransactionRequest{
			AccountID:      uuid.New(),
			Type:           shared.TransactionType("INVALID"), // Invalid type
			Amount:         10000,
			Currency:       "USD",
			IdempotencyKey: uuid.New().String(),
		}
		mockLedgerRepo.On("GetByIdempotencyKey", ctx, transactionRequest.IdempotencyKey).Return(nil, nil).Once()
		mockProducer.On("Publish", ctx, mock.AnythingOfType("string"), transactionRequest).Return(nil).Once()

		requestIDStr, actualLedgerEntry, err := service.CreateTransaction(ctx, transactionRequest)

		assert.NoError(t, err)
		assert.NotEmpty(t, requestIDStr)
		assert.Nil(t, actualLedgerEntry)

		mockProducer.AssertExpectations(t)
		mockLedgerRepo.AssertExpectations(t)
	})

	t.Run("ProducerPublishError", func(t *testing.T) {
		mockLedgerRepo := new(MockLedgerRepository)
		mockProducer := new(MockMessagingProducer)
		service := NewTransactionService(logger, mockLedgerRepo, mockProducer)
		transactionRequest := &shared.TransactionRequest{
			AccountID:      uuid.New(),
			Type:           shared.TransactionTypeWithdrawal,
			Amount:         5000,
			Currency:       "EUR",
			IdempotencyKey: uuid.New().String(),
		}
		publishError := errors.New("kafka unavailable")

		mockLedgerRepo.On("GetByIdempotencyKey", ctx, transactionRequest.IdempotencyKey).Return(nil, nil).Once()
		mockProducer.On("Publish", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("*shared.TransactionRequest")).Return(publishError).Once()

		requestIDStr, actualLedgerEntry, err := service.CreateTransaction(ctx, transactionRequest)

		assert.Error(t, err)
		assert.Empty(t, requestIDStr)
		assert.Nil(t, actualLedgerEntry)
		assert.True(t, errors.Is(err, publishError) || err.Error() == publishError.Error())
		mockProducer.AssertExpectations(t)
		mockLedgerRepo.AssertExpectations(t)
	})
}

func TestTransactionServiceImpl_GetTransactionByID(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockLedgerRepo := new(MockLedgerRepository)
		mockProducer := new(MockMessagingProducer)
		service := NewTransactionService(logger, mockLedgerRepo, mockProducer)
		transactionID := uuid.New()
		expectedEntry := &ledger.Entry{
			TransactionID: transactionID,
			AccountID:     uuid.New(),
			Type:          shared.TransactionTypeDeposit,
			Amount:        12345,
			Currency:      "CAD",
			Status:        shared.TransactionStatusPending,
			CreatedAt:     time.Now(),
		}

		mockLedgerRepo.On("GetByTransactionID", ctx, transactionID).Return(expectedEntry, nil).Once()

		entry, err := service.GetTransactionByID(ctx, transactionID)

		assert.NoError(t, err)
		assert.NotNil(t, entry)
		assert.Equal(t, expectedEntry, entry)
		mockLedgerRepo.AssertExpectations(t)
	})

	t.Run("NotFound", func(t *testing.T) {
		mockLedgerRepo := new(MockLedgerRepository)
		mockProducer := new(MockMessagingProducer)
		service := NewTransactionService(logger, mockLedgerRepo, mockProducer)
		transactionID := uuid.New()
		notFoundError := errors.New("entry not found")

		mockLedgerRepo.On("GetByTransactionID", ctx, transactionID).Return(nil, notFoundError).Once()

		entry, err := service.GetTransactionByID(ctx, transactionID)

		assert.Error(t, err)
		assert.Nil(t, entry)
		assert.Equal(t, notFoundError, err) // Or errors.Is(err, ledger.ErrEntryNotFound)
		mockLedgerRepo.AssertExpectations(t)
	})
}

func TestTransactionServiceImpl_GetTransactionsByAccountID(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	accountID := uuid.New()
	page := 1
	perPage := 10
	offset := 0

	t.Run("Success", func(t *testing.T) {
		mockLedgerRepo := new(MockLedgerRepository)
		mockProducer := new(MockMessagingProducer)
		service := NewTransactionService(logger, mockLedgerRepo, mockProducer)
		expectedEntries := []*ledger.Entry{
			{TransactionID: uuid.New(), AccountID: accountID, Amount: 100},
			{TransactionID: uuid.New(), AccountID: accountID, Amount: 200},
		}
		var expectedTotal int64 = 2

		mockLedgerRepo.On("GetByAccountID", ctx, accountID, perPage, offset).Return(expectedEntries, nil).Once()
		mockLedgerRepo.On("CountByAccountID", ctx, accountID).Return(expectedTotal, nil).Once()

		entries, total, err := service.GetTransactionsByAccountID(ctx, accountID, page, perPage)

		assert.NoError(t, err)
		assert.Equal(t, expectedEntries, entries)
		assert.Equal(t, expectedTotal, total)
		mockLedgerRepo.AssertExpectations(t)
	})

	t.Run("GetByAccountIDError", func(t *testing.T) {
		mockLedgerRepo := new(MockLedgerRepository)
		mockProducer := new(MockMessagingProducer)
		service := NewTransactionService(logger, mockLedgerRepo, mockProducer)
		getError := errors.New("db get error")
		mockLedgerRepo.On("GetByAccountID", ctx, accountID, perPage, offset).Return(nil, getError).Once()

		entries, total, err := service.GetTransactionsByAccountID(ctx, accountID, page, perPage)

		assert.Error(t, err)
		assert.Nil(t, entries)
		assert.Zero(t, total)
		assert.Equal(t, getError, err)
		mockLedgerRepo.AssertExpectations(t)
		mockLedgerRepo.AssertNotCalled(t, "CountByAccountID", ctx, accountID)
	})

	t.Run("CountByAccountIDError", func(t *testing.T) {
		mockLedgerRepo := new(MockLedgerRepository)
		mockProducer := new(MockMessagingProducer)
		service := NewTransactionService(logger, mockLedgerRepo, mockProducer)
		expectedEntries := []*ledger.Entry{
			{TransactionID: uuid.New(), AccountID: accountID},
		}
		countError := errors.New("db count error")

		mockLedgerRepo.On("GetByAccountID", ctx, accountID, perPage, offset).Return(expectedEntries, nil).Once()
		mockLedgerRepo.On("CountByAccountID", ctx, accountID).Return(int64(0), countError).Once()

		entries, total, err := service.GetTransactionsByAccountID(ctx, accountID, page, perPage)

		assert.Error(t, err)
		assert.Nil(t, entries)
		assert.Zero(t, total)
		assert.Equal(t, countError, err)
		mockLedgerRepo.AssertExpectations(t)
	})
}

var _ ledger.Repository = (*MockLedgerRepository)(nil)
var _ producers.MessagePublisher = (*MockMessagingProducer)(nil)
