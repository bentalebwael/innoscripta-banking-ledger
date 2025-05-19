// Package config provides configuration structures and validation for the application.
// It handles environment-based configuration for all major components including
// server settings, database connections, message queues, and operational parameters.
package config

import (
	"errors"
	"strings"
	"time"
)

// Config holds the complete application configuration with settings for all components.
// Each field represents a major subsystem's configuration (e.g., HTTP server, databases,
// message queues) and is validated during application startup.
type Config struct {
	Application ApplicationConfig
	Logging     LoggingConfig
	Server      ServerConfig
	Kafka       KafkaConfig
	Postgres    PostgresConfig
	MongoDB     MongoDBConfig
	Outbox      OutboxConfig
	WorkerPool  WorkerPoolConfig
}

// ApplicationConfig contains general application configuration
type ApplicationConfig struct {
	Env  string
	Name string
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level string
}

// ServerConfig contains HTTP server configuration settings
type ServerConfig struct {
	Port            int           // Port to listen on
	ShutdownTimeout time.Duration // Grace period for server shutdown
	ReadTimeout     time.Duration // Maximum duration for reading entire request
	WriteTimeout    time.Duration // Maximum duration for writing response
	IdleTimeout     time.Duration // Maximum duration to wait for next request
}

// KafkaConfig contains Kafka configuration
type KafkaConfig struct {
	Brokers           string
	TransactionTopic  string
	NumPartitions     int // Number of partitions for topics
	ReplicationFactor int // Replication factor for topics
	ConsumerGroup     string
	MinBytes          int
	MaxBytes          int
	MaxWait           time.Duration
	StartOffset       int64
	DLQTopic          string // Topic for Dead Letter Queue
}

// PostgresConfig contains PostgreSQL configuration
type PostgresConfig struct {
	URL             string        // Database connection string
	MaxConns        int32         // Maximum number of open connections
	MinConns        int32         // Maximum number of idle connections
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
	ConnMaxIdleTime time.Duration // Maximum idle time of a connection
	MigrationsPath  string        // Path to migration files
}

// MongoDBConfig contains MongoDB configuration
type MongoDBConfig struct {
	URI             string
	Database        string
	Timeout         time.Duration
	MaxPoolSize     uint64
	MinPoolSize     uint64
	MaxConnIdleTime time.Duration
}

// OutboxConfig contains outbox pattern configuration
type OutboxConfig struct {
	PollingInterval  time.Duration
	BatchSize        int
	MaxRetryAttempts int // Maximum number of retry attempts for outbox messages
}

// WorkerPoolConfig contains worker pool configuration
type WorkerPoolConfig struct {
	Size int // Maximum number of workers in the pool
}

// validate performs comprehensive validation of all configuration values,
// ensuring they meet minimum requirements and logical constraints
func (c *Config) validate() error {
	var validationErrors []string

	// Validate Server config
	if c.Server.Port <= 0 {
		validationErrors = append(validationErrors, "SERVER_PORT must be greater than 0")
	}
	if c.Server.ShutdownTimeout <= 0 {
		validationErrors = append(validationErrors, "SERVER_SHUTDOWN_TIMEOUT must be greater than 0")
	}
	if c.Server.ReadTimeout <= 0 {
		validationErrors = append(validationErrors, "SERVER_READ_TIMEOUT must be greater than 0")
	}
	if c.Server.WriteTimeout <= 0 {
		validationErrors = append(validationErrors, "SERVER_WRITE_TIMEOUT must be greater than 0")
	}
	if c.Server.IdleTimeout <= 0 {
		validationErrors = append(validationErrors, "SERVER_IDLE_TIMEOUT must be greater than 0")
	}

	// Validate Kafka config
	if len(c.Kafka.Brokers) == 0 {
		validationErrors = append(validationErrors, "KAFKA_BROKERS is required")
	}
	if c.Kafka.TransactionTopic == "" {
		validationErrors = append(validationErrors, "KAFKA_TRANSACTION_TOPIC is required")
	}
	if c.Kafka.ConsumerGroup == "" {
		validationErrors = append(validationErrors, "KAFKA_CONSUMER_GROUP is required")
	}
	if c.Kafka.MinBytes <= 0 {
		validationErrors = append(validationErrors, "KAFKA_CONSUMER_MIN_BYTES must be greater than 0")
	}
	if c.Kafka.MaxBytes <= 0 {
		validationErrors = append(validationErrors, "KAFKA_CONSUMER_MAX_BYTES must be greater than 0")
	}
	if c.Kafka.MaxWait <= 0 {
		validationErrors = append(validationErrors, "KAFKA_CONSUMER_MAX_WAIT must be greater than 0")
	}
	if c.Kafka.DLQTopic == "" {
		validationErrors = append(validationErrors, "KAFKA_DLQ_TOPIC is required")
	}

	// Validate PostgreSQL config
	if c.Postgres.URL == "" {
		validationErrors = append(validationErrors, "POSTGRES_URL is required")
	}
	if c.Postgres.MaxConns <= 0 {
		validationErrors = append(validationErrors, "POSTGRES_MAX_CONNS must be greater than 0")
	}
	if c.Postgres.MinConns <= 0 {
		validationErrors = append(validationErrors, "POSTGRES_MIN_CONNS must be greater than 0")
	}
	if c.Postgres.ConnMaxLifetime <= 0 {
		validationErrors = append(validationErrors, "POSTGRES_MAX_CONN_LIFETIME must be greater than 0")
	}
	if c.Postgres.ConnMaxIdleTime <= 0 {
		validationErrors = append(validationErrors, "POSTGRES_MAX_CONN_IDLE_TIME must be greater than 0")
	}

	// Validate MongoDB config
	if c.MongoDB.URI == "" {
		validationErrors = append(validationErrors, "MONGO_URI is required")
	}
	if c.MongoDB.Database == "" {
		validationErrors = append(validationErrors, "MONGO_DATABASE is required")
	}
	if c.MongoDB.Timeout <= 0 {
		validationErrors = append(validationErrors, "MONGO_TIMEOUT must be greater than 0")
	}
	if c.MongoDB.MaxPoolSize <= 0 {
		validationErrors = append(validationErrors, "MONGO_MAX_POOL_SIZE must be greater than 0")
	}
	if c.MongoDB.MinPoolSize <= 0 {
		validationErrors = append(validationErrors, "MONGO_MIN_POOL_SIZE must be greater than 0")
	}
	if c.MongoDB.MaxConnIdleTime <= 0 {
		validationErrors = append(validationErrors, "MONGO_MAX_CONN_IDLE_TIME must be greater than 0")
	}

	// Validate Outbox config
	if c.Outbox.PollingInterval <= 0 {
		validationErrors = append(validationErrors, "OUTBOX_POLLING_INTERVAL must be greater than 0")
	}
	if c.Outbox.BatchSize <= 0 {
		validationErrors = append(validationErrors, "OUTBOX_BATCH_SIZE must be greater than 0")
	}
	if c.Outbox.MaxRetryAttempts <= 0 {
		validationErrors = append(validationErrors, "OUTBOX_MAX_RETRY_ATTEMPTS must be greater than 0")
	}

	// Validate WorkerPool config
	if c.WorkerPool.Size <= 0 {
		validationErrors = append(validationErrors, "WORKER_POOL_SIZE must be greater than 0")
	}

	if len(validationErrors) > 0 {
		return errors.New(strings.Join(validationErrors, ", "))
	}

	return nil
}
