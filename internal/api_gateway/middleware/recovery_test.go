package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RecoversFromPanicAndLogs", func(t *testing.T) {
		var logBuffer bytes.Buffer
		testLogger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		router := gin.New()
		router.Use(CorrelationID())
		router.Use(Recovery(testLogger))

		panicMessage := "test panic"
		router.GET("/panic_test", func(c *gin.Context) {
			panic(panicMessage)
		})

		testCorrelationID := uuid.New().String()
		req, _ := http.NewRequest(http.MethodGet, "/panic_test", nil)
		req.Header.Set(CorrelationIDHeader, testCorrelationID)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var jsonResponse map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &jsonResponse)
		require.NoError(t, err)

		errorField, ok := jsonResponse["error"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "INTERNAL_SERVER_ERROR", errorField["code"])
		assert.Equal(t, "An internal server error occurred", errorField["message"])
		assert.Equal(t, testCorrelationID, jsonResponse["correlation_id"])

		logOutput := logBuffer.String()
		assert.NotEmpty(t, logOutput)

		assert.Contains(t, logOutput, `"level":"ERROR"`)
		assert.Contains(t, logOutput, `"msg":"Panic recovered"`)
		assert.Contains(t, logOutput, `"error":"`+panicMessage+`"`)
		assert.Contains(t, logOutput, `"stack":`)
		assert.Contains(t, logOutput, `"path":"/panic_test"`)
		assert.Contains(t, logOutput, `"method":"GET"`)
	})

	t.Run("NoPanicNoEffect", func(t *testing.T) {
		var logBuffer bytes.Buffer
		testLogger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

		router := gin.New()
		router.Use(Recovery(testLogger))
		router.GET("/no_panic", func(c *gin.Context) {
			c.String(http.StatusOK, "OK")
		})

		req, _ := http.NewRequest(http.MethodGet, "/no_panic", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, logBuffer.String())
	})
}
