package producers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockKafkaWriter is shared across package test files - defined in transaction_request_test.go

func TestDLQProducer_PublishToDLQ(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	dlqTopic := "test-dlq-topic"
	ctx := context.Background()

	t.Run("SuccessfulPublishToDLQ", func(t *testing.T) {
		mockWriter := new(MockKafkaWriter) // Assumes MockKafkaWriter is accessible
		producer := &DLQProducer{
			logger:   logger,
			writer:   mockWriter,
			dlqTopic: dlqTopic,
		}

		key := "original-key"
		originalMessageValue := []byte(`{"data":"original_payload"}`)
		reason := "processing_failed"

		mockWriter.On("WriteMessages", ctx, mock.MatchedBy(func(msgs []kafka.Message) bool {
			if len(msgs) != 1 {
				return false
			}
			msg := msgs[0]
			if string(msg.Key) != key {
				return false
			}
			var payload map[string]string
			if err := json.Unmarshal(msg.Value, &payload); err != nil {
				return false
			}
			return payload["original_key"] == key &&
				payload["original_value"] == string(originalMessageValue) &&
				payload["dlq_reason"] == reason &&
				payload["timestamp"] != ""
		})).Return(nil).Once()

		err := producer.PublishToDLQ(ctx, key, originalMessageValue, reason)
		require.NoError(t, err)
		mockWriter.AssertExpectations(t)
	})

	t.Run("PublishToDLQReturnsErrorOnWriterError", func(t *testing.T) {
		mockWriter := new(MockKafkaWriter)
		producer := &DLQProducer{
			logger:   logger,
			writer:   mockWriter,
			dlqTopic: dlqTopic,
		}

		key := "fail-key"
		originalMessageValue := []byte("fail_payload")
		reason := "writer_error"
		writerError := errors.New("kafka DLQ write error")

		mockWriter.On("WriteMessages", ctx, mock.AnythingOfType("[]kafka.Message")).Return(writerError).Once()

		err := producer.PublishToDLQ(ctx, key, originalMessageValue, reason)
		require.Error(t, err)
		assert.True(t, errors.Is(err, writerError) || strings.Contains(err.Error(), writerError.Error()))
		mockWriter.AssertExpectations(t)
	})

	t.Run("PublishToDLQWhenProducerIsNil (DLQ Disabled)", func(t *testing.T) {
		// Note: Actual nil producer would panic due to p.logger.Warn call - testing nil writer instead
		nonNilProducerWithNilWriter := &DLQProducer{
			logger:   logger, // Provide a logger to avoid panic on p.logger
			writer:   nil,    // Simulate disabled/uninitialized writer
			dlqTopic: dlqTopic,
		}

		err := nonNilProducerWithNilWriter.PublishToDLQ(ctx, "some-key", []byte("some_payload"), "disabled_test")
		require.Error(t, err)
		assert.Equal(t, "DLQ producer not initialized", err.Error())
	})
}

func TestDLQProducer_Close(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	dlqTopic := "test-dlq-topic-close"

	t.Run("SuccessfulClose", func(t *testing.T) {
		mockWriter := new(MockKafkaWriter)
		producer := &DLQProducer{
			logger:   logger,
			writer:   mockWriter,
			dlqTopic: dlqTopic,
		}
		mockWriter.On("Close").Return(nil).Once()
		err := producer.Close()
		require.NoError(t, err)
		mockWriter.AssertExpectations(t)
	})

	t.Run("CloseReturnsErrorOnWriterError", func(t *testing.T) {
		mockWriter := new(MockKafkaWriter)
		producer := &DLQProducer{
			logger:   logger,
			writer:   mockWriter,
			dlqTopic: dlqTopic,
		}
		closeError := errors.New("kafka DLQ close error")
		mockWriter.On("Close").Return(closeError).Once()
		err := producer.Close()
		require.Error(t, err)
		assert.True(t, errors.Is(err, closeError) || strings.Contains(err.Error(), closeError.Error()))
		mockWriter.AssertExpectations(t)
	})

	t.Run("CloseWhenProducerIsNilOrWriterIsNil (DLQ Disabled)", func(t *testing.T) {
		// Testing nil writer case as nil producer would panic
		producerWithNilWriter := &DLQProducer{
			logger:   logger, // Provide a logger
			writer:   nil,
			dlqTopic: dlqTopic,
		}
		err := producerWithNilWriter.Close()
		require.NoError(t, err, "Close should return nil if writer is nil (DLQ disabled)")
	})
}
