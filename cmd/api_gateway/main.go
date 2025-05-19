package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/innoscripta-banking-ledger/internal/api_gateway"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/service"
	"github.com/innoscripta-banking-ledger/internal/config"
	"github.com/innoscripta-banking-ledger/internal/data/mongo"
	"github.com/innoscripta-banking-ledger/internal/data/postgres"
	"github.com/innoscripta-banking-ledger/internal/logger"
	"github.com/innoscripta-banking-ledger/internal/platform/messaging/producers"
	"github.com/innoscripta-banking-ledger/internal/platform/persistence"
)

func main() {
	// Create base context with cancellation
	appCtx, cancelAppCtx := context.WithCancel(context.Background())
	defer cancelAppCtx()

	// Initialize configuration
	cfg, err := config.LoadConfig("api_gateway")
	if err != nil {
		// logger is not initialized yet, so we use fmt
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.NewLogger(cfg)

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

	// Initialize Kafka producer for API Gateway (publishes to main transaction topic)
	kafkaProducer, err := producers.NewTransactionReqMessageProducer(appCtx, log, &cfg.Kafka)
	if err != nil {
		log.Error("Failed to initialize API Gateway Kafka producer", "error", err)
		os.Exit(1)
	}

	// Initialize repositories
	accountRepo := postgres.NewAccountRepository(log, postgresDB)
	ledgerRepo := mongo.NewLedgerRepository(log, mongoDB.Database())

	// Initialize services
	accountService := service.NewAccountService(accountRepo)
	transactionService := service.NewTransactionService(log, ledgerRepo, kafkaProducer)

	// Initialize REST server
	server := api_gateway.NewServer(log, cfg, accountService, transactionService)
	log.Info("REST server initialized")

	// Create error channel for server errors
	errChan := make(chan error, 1)

	// Start server in goroutine
	go func() {
		log.Info("Starting HTTP server", "port", cfg.Server.Port)
		if err := server.Start(); err != nil {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Set up signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Wait for a shutdown signal or error
	var serverErr error
	select {
	case <-quit:
		log.Info("Shutdown signal received")
	case err := <-errChan:
		log.Error("Server error occurred", "error", err)
		serverErr = err
	}

	// Cancel the application context
	cancelAppCtx()

	// Create a shutdown context with timeout
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancelShutdown()

	// Graceful shutdown sequence
	log.Info("Starting graceful shutdown...")

	// Shutdown postgres connection pool
	postgresDB.Close()

	// Shutdown HTTP server
	if err = server.Stop(shutdownCtx); err != nil {
		log.Error("Error during server shutdown", "error", err)
	}

	if err = kafkaProducer.Close(); err != nil {
		log.Error("Error closing Kafka producer", "error", err)
	}

	if err = mongoDB.Close(shutdownCtx); err != nil {
		log.Error("Error closing MongoDB connection", "error", err)
	}

	// Final status
	if serverErr != nil {
		log.Error("HTTP server shutdown with errors", "error", serverErr)
	}
	if err != nil {
		log.Error("Server shutdown completed with errors")
	} else {
		log.Info("Server shutdown completed successfully")
	}
}
