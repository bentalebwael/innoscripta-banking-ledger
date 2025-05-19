package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/middleware"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/service"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
)

// TransactionHandler handles HTTP requests for transaction operations
type TransactionHandler struct {
	transactionService service.TransactionService
	logger             *slog.Logger
}

// NewTransactionHandler creates a new transaction handler
func NewTransactionHandler(logger *slog.Logger, transactionService service.TransactionService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
		logger:             logger,
	}
}

// Create initiates a new transaction (deposit or withdrawal) with idempotency support
func (h *TransactionHandler) Create(c *gin.Context) {
	var req CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err)
		RespondBadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		h.logger.Error("Invalid account ID", "account_id", req.AccountID, "error", err)
		RespondBadRequest(c, "Invalid account ID")
		return
	}

	var transactionType shared.TransactionType
	switch req.Type {
	case "DEPOSIT":
		transactionType = shared.TransactionTypeDeposit
	case "WITHDRAWAL":
		transactionType = shared.TransactionTypeWithdrawal
	default:
		h.logger.Error("Invalid transaction type", "type", req.Type)
		RespondBadRequest(c, "Invalid transaction type")
		return
	}

	if req.IdempotencyKey == "" {
		req.IdempotencyKey = uuid.New().String()
	}

	transactionRequest := &shared.TransactionRequest{
		TransactionID:  uuid.New(),
		AccountID:      accountID,
		Type:           transactionType,
		Amount:         req.Amount,
		Currency:       req.Currency,
		IdempotencyKey: req.IdempotencyKey,
		CorrelationID:  middleware.GetCorrelationID(c),
		Timestamp:      time.Now(),
	}

	transactionID, ledgerRes, err := h.transactionService.CreateTransaction(c.Request.Context(), transactionRequest)
	if err != nil {
		h.logger.Error("Failed to create transaction", "error", err)
		RespondInternalError(c)
		return
	}
	if ledgerRes != nil {
		RespondOK(c, mapLedgerEntryToResponse(ledgerRes))
		return
	}

	RespondAccepted(c, gin.H{
		"transaction_id": transactionID,
		"status":         "PENDING",
	})
}

// GetByID retrieves transaction details by its ID, returns 404 if not found
func (h *TransactionHandler) GetByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		h.logger.Error("Invalid transaction ID", "id", idParam, "error", err)
		RespondBadRequest(c, "Invalid transaction ID")
		return
	}

	entry, err := h.transactionService.GetTransactionByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get transaction", "id", idParam, "error", err)
		RespondInternalError(c)
		return
	}

	if entry == nil {
		RespondNotFound(c, "Transaction not found")
		return
	}

	response := mapLedgerEntryToResponse(entry)
	RespondOK(c, response)
}

// GetByAccountID retrieves paginated transaction history for an account
func (h *TransactionHandler) GetByAccountID(c *gin.Context) {
	accountIDParam := c.Param("id")
	accountID, err := uuid.Parse(accountIDParam)
	if err != nil {
		h.logger.Error("Invalid account ID", "account_id", accountIDParam, "error", err)
		RespondBadRequest(c, "Invalid account ID")
		return
	}

	var pagination PaginationParams
	if err := c.ShouldBindQuery(&pagination); err != nil {
		h.logger.Error("Invalid pagination parameters", "error", err)
		RespondBadRequest(c, "Invalid pagination parameters")
		return
	}

	entries, total, err := h.transactionService.GetTransactionsByAccountID(
		c.Request.Context(),
		accountID,
		pagination.Page,
		pagination.PerPage,
	)
	if err != nil {
		h.logger.Error("Failed to get transactions", "account_id", accountIDParam, "error", err)
		RespondInternalError(c)
		return
	}

	var transactions []TransactionResponse
	for _, entry := range entries {
		transactions = append(transactions, mapLedgerEntryToResponse(entry))
	}

	RespondWithPaginatedData(c, http.StatusOK, transactions, pagination.Page, pagination.PerPage, int(total))
}

// mapLedgerEntryToResponse maps a ledger entry to a transaction response DTO
func mapLedgerEntryToResponse(entry *ledger.Entry) TransactionResponse {
	response := TransactionResponse{
		TransactionID: entry.TransactionID.String(),
		AccountID:     entry.AccountID.String(),
		Type:          string(entry.Type),
		Amount:        entry.Amount,
		Currency:      entry.Currency,
		Status:        string(entry.Status),
		FailureReason: entry.FailureReason,
		CreatedAt:     entry.CreatedAt.Format(time.RFC3339),
	}

	if entry.ProcessedAt != nil {
		response.ProcessedAt = entry.ProcessedAt.Format(time.RFC3339)
	}

	return response
}
