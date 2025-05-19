package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockProcessingService for testing
type MockProcessingService struct {
	mock.Mock
}

func (m *MockProcessingService) ProcessTransaction(ctx context.Context, request *shared.TransactionRequest) error {
	args := m.Called(ctx, request)
	return args.Error(0)
}

// MockDeadLetterPublisher for testing
type MockDeadLetterPublisher struct {
	mock.Mock
}

func (m *MockDeadLetterPublisher) PublishToDLQ(ctx context.Context, key string, value []byte, reason string) error {
	args := m.Called(ctx, key, value, reason)
	return args.Error(0)
}

func (m *MockDeadLetterPublisher) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestHandleMessage(t *testing.T) {
	mockProcessingService := &MockProcessingService{}
	mockDLQPublisher := &MockDeadLetterPublisher{}
	logger := slog.Default()

	handler := NewTransactionEventHandler(logger, mockProcessingService, mockDLQPublisher)

	validRequest := &shared.TransactionRequest{
		TransactionID:  uuid.New(),
		AccountID:      uuid.New(),
		Type:           shared.TransactionTypeDeposit,
		Amount:         100,
		Currency:       "USD",
		IdempotencyKey: "key1",
		CorrelationID:  "corr1",
		Timestamp:      time.Now(),
	}

	validJSON, err := json.Marshal(validRequest)
	assert.NoError(t, err)

	tests := []struct {
		name          string
		key           []byte
		value         []byte
		setupMocks    func()
		expectedError error
	}{
		{
			name:  "successful processing",
			key:   []byte("test-key"),
			value: validJSON,
			setupMocks: func() {
				mockProcessingService.On("ProcessTransaction", mock.Anything, mock.MatchedBy(func(req *shared.TransactionRequest) bool {
					return req.TransactionID == validRequest.TransactionID
				})).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:  "processing error",
			key:   []byte("test-key"),
			value: validJSON,
			setupMocks: func() {
				mockProcessingService.On("ProcessTransaction", mock.Anything, mock.Anything).Return(errors.New("processing error"))
			},
			expectedError: errors.New("processing transaction"),
		},
		{
			name:  "unmarshal error with successful DLQ publish",
			key:   []byte("test-key"),
			value: []byte("invalid json"),
			setupMocks: func() {
				mockDLQPublisher.On("PublishToDLQ", mock.Anything, "test-key", []byte("invalid json"), mock.Anything).Return(nil)
			},
			expectedError: nil, // No error because message was successfully sent to DLQ
		},
		{
			name:  "unmarshal error with DLQ publish failure",
			key:   []byte("test-key"),
			value: []byte("invalid json"),
			setupMocks: func() {
				mockDLQPublisher.On("PublishToDLQ", mock.Anything, "test-key", []byte("invalid json"), mock.Anything).Return(errors.New("dlq error"))
			},
			expectedError: errors.New("failed to unmarshal"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProcessingService = &MockProcessingService{}
			mockDLQPublisher = &MockDeadLetterPublisher{}
			mockDLQPublisher.On("Close").Return(nil).Maybe()

			handler = NewTransactionEventHandler(logger, mockProcessingService, mockDLQPublisher)

			tt.setupMocks()
			ctx := context.Background()

			err := handler.HandleMessage(ctx, tt.key, tt.value)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			mockProcessingService.AssertExpectations(t)
			mockDLQPublisher.AssertExpectations(t)
		})
	}
}
