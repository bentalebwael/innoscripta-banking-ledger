package persistence

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // PostgreSQL driver
	_ "github.com/golang-migrate/migrate/v4/source/file"       // File source driver
)

// RunMigrations executes database migrations using DSN and migrations path (file://./migrations/postgres)
func RunMigrations(databaseURL string, migrationsPath string) error {
	if migrationsPath == "" {
		return errors.New("migrations path cannot be empty")
	}
	if databaseURL == "" {
		return errors.New("database URL cannot be empty")
	}

	// Prepend file:// to migrations path for correct format
	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	sourceErr, dbErr := m.Close()
	if sourceErr != nil {
		return fmt.Errorf("migration source error: %w", sourceErr)
	}
	if dbErr != nil {
		return fmt.Errorf("migration database error: %w", dbErr)
	}

	return nil
}
