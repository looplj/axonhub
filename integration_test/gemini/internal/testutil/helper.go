package testutil

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/genai"
)

// TestHelper provides common testing utilities for Gemini
type TestHelper struct {
	Config *Config
	Client *genai.Client
}

// NewTestHelper creates a new test helper with default configuration
func NewTestHelper(t *testing.T, name string) *TestHelper {
	config := DefaultConfigWithPrefix(name)
	if err := config.ValidateConfig(); err != nil {
		t.Skipf("Skipping test due to configuration error: %v", err)
	}

	client, err := config.NewClient()
	if err != nil {
		t.Skipf("Skipping test due to client creation error: %v", err)
	}

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

// ValidateChatResponse validates a Gemini generate content response
func (h *TestHelper) ValidateChatResponse(t *testing.T, response *genai.GenerateContentResponse, description string) {
	t.Helper()
	if response == nil {
		t.Fatalf("Response is nil for %s", description)
	}
	if len(response.Candidates) == 0 {
		t.Fatalf("No candidates in response for %s", description)
	}

	// Check if the candidate has content
	candidate := response.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		t.Fatalf("Empty content in response for %s", description)
	}

	t.Logf("%s - Response validated successfully: %d candidates", description, len(response.Candidates))
}

// GetModel returns the configured model for tests
func (h *TestHelper) GetModel() string {
	return h.Config.GetModel()
}

// GetModelWithFallback returns the configured model or fallback if not set
func (h *TestHelper) GetModelWithFallback(fallback string) string {
	return h.Config.GetModelWithFallback(fallback)
}

// SetModel sets the model for tests
func (h *TestHelper) SetModel(model string) {
	h.Config.SetModel(model)
}

// GetHTTPOptions returns HTTPOptions with the configured headers for call-time usage
func (h *TestHelper) GetHTTPOptions() *genai.HTTPOptions {
	return h.Config.GetHTTPOptions()
}

// MergeHTTPOptions merges the helper's HTTPOptions into the provided config
func (h *TestHelper) MergeHTTPOptions(config *genai.GenerateContentConfig) *genai.GenerateContentConfig {
	if config == nil {
		config = &genai.GenerateContentConfig{}
	}
	if config.HTTPOptions == nil {
		config.HTTPOptions = h.GetHTTPOptions()
	} else {
		// Merge headers
		helperHeaders := h.GetHTTPOptions().Headers
		if config.HTTPOptions.Headers == nil {
			config.HTTPOptions.Headers = helperHeaders
		} else {
			for k, v := range helperHeaders {
				config.HTTPOptions.Headers[k] = v
			}
		}
	}
	return config
}

// GenerateContentWithHeaders generates content with trace headers passed at call time
func (h *TestHelper) GenerateContentWithHeaders(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	config = h.MergeHTTPOptions(config)
	return h.Client.Models.GenerateContent(ctx, model, contents, config)
}

// CreateChatWithHeaders creates a chat session with trace headers passed at call time
func (h *TestHelper) CreateChatWithHeaders(ctx context.Context, model string, config *genai.GenerateContentConfig, history []*genai.Content) (*genai.Chat, error) {
	config = h.MergeHTTPOptions(config)
	return h.Client.Chats.Create(ctx, model, config, history)
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

	client, err := newConfig.NewClient()
	if err != nil {
		t.Skipf("Skipping test due to client creation error: %v", err)
	}

	return &TestHelper{
		Config: newConfig,
		Client: client,
	}
}

// ContainsCaseInsensitive checks if text contains substring (case insensitive)
func ContainsCaseInsensitive(text, substring string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(substring))
}

// ContainsAnyCaseInsensitive checks if text contains any of the substrings (case insensitive)
func ContainsAnyCaseInsensitive(text string, substrings ...string) bool {
	for _, substring := range substrings {
		if ContainsCaseInsensitive(text, substring) {
			return true
		}
	}
	return false
}

// ExtractTextFromResponse extracts text content from Gemini response
func ExtractTextFromResponse(response *genai.GenerateContentResponse) string {
	if response == nil || len(response.Candidates) == 0 {
		return ""
	}

	candidate := response.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return ""
	}

	var result strings.Builder
	for _, part := range candidate.Content.Parts {
		if part != nil && part.Text != "" {
			result.WriteString(part.Text)
		}
	}

	return result.String()
}
