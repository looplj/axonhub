package testutil

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

// TestHelper provides common testing utilities
type TestHelper struct {
	Config *Config
	Client anthropic.Client
}

// NewTestHelper creates a new test helper with default configuration
func NewTestHelper(t *testing.T) *TestHelper {
	config := DefaultConfig()
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

	// Create a new config based on existing one but with new trace ID
	newConfig := &Config{
		APIKey:     existingConfig.APIKey,
		BaseURL:    existingConfig.BaseURL,
		TraceID:    getRandomTraceID(),      // Generate new trace ID
		ThreadID:   existingConfig.ThreadID, // Keep same thread ID
		Timeout:    existingConfig.Timeout,
		MaxRetries: existingConfig.MaxRetries,
		Model:      existingConfig.Model,
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
