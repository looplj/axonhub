package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/tracing"
)

func TestWithTracing(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test request and response recorder
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Create a Gin engine and router
	engine := gin.New()
	engine.Use(WithLoggingTracing(tracing.Config{
		TraceHeader: "AH-Trace-Id",
	}))

	// Add a dummy handler to complete the middleware chain
	engine.GET("/", func(c *gin.Context) {
		traceID, ok := tracing.GetTraceID(c.Request.Context())
		assert.True(t, ok)
		assert.NotEmpty(t, traceID)
		assert.Contains(t, traceID, "at-")
		c.Status(http.StatusOK)
	})

	// Perform the request
	engine.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithTracingExistingHeader(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test request with an existing trace ID header
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Ah-Trace-Id", "at-existing-trace-id")

	w := httptest.NewRecorder()

	// Create a Gin engine and router
	engine := gin.New()
	engine.Use(WithLoggingTracing(tracing.Config{
		TraceHeader: "AH-Trace-Id",
	}))

	// Add a dummy handler to complete the middleware chain
	engine.GET("/", func(c *gin.Context) {
		traceID, ok := tracing.GetTraceID(c.Request.Context())
		assert.True(t, ok)
		assert.Equal(t, "at-existing-trace-id", traceID)
		c.Status(http.StatusOK)
	})

	// Perform the request
	engine.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithTracingCustomHeader(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test request with a custom trace ID header
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Custom-Trace-Id", "at-custom-trace-id")

	w := httptest.NewRecorder()

	// Create a Gin engine and router
	engine := gin.New()
	engine.Use(WithLoggingTracing(tracing.Config{
		TraceHeader: "X-Custom-Trace-Id",
	}))

	// Add a dummy handler to complete the middleware chain
	engine.GET("/", func(c *gin.Context) {
		traceID, ok := tracing.GetTraceID(c.Request.Context())
		assert.True(t, ok)
		assert.Equal(t, "at-custom-trace-id", traceID)
		c.Status(http.StatusOK)
	})

	// Perform the request
	engine.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithTracingEmptyConfig(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test request and response recorder
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Create a Gin engine and router
	engine := gin.New()
	engine.Use(WithLoggingTracing(tracing.Config{}))

	// Add a dummy handler to complete the middleware chain
	engine.GET("/", func(c *gin.Context) {
		traceID, ok := tracing.GetTraceID(c.Request.Context())
		assert.True(t, ok)
		assert.NotEmpty(t, traceID)
		assert.Contains(t, traceID, "at-")
		c.Status(http.StatusOK)
	})

	// Perform the request
	engine.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)
}
