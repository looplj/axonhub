package gemini

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
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
				APIVersion: "v1",
			},
		},
		{
			name: "config with API version and base URL with version suffix",
			input: Config{
				BaseURL:    "https://example.com/v1beta",
				APIVersion: "v1",
			},
			expected: Config{
				BaseURL:    "https://example.com/v1beta",
				APIVersion: "v1",
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
							"include_thoughts": true
						}
					}
				}`),
			},
			expectError: false,
			description: "Should convert extra body thinking config with integer budget to Gemini format",
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
							"include_thoughts": true
						}
					}
				}`),
			},
			expectError: false,
			description: "Should convert string thinking budget to thinking level",
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
					// After optimization: only ThinkingBudget OR ThinkingLevel should be set, not both
					// Integer budgets are preserved as ThinkingBudget, string budgets convert to ThinkingLevel
					if strings.Contains(string(tt.request.ExtraBody), "8192") {
						require.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
						require.Equal(t, int64(8192), *geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
						require.Empty(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel)
						require.True(t, geminiReq.GenerationConfig.ThinkingConfig.IncludeThoughts)
					} else if strings.Contains(string(tt.request.ExtraBody), "1024") {
						require.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
						require.Equal(t, int64(1024), *geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
						require.Empty(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel)
						require.True(t, geminiReq.GenerationConfig.ThinkingConfig.IncludeThoughts)
					} else if strings.Contains(string(tt.request.ExtraBody), `"thinking_budget": "low"`) {
						// String budget should convert to ThinkingLevel
						require.Equal(t, "low", geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel)
						require.Nil(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
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
	// Should use ThinkingLevel for standard "medium" reasoning effort
	require.Equal(t, "medium", geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel)
	require.Nil(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
}

func TestOutboundTransformer_TransformResponse_MultipleFunctionCalls(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-key")
	require.NoError(t, err)

	tests := []struct {
		name           string
		geminiResponse *GenerateContentResponse
		validate       func(t *testing.T, resp *llm.Response)
	}{
		{
			name: "single function call has index 0",
			geminiResponse: &GenerateContentResponse{
				ResponseID:   "resp-single-tool",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									FunctionCall: &FunctionCall{
										ID:   "call-1",
										Name: "get_weather",
										Args: map[string]any{"location": "Tokyo"},
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				t.Helper()
				require.Len(t, resp.Choices, 1)
				require.Len(t, resp.Choices[0].Message.ToolCalls, 1)
				require.Equal(t, 0, resp.Choices[0].Message.ToolCalls[0].Index)
				require.Equal(t, "call-1", resp.Choices[0].Message.ToolCalls[0].ID)
				require.Equal(t, "get_weather", resp.Choices[0].Message.ToolCalls[0].Function.Name)
			},
		},
		{
			name: "multiple function calls in single response have sequential indices",
			geminiResponse: &GenerateContentResponse{
				ResponseID:   "resp-multi-tool",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									FunctionCall: &FunctionCall{
										ID:   "call-1",
										Name: "get_weather",
										Args: map[string]any{"location": "Tokyo"},
									},
								},
								{
									FunctionCall: &FunctionCall{
										ID:   "call-2",
										Name: "get_time",
										Args: map[string]any{"timezone": "JST"},
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				t.Helper()
				require.Len(t, resp.Choices, 1)
				require.Len(t, resp.Choices[0].Message.ToolCalls, 2)

				// First tool call should have index 0
				require.Equal(t, 0, resp.Choices[0].Message.ToolCalls[0].Index)
				require.Equal(t, "call-1", resp.Choices[0].Message.ToolCalls[0].ID)
				require.Equal(t, "get_weather", resp.Choices[0].Message.ToolCalls[0].Function.Name)

				// Second tool call should have index 1
				require.Equal(t, 1, resp.Choices[0].Message.ToolCalls[1].Index)
				require.Equal(t, "call-2", resp.Choices[0].Message.ToolCalls[1].ID)
				require.Equal(t, "get_time", resp.Choices[0].Message.ToolCalls[1].Function.Name)
			},
		},
		{
			name: "three function calls have sequential indices 0, 1, 2",
			geminiResponse: &GenerateContentResponse{
				ResponseID:   "resp-three-tools",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									FunctionCall: &FunctionCall{
										ID:   "call-a",
										Name: "func_a",
										Args: map[string]any{"param": "a"},
									},
								},
								{
									FunctionCall: &FunctionCall{
										ID:   "call-b",
										Name: "func_b",
										Args: map[string]any{"param": "b"},
									},
								},
								{
									FunctionCall: &FunctionCall{
										ID:   "call-c",
										Name: "func_c",
										Args: map[string]any{"param": "c"},
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				t.Helper()
				require.Len(t, resp.Choices, 1)
				require.Len(t, resp.Choices[0].Message.ToolCalls, 3)

				require.Equal(t, 0, resp.Choices[0].Message.ToolCalls[0].Index)
				require.Equal(t, "func_a", resp.Choices[0].Message.ToolCalls[0].Function.Name)

				require.Equal(t, 1, resp.Choices[0].Message.ToolCalls[1].Index)
				require.Equal(t, "func_b", resp.Choices[0].Message.ToolCalls[1].Function.Name)

				require.Equal(t, 2, resp.Choices[0].Message.ToolCalls[2].Index)
				require.Equal(t, "func_c", resp.Choices[0].Message.ToolCalls[2].Function.Name)
			},
		},
		{
			name: "function calls with text content mixed",
			geminiResponse: &GenerateContentResponse{
				ResponseID:   "resp-mixed",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "I'll help you with that."},
								{
									FunctionCall: &FunctionCall{
										ID:   "call-1",
										Name: "search",
										Args: map[string]any{"query": "weather"},
									},
								},
								{
									FunctionCall: &FunctionCall{
										ID:   "call-2",
										Name: "calculate",
										Args: map[string]any{"expr": "1+1"},
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				t.Helper()
				require.Len(t, resp.Choices, 1)

				// Text content should be present
				require.NotNil(t, resp.Choices[0].Message.Content.Content)
				require.Equal(t, "I'll help you with that.", *resp.Choices[0].Message.Content.Content)

				// Tool calls should have correct indices
				require.Len(t, resp.Choices[0].Message.ToolCalls, 2)
				require.Equal(t, 0, resp.Choices[0].Message.ToolCalls[0].Index)
				require.Equal(t, "search", resp.Choices[0].Message.ToolCalls[0].Function.Name)
				require.Equal(t, 1, resp.Choices[0].Message.ToolCalls[1].Index)
				require.Equal(t, "calculate", resp.Choices[0].Message.ToolCalls[1].Function.Name)
			},
		},
		{
			name: "function call without ID gets generated UUID",
			geminiResponse: &GenerateContentResponse{
				ResponseID:   "resp-no-id",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									FunctionCall: &FunctionCall{
										Name: "get_data",
										Args: map[string]any{"key": "value"},
									},
								},
								{
									FunctionCall: &FunctionCall{
										Name: "process_data",
										Args: map[string]any{"data": "test"},
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				t.Helper()
				require.Len(t, resp.Choices, 1)
				require.Len(t, resp.Choices[0].Message.ToolCalls, 2)

				// IDs should be generated (non-empty UUIDs)
				require.NotEmpty(t, resp.Choices[0].Message.ToolCalls[0].ID)
				require.NotEmpty(t, resp.Choices[0].Message.ToolCalls[1].ID)

				// Indices should still be correct
				require.Equal(t, 0, resp.Choices[0].Message.ToolCalls[0].Index)
				require.Equal(t, 1, resp.Choices[0].Message.ToolCalls[1].Index)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the Gemini response to JSON
			respBody, err := json.Marshal(tt.geminiResponse)
			require.NoError(t, err)

			// Create HTTP response
			httpResp := &httpclient.Response{
				StatusCode: 200,
				Body:       respBody,
			}

			// Transform response
			resp, err := transformer.TransformResponse(nil, httpResp)
			require.NoError(t, err)
			require.NotNil(t, resp)

			// Run validation
			tt.validate(t, resp)
		})
	}
}
