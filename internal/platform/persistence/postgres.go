package persistence

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier supports database operations for both pool and transactions
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

// Ensure interfaces are satisfied (compile-time check)
var _ Querier = (*pgxpool.Pool)(nil)
var _ Querier = (pgx.Tx)(nil)

type PostgresDB struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewPostgresDB(ctx context.Context, logger *slog.Logger, cfg *config.PostgresConfig) (*PostgresDB, error) {
	err := RunMigrations(cfg.URL, cfg.MigrationsPath)
	if err != nil {
		return nil, err
	}
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PostgreSQL connection string: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	logger.Info("Connected to PostgreSQL")

	return &PostgresDB{
		pool:   pool,
		logger: logger,
	}, nil
}

func (db *PostgresDB) Pool() *pgxpool.Pool {
	return db.pool
}

func (db *PostgresDB) Close() {
	db.pool.Close()
	db.logger.Info("Closed PostgreSQL connection")
}

// ExecuteTx runs function in a transaction, rolling back on error or panic
func (db *PostgresDB) ExecuteTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx) // Attempt rollback on panic
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit(ctx)
}
