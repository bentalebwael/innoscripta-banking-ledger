package persistence

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunMigrations_InputValidation(t *testing.T) {
	t.Run("EmptyMigrationsPath", func(t *testing.T) {
		err := RunMigrations("postgres://test", "")
		assert.Error(t, err)
		assert.EqualError(t, err, "migrations path cannot be empty")
	})

	t.Run("EmptyDatabaseURL", func(t *testing.T) {
		err := RunMigrations("", "file://./migrations")
		assert.Error(t, err)
		assert.EqualError(t, err, "database URL cannot be empty")
	})

	// Only testing input validation since full migration tests require test DB or extensive mocking
}
