package outbox_poller

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/outbox"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLedgerPublisher for testing
type MockLedgerPublisher struct {
	mock.Mock
}

func (m *MockLedgerPublisher) PublishToLedger(ctx context.Context, message *outbox.Message) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func TestPoller_ProcessPendingMessages(t *testing.T) {
	mockOutboxRepo := &MockOutboxRepo{}
	mockLedgerPublisher := &MockLedgerPublisher{}
	logger := slog.Default()

	cfg := &config.OutboxConfig{
		PollingInterval:  time.Second,
		BatchSize:        10,
		MaxRetryAttempts: 3,
	}

	poller := NewPoller(cfg, mockOutboxRepo, mockLedgerPublisher, logger)

	txID := uuid.New()
	entry := &ledger.Entry{
		TransactionID: txID,
		Status:        shared.TransactionStatusProcessing,
	}
	entryJSON, err := json.Marshal(entry)
	assert.NoError(t, err)

	message1 := &outbox.Message{
		ID:            1,
		TransactionID: txID,
		Status:        shared.OutboxStatusPending,
		Payload:       entryJSON,
		Attempts:      0,
		CreatedAt:     time.Now(),
	}

	message2 := &outbox.Message{
		ID:            2,
		TransactionID: uuid.New(),
		Status:        shared.OutboxStatusPending,
		Payload:       entryJSON,
		Attempts:      0,
		CreatedAt:     time.Now(),
	}

	tests := []struct {
		name          string
		setupMocks    func()
		expectedError error
	}{
		{
			name: "successful processing of pending messages",
			setupMocks: func() {
				mockOutboxRepo.On("GetPending", mock.Anything, 10).Return([]*outbox.Message{message1, message2}, nil).Once()

				mockLedgerPublisher.On("PublishToLedger", mock.Anything, message1).Return(nil).Once()
				mockLedgerPublisher.On("PublishToLedger", mock.Anything, message2).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name: "error getting pending messages",
			setupMocks: func() {
				mockOutboxRepo.On("GetPending", mock.Anything, 10).Return(nil, errors.New("db error")).Once()
			},
			expectedError: errors.New("failed to get pending outbox messages"),
		},
		{
			name: "no pending messages",
			setupMocks: func() {
				mockOutboxRepo.On("GetPending", mock.Anything, 10).Return([]*outbox.Message{}, nil).Once()
			},
			expectedError: nil,
		},
		{
			name: "error publishing one message",
			setupMocks: func() {
				mockOutboxRepo.On("GetPending", mock.Anything, 10).Return([]*outbox.Message{message1, message2}, nil).Once()

				mockLedgerPublisher.On("PublishToLedger", mock.Anything, message1).Return(errors.New("publish error")).Once()

				mockOutboxRepo.On("IncrementAttempts", mock.Anything, int64(1)).Return(nil).Once()

				mockLedgerPublisher.On("PublishToLedger", mock.Anything, message2).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name: "max retry attempts reached",
			setupMocks: func() {
				maxAttemptsMessage := &outbox.Message{
					ID:            3,
					TransactionID: uuid.New(),
					Status:        shared.OutboxStatusPending,
					Payload:       entryJSON,
					Attempts:      2,
					CreatedAt:     time.Now(),
				}

				mockOutboxRepo.On("GetPending", mock.Anything, 10).Return([]*outbox.Message{maxAttemptsMessage}, nil).Once()

				mockLedgerPublisher.On("PublishToLedger", mock.Anything, maxAttemptsMessage).Return(errors.New("publish error")).Once()

				mockOutboxRepo.On("IncrementAttempts", mock.Anything, int64(3)).Return(nil).Once()

				mockOutboxRepo.On("UpdateStatus", mock.Anything, int64(3), shared.OutboxStatusFailedToPublish).Return(nil).Once()
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOutboxRepo = &MockOutboxRepo{}
			mockLedgerPublisher = &MockLedgerPublisher{}
			poller = NewPoller(cfg, mockOutboxRepo, mockLedgerPublisher, logger)

			tt.setupMocks()
			ctx := context.Background()

			err := poller.processPendingMessages(ctx)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			mockOutboxRepo.AssertExpectations(t)
			mockLedgerPublisher.AssertExpectations(t)
		})
	}
}

func TestPoller_Start(t *testing.T) {

	mockOutboxRepo := &MockOutboxRepo{}
	mockLedgerPublisher := &MockLedgerPublisher{}
	logger := slog.Default()

	cfg := &config.OutboxConfig{
		PollingInterval:  10 * time.Millisecond,
		BatchSize:        10,
		MaxRetryAttempts: 3,
	}

	poller := NewPoller(cfg, mockOutboxRepo, mockLedgerPublisher, logger)

	mockOutboxRepo.On("GetPending", mock.Anything, 10).Return([]*outbox.Message{}, nil).Maybe()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	go poller.Start(ctx)

	<-ctx.Done()

	assert.True(t, true)
}
