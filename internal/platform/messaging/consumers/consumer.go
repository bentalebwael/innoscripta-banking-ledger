package consumers

import (
	"context"
	"log/slog"
	"time"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/segmentio/kafka-go"
)

type MessageHandler func(ctx context.Context, key []byte, value []byte) error

// Consumer defines the message queue consumer interface
type Consumer interface {
	Subscribe(ctx context.Context, topic string, groupID string, handler MessageHandler) error
	Close() error
}

// KafkaConsumer implements Consumer using Kafka
type KafkaConsumer struct {
	reader *kafka.Reader
	logger *slog.Logger
}

func NewKafkaConsumer(_ context.Context, logger *slog.Logger, cfg *config.KafkaConfig) *KafkaConsumer {
	return &KafkaConsumer{
		logger: logger,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     []string{cfg.Brokers},
			Topic:       cfg.TransactionTopic,
			GroupID:     cfg.ConsumerGroup,
			MinBytes:    cfg.MinBytes,
			MaxBytes:    cfg.MaxBytes,
			MaxWait:     cfg.MaxWait,
			StartOffset: kafka.FirstOffset,
		}),
	}
}

// Subscribe subscribes to the specified topic and processes messages with the handler
func (c *KafkaConsumer) Subscribe(ctx context.Context, topic string, groupID string, handler MessageHandler) error {
	c.logger.Info("Subscribed to Kafka topic",
		"topic", topic,
		"group_id", groupID,
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("Context canceled, stopping consumer",
					"topic", topic,
					"group_id", groupID,
				)
				return
			default:
				msg, err := c.reader.FetchMessage(ctx)
				if err != nil {
					c.logger.Error("Failed to fetch message from Kafka",
						"topic", topic,
						"group_id", groupID,
						"error", err,
					)
					// If the context was canceled, return
					if ctx.Err() != nil {
						return
					}
					// Otherwise, wait a bit and try again
					time.Sleep(time.Second)
					continue
				}

				c.logger.Debug("Received message from Kafka",
					"topic", msg.Topic,
					"partition", msg.Partition,
					"offset", msg.Offset,
					"key", string(msg.Key),
				)

				processingErr := handler(ctx, msg.Key, msg.Value)
				if processingErr != nil {
					c.logger.Error("Failed to process message, will not commit offset",
						"topic", msg.Topic,
						"partition", msg.Partition,
						"offset", msg.Offset,
						"key", string(msg.Key),
						"error", processingErr,
					)
					// Failed messages are not committed to allow for reprocessing or DLQ handling
					continue
				}

				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					c.logger.Error("Failed to commit message after successful processing",
						"topic", msg.Topic,
						"partition", msg.Partition,
						"offset", msg.Offset,
						"key", string(msg.Key),
						"error", err,
					)
				} else {
					c.logger.Debug("Message committed successfully",
						"topic", msg.Topic,
						"offset", msg.Offset,
						"key", string(msg.Key),
					)
				}
			}
		}
	}()

	return nil
}

func (c *KafkaConsumer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}
