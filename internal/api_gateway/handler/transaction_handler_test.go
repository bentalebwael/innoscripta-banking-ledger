package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/service"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// PaginatedResponse is a generic version of Response for testing paginated data
type PaginatedResponse[T any] struct {
	Data          []T        `json:"data"`
	Error         *ErrorInfo `json:"error,omitempty"`
	CorrelationID string     `json:"correlation_id,omitempty"`
	Meta          *MetaInfo  `json:"meta,omitempty"`
}

type MockTransactionService struct {
	mock.Mock
}

func (m *MockTransactionService) CreateTransaction(ctx context.Context, transactionRequest *shared.TransactionRequest) (string, *ledger.Entry, error) {
	args := m.Called(ctx, transactionRequest)
	var entry *ledger.Entry
	if args.Get(1) != nil {
		entry = args.Get(1).(*ledger.Entry)
	}
	return args.String(0), entry, args.Error(2)
}

func (m *MockTransactionService) GetTransactionByID(ctx context.Context, transactionID uuid.UUID) (*ledger.Entry, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ledger.Entry), args.Error(1)
}

func (m *MockTransactionService) GetTransactionsByAccountID(ctx context.Context, accountID uuid.UUID, page, perPage int) ([]*ledger.Entry, int64, error) {
	args := m.Called(ctx, accountID, page, perPage)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*ledger.Entry), args.Get(1).(int64), args.Error(2)
}

func TestTransactionHandler_Create(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)

		expectedTransactionID := uuid.New().String()
		mockService.On("CreateTransaction", mock.Anything, mock.MatchedBy(func(req *shared.TransactionRequest) bool {
			return req.Type == shared.TransactionTypeDeposit && req.Amount == 10000 && req.Currency == "USD"
		})).Return(expectedTransactionID, nil, nil)

		router := gin.Default()
		router.POST("/transactions", handler.Create)

		accountID := uuid.New()
		idempotencyKey := uuid.New().String()
		reqBody := CreateTransactionRequest{
			AccountID:      accountID.String(),
			Type:           "DEPOSIT",
			Amount:         10000, // Cents
			Currency:       "USD",
			IdempotencyKey: idempotencyKey,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest(http.MethodPost, "/transactions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusAccepted, rr.Code)

		var topLevelResponse map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &topLevelResponse)
		assert.NoError(t, err, "Failed to unmarshal top-level response")

		dataField, ok := topLevelResponse["data"]
		assert.True(t, ok, "'data' field should exist in response")

		responseBody, ok := dataField.(map[string]interface{})
		assert.True(t, ok, "'data' field should be a map")

		assert.Equal(t, expectedTransactionID, responseBody["transaction_id"])
		assert.Equal(t, "PENDING", responseBody["status"])

		mockService.AssertExpectations(t)
	})

	t.Run("InvalidRequestBody", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)
		router := gin.Default()
		router.POST("/transactions", handler.Create)

		req, _ := http.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString(`{"invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("InvalidAccountID", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)
		router := gin.Default()
		router.POST("/transactions", handler.Create)

		reqBody := CreateTransactionRequest{AccountID: "not-a-uuid"}
		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/transactions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("InvalidTransactionType", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)
		router := gin.Default()
		router.POST("/transactions", handler.Create)

		reqBody := CreateTransactionRequest{AccountID: uuid.New().String(), Type: "INVALID_TYPE"}
		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/transactions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)

		accountID := uuid.New()
		mockService.On("CreateTransaction", mock.Anything, mock.MatchedBy(func(req *shared.TransactionRequest) bool {
			return req.Type == shared.TransactionTypeWithdrawal && req.Amount == 5000 && req.Currency == "EUR"
		})).Return("", nil, errors.New("service unavailable"))

		router := gin.Default()
		router.POST("/transactions", handler.Create)

		idempotencyKey := uuid.New().String()
		reqBody := CreateTransactionRequest{
			AccountID:      accountID.String(), // accountID from line 170
			Type:           "WITHDRAWAL",
			Amount:         5000,
			Currency:       "EUR",
			IdempotencyKey: idempotencyKey,
		}
		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/transactions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		mockService.AssertExpectations(t)
	})
}

func TestTransactionHandler_GetByID(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)

		transactionID := uuid.New()
		accountID := uuid.New()
		now := time.Now()
		processedAt := now.Add(time.Minute)

		expectedEntry := &ledger.Entry{
			TransactionID: transactionID,
			AccountID:     accountID,
			Type:          shared.TransactionTypeDeposit,
			Amount:        15000,
			Currency:      "USD",
			Status:        shared.TransactionStatusCompleted,
			CreatedAt:     now,
			ProcessedAt:   &processedAt,
		}
		mockService.On("GetTransactionByID", mock.Anything, transactionID).Return(expectedEntry, nil)

		router := gin.Default()
		router.GET("/transactions/:id", handler.GetByID)

		req, _ := http.NewRequest(http.MethodGet, "/transactions/"+transactionID.String(), nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var topLevelResponse Response
		err := json.Unmarshal(rr.Body.Bytes(), &topLevelResponse)
		assert.NoError(t, err)

		require.NotNil(t, topLevelResponse.Data)

		var respBody TransactionResponse
		dataBytes, marshalErr := json.Marshal(topLevelResponse.Data)
		require.NoError(t, marshalErr, "Failed to marshal topLevelResponse.Data")
		unmarshalErr := json.Unmarshal(dataBytes, &respBody)
		require.NoError(t, unmarshalErr, "Failed to unmarshal data field into TransactionResponse")

		assert.Equal(t, expectedEntry.TransactionID.String(), respBody.TransactionID)
		assert.Equal(t, expectedEntry.AccountID.String(), respBody.AccountID)
		assert.Equal(t, string(expectedEntry.Type), respBody.Type)
		assert.Equal(t, expectedEntry.Amount, respBody.Amount)
		assert.Equal(t, expectedEntry.Currency, respBody.Currency)
		assert.Equal(t, string(expectedEntry.Status), respBody.Status)
		assert.Equal(t, expectedEntry.CreatedAt.Format(time.RFC3339), respBody.CreatedAt)
		assert.Equal(t, expectedEntry.ProcessedAt.Format(time.RFC3339), respBody.ProcessedAt)
		mockService.AssertExpectations(t)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)
		router := gin.Default()
		router.GET("/transactions/:id", handler.GetByID)

		req, _ := http.NewRequest(http.MethodGet, "/transactions/not-a-uuid", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("TransactionNotFound", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)
		transactionID := uuid.New()
		mockService.On("GetTransactionByID", mock.Anything, transactionID).Return(nil, nil)

		router := gin.Default()
		router.GET("/transactions/:id", handler.GetByID)

		req, _ := http.NewRequest(http.MethodGet, "/transactions/"+transactionID.String(), nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)
		transactionID := uuid.New()
		mockService.On("GetTransactionByID", mock.Anything, transactionID).Return(nil, errors.New("db error"))

		router := gin.Default()
		router.GET("/transactions/:id", handler.GetByID)

		req, _ := http.NewRequest(http.MethodGet, "/transactions/"+transactionID.String(), nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		mockService.AssertExpectations(t)
	})
}

func TestTransactionHandler_GetByAccountID(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)

		accountID := uuid.New()
		transactionID1 := uuid.New()
		transactionID2 := uuid.New()
		now := time.Now()

		entries := []*ledger.Entry{
			{TransactionID: transactionID1, AccountID: accountID, Type: shared.TransactionTypeDeposit, Amount: 100, Currency: "USD", Status: shared.TransactionStatusCompleted, CreatedAt: now},
			{TransactionID: transactionID2, AccountID: accountID, Type: shared.TransactionTypeWithdrawal, Amount: 50, Currency: "USD", Status: shared.TransactionStatusPending, CreatedAt: now.Add(time.Second)},
		}
		var total int64 = 2

		mockService.On("GetTransactionsByAccountID", mock.Anything, accountID, 1, 10).Return(entries, total, nil)

		router := gin.Default()
		router.GET("/accounts/:id/transactions", handler.GetByAccountID)

		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/accounts/%s/transactions?page=1&per_page=10", accountID.String()), nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var respBody PaginatedResponse[TransactionResponse]
		err := json.Unmarshal(rr.Body.Bytes(), &respBody)
		assert.NoError(t, err)
		require.NotNil(t, respBody.Meta, "Response metadata should not be nil")
		assert.Equal(t, 1, respBody.Meta.Page)
		assert.Equal(t, 10, respBody.Meta.PerPage)
		assert.Equal(t, int(total), respBody.Meta.TotalItems)
		assert.Len(t, respBody.Data, 2)
		assert.Equal(t, transactionID1.String(), respBody.Data[0].TransactionID)
		assert.Equal(t, transactionID2.String(), respBody.Data[1].TransactionID)

		mockService.AssertExpectations(t)
	})

	t.Run("InvalidAccountID", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)
		router := gin.Default()
		router.GET("/accounts/:id/transactions", handler.GetByAccountID)

		req, _ := http.NewRequest(http.MethodGet, "/accounts/not-a-uuid/transactions", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("InvalidPaginationParams", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)
		router := gin.Default()
		router.GET("/accounts/:id/transactions", handler.GetByAccountID)

		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/accounts/%s/transactions?page=invalid", uuid.New().String()), nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockService := new(MockTransactionService)
		handler := NewTransactionHandler(logger, mockService)
		accountID := uuid.New()
		mockService.On("GetTransactionsByAccountID", mock.Anything, accountID, 1, 10).Return(nil, int64(0), errors.New("db error"))

		router := gin.Default()
		router.GET("/accounts/:id/transactions", handler.GetByAccountID)

		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/accounts/%s/transactions?page=1&per_page=10", accountID.String()), nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		mockService.AssertExpectations(t)
	})
}

var _ service.TransactionService = (*MockTransactionService)(nil)
