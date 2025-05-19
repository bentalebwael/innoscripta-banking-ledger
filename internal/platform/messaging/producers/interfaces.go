package producers

import (
	"context"

	"github.com/segmentio/kafka-go"
)

// MessagePublisher handles publishing messages to a primary topic
type MessagePublisher interface {
	Publish(ctx context.Context, key string, value interface{}) error
	Close() error
}

// DeadLetterPublisher handles publishing messages to a Dead Letter Queue
type DeadLetterPublisher interface {
	PublishToDLQ(ctx context.Context, key string, originalMessageValue []byte, reason string) error
	Close() error
}

// KafkaWriter wraps kafka.Writer methods for testing
type KafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}
