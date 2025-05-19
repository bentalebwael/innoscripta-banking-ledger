package shared

// TransactionType defines possible transaction operations
type TransactionType string

const (
	TransactionTypeDeposit    TransactionType = "DEPOSIT"
	TransactionTypeWithdrawal TransactionType = "WITHDRAWAL"
)

// TransactionStatus defines transaction processing states
type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "PENDING"
	TransactionStatusProcessing TransactionStatus = "PROCESSING"
	TransactionStatusCompleted  TransactionStatus = "COMPLETED"
	TransactionStatusFailed     TransactionStatus = "FAILED"
)

// FailureReason defines transaction failure categories
type FailureReason string

const (
	FailureReasonAccountNotFound         FailureReason = "ACCOUNT_NOT_FOUND"
	FailureReasonCurrencyMismatchFormat  FailureReason = "CURRENCY_MISMATCH:_REQUEST_%s_ACCOUNT_%s" // To be used with fmt.Sprintf
	FailureReasonInsufficientFunds       FailureReason = "INSUFFICIENT_FUNDS"
	FailureReasonInvalidAmount           FailureReason = "INVALID_AMOUNT"
	FailureReasonWithdrawalFailed        FailureReason = "WITHDRAWAL_FAILED" // Generic reason if more specific one isn't identified
	FailureReasonTransactionCommitFailed FailureReason = "TRANSACTION_COMMIT_FAILED"
	FailureReasonUnknownError            FailureReason = "UNKNOWN_ERROR"
)

// OutboxStatus defines message publishing states
type OutboxStatus string

const (
	OutboxStatusPending         OutboxStatus = "PENDING"
	OutboxStatusProcessed       OutboxStatus = "PROCESSED"
	OutboxStatusFailedToPublish OutboxStatus = "FAILED_TO_PUBLISH"
)
