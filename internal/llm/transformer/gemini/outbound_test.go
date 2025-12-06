package gemini

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestClenupConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name:  "empty config uses defaults",
			input: Config{},
			expected: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "v1beta",
			},
		},
		{
			name: "config with base URL only",
			input: Config{
				BaseURL: "https://custom.example.com",
			},
			expected: Config{
				BaseURL:    "https://custom.example.com",
				APIVersion: "v1beta",
			},
		},
		{
			name: "config with API version only",
			input: Config{
				APIVersion: "v1",
			},
			expected: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "v1",
			},
		},
		{
			name: "config with base URL containing v1beta suffix",
			input: Config{
				BaseURL: "https://generativelanguage.googleapis.com/v1beta",
			},
			expected: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "v1beta",
			},
		},
		{
			name: "config with base URL containing v1 suffix",
			input: Config{
				BaseURL: "https://generativelanguage.googleapis.com/v1",
			},
			expected: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "v1beta",
			},
		},
		{
			name: "config with API version and base URL with version suffix",
			input: Config{
				BaseURL:    "https://example.com/v1beta",
				APIVersion: "v1",
			},
			expected: Config{
				BaseURL:    "https://example.com",
				APIVersion: "v1beta",
			},
		},
		{
			name: "config with trailing slash in base URL",
			input: Config{
				BaseURL: "https://generativelanguage.googleapis.com/",
			},
			expected: Config{
				BaseURL:    "https://generativelanguage.googleapis.com/",
				APIVersion: "v1beta",
			},
		},
		{
			name: "complete config",
			input: Config{
				BaseURL:    "https://custom.api.com",
				APIKey:     "test-key",
				APIVersion: "v1",
			},
			expected: Config{
				BaseURL:    "https://custom.api.com",
				APIKey:     "test-key",
				APIVersion: "v1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clenupConfig(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestOutboundTransformer_buildFullRequestURL(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		request  *llm.Request
		expected string
	}{
		{
			name: "non-streaming request with default config",
			config: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "v1beta",
			},
			request: &llm.Request{
				Model:  "gemini-2.5-flash",
				Stream: lo.ToPtr(false),
			},
			expected: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
		},
		{
			name: "streaming request with default config",
			config: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "v1beta",
			},
			request: &llm.Request{
				Model:  "gemini-2.5-flash",
				Stream: lo.ToPtr(true),
			},
			expected: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse",
		},
		{
			name: "non-streaming request with v1",
			config: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "v1",
			},
			request: &llm.Request{
				Model:  "gemini-2.5-flash",
				Stream: lo.ToPtr(false),
			},
			expected: "https://generativelanguage.googleapis.com/v1/models/gemini-2.5-flash:generateContent",
		},
		{
			name: "streaming request with v1",
			config: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "v1",
			},
			request: &llm.Request{
				Model:  "gemini-2.5-flash",
				Stream: lo.ToPtr(true),
			},
			expected: "https://generativelanguage.googleapis.com/v1/models/gemini-2.5-flash:streamGenerateContent?alt=sse",
		},
		{
			name: "request with custom base URL",
			config: Config{
				BaseURL:    "https://custom.api.com",
				APIVersion: "v1beta",
			},
			request: &llm.Request{
				Model:  "gemini-pro",
				Stream: lo.ToPtr(false),
			},
			expected: "https://custom.api.com/v1beta/models/gemini-pro:generateContent",
		},
		{
			name: "request with nil stream (should default to non-streaming)",
			config: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "v1beta",
			},
			request: &llm.Request{
				Model:  "gemini-2.5-flash",
				Stream: nil,
			},
			expected: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
		},
		{
			name: "request with raw request containing version",
			config: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIVersion: "", // Empty to trigger raw request lookup
			},
			request: &llm.Request{
				Model:      "gemini-2.5-flash",
				Stream:     lo.ToPtr(false),
				RawRequest: &httpclient.Request{
					// Mock PathValue method through a simple implementation
				},
			},
			expected: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent", // Falls back to default since PathValue isn't easily testable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := &OutboundTransformer{config: tt.config}
			result := transformer.buildFullRequestURL(tt.request)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNewOutboundTransformer(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid parameters",
			baseURL: "https://generativelanguage.googleapis.com",
			apiKey:  "test-key",
			wantErr: false,
		},
		{
			name:    "empty base URL",
			baseURL: "",
			apiKey:  "test-key",
			wantErr: false, // Should use default
		},
		{
			name:    "empty API key",
			baseURL: "https://generativelanguage.googleapis.com",
			apiKey:  "",
			wantErr: false, // API key can be empty
		},
		{
			name:    "both empty",
			baseURL: "",
			apiKey:  "",
			wantErr: false, // Should use defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer(tt.baseURL, tt.apiKey)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, transformer)
			} else {
				require.NoError(t, err)
				require.NotNil(t, transformer)

				// Test that the transformer has the expected methods
				require.Equal(t, llm.APIFormatGeminiContents, transformer.APIFormat())
			}
		})
	}
}

func TestNewOutboundTransformerWithConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				BaseURL:    "https://generativelanguage.googleapis.com",
				APIKey:     "test-key",
				APIVersion: "v1beta",
			},
			wantErr: false,
		},
		{
			name:    "empty config",
			config:  Config{},
			wantErr: false,
		},
		{
			name: "config with version suffix in base URL",
			config: Config{
				BaseURL: "https://generativelanguage.googleapis.com/v1beta",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformerWithConfig(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, transformer)
			} else {
				require.NoError(t, err)
				require.NotNil(t, transformer)
				require.Equal(t, llm.APIFormatGeminiContents, transformer.APIFormat())
			}
		})
	}
}

func TestTransformRequestWithExtraBody(t *testing.T) {
	tests := []struct {
		name        string
		request     *llm.Request
		expectError bool
		description string
	}{
		{
			name: "request with extra body thinking config integer budget",
			request: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
				ExtraBody: json.RawMessage(`{
					"google": {
						"thinking_config": {
							"thinking_budget": 8192,
							"thinking_level": "high",
							"include_thoughts": true
						}
					}
				}`),
			},
			expectError: false,
			description: "Should convert extra body thinking config to Gemini format",
		},
		{
			name: "request with extra body thinking config string budget",
			request: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
				ExtraBody: json.RawMessage(`{
					"google": {
						"thinking_config": {
							"thinking_budget": "low",
							"thinking_level": "low",
							"include_thoughts": true
						}
					}
				}`),
			},
			expectError: false,
			description: "Should convert string thinking budget to integer",
		},
		{
			name: "request with extra body and reasoning effort (extra body should take priority)",
			request: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
				ReasoningEffort: "high",
				ExtraBody: json.RawMessage(`{
					"google": {
						"thinking_config": {
							"thinking_budget": 1024,
							"thinking_level": "low",
							"include_thoughts": true
						}
					}
				}`),
			},
			expectError: false,
			description: "Extra body should take priority over reasoning_effort",
		},
		{
			name: "request with invalid extra body",
			request: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
				ExtraBody: json.RawMessage(`{invalid`),
			},
			expectError: false,
			description: "Should handle invalid extra body gracefully",
		},
		{
			name: "request with empty extra body",
			request: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
				ExtraBody: json.RawMessage{},
			},
			expectError: false,
			description: "Should handle empty extra body gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-key")
			require.NoError(t, err)
			require.NotNil(t, transformer)

			httpReq, err := transformer.TransformRequest(nil, tt.request)
			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, httpReq)
			} else {
				require.NoError(t, err)
				require.NotNil(t, httpReq)

				// Verify the request body contains the expected thinking config
				var geminiReq GenerateContentRequest

				err = json.Unmarshal(httpReq.Body, &geminiReq)
				require.NoError(t, err)

				if len(tt.request.ExtraBody) > 0 && string(tt.request.ExtraBody) != "{invalid" {
					// Should have thinking config from extra body
					require.NotNil(t, geminiReq.GenerationConfig)
					require.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig)

					// Verify thinking config values based on the test case
					if strings.Contains(string(tt.request.ExtraBody), "8192") {
						require.Equal(t, int64(8192), *geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
						require.Equal(t, "high", geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel)
						require.True(t, geminiReq.GenerationConfig.ThinkingConfig.IncludeThoughts)
					} else if strings.Contains(string(tt.request.ExtraBody), "1024") {
						require.Equal(t, int64(1024), *geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
						require.Equal(t, "low", geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel)
						require.True(t, geminiReq.GenerationConfig.ThinkingConfig.IncludeThoughts)
					}
				}
			}
		})
	}
}

func TestReasoningEffortFallback(t *testing.T) {
	// Test that reasoning_effort is used when extra body is not present
	request := &llm.Request{
		Model: "gemini-2.5-flash",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
			},
		},
		ReasoningEffort: "medium",
		ExtraBody:       json.RawMessage{}, // Empty extra body
	}

	transformer, err := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-key")
	require.NoError(t, err)
	require.NotNil(t, transformer)

	httpReq, err := transformer.TransformRequest(nil, request)
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	// Verify the request body contains thinking config from reasoning effort
	var geminiReq GenerateContentRequest

	err = json.Unmarshal(httpReq.Body, &geminiReq)
	require.NoError(t, err)

	require.NotNil(t, geminiReq.GenerationConfig)
	require.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig)
	require.True(t, geminiReq.GenerationConfig.ThinkingConfig.IncludeThoughts)
	// Should use default mapping for "medium" reasoning effort
	require.Equal(t, int64(8192), *geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
}
