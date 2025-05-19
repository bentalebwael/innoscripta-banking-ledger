package persistence

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/innoscripta-banking-ledger/internal/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoConfig struct {
	URI             string
	Database        string
	Timeout         time.Duration
	MaxPoolSize     uint64
	MinPoolSize     uint64
	MaxConnIdleTime time.Duration
}

type MongoDB struct {
	logger   *slog.Logger
	client   *mongo.Client
	database *mongo.Database
}

func NewMongoDB(ctx context.Context, logger *slog.Logger, cfg *config.MongoDBConfig) (*MongoDB, error) {
	clientOptions := options.Client().
		ApplyURI(cfg.URI).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetMinPoolSize(cfg.MinPoolSize).
		SetMaxConnIdleTime(cfg.MaxConnIdleTime)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	database := client.Database(cfg.Database)

	return &MongoDB{
		logger:   logger,
		client:   client,
		database: database,
	}, nil
}

func (m *MongoDB) Database() *mongo.Database {
	return m.database
}

func (m *MongoDB) Collection(name string) *mongo.Collection {
	return m.database.Collection(name)
}

func (m *MongoDB) Close(ctx context.Context) error {
	if err := m.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("failed to disconnect from MongoDB: %w", err)
	}
	m.logger.Info("Closed MongoDB connection")
	return nil
}
