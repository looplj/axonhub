package testutil

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"time"

	"google.golang.org/genai"
)

// Config holds configuration for Gemini tests
type Config struct {
	APIKey     string
	BaseURL    string
	TraceID    string
	ThreadID   string
	Timeout    time.Duration
	MaxRetries int
	Model      string // Default model for tests
}

// DefaultConfig returns a default configuration for Gemini tests
func DefaultConfig() *Config {
	return &Config{
		APIKey:     getEnvOrDefault("TEST_AXONHUB_API_KEY", ""),
		BaseURL:    getEnvOrDefault("TEST_GEMINI_BASE_URL", "http://localhost:8090/gemini"),
		TraceID:    getRandomTraceID(),
		ThreadID:   getRandomThreadID(),
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		Model:      getEnvOrDefault("TEST_MODEL", "gemini-2.5-flash"),
	}
}

// NewClient creates a new Gemini client with the given configuration
func (c *Config) NewClient() (*genai.Client, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("TEST_AXONHUB_API_KEY environment variable is required")
	}

	ctx := context.Background()

	// For AxonHub integration, we'll use Gemini API backend
	clientConfig := &genai.ClientConfig{
		APIKey:  c.APIKey,
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			BaseURL: c.BaseURL,
		},
	}

	// If custom base URL is provided, we need to handle it differently
	// For now, we'll use the standard Gemini API endpoint
	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return client, nil
}

// WithHeaders creates a context with the configured headers
func (c *Config) WithHeaders(ctx context.Context) context.Context {
	// Add headers to context for request interception
	ctx = context.WithValue(ctx, "trace_id", c.TraceID)
	ctx = context.WithValue(ctx, "thread_id", c.ThreadID)
	return ctx
}

// generateRandomID generates a random ID string
func generateRandomID(prefix string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("%s-%x", prefix, bytes)
}

// getRandomTraceID returns a random trace ID or from environment variable
func getRandomTraceID() string {
	if traceID := os.Getenv("TEST_TRACE_ID"); traceID != "" {
		return traceID
	}
	return generateRandomID("trace")
}

// getRandomThreadID returns a random thread ID or from environment variable
func getRandomThreadID() string {
	if threadID := os.Getenv("TEST_THREAD_ID"); threadID != "" {
		return threadID
	}
	return generateRandomID("thread")
}

// GetHeaders returns the standard headers used in AxonHub
func (c *Config) GetHeaders() map[string]string {
	return map[string]string{
		"AH-Trace-Id":  c.TraceID,
		"AH-Thread-Id": c.ThreadID,
	}
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ValidateConfig validates the test configuration
func (c *Config) ValidateConfig() error {
	if c.APIKey == "" {
		return fmt.Errorf("API key is required (set TEST_AXONHUB_API_KEY environment variable)")
	}
	if c.TraceID == "" {
		return fmt.Errorf("trace ID is required")
	}
	if c.ThreadID == "" {
		return fmt.Errorf("thread ID is required")
	}
	if c.Model == "" {
		return fmt.Errorf("model is required (set TEST_MODEL environment variable)")
	}
	return nil
}

// GetModel returns the configured model
func (c *Config) GetModel() string {
	return c.Model
}

// GetModelWithFallback returns the configured model, or fallback if empty
func (c *Config) GetModelWithFallback(fallback string) string {
	if c.Model != "" {
		return c.Model
	}
	return fallback
}

// SetModel sets the model configuration
func (c *Config) SetModel(model string) {
	c.Model = model
}

// IsModelSet returns true if a model is configured
func (c *Config) IsModelSet() bool {
	return c.Model != ""
}
