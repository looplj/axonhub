package testutil

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// TestHelper provides common testing utilities
type TestHelper struct {
	Config *Config
	Client anthropic.Client
}

// NewTestHelper creates a new test helper with default configuration
func NewTestHelper(t *testing.T, name string) *TestHelper {
	config := DefaultConfigWithPrefix(name)
	if err := config.ValidateConfig(); err != nil {
		t.Skipf("Skipping test due to configuration error: %v", err)
	}

	client := config.NewClient()

	return &TestHelper{
		Config: config,
		Client: client,
	}
}

// AssertNoError fails the test if err is not nil
func (h *TestHelper) AssertNoError(t *testing.T, err error, msg ...interface{}) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v - %v", err, msg)
	}
}

// LogResponse logs the response for debugging
func (h *TestHelper) LogResponse(t *testing.T, response interface{}, description string) {
	t.Helper()
	t.Logf("%s: %+v", description, response)
}

// PrintHeaders prints the standard headers for debugging
func (h *TestHelper) PrintHeaders(t *testing.T) {
	t.Helper()
	t.Logf("Using headers: %+v", h.Config.GetHeaders())
}

// CreateTestContext creates a context with the configured headers
func (h *TestHelper) CreateTestContext() context.Context {
	ctx := context.Background()
	return h.Config.WithHeaders(ctx)
}

// RunWithHeaders executes a test function with the configured headers
func (h *TestHelper) RunWithHeaders(t *testing.T, testFunc func(ctx context.Context) error) {
	t.Helper()
	ctx := h.CreateTestContext()
	if err := testFunc(ctx); err != nil {
		h.AssertNoError(t, err)
	}
}

// CreateMessageWithHeaders creates a message with trace headers passed at call time
func (h *TestHelper) CreateMessageWithHeaders(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	headerOpts := h.Config.GetHeaderOptions()
	return h.Client.Messages.New(ctx, params, headerOpts...)
}

// CreateMessageStreamWithHeaders creates a streaming message with trace headers passed at call time
func (h *TestHelper) CreateMessageStreamWithHeaders(ctx context.Context, params anthropic.MessageNewParams) *ssestream.Stream[anthropic.MessageStreamEventUnion] {
	headerOpts := h.Config.GetHeaderOptions()
	return h.Client.Messages.NewStreaming(ctx, params, headerOpts...)
}

// GetHeaderOptions returns request options with the configured headers for call-time usage
func (h *TestHelper) GetHeaderOptions() []option.RequestOption {
	return h.Config.GetHeaderOptions()
}

// ValidateMessageResponse validates a message response
func (h *TestHelper) ValidateMessageResponse(t *testing.T, response *anthropic.Message, description string) {
	t.Helper()
	if response == nil {
		t.Fatalf("Response is nil for %s", description)
	}
	if len(response.Content) == 0 {
		t.Fatalf("No content in response for %s", description)
	}

	t.Logf("%s - Response validated successfully: %d content blocks", description, len(response.Content))
}

// GetModel returns the configured model for tests
func (h *TestHelper) GetModel() anthropic.Model {
	return h.Config.GetModel()
}

// GetModelWithFallback returns the configured model or fallback if not set
func (h *TestHelper) GetModelWithFallback(fallback string) anthropic.Model {
	return anthropic.Model(h.Config.GetModelWithFallback(fallback))
}

// SetModel sets the model for tests
func (h *TestHelper) SetModel(model anthropic.Model) {
	h.Config.SetModel(string(model))
}

// CreateTestHelperWithNewTrace creates a new test helper with the same thread but new trace ID
func CreateTestHelperWithNewTrace(t *testing.T, existingConfig *Config) *TestHelper {
	t.Helper()

	// Create a new config based on existing one
	newConfig := &Config{
		APIKey:        existingConfig.APIKey,
		BaseURL:       existingConfig.BaseURL,
		Timeout:       existingConfig.Timeout,
		MaxRetries:    existingConfig.MaxRetries,
		Model:         existingConfig.Model,
		DisableTrace:  existingConfig.DisableTrace,
		DisableThread: existingConfig.DisableThread,
		ThreadID:      existingConfig.ThreadID, // Keep same thread ID
	}

	// Only generate new trace ID if not disabled
	if !existingConfig.DisableTrace {
		// Use existing trace ID prefix if available, otherwise default to "trace"
		prefix := "trace"
		if existingConfig.TraceID != "" {
			// Extract prefix from existing trace ID (everything before the first hyphen)
			if idx := strings.Index(existingConfig.TraceID, "-"); idx > 0 {
				prefix = existingConfig.TraceID[:idx]
			}
		}
		newConfig.TraceID = getRandomTraceIDWithPrefix(prefix)
	}

	client := newConfig.NewClient()

	return &TestHelper{
		Config: newConfig,
		Client: client,
	}
}

func ContainsCaseInsensitive(text, substring string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(substring))
}

func ContainsAnyCaseInsensitive(text string, substrings ...string) bool {
	for _, substring := range substrings {
		if ContainsCaseInsensitive(text, substring) {
			return true
		}
	}
	return false
}
