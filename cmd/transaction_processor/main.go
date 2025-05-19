package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/innoscripta-banking-ledger/internal/data/mongo"
	"github.com/innoscripta-banking-ledger/internal/data/postgres"
	"github.com/innoscripta-banking-ledger/internal/logger"
	"github.com/innoscripta-banking-ledger/internal/platform/messaging/consumers"
	"github.com/innoscripta-banking-ledger/internal/platform/messaging/producers"
	"github.com/innoscripta-banking-ledger/internal/platform/persistence"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/components"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/consumer"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/outbox_poller"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/service"
)

func main() {
	// Create base context with cancellation
	appCtx, cancelAppCtx := context.WithCancel(context.Background())
	defer cancelAppCtx()

	// Initialize configuration
	cfg, err := config.LoadConfig("transaction_processor")
	if err != nil {
		// logger is not initialized yet, so we use fmt
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.NewLogger(cfg)

	log.Info("Starting Transaction Processor",
		"app_name", cfg.Application.Name,
		"env", cfg.Application.Env,
	)

	// Initialize databases with app context
	postgresDB, err := persistence.NewPostgresDB(appCtx, log, &cfg.Postgres)
	if err != nil {
		log.Error("Failed to initialize PostgreSQL", "error", err)
		os.Exit(1)
	}

	mongoDB, err := persistence.NewMongoDB(appCtx, log, &cfg.MongoDB)
	if err != nil {
		log.Error("Failed to initialize MongoDB", "error", err)
		os.Exit(1)
	}

	// Initialize repositories
	accountRepo := postgres.NewAccountRepository(log, postgresDB)
	outboxRepo := postgres.NewOutboxRepository(log, postgresDB)
	ledgerRepo := mongo.NewLedgerRepository(log, mongoDB.Database())

	// Initialize Kafka consumer
	kafkaConsumer := consumers.NewKafkaConsumer(appCtx, log, &cfg.Kafka)

	// Initialize Kafka DLQ producer
	dlqProducer, err := producers.NewDLQProducer(appCtx, log, &cfg.Kafka)
	if err != nil {
		log.Error("Failed to initialize DLQ Kafka producer", "error", err)
		os.Exit(1)
	}
	// dlqProducer might be nil if DLQTopic is not configured. Handler should be nil-safe.

	// Initialize processing service with separated concerns
	processingService := components.CreateProcessingService(
		postgresDB,
		accountRepo,
		outboxRepo,
		ledgerRepo,
		log,
		cfg,
	)

	// Initialize transaction event handler
	transactionEventHandler := consumer.NewTransactionEventHandler(
		log,
		processingService,
		dlqProducer, // Pass the DLQ producer
	)

	// Initialize outbox poller
	ledgerPublisher := outbox_poller.NewLedgerPublisher(
		outboxRepo,
		ledgerRepo,
		log,
	)
	poller := outbox_poller.NewPoller(
		&cfg.Outbox,
		outboxRepo,
		ledgerPublisher,
		log,
	)

	// Create error channel for service errors
	errChan := make(chan error, 2)

	// Create wait group for graceful shutdown
	var wg sync.WaitGroup

	// Start Kafka consumer in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("Starting Kafka consumer",
			"topic", cfg.Kafka.TransactionTopic,
			"group", cfg.Kafka.ConsumerGroup,
		)
		if err := kafkaConsumer.Subscribe(appCtx, cfg.Kafka.TransactionTopic, cfg.Kafka.ConsumerGroup, transactionEventHandler.HandleMessage); err != nil {
			errChan <- fmt.Errorf("kafka consumer error: %w", err)
		}
	}()

	// Start outbox poller in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("Starting Outbox Poller",
			"interval", cfg.Outbox.PollingInterval.String(),
			"batch_size", cfg.Outbox.BatchSize,
		)
		poller.Start(appCtx)
	}()

	// Set up signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Wait for a shutdown signal or error
	var serviceErr error
	select {
	case <-quit:
		log.Info("Shutdown signal received")
	case err := <-errChan:
		log.Error("Service error occurred", "error", err)
		serviceErr = err
	}

	// Cancel the application context
	cancelAppCtx()

	// Shutdown the worker pool if it's a WorkerPoolProcessingService
	if wpService, ok := processingService.(*service.WorkerPoolProcessingService); ok {
		log.Info("Shutting down worker pool", "running_workers", wpService.Running())
		wpService.Shutdown()
	}

	// Create a shutdown context with timeout
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	// Graceful shutdown sequence
	log.Info("Starting graceful shutdown...")

	// Wait for all goroutines to finish
	log.Info("Waiting for services to stop...")
	wgChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgChan)
	}()

	select {
	case <-wgChan:
		log.Info("All services stopped successfully")
	case <-shutdownCtx.Done():
		log.Warn("Shutdown timeout reached, forcing exit")
	}

	// Close DLQ Kafka producer
	if dlqProducer != nil { // dlqProducer can be nil if DLQTopic was not configured
		if err = dlqProducer.Close(); err != nil {
			log.Error("Error closing DLQ Kafka producer", "error", err)
		}
	}

	// Close Kafka consumer
	if err = kafkaConsumer.Close(); err != nil {
		log.Error("Error closing Kafka consumer", "error", err)
	}

	// Shutdown postgres connection pool
	postgresDB.Close()

	// Close MongoDB connection
	if err = mongoDB.Close(shutdownCtx); err != nil {
		log.Error("Error closing MongoDB connection", "error", err)
	}

	// Final status
	if serviceErr != nil {
		log.Error("Transaction Processor shutdown with errors", "error", serviceErr)
	}
	if err != nil {
		log.Error("Transaction Processor shutdown completed with errors")
	} else {
		log.Info("Transaction Processor shutdown completed successfully")
	}
}
