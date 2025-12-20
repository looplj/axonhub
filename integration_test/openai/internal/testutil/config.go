package testutil

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Config holds configuration for tests
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

// DefaultConfig returns a default configuration for tests
func DefaultConfig() *Config {
	return DefaultConfigWithPrefix("")
}

// DefaultConfigWithPrefix returns a default configuration for tests with custom prefix for IDs
func DefaultConfigWithPrefix(prefix string) *Config {
	disableTrace := strings.EqualFold(getEnvOrDefault("TEST_DISABLE_TRACE", "false"), "true")
	disableThread := strings.EqualFold(getEnvOrDefault("TEST_DISABLE_THREAD", "false"), "true")

	config := &Config{
		APIKey:        getEnvOrDefault("TEST_AXONHUB_API_KEY", ""),
		BaseURL:       getEnvOrDefault("TEST_OPENAI_BASE_URL", "http://localhost:8090/v1"),
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		Model:         getEnvOrDefault("TEST_MODEL", "deepseek-chat"),
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

// NewClient creates a new OpenAI client with the given configuration
func (c *Config) NewClient() openai.Client {
	if c.APIKey == "" {
		panic("TEST_AXONHUB_API_KEY environment variable is required")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(c.APIKey),
		option.WithBaseURL(c.BaseURL),
	}
	// Remove headers from client initialization - they will be passed at call point
	return openai.NewClient(opts...)
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

// GetHeaderOptions returns request options with the configured headers for call-time usage
func (c *Config) GetHeaderOptions() []option.RequestOption {
	var opts []option.RequestOption
	for k, v := range c.GetHeaders() {
		opts = append(opts, option.WithHeader(k, v))
	}
	return opts
}

// GetHeaders returns the standard headers used in axonhub
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

// GetModel returns the configured model as a ChatModel type
func (c *Config) GetModel() openai.ChatModel {
	return openai.ChatModel(c.Model)
}

// GetModelWithFallback returns the configured model, or fallback to GPT-4o if empty
func (c *Config) GetModelWithFallback(fallback openai.ChatModel) openai.ChatModel {
	if c.Model != "" {
		return openai.ChatModel(c.Model)
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
