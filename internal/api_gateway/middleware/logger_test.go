package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestLoggerMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("LogsRequestDetails", func(t *testing.T) {
		var logBuffer bytes.Buffer
		testLogger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		router := gin.New()
		router.Use(CorrelationID())
		router.Use(Logger(testLogger))

		router.GET("/test_log", func(c *gin.Context) {
			c.String(http.StatusOK, "OK")
		})

		req, _ := http.NewRequest(http.MethodGet, "/test_log?param=value", nil)
		req.Header.Set("User-Agent", "test-agent")
		testCorrelationID := uuid.New().String()
		req.Header.Set(CorrelationIDHeader, testCorrelationID)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		logOutput := logBuffer.String()
		assert.NotEmpty(t, logOutput, "Log output should not be empty")

		assert.Contains(t, logOutput, `"level":"INFO"`)
		assert.Contains(t, logOutput, `"msg":"HTTP request"`)
		assert.Contains(t, logOutput, `"method":"GET"`)
		assert.Contains(t, logOutput, `"path":"/test_log?param=value"`)
		assert.Contains(t, logOutput, `"status":200`)
		assert.Contains(t, logOutput, `"latency":`)
		assert.Contains(t, logOutput, `"client_ip":`)
		assert.Contains(t, logOutput, `"user_agent":"test-agent"`)
		assert.Contains(t, logOutput, `"correlation_id":"`+testCorrelationID+`"`)
	})

	t.Run("LogsRequestDetailsWithoutProvidedCorrelationID", func(t *testing.T) {
		var logBuffer bytes.Buffer
		testLogger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		router := gin.New()
		router.Use(CorrelationID())
		router.Use(Logger(testLogger))

		router.POST("/another_log", func(c *gin.Context) {
			c.String(http.StatusCreated, "Created")
		})

		req, _ := http.NewRequest(http.MethodPost, "/another_log", strings.NewReader("body"))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		logOutput := logBuffer.String()

		assert.Contains(t, logOutput, `"msg":"HTTP request"`)
		assert.Contains(t, logOutput, `"method":"POST"`)
		assert.Contains(t, logOutput, `"path":"/another_log"`)
		assert.Contains(t, logOutput, `"status":201`)
		assert.Contains(t, logOutput, `"correlation_id":`)
	})
}
