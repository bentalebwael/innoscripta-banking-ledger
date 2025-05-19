package account

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccount(t *testing.T) {
	t.Run("SuccessfulCreation", func(t *testing.T) {
		ownerName := "John Doe"
		nationalID := "AB123456"
		initialBalance := int64(10000) // 100.00
		currency := "USD"

		beforeCreation := time.Now()
		account, err := NewAccount(ownerName, nationalID, initialBalance, currency)
		afterCreation := time.Now()

		require.NoError(t, err)
		require.NotNil(t, account)

		assert.NotEqual(t, uuid.Nil, account.ID, "Account ID should not be nil")
		assert.Equal(t, ownerName, account.OwnerName)
		assert.Equal(t, nationalID, account.NationalID)
		assert.Equal(t, initialBalance, account.Balance)
		assert.Equal(t, currency, account.Currency)
		assert.Equal(t, 1, account.Version, "Initial version should be 1")

		assert.WithinDuration(t, beforeCreation, account.CreatedAt, afterCreation.Sub(beforeCreation)+time.Millisecond, "CreatedAt should be around the time of creation")
		assert.WithinDuration(t, beforeCreation, account.UpdatedAt, afterCreation.Sub(beforeCreation)+time.Millisecond, "UpdatedAt should be around the time of creation")
		// Check that UpdatedAt is very close to CreatedAt, allowing for a small difference due to separate time.Now() calls
		assert.WithinDuration(t, account.CreatedAt, account.UpdatedAt, time.Millisecond, "CreatedAt and UpdatedAt should be very close on creation")
	})
}

func TestAccount_Deposit(t *testing.T) {
	t.Run("SuccessfulDeposit", func(t *testing.T) {
		acc := &Account{
			ID:         uuid.New(),
			OwnerName:  "Jane Doe",
			NationalID: "CD789012",
			Balance:    5000, // 50.00
			Currency:   "EUR",
			Version:    1,
			CreatedAt:  time.Now().Add(-time.Hour),
			UpdatedAt:  time.Now().Add(-time.Hour),
		}
		depositAmount := int64(2000) // 20.00
		initialBalance := acc.Balance
		initialVersion := acc.Version
		beforeUpdate := time.Now()

		err := acc.Deposit(depositAmount)
		afterUpdate := time.Now()

		require.NoError(t, err)
		assert.Equal(t, initialBalance+depositAmount, acc.Balance)
		assert.Equal(t, initialVersion+1, acc.Version)
		assert.True(t, acc.UpdatedAt.After(acc.CreatedAt), "UpdatedAt should be after CreatedAt")
		assert.WithinDuration(t, beforeUpdate, acc.UpdatedAt, afterUpdate.Sub(beforeUpdate)+time.Millisecond, "UpdatedAt should be around the time of deposit")
	})
}

func TestAccount_Withdraw(t *testing.T) {
	t.Run("SuccessfulWithdrawal", func(t *testing.T) {
		acc := &Account{
			ID:         uuid.New(),
			OwnerName:  "Peter Pan",
			NationalID: "EF345678",
			Balance:    10000, // 100.00
			Currency:   "GBP",
			Version:    2,
			CreatedAt:  time.Now().Add(-2 * time.Hour),
			UpdatedAt:  time.Now().Add(-time.Minute),
		}
		withdrawalAmount := int64(3000) // 30.00
		initialBalance := acc.Balance
		initialVersion := acc.Version
		beforeUpdate := time.Now()

		err := acc.Withdraw(withdrawalAmount)
		afterUpdate := time.Now()

		require.NoError(t, err)
		assert.Equal(t, initialBalance-withdrawalAmount, acc.Balance)
		assert.Equal(t, initialVersion+1, acc.Version)
		assert.True(t, acc.UpdatedAt.After(acc.CreatedAt), "UpdatedAt should be after CreatedAt")
		assert.WithinDuration(t, beforeUpdate, acc.UpdatedAt, afterUpdate.Sub(beforeUpdate)+time.Millisecond, "UpdatedAt should be around the time of withdrawal")
	})
}

func TestAccount_CanWithdraw(t *testing.T) {
	t.Run("CanWithdrawSufficientFunds", func(t *testing.T) {
		acc := &Account{Balance: 1000}
		assert.True(t, acc.CanWithdraw(500))
		assert.True(t, acc.CanWithdraw(1000))
	})

	t.Run("CannotWithdrawInsufficientFunds", func(t *testing.T) {
		acc := &Account{Balance: 1000}
		assert.False(t, acc.CanWithdraw(1001))
	})
}
