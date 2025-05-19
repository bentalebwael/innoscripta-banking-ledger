package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/segmentio/kafka-go"
)

type DLQProducer struct {
	logger   *slog.Logger
	writer   KafkaWriter
	dlqTopic string
}

// Returns nil producer if cfg.DLQTopic is empty (DLQ disabled)
func NewDLQProducer(ctx context.Context, logger *slog.Logger, cfg *config.KafkaConfig) (*DLQProducer, error) {
	if cfg.DLQTopic == "" {
		logger.Info("DLQ topic is not configured. DLQProducer will not be initialized.")
		return nil, nil // DLQ is disabled, not an error.
	}

	conn, err := kafka.Dial("tcp", cfg.Brokers)
	if err != nil {
		return nil, fmt.Errorf("failed to dial kafka for dlq producer: %w", err)
	}
	defer conn.Close()

	err = createKafkaTopicIfNotExists(conn, cfg.DLQTopic, cfg.NumPartitions, cfg.ReplicationFactor, logger)
	if err != nil {
		// Return error to make DLQ topic creation failure explicit
		return nil, fmt.Errorf("failed to ensure DLQ topic %s exists for dlq producer: %w", cfg.DLQTopic, err)
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers),
		Topic:        cfg.DLQTopic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		WriteTimeout: cfg.MaxWait,
		Completion: func(messages []kafka.Message, err error) {
			if err != nil {
				logger.Error("Failed to write DLQ messages synchronously", "topic", cfg.DLQTopic, "error", err, "count", len(messages))
			} else {
				logger.Debug("Successfully wrote DLQ messages synchronously", "topic", cfg.DLQTopic, "count", len(messages))
			}
		},
	}

	return &DLQProducer{
		logger:   logger,
		writer:   writer,
		dlqTopic: cfg.DLQTopic,
	}, nil
}

func (p *DLQProducer) PublishToDLQ(ctx context.Context, key string, originalMessageValue []byte, reason string) error {
	if p == nil || p.writer == nil {
		p.logger.Warn("DLQ producer is not initialized (DLQ disabled), cannot publish message", "key", key, "reason", reason)
		return fmt.Errorf("DLQ producer not initialized") // Or return nil if we want to silently drop
	}

	dlqMessagePayload := struct {
		OriginalKey   string `json:"original_key"`
		OriginalValue string `json:"original_value"`
		DLQReason     string `json:"dlq_reason"`
		Timestamp     string `json:"timestamp"`
	}{
		OriginalKey:   key,
		OriginalValue: string(originalMessageValue),
		DLQReason:     reason,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	jsonDLQValue, err := json.Marshal(dlqMessagePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message value for dlq producer: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: jsonDLQValue,
		Headers: []kafka.Header{
			{Key: "dlq-reason", Value: []byte(reason)},
		},
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("Failed to publish message to DLQ via dlq producer",
			"topic", p.dlqTopic,
			"key", key,
			"error", err,
		)
		return fmt.Errorf("failed to publish message to DLQ %s via dlq producer: %w", p.dlqTopic, err)
	}

	p.logger.Info("Successfully published message to DLQ via dlq producer",
		"topic", p.dlqTopic,
		"key", key,
		"reason", reason,
	)
	return nil
}

func (p *DLQProducer) Close() error {
	if p == nil || p.writer == nil {
		p.logger.Info("DLQ producer not initialized or already closed.")
		return nil
	}
	p.logger.Info("Closing DLQ Kafka message producer", "topic", p.dlqTopic)
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("failed to close dlq kafka writer for topic %s: %w", p.dlqTopic, err)
	}
	return nil
}
