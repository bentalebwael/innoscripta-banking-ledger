package handler

// CreateAccountRequest represents a request to create a new account
type CreateAccountRequest struct {
	OwnerName      string `json:"owner_name" binding:"required"`
	NationalID     string `json:"national_id" binding:"required"`
	InitialBalance int64  `json:"initial_balance" binding:"min=0"`
	Currency       string `json:"currency" binding:"required,len=3"`
}

// AccountResponse represents an account in API responses
type AccountResponse struct {
	ID         string `json:"id"`
	OwnerName  string `json:"owner_name"`
	NationalID string `json:"national_id"`
	Balance    int64  `json:"balance"`
	Currency   string `json:"currency"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// CreateTransactionRequest represents a request to create a new transaction
type CreateTransactionRequest struct {
	AccountID      string `json:"account_id" binding:"required,uuid"`
	Type           string `json:"type" binding:"required,oneof=DEPOSIT WITHDRAWAL"`
	Amount         int64  `json:"amount" binding:"required,gt=0"`
	Currency       string `json:"currency" binding:"required,len=3"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// TransactionResponse represents a transaction in API responses
type TransactionResponse struct {
	TransactionID string `json:"transaction_id"`
	AccountID     string `json:"account_id"`
	Type          string `json:"type"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
	FailureReason string `json:"failure_reason,omitempty"`
	CreatedAt     string `json:"created_at"`
	ProcessedAt   string `json:"processed_at,omitempty"`
}

// TransactionListResponse represents a list of transactions in API responses
type TransactionListResponse struct {
	Transactions []TransactionResponse `json:"transactions"`
}

// PaginationParams represents pagination parameters for list endpoints
type PaginationParams struct {
	Page    int `form:"page,default=1" binding:"min=1"`
	PerPage int `form:"per_page,default=10" binding:"min=1,max=100"`
}
