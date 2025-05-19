package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_HappyPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempConfigsSubDir := filepath.Join(tempDir, "configs")
	err = os.Mkdir(tempConfigsSubDir, 0755)
	require.NoError(t, err)

	testAppName := "TestApp"
	testPort := 9090
	testLogLevel := "debug"
	testKafkaBrokers := "kafka1:9092,kafka2:9092"

	envContent := fmt.Sprintf(
		"APP_NAME=%s\nSERVER_PORT=%d\nLOG_LEVEL=%s\nKAFKA_BROKERS=%s\n",
		testAppName, testPort, testLogLevel, testKafkaBrokers,
	)
	envFilePath := filepath.Join(tempConfigsSubDir, "test_happy.env")
	err = os.WriteFile(envFilePath, []byte(envContent), 0644)
	require.NoError(t, err)

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	cfg, err := LoadConfig("test_happy")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, testAppName, cfg.Application.Name)
	assert.Equal(t, testPort, cfg.Server.Port)
	assert.Equal(t, testLogLevel, cfg.Logging.Level)
	assert.Equal(t, testKafkaBrokers, cfg.Kafka.Brokers)

	assert.Equal(t, "development", cfg.Application.Env)
	assert.Equal(t, 30*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "transaction_requests", cfg.Kafka.TransactionTopic)
	assert.Equal(t, "mongodb://localhost:27017", cfg.MongoDB.URI)
	assert.Equal(t, 10, cfg.WorkerPool.Size)

	cfgWithName, err := LoadConfigWithName("configs/test_happy") // Viper will look for configs/test_happy.env
	require.NoError(t, err)
	require.NotNil(t, cfgWithName)
	assert.Equal(t, testAppName, cfgWithName.Application.Name)

	// Test LoadConfigWithNameAndType
	cfgWithNameAndType, err := LoadConfigWithNameAndType("configs/test_happy", "env")
	require.NoError(t, err)
	require.NotNil(t, cfgWithNameAndType)
	assert.Equal(t, testAppName, cfgWithNameAndType.Application.Name)

}

func TestConfig_Validate_HappyPath(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	cfg := &Config{
		Application: ApplicationConfig{Env: v.GetString("APP_ENV"), Name: v.GetString("APP_NAME")},
		Logging:     LoggingConfig{Level: v.GetString("LOG_LEVEL")},
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
	err := cfg.validate()
	assert.NoError(t, err, "Default config should be valid")
}
