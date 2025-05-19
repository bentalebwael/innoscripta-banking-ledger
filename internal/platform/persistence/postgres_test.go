package persistence

import (
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

func TestPostgresDB_Pool(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// Using nil pool since pgxpool requires real DB connection
	var nilPool *pgxpool.Pool
	db := &PostgresDB{
		pool:   nilPool,
		logger: logger,
	}
	assert.Equal(t, nilPool, db.Pool(), "Pool() should return the initialized pool")

}

// Limited testing due to pgxpool requiring live DB or interface changes
