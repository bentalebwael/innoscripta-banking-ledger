package producers

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// createKafkaTopicIfNotExists creates Kafka topic if not found, retries on partition read errors
func createKafkaTopicIfNotExists(conn *kafka.Conn, topicName string, numPartitions int, replicationFactor int, log *slog.Logger) error {
	var partitions []kafka.Partition
	var err error

	log.Info("Checking if Kafka topic exists", "topic", topicName)
	for i := 0; i < 5; i++ { // Retry topic partition read
		partitions, err = conn.ReadPartitions(topicName)
		if err == nil {
			break // Topic exists and partitions read
		}
		log.Warn("Failed to read partitions, retrying...", "topic", topicName, "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second) // Wait before retrying
	}

	if err != nil && len(partitions) == 0 {
		log.Info("Could not definitively read partitions (or topic does not exist), attempting to create topic", "topic", topicName, "last_error_read", err)
	}

	if len(partitions) == 0 {
		log.Info("Kafka topic does not exist or is not accessible, attempting to create it", "topic", topicName)
		topicConfig := kafka.TopicConfig{
			Topic:             topicName,
			NumPartitions:     numPartitions,
			ReplicationFactor: replicationFactor,
		}
		if topicConfig.NumPartitions == 0 {
			topicConfig.NumPartitions = 1
			log.Debug("Defaulting NumPartitions to 1 for topic creation", "topic", topicName)
		}
		if topicConfig.ReplicationFactor == 0 {
			topicConfig.ReplicationFactor = 1
			log.Debug("Defaulting ReplicationFactor to 1 for topic creation", "topic", topicName)
		}

		creationErr := conn.CreateTopics(topicConfig)
		if creationErr != nil {
			return fmt.Errorf("failed to create kafka topic %s: %w", topicName, creationErr)
		}
		log.Info("Successfully created Kafka topic", "topic", topicName)
	} else if err == nil {
		log.Info("Kafka topic already exists", "topic", topicName)
	} else {
		log.Warn("Kafka topic seems to exist but there was an error during final partition read attempt", "topic", topicName, "error", err)
	}
	return nil
}
