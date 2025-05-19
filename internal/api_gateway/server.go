package api_gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/handler"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/service"
	"github.com/innoscripta-banking-ledger/internal/config"
)

// Server handles HTTP requests and manages the application's lifecycle
type Server struct {
	logger     *slog.Logger // For structured logging
	httpServer *http.Server // Underlying HTTP server
	httpRouter *gin.Engine  // Gin router instance
}

// NewServer creates and configures a new HTTP server with the given services
func NewServer(log *slog.Logger, cfg *config.Config, accountService service.AccountService, transactionService service.TransactionService) *Server {
	if cfg.Application.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	httpRouter := gin.New()

	accountHandler := handler.NewAccountHandler(log, accountService)
	transactionHandler := handler.NewTransactionHandler(log, transactionService)

	setupRouter(log, httpRouter, accountHandler, transactionHandler)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      httpRouter,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return &Server{
		logger:     log,
		httpServer: httpServer,
		httpRouter: httpRouter,
	}
}

// Start begins listening for HTTP requests
func (s *Server) Start() error {
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the HTTP server with a timeout
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping HTTP server")

	// Use server's write timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, s.httpServer.WriteTimeout)
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to stop HTTP server: %w", err)
	}
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to stop HTTP server: %w", err)
	}

	return nil
}
