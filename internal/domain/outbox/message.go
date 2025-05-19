package outbox

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
)

// Message stores transaction data for reliable message publishing
type Message struct {
	ID            int64               `json:"id"`
	TransactionID uuid.UUID           `json:"transaction_id"`
	AccountID     uuid.UUID           `json:"account_id"`
	Payload       json.RawMessage     `json:"payload"`
	Status        shared.OutboxStatus `json:"status"`
	Attempts      int                 `json:"attempts"`
	CreatedAt     time.Time           `json:"created_at"`
	LastAttemptAt *time.Time          `json:"last_attempt_at,omitempty"`
}

func NewMessage(entry *ledger.Entry) (*Message, error) {
	payload, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}

	return &Message{
		TransactionID: entry.TransactionID,
		AccountID:     entry.AccountID,
		Payload:       payload,
		Status:        shared.OutboxStatusPending,
		Attempts:      0,
		CreatedAt:     time.Now(),
	}, nil
}

func (m *Message) IncrementAttempts() {
	m.Attempts++
	now := time.Now()
	m.LastAttemptAt = &now
}

func (m *Message) MarkAsProcessed() {
	m.Status = shared.OutboxStatusProcessed
	now := time.Now()
	m.LastAttemptAt = &now
}

func (m *Message) MarkAsFailed() {
	m.Status = shared.OutboxStatusFailedToPublish
	now := time.Now()
	m.LastAttemptAt = &now
}

// GetLedgerEntry extracts the ledger entry from the payload
func (m *Message) GetLedgerEntry() (*ledger.Entry, error) {
	var entry ledger.Entry
	if err := json.Unmarshal(m.Payload, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}
