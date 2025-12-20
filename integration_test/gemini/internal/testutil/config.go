package testutil

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

// Config holds configuration for Gemini tests
type Config struct {
	APIKey        string
	BaseURL       string
	TraceID       string
	ThreadID      string
	Timeout       time.Duration
	MaxRetries    int
	Model         string // Default model for tests
	DisableTrace  bool   // Disable trace ID generation
	DisableThread bool   // Disable thread ID generation
}

// DefaultConfig returns a default configuration for Gemini tests
func DefaultConfig() *Config {
	return DefaultConfigWithPrefix("")
}

// DefaultConfigWithPrefix returns a default configuration for Gemini tests with custom prefix for IDs
func DefaultConfigWithPrefix(prefix string) *Config {
	disableTrace := strings.EqualFold(getEnvOrDefault("TEST_DISABLE_TRACE", "false"), "true")
	disableThread := strings.EqualFold(getEnvOrDefault("TEST_DISABLE_THREAD", "false"), "true")

	config := &Config{
		APIKey:        getEnvOrDefault("TEST_AXONHUB_API_KEY", ""),
		BaseURL:       getEnvOrDefault("TEST_GEMINI_BASE_URL", "http://localhost:8090/gemini"),
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		Model:         getEnvOrDefault("TEST_MODEL", "gemini-2.5-flash"),
		DisableTrace:  disableTrace,
		DisableThread: disableThread,
	}

	// Only generate trace ID if not disabled
	if !disableTrace {
		tracePrefix := "trace"
		if prefix != "" {
			tracePrefix = prefix
		}
		config.TraceID = getRandomTraceIDWithPrefix(tracePrefix)
	}

	// Only generate thread ID if not disabled
	if !disableThread {
		threadPrefix := "thread"
		if prefix != "" {
			threadPrefix = prefix
		}
		config.ThreadID = getRandomThreadIDWithPrefix(threadPrefix)
	}

	return config
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
	// Add headers to context for request interception only if not disabled
	if !c.DisableTrace && c.TraceID != "" {
		ctx = context.WithValue(ctx, "trace_id", c.TraceID)
	}
	if !c.DisableThread && c.ThreadID != "" {
		ctx = context.WithValue(ctx, "thread_id", c.ThreadID)
	}
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

// getRandomTraceIDWithPrefix returns a random trace ID with custom prefix or from environment variable
func getRandomTraceIDWithPrefix(prefix string) string {
	if traceID := os.Getenv("TEST_TRACE_ID"); traceID != "" {
		return traceID
	}
	return generateRandomID(prefix)
}

// getRandomThreadID returns a random thread ID or from environment variable
func getRandomThreadID() string {
	if threadID := os.Getenv("TEST_THREAD_ID"); threadID != "" {
		return threadID
	}
	return generateRandomID("thread")
}

// getRandomThreadIDWithPrefix returns a random thread ID with custom prefix or from environment variable
func getRandomThreadIDWithPrefix(prefix string) string {
	if threadID := os.Getenv("TEST_THREAD_ID"); threadID != "" {
		return threadID
	}
	return generateRandomID(prefix)
}

// GetHeaders returns the standard headers used in AxonHub
func (c *Config) GetHeaders() map[string]string {
	headers := make(map[string]string)

	// Only add trace ID header if not disabled and trace ID exists
	if !c.DisableTrace && c.TraceID != "" {
		headers["AH-Trace-Id"] = c.TraceID
	}

	// Only add thread ID header if not disabled and thread ID exists
	if !c.DisableThread && c.ThreadID != "" {
		headers["AH-Thread-Id"] = c.ThreadID
	}

	return headers
}

// GetHTTPOptions returns HTTPOptions with the configured headers for call-time usage
func (c *Config) GetHTTPOptions() *genai.HTTPOptions {
	headers := c.GetHeaders()
	httpHeaders := make(map[string][]string)
	for k, v := range headers {
		httpHeaders[k] = []string{v}
	}
	return &genai.HTTPOptions{
		Headers: httpHeaders,
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

	// Only validate trace ID if not disabled
	if !c.DisableTrace && c.TraceID == "" {
		return fmt.Errorf("trace ID is required")
	}

	// Only validate thread ID if not disabled
	if !c.DisableThread && c.ThreadID == "" {
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
