package shared

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidTransactionType = errors.New("invalid transaction type")
	ErrInvalidCurrency        = errors.New("invalid currency")
)

// TransactionRequest defines a Kafka message for transaction processing
type TransactionRequest struct {
	TransactionID  uuid.UUID       `json:"transaction_id"`
	AccountID      uuid.UUID       `json:"account_id"`
	Type           TransactionType `json:"type"`
	Amount         int64           `json:"amount"` // Stored in cents/minor units
	Currency       string          `json:"currency"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	CorrelationID  string          `json:"correlation_id"`
	Timestamp      time.Time       `json:"timestamp"`
}
