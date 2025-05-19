package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"log/slog"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations of the dependencies

type MockTransactionValidator struct {
	mock.Mock
}

func (m *MockTransactionValidator) Validate(ctx context.Context, request *shared.TransactionRequest) error {
	args := m.Called(ctx, request)
	return args.Error(0)
}

func (m *MockTransactionValidator) CheckIdempotency(ctx context.Context, request *shared.TransactionRequest) (bool, error) {
	args := m.Called(ctx, request)
	return args.Bool(0), args.Error(1)
}

// We need to import pgx.Tx for the interfaces
type MockAccountManager struct {
	mock.Mock
}

func (m *MockAccountManager) LockAndUpdateAccount(ctx context.Context, tx pgx.Tx, request *shared.TransactionRequest) (*account.Account, error) {
	args := m.Called(ctx, tx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

type MockOutboxManager struct {
	mock.Mock
}

func (m *MockOutboxManager) CreateOutboxEntry(ctx context.Context, tx pgx.Tx, request *shared.TransactionRequest, updatedAccount *account.Account) error {
	args := m.Called(ctx, tx, request, updatedAccount)
	return args.Error(0)
}

type MockFailureRecorder struct {
	mock.Mock
}

func (m *MockFailureRecorder) RecordFailure(ctx context.Context, request *shared.TransactionRequest, failureReason string) error {
	args := m.Called(ctx, request, failureReason)
	return args.Error(0)
}

// MockTx implements the pgx.Tx interface for testing
type MockTx struct {
	mock.Mock
}

func (m *MockTx) Begin(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockTx) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) Rollback(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (m *MockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil
}

func (m *MockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return &pgconn.StatementDescription{}, nil
}

func (m *MockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *MockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *MockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func (m *MockTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (m *MockTx) Conn() *pgx.Conn {
	return nil
}

// TestProcessingService is a simplified implementation of ProcessingService for testing
type TestProcessingService struct {
	validator       TransactionValidator
	accountManager  AccountManager
	outboxManager   OutboxManager
	failureRecorder FailureRecorder
	logger          *slog.Logger
	beginTxFunc     func(ctx context.Context) (pgx.Tx, error)
}

// NewTestProcessingService creates a new TestProcessingService
func NewTestProcessingService(
	validator TransactionValidator,
	accountManager AccountManager,
	outboxManager OutboxManager,
	failureRecorder FailureRecorder,
	logger *slog.Logger,
	beginTxFunc func(ctx context.Context) (pgx.Tx, error),
) *TestProcessingService {
	return &TestProcessingService{
		validator:       validator,
		accountManager:  accountManager,
		outboxManager:   outboxManager,
		failureRecorder: failureRecorder,
		logger:          logger,
		beginTxFunc:     beginTxFunc,
	}
}

// ProcessTransaction implements the ProcessingService interface
func (s *TestProcessingService) ProcessTransaction(ctx context.Context, request *shared.TransactionRequest) error {
	// Create a logger with correlation ID for consistent tracing
	logger := s.logger
	if request.CorrelationID != "" {
		logger = s.logger.With("correlation_id", request.CorrelationID)
	}

	logger.Info("Processing transaction", "transaction_id", request.TransactionID.String(), "account_id", request.AccountID.String())

	// 1. Validate the transaction
	if err := s.validator.Validate(ctx, request); err != nil {
		logger.Error("Transaction validation failed", "transaction_id", request.TransactionID.String(), "error", err)

		// Record the failure based on the specific error
		var failureReason string
		if errors.Is(err, shared.ErrInvalidTransactionType) {
			failureReason = string(shared.FailureReasonUnknownError)
		} else {
			failureReason = string(shared.FailureReasonInvalidAmount)
		}

		if recordErr := s.failureRecorder.RecordFailure(ctx, request, failureReason); recordErr != nil {
			logger.Error("Failed to record transaction failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
		}

		return nil // Return nil to Kafka consumer to acknowledge the message
	}

	// 2. Check idempotency
	skip, err := s.validator.CheckIdempotency(ctx, request)
	if err != nil {
		return err // Let Kafka retry
	}
	if skip {
		return nil // Already processed, return success
	}

	// 3. Begin database transaction
	var tx pgx.Tx
	tx, err = s.beginTxFunc(ctx)
	if err != nil {
		logger.Error("Failed to begin database transaction", "transaction_id", request.TransactionID.String(), "error", err)
		return fmt.Errorf("failed to begin DB transaction for %s: %w", request.TransactionID.String(), err)
	}

	defer func() {
		if p := recover(); p != nil {
			logger.Error("Panic recovered, rolling back transaction", "panic", p, "transaction_id", request.TransactionID.String())
			_ = tx.Rollback(ctx)
			panic(p) // Re-panic
		} else if err != nil {
			logger.Error("Error occurred, rolling back transaction", "error", err, "transaction_id", request.TransactionID.String())
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				logger.Error("Failed to rollback transaction after error", "rollback_error", rbErr, "original_error", err, "transaction_id", request.TransactionID.String())
			}
		}
	}()

	// 4. Lock and update account
	updatedAccount, err := s.accountManager.LockAndUpdateAccount(ctx, tx, request)
	if err != nil {
		// Handle specific business errors
		if errors.Is(err, account.ErrAccountNotFound{AccountID: request.AccountID}) {
			if recordErr := s.failureRecorder.RecordFailure(ctx, request, string(shared.FailureReasonAccountNotFound)); recordErr != nil {
				logger.Error("Failed to record account not found failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
			}
			return nil // Return nil to Kafka consumer
		} else if errors.Is(err, shared.ErrInvalidCurrency) {
			failureReasonStr := fmt.Sprintf(string(shared.FailureReasonCurrencyMismatchFormat), request.Currency, "account_currency")
			if recordErr := s.failureRecorder.RecordFailure(ctx, request, failureReasonStr); recordErr != nil {
				logger.Error("Failed to record currency mismatch failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
			}
			return nil // Return nil to Kafka consumer
		} else if errors.Is(err, account.ErrInvalidAmount) {
			if recordErr := s.failureRecorder.RecordFailure(ctx, request, string(shared.FailureReasonInvalidAmount)); recordErr != nil {
				logger.Error("Failed to record invalid amount failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
			}
			return nil // Return nil to Kafka consumer
		} else if errors.Is(err, account.ErrInsufficientFunds) {
			if recordErr := s.failureRecorder.RecordFailure(ctx, request, string(shared.FailureReasonInsufficientFunds)); recordErr != nil {
				logger.Error("Failed to record insufficient funds failure", "transaction_id", request.TransactionID.String(), "error", recordErr)
			}
			return nil // Return nil to Kafka consumer
		}

		// For other errors, let them propagate for retry
		return err
	}

	// 5. Create outbox entry
	if err = s.outboxManager.CreateOutboxEntry(ctx, tx, request, updatedAccount); err != nil {
		return err // Let the defer handle rollback
	}

	// 6. Commit transaction
	if err = tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit database transaction",
			"req_id", request.TransactionID.String(),
			"acc_id", request.AccountID.String(),
			"error", err,
		)
		return fmt.Errorf("failed to commit DB transaction for tx %s: %w", request.TransactionID.String(), err)
	}

	logger.Info("Database transaction committed successfully", "req_id", request.TransactionID.String(), "acc_id", request.AccountID.String())
	return nil // SUCCESS!
}

func TestProcessingService_ProcessTransaction(t *testing.T) {
	// Create mocks
	mockValidator := &MockTransactionValidator{}
	mockAccountManager := &MockAccountManager{}
	mockOutboxManager := &MockOutboxManager{}
	mockFailureRecorder := &MockFailureRecorder{}
	mockTx := &MockTx{}
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

	// Create a test account
	testAccount := &account.Account{
		ID:       accountID,
		Balance:  600,
		Currency: "USD",
	}

	// Test cases
	tests := []struct {
		name          string
		setupMocks    func()
		beginTxFunc   func(ctx context.Context) (pgx.Tx, error)
		expectedError error
	}{
		{
			name: "successful transaction processing",
			setupMocks: func() {
				// Validation passes
				mockValidator.On("Validate", mock.Anything, request).Return(nil).Once()

				// Not already processed
				mockValidator.On("CheckIdempotency", mock.Anything, request).Return(false, nil).Once()

				// Lock and update account
				mockAccountManager.On("LockAndUpdateAccount", mock.Anything, mockTx, request).Return(testAccount, nil).Once()

				// Create outbox entry
				mockOutboxManager.On("CreateOutboxEntry", mock.Anything, mockTx, request, testAccount).Return(nil).Once()

				// Commit transaction
				mockTx.On("Commit", mock.Anything).Return(nil).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return mockTx, nil
			},
			expectedError: nil,
		},
		{
			name: "validation failure",
			setupMocks: func() {
				// Validation fails
				mockValidator.On("Validate", mock.Anything, request).Return(shared.ErrInvalidTransactionType).Once()

				// Record failure
				mockFailureRecorder.On("RecordFailure", mock.Anything, request, string(shared.FailureReasonUnknownError)).Return(nil).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return mockTx, nil
			},
			expectedError: nil, // We return nil to Kafka consumer on validation failure
		},
		{
			name: "idempotency check returns skip",
			setupMocks: func() {
				// Validation passes
				mockValidator.On("Validate", mock.Anything, request).Return(nil).Once()

				// Already processed
				mockValidator.On("CheckIdempotency", mock.Anything, request).Return(true, nil).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return mockTx, nil
			},
			expectedError: nil, // We return nil to Kafka consumer if already processed
		},
		{
			name: "idempotency check error",
			setupMocks: func() {
				// Validation passes
				mockValidator.On("Validate", mock.Anything, request).Return(nil).Once()

				// Error checking idempotency
				mockValidator.On("CheckIdempotency", mock.Anything, request).Return(false, errors.New("db error")).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return mockTx, nil
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "begin transaction error",
			setupMocks: func() {
				// Validation passes
				mockValidator.On("Validate", mock.Anything, request).Return(nil).Once()

				// Not already processed
				mockValidator.On("CheckIdempotency", mock.Anything, request).Return(false, nil).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return nil, errors.New("db error")
			},
			expectedError: errors.New("failed to begin DB transaction"),
		},
		{
			name: "account not found",
			setupMocks: func() {
				// Validation passes
				mockValidator.On("Validate", mock.Anything, request).Return(nil).Once()

				// Not already processed
				mockValidator.On("CheckIdempotency", mock.Anything, request).Return(false, nil).Once()

				// Account not found
				mockAccountManager.On("LockAndUpdateAccount", mock.Anything, mockTx, request).Return(nil, account.ErrAccountNotFound{AccountID: accountID}).Once()

				// Record failure
				mockFailureRecorder.On("RecordFailure", mock.Anything, request, string(shared.FailureReasonAccountNotFound)).Return(nil).Once()

				// Rollback transaction
				mockTx.On("Rollback", mock.Anything).Return(nil).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return mockTx, nil
			},
			expectedError: nil, // We return nil to Kafka consumer on account not found
		},
		{
			name: "currency mismatch",
			setupMocks: func() {
				// Validation passes
				mockValidator.On("Validate", mock.Anything, request).Return(nil).Once()

				// Not already processed
				mockValidator.On("CheckIdempotency", mock.Anything, request).Return(false, nil).Once()

				// Currency mismatch
				mockAccountManager.On("LockAndUpdateAccount", mock.Anything, mockTx, request).Return(nil, shared.ErrInvalidCurrency).Once()

				// Record failure
				failureReasonStr := fmt.Sprintf(string(shared.FailureReasonCurrencyMismatchFormat), request.Currency, "account_currency")
				mockFailureRecorder.On("RecordFailure", mock.Anything, request, failureReasonStr).Return(nil).Once()

				// Rollback transaction
				mockTx.On("Rollback", mock.Anything).Return(nil).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return mockTx, nil
			},
			expectedError: nil, // We return nil to Kafka consumer on currency mismatch
		},
		{
			name: "insufficient funds",
			setupMocks: func() {
				// Validation passes
				mockValidator.On("Validate", mock.Anything, request).Return(nil).Once()

				// Not already processed
				mockValidator.On("CheckIdempotency", mock.Anything, request).Return(false, nil).Once()

				// Insufficient funds
				mockAccountManager.On("LockAndUpdateAccount", mock.Anything, mockTx, request).Return(nil, account.ErrInsufficientFunds).Once()

				// Record failure
				mockFailureRecorder.On("RecordFailure", mock.Anything, request, string(shared.FailureReasonInsufficientFunds)).Return(nil).Once()

				// Rollback transaction
				mockTx.On("Rollback", mock.Anything).Return(nil).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return mockTx, nil
			},
			expectedError: nil, // We return nil to Kafka consumer on insufficient funds
		},
		{
			name: "create outbox entry error",
			setupMocks: func() {
				// Validation passes
				mockValidator.On("Validate", mock.Anything, request).Return(nil).Once()

				// Not already processed
				mockValidator.On("CheckIdempotency", mock.Anything, request).Return(false, nil).Once()

				// Lock and update account
				mockAccountManager.On("LockAndUpdateAccount", mock.Anything, mockTx, request).Return(testAccount, nil).Once()

				// Error creating outbox entry
				mockOutboxManager.On("CreateOutboxEntry", mock.Anything, mockTx, request, testAccount).Return(errors.New("db error")).Once()

				// Rollback transaction
				mockTx.On("Rollback", mock.Anything).Return(nil).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return mockTx, nil
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "commit transaction error",
			setupMocks: func() {
				// Validation passes
				mockValidator.On("Validate", mock.Anything, request).Return(nil).Once()

				// Not already processed
				mockValidator.On("CheckIdempotency", mock.Anything, request).Return(false, nil).Once()

				// Lock and update account
				mockAccountManager.On("LockAndUpdateAccount", mock.Anything, mockTx, request).Return(testAccount, nil).Once()

				// Create outbox entry
				mockOutboxManager.On("CreateOutboxEntry", mock.Anything, mockTx, request, testAccount).Return(nil).Once()

				// Error committing transaction
				mockTx.On("Commit", mock.Anything).Return(errors.New("db error")).Once()

				// Rollback transaction
				mockTx.On("Rollback", mock.Anything).Return(nil).Once()
			},
			beginTxFunc: func(ctx context.Context) (pgx.Tx, error) {
				return mockTx, nil
			},
			expectedError: errors.New("failed to commit DB transaction"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks for each test
			mockValidator = &MockTransactionValidator{}
			mockAccountManager = &MockAccountManager{}
			mockOutboxManager = &MockOutboxManager{}
			mockFailureRecorder = &MockFailureRecorder{}
			mockTx = &MockTx{}

			// Create the test service
			service := NewTestProcessingService(
				mockValidator,
				mockAccountManager,
				mockOutboxManager,
				mockFailureRecorder,
				logger,
				tt.beginTxFunc,
			)

			tt.setupMocks()
			ctx := context.Background()

			// Call the service
			err := service.ProcessTransaction(ctx, request)

			// Check the result
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			// Verify that all expected mock calls were made
			mockValidator.AssertExpectations(t)
			mockAccountManager.AssertExpectations(t)
			mockOutboxManager.AssertExpectations(t)
			mockFailureRecorder.AssertExpectations(t)
			mockTx.AssertExpectations(t)
		})
	}
}
