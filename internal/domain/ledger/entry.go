package ledger

import (
	"time"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
)

// Entry represents a transaction entry in the ledger
type Entry struct {
	TransactionID  uuid.UUID                `json:"transaction_id" bson:"transaction_id"`
	AccountID      uuid.UUID                `json:"account_id" bson:"account_id"`
	Type           shared.TransactionType   `json:"type" bson:"type"`
	Amount         int64                    `json:"amount" bson:"amount"` // Stored in cents/minor units
	Currency       string                   `json:"currency" bson:"currency"`
	IdempotencyKey string                   `json:"idempotency_key,omitempty" bson:"idempotency_key,omitempty"`
	CorrelationID  string                   `json:"correlation_id,omitempty" bson:"correlation_id,omitempty"`
	Status         shared.TransactionStatus `json:"status" bson:"status"`
	FailureReason  string                   `json:"failure_reason,omitempty" bson:"failure_reason,omitempty"`
	CreatedAt      time.Time                `json:"created_at" bson:"created_at"`
	ProcessedAt    *time.Time               `json:"processed_at,omitempty" bson:"processed_at,omitempty"`
}
