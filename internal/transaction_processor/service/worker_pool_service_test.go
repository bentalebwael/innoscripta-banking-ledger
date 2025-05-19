package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockProcessingService mocks the ProcessingService interface
type MockProcessingService struct {
	mock.Mock
}

func (m *MockProcessingService) ProcessTransaction(ctx context.Context, request *shared.TransactionRequest) error {
	args := m.Called(ctx, request)
	return args.Error(0)
}

func TestWorkerPoolProcessingService_ProcessTransaction(t *testing.T) {
	// Create mocks
	mockBaseService := &MockProcessingService{}
	logger := slog.Default()

	// Create a test request
	txID := uuid.New()
	accountID := uuid.New()
	request := &shared.TransactionRequest{
		TransactionID:  txID,
		AccountID:      accountID,
		Type:           shared.TransactionTypeDeposit,
		Amount:         100,
		Currency:       "USD",
		IdempotencyKey: "key1",
		CorrelationID:  "corr1",
	}

	// Create a worker pool service with a small pool size
	workerPoolService, err := NewWorkerPoolProcessingService(
		mockBaseService,
		WorkerPoolConfig{
			Size: 2,
		},
		logger,
	)
	assert.NoError(t, err)
	defer workerPoolService.Shutdown()

	// Test cases
	tests := []struct {
		name          string
		setupMocks    func()
		expectedError error
	}{
		{
			name: "successful processing",
			setupMocks: func() {
				mockBaseService.On("ProcessTransaction", mock.Anything, request).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name: "processing error",
			setupMocks: func() {
				mockBaseService.On("ProcessTransaction", mock.Anything, request).Return(errors.New("processing error")).Once()
			},
			expectedError: errors.New("processing error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks for each test
			mockBaseService = &MockProcessingService{}

			// Create a new worker pool service for each test
			workerPoolService, err := NewWorkerPoolProcessingService(
				mockBaseService,
				WorkerPoolConfig{
					Size: 2,
				},
				logger,
			)
			assert.NoError(t, err)
			defer workerPoolService.Shutdown()

			tt.setupMocks()
			ctx := context.Background()

			// Call the service
			err = workerPoolService.ProcessTransaction(ctx, request)

			// Check the result
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			// Verify that all expected mock calls were made
			mockBaseService.AssertExpectations(t)
		})
	}
}

func TestWorkerPoolProcessingService_Concurrency(t *testing.T) {
	// Create mocks
	mockBaseService := &MockProcessingService{}
	logger := slog.Default()

	// Create a worker pool service with a small pool size
	workerPoolService, err := NewWorkerPoolProcessingService(
		mockBaseService,
		WorkerPoolConfig{
			Size: 5,
		},
		logger,
	)
	assert.NoError(t, err)
	defer workerPoolService.Shutdown()

	// Create a mutex to protect access to the counter
	var mu sync.Mutex
	counter := 0

	// Setup the mock to increment the counter
	mockBaseService.On("ProcessTransaction", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		// Increment the counter
		mu.Lock()
		counter++
		mu.Unlock()
	}).Return(nil)

	// Create multiple requests
	numRequests := 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	// Process the requests concurrently
	for i := 0; i < numRequests; i++ {
		go func(i int) {
			defer wg.Done()

			// Create a unique request
			request := &shared.TransactionRequest{
				TransactionID:  uuid.New(),
				AccountID:      uuid.New(),
				Type:           shared.TransactionTypeDeposit,
				Amount:         100,
				Currency:       "USD",
				IdempotencyKey: "key" + string(rune(i)),
				CorrelationID:  "corr" + string(rune(i)),
			}

			// Process the request
			ctx := context.Background()
			err := workerPoolService.ProcessTransaction(ctx, request)
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all requests to be processed
	wg.Wait()

	// Verify that all requests were processed
	assert.Equal(t, numRequests, counter)

	// Verify that the worker pool is still running
	assert.True(t, workerPoolService.Running() > 0)
	assert.Equal(t, 5, workerPoolService.Capacity())
}
