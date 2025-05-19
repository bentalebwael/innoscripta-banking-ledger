package persistence

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoDB_Database(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// Using disconnected dummy database since mocking mongo.Database is complex
	dummyClient, _ := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	dummyDbInstance := dummyClient.Database("testdb")

	mdb := &MongoDB{
		logger:   logger,
		database: dummyDbInstance,
	}
	assert.Equal(t, dummyDbInstance, mdb.Database(), "Database() should return the initialized database instance")
}

// Limited testing due to mongo driver's concrete types requiring live DB or source changes
