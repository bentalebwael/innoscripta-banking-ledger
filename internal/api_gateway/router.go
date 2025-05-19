package api_gateway

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/handler"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/middleware"
)

// setupRouter configures API routes and middleware for the application
func setupRouter(
	logger *slog.Logger,
	r *gin.Engine,
	accountHandler *handler.AccountHandler,
	transactionHandler *handler.TransactionHandler,
) {
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CorrelationID())

	// API v1 endpoints
	v1 := r.Group("/api/v1")
	{
		// Account operations
		accounts := v1.Group("/accounts")
		{
			accounts.POST("", accountHandler.Create)
			accounts.GET("/:id", accountHandler.GetByID)
			accounts.GET("/:id/transactions", transactionHandler.GetByAccountID)
		}

		// Transaction operations
		transactions := v1.Group("/transactions")
		{
			transactions.POST("", transactionHandler.Create)
			transactions.GET("/:id", transactionHandler.GetByID)
		}
	}

	// Health check endpoint for monitoring
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().UTC()})
	})
}
