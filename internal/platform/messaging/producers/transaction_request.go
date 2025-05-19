package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/segmentio/kafka-go"
)

type TransactionReqMessageProducer struct {
	logger *slog.Logger
	writer KafkaWriter // Interface for testability
	topic  string
}

// Creates a new API Gateway producer and ensures topic exists
func NewTransactionReqMessageProducer(ctx context.Context, logger *slog.Logger, cfg *config.KafkaConfig) (*TransactionReqMessageProducer, error) {
	if cfg.TransactionTopic == "" {
		return nil, fmt.Errorf("kafka transaction topic is not configured")
	}

	conn, err := kafka.Dial("tcp", cfg.Brokers)
	if err != nil {
		return nil, fmt.Errorf("failed to dial kafka for api gateway producer: %w", err)
	}
	defer conn.Close()

	err = createKafkaTopicIfNotExists(conn, cfg.TransactionTopic, cfg.NumPartitions, cfg.ReplicationFactor, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure transaction topic %s exists for api gateway producer: %w", cfg.TransactionTopic, err)
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers),
		Topic:        cfg.TransactionTopic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        true, // Using async for high throughput
		WriteTimeout: cfg.MaxWait,
		Completion: func(messages []kafka.Message, err error) {
			if err != nil {
				logger.Error("Failed to write messages asynchronously", "topic", cfg.TransactionTopic, "error", err, "count", len(messages))
			} else {
				logger.Debug("Successfully wrote messages asynchronously", "topic", cfg.TransactionTopic, "count", len(messages))
			}
		},
	}

	return &TransactionReqMessageProducer{
		logger: logger,
		writer: writer,
		topic:  cfg.TransactionTopic,
	}, nil
}

func (p *TransactionReqMessageProducer) Publish(ctx context.Context, key string, value interface{}) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal message value for api gateway producer: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: jsonValue,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("Failed to publish message via api gateway producer",
			"topic", p.topic,
			"key", key,
			"error", err,
		)
		return fmt.Errorf("failed to publish message to %s via api gateway producer: %w", p.topic, err)
	}

	p.logger.Debug("Published message via api gateway producer",
		"topic", p.topic,
		"key", key,
	)
	return nil
}

func (p *TransactionReqMessageProducer) Close() error {
	p.logger.Info("Closing API Gateway Kafka message producer", "topic", p.topic)
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("failed to close api gateway kafka writer for topic %s: %w", p.topic, err)
	}
	return nil
}
