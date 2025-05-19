package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery middleware catches panics, logs them with stack traces, and returns a 500 error
// with correlation ID (if available) to maintain request traceability
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())

				logger.Error("Panic recovered",
					"error", r,
					"stack", stack,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)

				correlationID := GetCorrelationID(c)

				response := gin.H{
					"error": gin.H{
						"code":    "INTERNAL_SERVER_ERROR",
						"message": "An internal server error occurred",
					},
				}

				if correlationID != "" {
					response["correlation_id"] = correlationID
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError, response)
			}
		}()

		c.Next()
	}
}
