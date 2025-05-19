package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCorrelationIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GeneratesCorrelationIDIfNotProvided", func(t *testing.T) {
		router := gin.New()
		router.Use(CorrelationID())
		var capturedContextID string
		router.GET("/test", func(c *gin.Context) {
			capturedContextID = c.GetString(CorrelationIDKey)
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		respHeaderID := rr.Header().Get(CorrelationIDHeader)
		assert.NotEmpty(t, respHeaderID)
		_, err := uuid.Parse(respHeaderID)
		assert.NoError(t, err, "Generated Correlation ID in header should be a valid UUID")

		assert.NotEmpty(t, capturedContextID)
		_, err = uuid.Parse(capturedContextID)
		assert.NoError(t, err, "Generated Correlation ID in context should be a valid UUID")

		assert.Equal(t, respHeaderID, capturedContextID)
	})

	t.Run("UsesCorrelationIDIfProvided", func(t *testing.T) {
		router := gin.New()
		router.Use(CorrelationID())
		var capturedContextID string
		router.GET("/test", func(c *gin.Context) {
			capturedContextID = c.GetString(CorrelationIDKey)
			c.Status(http.StatusOK)
		})

		providedID := uuid.New().String()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(CorrelationIDHeader, providedID)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		respHeaderID := rr.Header().Get(CorrelationIDHeader)
		assert.Equal(t, providedID, respHeaderID)

		assert.Equal(t, providedID, capturedContextID)
	})
}

func TestGetCorrelationID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ReturnsIDFromContextIfExists", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		expectedID := uuid.New().String()
		c.Set(CorrelationIDKey, expectedID)

		retrievedID := GetCorrelationID(c)
		assert.Equal(t, expectedID, retrievedID)
	})

	t.Run("ReturnsEmptyStringIfNoIDInContext", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		retrievedID := GetCorrelationID(c)
		assert.Empty(t, retrievedID)
	})

	t.Run("ReturnsEmptyStringIfIDInContextIsNotString", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(CorrelationIDKey, 12345)

		retrievedID := GetCorrelationID(c)
		assert.Empty(t, retrievedID)
	})
}
