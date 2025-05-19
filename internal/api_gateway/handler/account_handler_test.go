package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/service"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockAccountService struct {
	mock.Mock
}

func (m *MockAccountService) CreateAccount(ctx context.Context, ownerName string, nationalID string, initialBalance int64, currency string) (*account.Account, error) {
	args := m.Called(ctx, ownerName, nationalID, initialBalance, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *MockAccountService) GetAccountByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	return r
}

func TestAccountHandler_Create(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	t.Run("Success", func(t *testing.T) {
		mockService := new(MockAccountService)
		handler := NewAccountHandler(logger, mockService)

		accountID := uuid.New()
		now := time.Now()
		expectedAccount := &account.Account{
			ID:         accountID,
			OwnerName:  "John Doe",
			NationalID: "AB123456789",
			Balance:    int64(10000),
			Currency:   "USD",
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		mockService.On("CreateAccount", mock.Anything, "John Doe", "AB123456789", int64(10000), "USD").Return(expectedAccount, nil)

		router := setupTestRouter()
		router.POST("/accounts", handler.Create)

		reqBody := CreateAccountRequest{
			OwnerName:      "John Doe",
			NationalID:     "AB123456789",
			InitialBalance: int64(10000),
			Currency:       "USD",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var topLevelResponse Response
		err := json.Unmarshal(rr.Body.Bytes(), &topLevelResponse)
		assert.NoError(t, err, "Failed to unmarshal top-level response")

		require.NotNil(t, topLevelResponse.Data, "'data' field should not be nil")

		responseBody, ok := topLevelResponse.Data.(AccountResponse)
		if !ok {
			dataBytes, marshalErr := json.Marshal(topLevelResponse.Data)
			require.NoError(t, marshalErr, "Failed to marshal topLevelResponse.Data")
			unmarshalErr := json.Unmarshal(dataBytes, &responseBody)
			require.NoError(t, unmarshalErr, "Failed to unmarshal data field into AccountResponse")
		}

		assert.Equal(t, expectedAccount.ID.String(), responseBody.ID)
		assert.Equal(t, expectedAccount.OwnerName, responseBody.OwnerName)
		assert.Equal(t, expectedAccount.NationalID, responseBody.NationalID)
		assert.Equal(t, expectedAccount.Balance, responseBody.Balance)
		assert.Equal(t, expectedAccount.Currency, responseBody.Currency)

		mockService.AssertExpectations(t)
	})

	t.Run("InvalidRequestBody", func(t *testing.T) {
		mockService := new(MockAccountService)
		handler := NewAccountHandler(logger, mockService)

		router := setupTestRouter()
		router.POST("/accounts", handler.Create)

		req, _ := http.NewRequest(http.MethodPost, "/accounts", bytes.NewBufferString(`{"invalid`)) // Malformed JSON
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockService.AssertExpectations(t) // Ensure no service methods were called
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockService := new(MockAccountService)
		handler := NewAccountHandler(logger, mockService)

		mockService.On("CreateAccount", mock.Anything, "Jane Doe", "XY987654321", int64(5000), "EUR").Return(nil, errors.New("service unavailable"))

		router := setupTestRouter()
		router.POST("/accounts", handler.Create)

		reqBody := CreateAccountRequest{
			OwnerName:      "Jane Doe",
			NationalID:     "XY987654321",
			InitialBalance: int64(5000),
			Currency:       "EUR",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("DuplicateNationalID", func(t *testing.T) {
		mockService := new(MockAccountService)
		handler := NewAccountHandler(logger, mockService)

		nationalID := "AB123456789"
		mockService.On("CreateAccount", mock.Anything, "Jane Smith", nationalID, int64(15000), "EUR").
			Return(nil, account.ErrDuplicateNationalID{NationalID: nationalID})

		router := setupTestRouter()
		router.POST("/accounts", handler.Create)

		reqBody := CreateAccountRequest{
			OwnerName:      "Jane Smith",
			NationalID:     nationalID,
			InitialBalance: int64(15000),
			Currency:       "EUR",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var response Response
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		require.NotNil(t, response.Error, "Error field in response should not be nil")
		assert.Equal(t, "Account with this National ID already exists", response.Error.Message)
		assert.Equal(t, "BAD_REQUEST", response.Error.Code) // Also check the code
		mockService.AssertExpectations(t)
	})
}

func TestAccountHandler_GetByID(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	t.Run("Success", func(t *testing.T) {
		mockService := new(MockAccountService)
		handler := NewAccountHandler(logger, mockService)

		accountID := uuid.New()
		now := time.Now()
		expectedAccount := &account.Account{
			ID:         accountID,
			OwnerName:  "Alice Wonderland",
			NationalID: "CD456789012",
			Balance:    int64(20000),
			Currency:   "GBP",
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		mockService.On("GetAccountByID", mock.Anything, accountID).Return(expectedAccount, nil)

		router := setupTestRouter()
		router.GET("/accounts/:id", handler.GetByID)

		req, _ := http.NewRequest(http.MethodGet, "/accounts/"+accountID.String(), nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var topLevelResponse Response
		err := json.Unmarshal(rr.Body.Bytes(), &topLevelResponse)
		assert.NoError(t, err)

		require.NotNil(t, topLevelResponse.Data)

		var responseBody AccountResponse
		dataBytes, marshalErr := json.Marshal(topLevelResponse.Data)
		require.NoError(t, marshalErr, "Failed to marshal topLevelResponse.Data")
		unmarshalErr := json.Unmarshal(dataBytes, &responseBody)
		require.NoError(t, unmarshalErr, "Failed to unmarshal data field into AccountResponse")

		assert.Equal(t, expectedAccount.ID.String(), responseBody.ID)
		assert.Equal(t, expectedAccount.OwnerName, responseBody.OwnerName)
		assert.Equal(t, expectedAccount.NationalID, responseBody.NationalID)
		assert.Equal(t, expectedAccount.Balance, responseBody.Balance) // Both are int64

		mockService.AssertExpectations(t)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		mockService := new(MockAccountService)
		handler := NewAccountHandler(logger, mockService)

		router := setupTestRouter()
		router.GET("/accounts/:id", handler.GetByID)

		req, _ := http.NewRequest(http.MethodGet, "/accounts/not-a-uuid", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockService.AssertExpectations(t) // No service calls expected
	})

	t.Run("AccountNotFound", func(t *testing.T) {
		mockService := new(MockAccountService)
		handler := NewAccountHandler(logger, mockService)

		accountID := uuid.New()
		mockService.On("GetAccountByID", mock.Anything, accountID).Return(nil, account.ErrAccountNotFound{AccountID: accountID})

		router := setupTestRouter()
		router.GET("/accounts/:id", handler.GetByID)

		req, _ := http.NewRequest(http.MethodGet, "/accounts/"+accountID.String(), nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockService := new(MockAccountService)
		handler := NewAccountHandler(logger, mockService)

		accountID := uuid.New()
		mockService.On("GetAccountByID", mock.Anything, accountID).Return(nil, errors.New("database connection lost"))

		router := setupTestRouter()
		router.GET("/accounts/:id", handler.GetByID)

		req, _ := http.NewRequest(http.MethodGet, "/accounts/"+accountID.String(), nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		mockService.AssertExpectations(t)
	})
}

var _ service.AccountService = (*MockAccountService)(nil)
