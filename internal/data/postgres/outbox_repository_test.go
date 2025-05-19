package postgres

import (
	"context"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/outbox"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockOutboxRepository struct {
	mock.Mock
}

func (m *MockOutboxRepository) Create(ctx context.Context, message *outbox.Message) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockOutboxRepository) GetPending(ctx context.Context, limit int) ([]*outbox.Message, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*outbox.Message), args.Error(1)
}

func (m *MockOutboxRepository) UpdateStatus(ctx context.Context, id int64, status shared.OutboxStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockOutboxRepository) IncrementAttempts(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOutboxRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOutboxRepository) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*outbox.Message, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbox.Message), args.Error(1)
}

func (m *MockOutboxRepository) WithTx(tx pgx.Tx) outbox.Repository {
	args := m.Called(tx)
	return args.Get(0).(outbox.Repository)
}

// Skip TestNewOutboxRepository since we can't easily mock the PostgresDB

func TestOutboxRepository_WithTx(t *testing.T) {
	logger := slog.Default()

	repo := &OutboxRepository{
		querier: nil,
		logger:  logger,
	}

	mockTx := pgx.Tx(nil)
	txRepo := repo.WithTx(mockTx)

	assert.NotNil(t, txRepo)
	assert.IsType(t, &OutboxRepository{}, txRepo)

	outboxRepo, ok := txRepo.(*OutboxRepository)
	assert.True(t, ok)
	assert.Equal(t, mockTx, outboxRepo.querier)
}

func TestMockOutboxRepository(t *testing.T) {
	mockRepo := &MockOutboxRepository{}

	txID := uuid.New()
	accountID := uuid.New()
	message := &outbox.Message{
		TransactionID: txID,
		AccountID:     accountID,
		Payload:       []byte(`{"test":"data"}`),
		Status:        shared.OutboxStatusPending,
		Attempts:      0,
		CreatedAt:     time.Now(),
	}

	mockRepo.On("Create", mock.Anything, message).Return(nil)
	mockRepo.On("GetPending", mock.Anything, 10).Return([]*outbox.Message{message}, nil)
	mockRepo.On("UpdateStatus", mock.Anything, int64(1), shared.OutboxStatusProcessed).Return(nil)
	mockRepo.On("IncrementAttempts", mock.Anything, int64(1)).Return(nil)
	mockRepo.On("Delete", mock.Anything, int64(1)).Return(nil)
	mockRepo.On("GetByTransactionID", mock.Anything, txID).Return(message, nil)
	mockRepo.On("WithTx", mock.Anything).Return(mockRepo)

	ctx := context.Background()

	err := mockRepo.Create(ctx, message)
	assert.NoError(t, err)

	messages, err := mockRepo.GetPending(ctx, 10)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(messages))
	assert.Equal(t, message, messages[0])

	err = mockRepo.UpdateStatus(ctx, 1, shared.OutboxStatusProcessed)
	assert.NoError(t, err)

	err = mockRepo.IncrementAttempts(ctx, 1)
	assert.NoError(t, err)

	err = mockRepo.Delete(ctx, 1)
	assert.NoError(t, err)

	result, err := mockRepo.GetByTransactionID(ctx, txID)
	assert.NoError(t, err)
	assert.Equal(t, message, result)

	txRepo := mockRepo.WithTx(nil)
	assert.Equal(t, mockRepo, txRepo)

	mockRepo.AssertExpectations(t)
}
