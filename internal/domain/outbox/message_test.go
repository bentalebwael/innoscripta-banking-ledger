package outbox

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessage(t *testing.T) {
	t.Run("SuccessfulCreation", func(t *testing.T) {
		entry := &ledger.Entry{
			TransactionID: uuid.New(),
			AccountID:     uuid.New(),
			Type:          shared.TransactionTypeDeposit,
			Amount:        1000,
			Currency:      "USD",
			Status:        shared.TransactionStatusPending,
			CreatedAt:     time.Now().Add(-time.Minute),
		}

		beforeCreation := time.Now()
		msg, err := NewMessage(entry)
		afterCreation := time.Now()

		require.NoError(t, err)
		require.NotNil(t, msg)

		assert.Equal(t, entry.TransactionID, msg.TransactionID)
		assert.Equal(t, entry.AccountID, msg.AccountID)
		assert.Equal(t, shared.OutboxStatusPending, msg.Status)
		assert.Equal(t, 0, msg.Attempts)
		assert.Nil(t, msg.LastAttemptAt)
		assert.WithinDuration(t, beforeCreation, msg.CreatedAt, afterCreation.Sub(beforeCreation)+time.Millisecond)

		// Check payload
		var decodedEntry ledger.Entry
		err = json.Unmarshal(msg.Payload, &decodedEntry)
		require.NoError(t, err)
		assert.Equal(t, entry.TransactionID, decodedEntry.TransactionID)
		assert.Equal(t, entry.Amount, decodedEntry.Amount)
	})
}

func TestMessage_IncrementAttempts(t *testing.T) {
	t.Run("SuccessfulIncrement", func(t *testing.T) {
		initialTime := time.Now().Add(-time.Hour)
		msg := &Message{
			Attempts:      1,
			LastAttemptAt: &initialTime,
		}
		initialAttempts := msg.Attempts

		time.Sleep(10 * time.Millisecond) // Ensure time changes
		beforeUpdate := time.Now()
		msg.IncrementAttempts()
		afterUpdate := time.Now()

		assert.Equal(t, initialAttempts+1, msg.Attempts)
		require.NotNil(t, msg.LastAttemptAt)
		assert.True(t, msg.LastAttemptAt.After(initialTime))
		assert.WithinDuration(t, beforeUpdate, *msg.LastAttemptAt, afterUpdate.Sub(beforeUpdate)+time.Millisecond)
	})
}

func TestMessage_MarkAsProcessed(t *testing.T) {
	t.Run("SuccessfulMarkAsProcessed", func(t *testing.T) {
		initialTime := time.Now().Add(-time.Hour)
		msg := &Message{
			Status:        shared.OutboxStatusPending,
			LastAttemptAt: &initialTime,
		}
		time.Sleep(10 * time.Millisecond) // Ensure time changes
		beforeUpdate := time.Now()
		msg.MarkAsProcessed()
		afterUpdate := time.Now()

		assert.Equal(t, shared.OutboxStatusProcessed, msg.Status)
		require.NotNil(t, msg.LastAttemptAt)
		assert.True(t, msg.LastAttemptAt.After(initialTime))
		assert.WithinDuration(t, beforeUpdate, *msg.LastAttemptAt, afterUpdate.Sub(beforeUpdate)+time.Millisecond)
	})
}

func TestMessage_MarkAsFailed(t *testing.T) {
	t.Run("SuccessfulMarkAsFailed", func(t *testing.T) {
		initialTime := time.Now().Add(-time.Hour)
		msg := &Message{
			Status:        shared.OutboxStatusPending,
			LastAttemptAt: &initialTime,
		}
		time.Sleep(10 * time.Millisecond) // Ensure time changes
		beforeUpdate := time.Now()
		msg.MarkAsFailed()
		afterUpdate := time.Now()

		assert.Equal(t, shared.OutboxStatusFailedToPublish, msg.Status)
		require.NotNil(t, msg.LastAttemptAt)
		assert.True(t, msg.LastAttemptAt.After(initialTime))
		assert.WithinDuration(t, beforeUpdate, *msg.LastAttemptAt, afterUpdate.Sub(beforeUpdate)+time.Millisecond)
	})
}

func TestMessage_GetLedgerEntry(t *testing.T) {
	t.Run("SuccessfulGetLedgerEntry", func(t *testing.T) {
		originalEntry := &ledger.Entry{
			TransactionID: uuid.New(),
			AccountID:     uuid.New(),
			Type:          shared.TransactionTypeWithdrawal,
			Amount:        500,
			Currency:      "EUR",
			Status:        shared.TransactionStatusCompleted,
			CreatedAt:     time.Now().Truncate(time.Millisecond), // Truncate for consistent comparison
		}
		payload, err := json.Marshal(originalEntry)
		require.NoError(t, err)

		msg := &Message{Payload: payload}
		decodedEntry, err := msg.GetLedgerEntry()

		require.NoError(t, err)
		require.NotNil(t, decodedEntry)
		assert.Equal(t, originalEntry.TransactionID, decodedEntry.TransactionID)
		assert.Equal(t, originalEntry.AccountID, decodedEntry.AccountID)
		assert.Equal(t, originalEntry.Type, decodedEntry.Type)
		assert.Equal(t, originalEntry.Amount, decodedEntry.Amount)
		assert.Equal(t, originalEntry.Currency, decodedEntry.Currency)
		assert.Equal(t, originalEntry.Status, decodedEntry.Status)
		// time.Time can be tricky with monotonic clocks, compare specific parts or use .Equal for full comparison if marshaling preserves it
		assert.True(t, originalEntry.CreatedAt.Equal(decodedEntry.CreatedAt), "CreatedAt should match")
	})
}
