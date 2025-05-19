package account

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Common errors
var (
	ErrInsufficientFunds     = errors.New("insufficient funds for withdrawal")
	ErrInvalidAmount         = errors.New("amount must be positive")
	ErrEmptyOwnerName        = errors.New("owner name cannot be empty")
	ErrInvalidCurrencyFormat = errors.New("currency must be a 3-letter code")
)

// Account represents a bank account
type Account struct {
	ID         uuid.UUID `json:"id"`
	OwnerName  string    `json:"owner_name"`
	NationalID string    `json:"national_id"`
	Balance    int64     `json:"balance"` // Stored in cents/minor units
	Currency   string    `json:"currency"`
	Version    int       `json:"version"` // For optimistic locking
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NewAccount creates a new account with the given parameters
func NewAccount(ownerName string, nationalID string, initialBalance int64, currency string) (*Account, error) {
	if ownerName == "" {
		return nil, ErrEmptyOwnerName
	}
	if len(currency) != 3 { // Basic validation for currency code length
		return nil, ErrInvalidCurrencyFormat
	}
	if initialBalance < 0 {
		return nil, ErrInvalidAmount
	}

	return &Account{
		ID:         uuid.New(),
		OwnerName:  ownerName,
		NationalID: nationalID,
		Balance:    initialBalance,
		Currency:   currency,
		Version:    1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// Deposit adds the specified amount to the account balance
func (a *Account) Deposit(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	a.Balance += amount
	a.UpdatedAt = time.Now()
	a.Version++
	return nil
}

// Withdraw subtracts the specified amount from the account balance
func (a *Account) Withdraw(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	if a.Balance < amount {
		return ErrInsufficientFunds
	}

	a.Balance -= amount
	a.UpdatedAt = time.Now()
	a.Version++
	return nil
}

// CanWithdraw checks if the account has sufficient funds for a withdrawal
func (a *Account) CanWithdraw(amount int64) bool {
	return a.Balance >= amount
}
