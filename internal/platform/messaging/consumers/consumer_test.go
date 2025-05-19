package consumers

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKafkaConsumer(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := &config.KafkaConfig{
		Brokers:          "localhost:9092",
		TransactionTopic: "test-topic",
		ConsumerGroup:    "test-group",
		MinBytes:         1024,
		MaxBytes:         10240,
		MaxWait:          time.Second,
	}

	consumer := NewKafkaConsumer(context.Background(), logger, cfg)
	require.NotNil(t, consumer)
	require.NotNil(t, consumer.reader, "Kafka reader should be initialized")
	assert.Equal(t, logger, consumer.logger)

	// Limited verification possible as kafka.Reader config is not publicly accessible
}

func TestKafkaConsumer_Close(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	t.Run("CloseWithNilReader", func(t *testing.T) {
		consumer := &KafkaConsumer{
			reader: nil,
			logger: logger,
		}
		err := consumer.Close()
		require.NoError(t, err, "Close should return nil if reader is nil")
	})
}

// Subscribe and Close methods with non-nil reader require mock interfaces for proper testing
