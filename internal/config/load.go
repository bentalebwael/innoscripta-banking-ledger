package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// LoadConfigWithName loads configuration using the specified name, auto-detecting the file type
// This is useful when the configuration file extension is unknown or variable
func LoadConfigWithName(configName string) (*Config, error) {
	return loadConfig(configName, "")
}

// LoadConfigWithNameAndType loads configuration with explicit name and type specification
// Use this when you need to force a specific configuration format (e.g., "yaml", "json")
func LoadConfigWithNameAndType(configName, configType string) (*Config, error) {
	return loadConfig(configName, configType)
}

// LoadConfig loads configuration from a .env file using the provided base name
// This is the preferred method for loading environment-specific configurations
func LoadConfig(configName string) (*Config, error) {
	configFileName := fmt.Sprintf("%s.env", configName)
	return loadConfig(configFileName, "env")
}

// loadConfig handles configuration loading from files and environment variables.
// It implements a layered approach to configuration:
// 1. Load defaults
// 2. Override with config file values (if found)
// 3. Override with environment variables
// 4. Validate the final configuration
func loadConfig(configName, configType string) (*Config, error) {
	// Initialize viper with default values
	v := viper.New()
	setDefaults(v)

	v.SetConfigName(configName)
	if configType != "" {
		v.SetConfigType(configType)
	}

	// Add config paths in order of priority
	v.AddConfigPath("./configs") // First check in configs directory
	v.AddConfigPath(".")         // Then check in root directory

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Printf("INFO: No config file '%s' found, relying on environment variables and defaults.\n", configName)
		} else {
			fmt.Printf("WARNING: Error reading config file (%s): %v\n", v.ConfigFileUsed(), err)
		}
	} else {
		fmt.Printf("INFO: Config loaded from file: %s\n", v.ConfigFileUsed())
	}

	v.AutomaticEnv() // Automatically read matching environment variables

	// Build the config struct
	config := &Config{
		Application: ApplicationConfig{
			Env:  v.GetString("APP_ENV"),
			Name: v.GetString("APP_NAME"),
		},
		Logging: LoggingConfig{
			Level: v.GetString("LOG_LEVEL"),
		},
		Server: ServerConfig{
			Port:            v.GetInt("SERVER_PORT"),
			ShutdownTimeout: v.GetDuration("SERVER_SHUTDOWN_TIMEOUT"),
			ReadTimeout:     v.GetDuration("SERVER_READ_TIMEOUT"),
			WriteTimeout:    v.GetDuration("SERVER_WRITE_TIMEOUT"),
			IdleTimeout:     v.GetDuration("SERVER_IDLE_TIMEOUT"),
		},
		Kafka: KafkaConfig{
			Brokers:           v.GetString("KAFKA_BROKERS"),
			TransactionTopic:  v.GetString("KAFKA_TRANSACTION_TOPIC"),
			NumPartitions:     v.GetInt("KAFKA_NUM_PARTITIONS"),
			ReplicationFactor: v.GetInt("KAFKA_REPLICATION_FACTOR"),
			ConsumerGroup:     v.GetString("KAFKA_CONSUMER_GROUP"),
			MinBytes:          v.GetInt("KAFKA_CONSUMER_MIN_BYTES"),
			MaxBytes:          v.GetInt("KAFKA_CONSUMER_MAX_BYTES"),
			MaxWait:           v.GetDuration("KAFKA_CONSUMER_MAX_WAIT"),
			StartOffset:       v.GetInt64("KAFKA_CONSUMER_START_OFFSET"),
			DLQTopic:          v.GetString("KAFKA_DLQ_TOPIC"),
		},
		Postgres: PostgresConfig{
			URL:             v.GetString("POSTGRES_URL"),
			MaxConns:        int32(v.GetInt("POSTGRES_MAX_CONNS")),
			MinConns:        int32(v.GetInt("POSTGRES_MIN_CONNS")),
			ConnMaxLifetime: v.GetDuration("POSTGRES_MAX_CONN_LIFETIME"),
			ConnMaxIdleTime: v.GetDuration("POSTGRES_MAX_CONN_IDLE_TIME"),
			MigrationsPath:  v.GetString("POSTGRES_MIGRATIONS_PATH"),
		},
		MongoDB: MongoDBConfig{
			URI:             v.GetString("MONGO_URI"),
			Database:        v.GetString("MONGO_DATABASE"),
			Timeout:         v.GetDuration("MONGO_TIMEOUT"),
			MaxPoolSize:     uint64(v.GetInt("MONGO_MAX_POOL_SIZE")),
			MinPoolSize:     uint64(v.GetInt("MONGO_MIN_POOL_SIZE")),
			MaxConnIdleTime: v.GetDuration("MONGO_MAX_CONN_IDLE_TIME"),
		},
		Outbox: OutboxConfig{
			PollingInterval:  v.GetDuration("OUTBOX_POLLING_INTERVAL"),
			BatchSize:        v.GetInt("OUTBOX_BATCH_SIZE"),
			MaxRetryAttempts: v.GetInt("OUTBOX_MAX_RETRY_ATTEMPTS"),
		},
		WorkerPool: WorkerPoolConfig{
			Size: v.GetInt("WORKER_POOL_SIZE"),
		},
	}

	// Validate the configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// setDefaults initializes configuration with sensible default values.
// These values are used when no configuration file or environment variables are present.
func setDefaults(v *viper.Viper) {
	// HTTP Server defaults - tuned for typical web application workloads
	v.SetDefault("SERVER_PORT", 8080)
	v.SetDefault("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second)
	v.SetDefault("SERVER_READ_TIMEOUT", 30*time.Second)
	v.SetDefault("SERVER_WRITE_TIMEOUT", 30*time.Second)
	v.SetDefault("SERVER_IDLE_TIMEOUT", 120*time.Second)

	// Kafka defaults - configured for development environment
	// Production environments should override these with appropriate values
	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("KAFKA_TRANSACTION_TOPIC", "transaction_requests")
	v.SetDefault("KAFKA_NUM_PARTITIONS", 1)
	v.SetDefault("KAFKA_REPLICATION_FACTOR", 1)
	v.SetDefault("KAFKA_CONSUMER_GROUP", "transaction-processor-group")
	v.SetDefault("KAFKA_CONSUMER_MIN_BYTES", 10240)
	v.SetDefault("KAFKA_CONSUMER_MAX_BYTES", 10485760)
	v.SetDefault("KAFKA_CONSUMER_MAX_WAIT", time.Second)
	v.SetDefault("KAFKA_CONSUMER_START_OFFSET", 0)
	v.SetDefault("KAFKA_DLQ_TOPIC", "transaction_requests_dlq") // Default DLQ topic name

	// PostgreSQL defaults - balanced settings for moderate workloads
	// Adjust pool sizes based on application requirements
	v.SetDefault("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/banking_ledger?sslmode=disable")
	v.SetDefault("POSTGRES_MAX_CONNS", 20)
	v.SetDefault("POSTGRES_MIN_CONNS", 5)
	v.SetDefault("POSTGRES_MAX_CONN_LIFETIME", time.Hour)
	v.SetDefault("POSTGRES_MAX_CONN_IDLE_TIME", 30*time.Minute)
	v.SetDefault("POSTGRES_MIGRATIONS_PATH", "migrations/postgres") // Default migration path

	// MongoDB defaults - configured for typical application needs
	// Pool sizes should be adjusted based on workload characteristics
	v.SetDefault("MONGO_URI", "mongodb://localhost:27017")
	v.SetDefault("MONGO_DATABASE", "banking_ledger")
	v.SetDefault("MONGO_TIMEOUT", 10*time.Second)
	v.SetDefault("MONGO_MAX_POOL_SIZE", 100)
	v.SetDefault("MONGO_MIN_POOL_SIZE", 10)
	v.SetDefault("MONGO_MAX_CONN_IDLE_TIME", 30*time.Minute)

	// Outbox pattern defaults - balanced between throughput and resource usage
	v.SetDefault("OUTBOX_POLLING_INTERVAL", 5*time.Second)
	v.SetDefault("OUTBOX_BATCH_SIZE", 100)
	v.SetDefault("OUTBOX_MAX_RETRY_ATTEMPTS", 5)

	// Logging defaults - 'info' provides good balance of information vs noise
	v.SetDefault("LOG_LEVEL", "info")

	// Application defaults - development-friendly baseline configuration
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_NAME", "banking-ledger")

	// Worker Pool defaults - suitable for most applications
	v.SetDefault("WORKER_POOL_SIZE", 10) // Provides good concurrency without overwhelming resources
}
