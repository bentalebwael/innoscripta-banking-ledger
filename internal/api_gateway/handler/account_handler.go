package handler

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/service"
	"github.com/innoscripta-banking-ledger/internal/domain/account"
)

// AccountHandler handles HTTP requests for account operations
type AccountHandler struct {
	accountService service.AccountService
	logger         *slog.Logger
}

// NewAccountHandler creates a new account handler
func NewAccountHandler(logger *slog.Logger, accountService service.AccountService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
		logger:         logger,
	}
}

// Create handles creation of a new account, validating the request and checking for duplicate national IDs
func (h *AccountHandler) Create(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err)
		RespondBadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	acc, err := h.accountService.CreateAccount(c.Request.Context(), req.OwnerName, req.NationalID, req.InitialBalance, req.Currency)
	if err != nil {
		var duplicateNationalIDErr account.ErrDuplicateNationalID
		if errors.As(err, &duplicateNationalIDErr) {
			h.logger.Warn("Attempt to create account with duplicate National ID", "nationalID", duplicateNationalIDErr.NationalID)
			RespondBadRequest(c, "Account with this National ID already exists")
			return
		}
		h.logger.Error("Failed to create account", "error", err)
		RespondInternalError(c)
		return
	}

	response := mapAccountToResponse(acc)
	RespondCreated(c, response)
}

// GetByID retrieves an account by its ID, returning 404 if not found
func (h *AccountHandler) GetByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		h.logger.Error("Invalid account ID", "id", idParam, "error", err)
		RespondBadRequest(c, "Invalid account ID")
		return
	}

	acc, err := h.accountService.GetAccountByID(c.Request.Context(), id)
	if err != nil {
		var accNotFound account.ErrAccountNotFound
		if errors.As(err, &accNotFound) {
			RespondNotFound(c, "Account not found")
			return
		}
		h.logger.Error("Failed to get account", "id", idParam, "error", err)
		RespondInternalError(c)
		return
	}

	response := mapAccountToResponse(acc)
	RespondOK(c, response)
}

// mapAccountToResponse maps an account entity to an account response DTO
func mapAccountToResponse(acc *account.Account) AccountResponse {
	return AccountResponse{
		ID:         acc.ID.String(),
		OwnerName:  acc.OwnerName,
		NationalID: acc.NationalID,
		Balance:    acc.Balance,
		Currency:   acc.Currency,
		CreatedAt:  acc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  acc.UpdatedAt.Format(time.RFC3339),
	}
}
