package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// CorrelationIDHeader is the HTTP header for correlation ID
	CorrelationIDHeader = "X-Correlation-ID"

	// CorrelationIDKey is the key used to store correlation ID in the context
	CorrelationIDKey = "correlation_id"
)

// CorrelationID middleware ensures each request has a unique identifier for tracing
func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		c.Header(CorrelationIDHeader, correlationID)
		c.Set(CorrelationIDKey, correlationID)

		c.Next()
	}
}

// GetCorrelationID retrieves the correlation ID from the gin context if present
func GetCorrelationID(c *gin.Context) string {
	if id, exists := c.Get(CorrelationIDKey); exists {
		if correlationID, ok := id.(string); ok {
			return correlationID
		}
	}
	return ""
}
