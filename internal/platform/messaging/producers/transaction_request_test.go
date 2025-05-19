package producers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockKafkaWriter mocks KafkaWriter interface
type MockKafkaWriter struct {
	mock.Mock
}

func (m *MockKafkaWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	args := m.Called(ctx, msgs)
	return args.Error(0)
}

func (m *MockKafkaWriter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestTransactionReqMessageProducer_Publish(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	topic := "test-transactions"
	ctx := context.Background()

	t.Run("SuccessfulPublish", func(t *testing.T) {
		mockWriter := new(MockKafkaWriter)
		producer := &TransactionReqMessageProducer{
			logger: logger,
			writer: mockWriter,
			topic:  topic,
		}

		key := "test-key"
		value := &shared.TransactionRequest{AccountID: uuid.New(), Amount: 100}
		expectedJSONValue, _ := json.Marshal(value)

		mockWriter.On("WriteMessages", ctx, mock.MatchedBy(func(msgs []kafka.Message) bool {
			if len(msgs) != 1 {
				return false
			}
			msg := msgs[0]
			return string(msg.Key) == key && string(msg.Value) == string(expectedJSONValue)
		})).Return(nil).Once()

		err := producer.Publish(ctx, key, value)
		require.NoError(t, err)
		mockWriter.AssertExpectations(t)
	})

	t.Run("PublishReturnsErrorOnWriterError", func(t *testing.T) {
		mockWriter := new(MockKafkaWriter)
		producer := &TransactionReqMessageProducer{
			logger: logger,
			writer: mockWriter,
			topic:  topic,
		}

		key := "test-key-fail"
		value := map[string]string{"data": "test-data"}
		writerError := errors.New("kafka write error")

		mockWriter.On("WriteMessages", ctx, mock.AnythingOfType("[]kafka.Message")).Return(writerError).Once()

		err := producer.Publish(ctx, key, value)
		require.Error(t, err)
		assert.True(t, errors.Is(err, writerError) || strings.Contains(err.Error(), writerError.Error()))
		mockWriter.AssertExpectations(t)
	})
}

func TestTransactionReqMessageProducer_Close(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	topic := "test-transactions-close"

	t.Run("SuccessfulClose", func(t *testing.T) {
		mockWriter := new(MockKafkaWriter)
		producer := &TransactionReqMessageProducer{
			logger: logger,
			writer: mockWriter,
			topic:  topic,
		}

		mockWriter.On("Close").Return(nil).Once()

		err := producer.Close()
		require.NoError(t, err)
		mockWriter.AssertExpectations(t)
	})

	t.Run("CloseReturnsErrorOnWriterCloseError", func(t *testing.T) {
		mockWriter := new(MockKafkaWriter)
		producer := &TransactionReqMessageProducer{
			logger: logger,
			writer: mockWriter,
			topic:  topic,
		}
		closeError := errors.New("kafka close error")

		mockWriter.On("Close").Return(closeError).Once()

		err := producer.Close()
		require.Error(t, err)
		assert.True(t, errors.Is(err, closeError) || strings.Contains(err.Error(), closeError.Error()))
		mockWriter.AssertExpectations(t)
	})
}

// Verify interface implementation
var _ KafkaWriter = (*MockKafkaWriter)(nil)
