package mongo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/innoscripta-banking-ledger/internal/domain/ledger"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
)

const (
	// LedgerCollectionName is the name of the ledger collection in MongoDB
	LedgerCollectionName = "ledger_entries"
)

// LedgerRepository implements the ledger.Repository interface for MongoDB
type LedgerRepository struct {
	db     *mongo.Database
	logger *slog.Logger
}

// NewLedgerRepository creates a new MongoDB ledger repository
func NewLedgerRepository(logger *slog.Logger, db *mongo.Database) ledger.Repository {
	return &LedgerRepository{
		db:     db,
		logger: logger,
	}
}

// Create stores a new ledger entry after checking for duplicates.
// Returns ErrDuplicateEntry if an entry with the same transaction ID exists.
func (r *LedgerRepository) Create(ctx context.Context, entry *ledger.Entry) error {
	collection := r.db.Collection(LedgerCollectionName)

	// Check if entry already exists
	existingEntry, err := r.GetByTransactionID(ctx, entry.TransactionID)
	if err != nil && !errors.Is(err, ledger.ErrEntryNotFound{}) {
		r.logger.Error("Failed to check for existing ledger entry",
			"transaction_id", entry.TransactionID.String(),
			"error", err)
		return fmt.Errorf("failed to check for existing ledger entry: %w", err)
	}

	if existingEntry != nil {
		return ledger.ErrDuplicateEntry{TransactionID: entry.TransactionID}
	}

	// Insert the entry
	_, err = collection.InsertOne(ctx, entry)
	if err != nil {
		r.logger.Error("Failed to create ledger entry",
			"transaction_id", entry.TransactionID.String(),
			"error", err)
		return fmt.Errorf("failed to create ledger entry: %w", err)
	}

	return nil
}

// GetByTransactionID retrieves a ledger entry by its transaction ID.
// Returns ErrEntryNotFound if no entry exists for the given transaction.
func (r *LedgerRepository) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*ledger.Entry, error) {
	collection := r.db.Collection(LedgerCollectionName)

	filter := bson.M{"transaction_id": transactionID}
	var entry ledger.Entry
	err := collection.FindOne(ctx, filter).Decode(&entry)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ledger.ErrEntryNotFound{TransactionID: transactionID}
		}
		r.logger.Error("Failed to get ledger entry",
			"transaction_id", transactionID.String(),
			"error", err)
		return nil, fmt.Errorf("failed to get ledger entry: %w", err)
	}

	return &entry, nil
}

// GetByIdempotencyKey retrieves a ledger entry using its idempotency key.
// Returns nil if no entry exists, enabling idempotent transaction processing.
func (r *LedgerRepository) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*ledger.Entry, error) {
	if idempotencyKey == "" {
		return nil, errors.New("idempotency key cannot be empty")
	}

	collection := r.db.Collection(LedgerCollectionName)

	filter := bson.M{"idempotency_key": idempotencyKey}
	var entry ledger.Entry
	err := collection.FindOne(ctx, filter).Decode(&entry)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // No entry found with this idempotency key
		}
		r.logger.Error("Failed to get ledger entry by idempotency key",
			"idempotency_key", idempotencyKey,
			"error", err)
		return nil, fmt.Errorf("failed to get ledger entry by idempotency key: %w", err)
	}

	return &entry, nil
}

// GetByAccountID retrieves paginated ledger entries for an account.
// Results are sorted by creation time in descending order (newest first).
func (r *LedgerRepository) GetByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*ledger.Entry, error) {
	collection := r.db.Collection(LedgerCollectionName)

	filter := bson.M{"account_id": accountID}
	opts := options.Find().
		SetSort(bson.M{"created_at": -1}). // Sort by created_at in descending order
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to get ledger entries",
			"account_id", accountID.String(),
			"error", err)
		return nil, fmt.Errorf("failed to get ledger entries: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []*ledger.Entry
	if err := cursor.All(ctx, &entries); err != nil {
		r.logger.Error("Failed to decode ledger entries",
			"account_id", accountID.String(),
			"error", err)
		return nil, fmt.Errorf("failed to decode ledger entries: %w", err)
	}

	return entries, nil
}

// CountByAccountID counts the total number of ledger entries for an account
func (r *LedgerRepository) CountByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error) {
	collection := r.db.Collection(LedgerCollectionName)

	filter := bson.M{"account_id": accountID}
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		r.logger.Error("Failed to count ledger entries",
			"account_id", accountID.String(),
			"error", err)
		return 0, fmt.Errorf("failed to count ledger entries: %w", err)
	}

	return count, nil
}

// UpdateStatus updates the entry's status, failure reason, and processed timestamp.
// Returns ErrEntryNotFound if the entry doesn't exist.
func (r *LedgerRepository) UpdateStatus(ctx context.Context, transactionID uuid.UUID, status shared.TransactionStatus, reason string) error {
	collection := r.db.Collection(LedgerCollectionName)

	filter := bson.M{"transaction_id": transactionID}
	update := bson.M{
		"$set": bson.M{
			"status":         status,
			"failure_reason": reason,
			"processed_at":   time.Now(),
		},
	}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		r.logger.Error("Failed to update ledger entry status",
			"transaction_id", transactionID.String(),
			"status", string(status),
			"error", err)
		return fmt.Errorf("failed to update ledger entry status: %w", err)
	}

	if result.MatchedCount == 0 {
		return ledger.ErrEntryNotFound{TransactionID: transactionID}
	}

	return nil
}

// GetByTimeRange retrieves paginated ledger entries within the specified time window.
// Results are sorted by creation time in descending order for recent-first access.
func (r *LedgerRepository) GetByTimeRange(ctx context.Context, startTime, endTime time.Time, limit, offset int) ([]*ledger.Entry, error) {
	collection := r.db.Collection(LedgerCollectionName)

	filter := bson.M{
		"created_at": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}
	opts := options.Find().
		SetSort(bson.M{"created_at": -1}). // Sort by created_at in descending order
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to get ledger entries by time range",
			"start_time", startTime,
			"end_time", endTime,
			"error", err)
		return nil, fmt.Errorf("failed to get ledger entries by time range: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []*ledger.Entry
	if err := cursor.All(ctx, &entries); err != nil {
		r.logger.Error("Failed to decode ledger entries",
			"start_time", startTime,
			"end_time", endTime,
			"error", err)
		return nil, fmt.Errorf("failed to decode ledger entries: %w", err)
	}

	return entries, nil
}
