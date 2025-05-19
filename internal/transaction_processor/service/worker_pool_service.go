package service

import (
	"context"
	"log/slog"
	"sync"

	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/panjf2000/ants/v2"
)

// WorkerPoolProcessingService implements the ProcessingService interface
type WorkerPoolProcessingService struct {
	baseService ProcessingService
	pool        *ants.Pool
	logger      *slog.Logger
	// Use a mutex to protect access to the results map
	mu      sync.Mutex
	results map[string]chan error
}

type WorkerPoolConfig struct {
	Size int
}

func NewWorkerPoolProcessingService(
	baseService ProcessingService,
	config WorkerPoolConfig,
	logger *slog.Logger,
) (*WorkerPoolProcessingService, error) {
	// Create a new worker pool with the specified size
	pool, err := ants.NewPool(config.Size)
	if err != nil {
		return nil, err
	}

	return &WorkerPoolProcessingService{
		baseService: baseService,
		pool:        pool,
		logger:      logger,
		results:     make(map[string]chan error),
	}, nil
}

// ProcessTransaction submits a transaction to the worker pool for processing.
func (s *WorkerPoolProcessingService) ProcessTransaction(ctx context.Context, request *shared.TransactionRequest) error {
	logger := s.logger
	if request.CorrelationID != "" {
		logger = s.logger.With("correlation_id", request.CorrelationID)
	}

	logger.Info("Submitting transaction to worker pool",
		"transaction_id", request.TransactionID.String(),
		"account_id", request.AccountID.String(),
	)

	// Create a channel to receive the result of the transaction processing
	resultChan := make(chan error, 1)

	// Store the result channel in the result map
	transactionID := request.TransactionID.String()
	s.mu.Lock()
	s.results[transactionID] = resultChan
	s.mu.Unlock()

	// Create a copy of the request to avoid data races
	requestCopy := *request

	// Submit the task to the worker pool
	err := s.pool.Submit(func() {
		// Process the transaction using the base service
		err := s.baseService.ProcessTransaction(ctx, &requestCopy)

		// Send the result to the channel
		resultChan <- err

		// Remove the result channel from the map
		s.mu.Lock()
		delete(s.results, transactionID)
		close(resultChan)
		s.mu.Unlock()
	})

	if err != nil {
		// If we couldn't submit the task to the pool, remove the result channel
		s.mu.Lock()
		delete(s.results, transactionID)
		close(resultChan)
		s.mu.Unlock()

		logger.Error("Failed to submit transaction to worker pool",
			"transaction_id", request.TransactionID.String(),
			"error", err,
		)
		return err
	}

	// Wait for the result from the worker
	return <-resultChan
}

// Shutdown gracefully shuts down the worker pool.
func (s *WorkerPoolProcessingService) Shutdown() {
	s.logger.Info("Shutting down worker pool", "running_workers", s.pool.Running())
	s.pool.Release()
}

// Running returns the number of running workers in the pool.
func (s *WorkerPoolProcessingService) Running() int {
	return s.pool.Running()
}

// Capacity returns the capacity of the worker pool.
func (s *WorkerPoolProcessingService) Capacity() int {
	return s.pool.Cap()
}
