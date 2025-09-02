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

	// Create a test context and response recorder
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a test request
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	c.Request = req

	// Apply the tracing middleware with default config
	config := tracing.Config{
		TraceHeader: "AH-Trace-Id",
	}

	tracingMiddleware := WithTracing(config)
	tracingMiddleware(c)

	// Check that the trace ID header is set
	traceIDHeader := w.Header().Get("Ah-Trace-Id")
	assert.NotEmpty(t, traceIDHeader)
	assert.Contains(t, traceIDHeader, "at-")

	// Check that the trace ID in the context matches the header
	traceID, ok := tracing.GetTraceID(c.Request.Context())
	assert.True(t, ok)
	assert.Equal(t, tracing.TraceID(traceIDHeader), traceID)
}

func TestWithTracingExistingHeader(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test context and response recorder
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a test request with an existing trace ID header
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Ah-Trace-Id", "at-existing-trace-id")
	c.Request = req

	// Apply the tracing middleware with default config
	config := tracing.Config{
		TraceHeader: "AH-Trace-Id",
	}

	tracingMiddleware := WithTracing(config)
	tracingMiddleware(c)

	// Check that the trace ID header is preserved
	traceIDHeader := w.Header().Get("Ah-Trace-Id")
	assert.Equal(t, "at-existing-trace-id", traceIDHeader)

	// Check that the trace ID in the context matches the header
	traceID, ok := tracing.GetTraceID(c.Request.Context())
	assert.True(t, ok)
	assert.Equal(t, tracing.TraceID(traceIDHeader), traceID)
}

func TestWithTracingCustomHeader(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test context and response recorder
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a test request with a custom trace ID header
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Custom-Trace-Id", "at-custom-trace-id")
	c.Request = req

	// Apply the tracing middleware with custom header config
	config := tracing.Config{
		TraceHeader: "X-Custom-Trace-ID",
	}

	tracingMiddleware := WithTracing(config)
	tracingMiddleware(c)

	// Check that the trace ID header is preserved
	traceIDHeader := w.Header().Get("X-Custom-Trace-Id")
	assert.Equal(t, "at-custom-trace-id", traceIDHeader)

	// Check that the trace ID in the context matches the header
	traceID, ok := tracing.GetTraceID(c.Request.Context())
	assert.True(t, ok)
	assert.Equal(t, tracing.TraceID(traceIDHeader), traceID)
}

func TestWithTracingEmptyConfig(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test context and response recorder
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a test request
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	c.Request = req

	// Apply the tracing middleware with empty config (should default to AH-Trace-Id)
	config := tracing.Config{}

	tracingMiddleware := WithTracing(config)
	tracingMiddleware(c)

	// Check that the trace ID header is set with default name
	traceIDHeader := w.Header().Get("Ah-Trace-Id")
	assert.NotEmpty(t, traceIDHeader)
	assert.Contains(t, traceIDHeader, "at-")

	// Check that the trace ID in the context matches the header
	traceID, ok := tracing.GetTraceID(c.Request.Context())
	assert.True(t, ok)
	assert.Equal(t, tracing.TraceID(traceIDHeader), traceID)
}
