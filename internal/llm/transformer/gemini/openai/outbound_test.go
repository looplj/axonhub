package geminioai

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestNewOutboundTransformer(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		apiKey    string
		wantErr   bool
		errString string
	}{
		{
			name:    "valid config",
			baseURL: "https://generativelanguage.googleapis.com",
			apiKey:  "test-api-key",
			wantErr: false,
		},
		{
			name:      "empty base URL",
			baseURL:   "",
			apiKey:    "test-api-key",
			wantErr:   true,
			errString: "base URL is required",
		},
		{
			name:      "empty API key",
			baseURL:   "https://generativelanguage.googleapis.com",
			apiKey:    "",
			wantErr:   true,
			errString: "API key is required",
		},
		{
			name:    "base URL with trailing slash",
			baseURL: "https://generativelanguage.googleapis.com/",
			apiKey:  "test-api-key",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOutboundTransformer(tt.baseURL, tt.apiKey)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestNewOutboundTransformerWithConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		errString string
		validate  func(*OutboundTransformer) bool
	}{
		{
			name: "valid config",
			config: &Config{
				BaseURL: "https://generativelanguage.googleapis.com",
				APIKey:  "test-api-key",
			},
			wantErr: false,
			validate: func(t *OutboundTransformer) bool {
				return t.BaseURL == "https://generativelanguage.googleapis.com" && t.APIKey == "test-api-key"
			},
		},
		{
			name: "valid config with trailing slash",
			config: &Config{
				BaseURL: "https://generativelanguage.googleapis.com/",
				APIKey:  "test-api-key",
			},
			wantErr: false,
			validate: func(t *OutboundTransformer) bool {
				return t.BaseURL == "https://generativelanguage.googleapis.com" && t.APIKey == "test-api-key"
			},
		},
		{
			name: "empty base URL",
			config: &Config{
				BaseURL: "",
				APIKey:  "test-api-key",
			},
			wantErr:   true,
			errString: "base URL is required",
		},
		{
			name: "empty API key",
			config: &Config{
				BaseURL: "https://generativelanguage.googleapis.com",
				APIKey:  "",
			},
			wantErr:   true,
			errString: "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformerWithConfig(tt.config)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, transformer)

			if tt.validate != nil {
				geminioaiTransformer := transformer.(*OutboundTransformer)
				assert.True(t, tt.validate(geminioaiTransformer))
			}
		})
	}
}

func TestThinkingBudget_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		budget   ThinkingBudget
		expected string
	}{
		{
			name:     "int value",
			budget:   ThinkingBudget{IntValue: lo.ToPtr(1024)},
			expected: "1024",
		},
		{
			name:     "string value",
			budget:   ThinkingBudget{StringValue: lo.ToPtr("low")},
			expected: `"low"`,
		},
		{
			name:     "nil values",
			budget:   ThinkingBudget{},
			expected: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.budget)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestThinkingBudget_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedInt *int
		expectedStr *string
		wantErr     bool
	}{
		{
			name:        "int value",
			input:       "1024",
			expectedInt: lo.ToPtr(1024),
		},
		{
			name:        "string value",
			input:       `"low"`,
			expectedStr: lo.ToPtr("low"),
		},
		{
			name:        "string value high",
			input:       `"high"`,
			expectedStr: lo.ToPtr("high"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var budget ThinkingBudget

			err := json.Unmarshal([]byte(tt.input), &budget)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.expectedInt != nil {
				assert.NotNil(t, budget.IntValue)
				assert.Equal(t, *tt.expectedInt, *budget.IntValue)
			}

			if tt.expectedStr != nil {
				assert.NotNil(t, budget.StringValue)
				assert.Equal(t, *tt.expectedStr, *budget.StringValue)
			}
		})
	}
}

func TestThinkingConfigToReasoningEffort(t *testing.T) {
	tests := []struct {
		name           string
		config         *ThinkingConfig
		expectedEffort string
	}{
		{
			name:           "nil config returns empty",
			config:         nil,
			expectedEffort: "",
		},
		{
			name: "thinking_level takes priority",
			config: &ThinkingConfig{
				ThinkingLevel:  "high",
				ThinkingBudget: NewThinkingBudgetInt(1024),
			},
			expectedEffort: "high",
		},
		{
			name: "thinking_budget 1024 converts to low",
			config: &ThinkingConfig{
				ThinkingBudget: NewThinkingBudgetInt(1024),
			},
			expectedEffort: "low",
		},
		{
			name: "thinking_budget 8192 converts to medium",
			config: &ThinkingConfig{
				ThinkingBudget: NewThinkingBudgetInt(8192),
			},
			expectedEffort: "medium",
		},
		{
			name: "thinking_budget 24576 converts to high",
			config: &ThinkingConfig{
				ThinkingBudget: NewThinkingBudgetInt(24576),
			},
			expectedEffort: "high",
		},
		{
			name: "thinking_budget 0 converts to none",
			config: &ThinkingConfig{
				ThinkingBudget: NewThinkingBudgetInt(0),
			},
			expectedEffort: "none",
		},
		{
			name: "thinking_budget string maps directly",
			config: &ThinkingConfig{
				ThinkingBudget: NewThinkingBudgetString("low"),
			},
			expectedEffort: "low",
		},
		{
			name: "thinking_level minimal",
			config: &ThinkingConfig{
				ThinkingLevel: "minimal",
			},
			expectedEffort: "minimal",
		},
		{
			name: "thinking_level low",
			config: &ThinkingConfig{
				ThinkingLevel: "low",
			},
			expectedEffort: "low",
		},
		{
			name: "thinking_level medium",
			config: &ThinkingConfig{
				ThinkingLevel: "medium",
			},
			expectedEffort: "medium",
		},
		{
			name:           "empty config returns empty",
			config:         &ThinkingConfig{},
			expectedEffort: "",
		},
		{
			name: "unknown budget value returns empty",
			config: &ThinkingConfig{
				ThinkingBudget: NewThinkingBudgetInt(9999),
			},
			expectedEffort: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effort := thinkingConfigToReasoningEffort(tt.config)
			require.Equal(t, tt.expectedEffort, effort)
		})
	}
}

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	createTransformer := func(baseURL, apiKey string) *OutboundTransformer {
		transformerInterface, err := NewOutboundTransformer(baseURL, apiKey)
		if err != nil {
			t.Fatalf("Failed to create transformer: %v", err)
		}

		return transformerInterface.(*OutboundTransformer)
	}

	tests := []struct {
		name        string
		transformer *OutboundTransformer
		request     *llm.Request
		wantErr     bool
		errContains string
		validate    func(*httpclient.Request) bool
	}{
		{
			name:        "valid chat completion request",
			transformer: createTransformer("https://generativelanguage.googleapis.com", "test-api-key"),
			request: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				if req.Method != http.MethodPost {
					return false
				}

				if req.URL != "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" {
					return false
				}

				if req.Headers.Get("Content-Type") != "application/json" {
					return false
				}

				if req.Auth == nil || req.Auth.Type != "bearer" || req.Auth.APIKey != "test-api-key" {
					return false
				}

				var geminiReq Request

				err := json.Unmarshal(req.Body, &geminiReq)
				if err != nil {
					return false
				}

				return geminiReq.Model == "gemini-2.5-flash" &&
					len(geminiReq.Messages) == 1 &&
					geminiReq.Messages[0].Role == "user" &&
					geminiReq.Metadata == nil
			},
		},
		{
			name:        "request with reasoning_effort is preserved",
			transformer: createTransformer("https://generativelanguage.googleapis.com", "test-api-key"),
			request: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Explain AI"),
						},
					},
				},
				ReasoningEffort: "medium",
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var geminiReq Request

				err := json.Unmarshal(req.Body, &geminiReq)
				if err != nil {
					return false
				}

				// reasoning_effort should be preserved
				if geminiReq.ReasoningEffort != "medium" {
					return false
				}

				// extra_body should be nil or empty
				if geminiReq.ExtraBody != nil && geminiReq.ExtraBody.Google != nil && geminiReq.ExtraBody.Google.ThinkingConfig != nil {
					return false
				}

				return true
			},
		},
		{
			name:        "extra_body thinking_config converts to reasoning_effort",
			transformer: createTransformer("https://generativelanguage.googleapis.com", "test-api-key"),
			request: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Explain AI"),
						},
					},
				},
				ReasoningEffort: "high", // This should be overridden
				ExtraBody: json.RawMessage(`{
					"google": {
						"thinking_config": {
							"thinking_budget": 1024,
							"include_thoughts": true
						}
					}
				}`),
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var geminiReq Request

				err := json.Unmarshal(req.Body, &geminiReq)
				if err != nil {
					return false
				}

				// reasoning_effort should be omitted when custom thinking_config is present
				if geminiReq.ReasoningEffort != "" {
					return false
				}

				// extra_body thinking_config should exist and preserve the original thinking_budget/include_thoughts
				if geminiReq.ExtraBody == nil || geminiReq.ExtraBody.Google == nil || geminiReq.ExtraBody.Google.ThinkingConfig == nil {
					return false
				}

				tc := geminiReq.ExtraBody.Google.ThinkingConfig
				if tc.ThinkingBudget == nil || tc.ThinkingBudget.IntValue == nil || *tc.ThinkingBudget.IntValue != 1024 {
					return false
				}

				if tc.ThinkingLevel != "" || !tc.IncludeThoughts {
					return false
				}

				return true
			},
		},
		{
			name:        "when both reasoning_effort and thinking_config are provided, prefer thinking_config and fill missing budget",
			transformer: createTransformer("https://generativelanguage.googleapis.com", "test-api-key"),
			request: &llm.Request{
				Model: "models/gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Explain AI"),
						},
					},
				},
				ReasoningEffort: "low",
				ExtraBody: json.RawMessage(`{
					"google": {
						"thinking_config": {
							"include_thoughts": true
						}
					}
				}`),
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var geminiReq Request

				err := json.Unmarshal(req.Body, &geminiReq)
				if err != nil {
					return false
				}

				if geminiReq.ReasoningEffort != "" {
					return false
				}

				if geminiReq.ExtraBody == nil || geminiReq.ExtraBody.Google == nil || geminiReq.ExtraBody.Google.ThinkingConfig == nil {
					return false
				}

				tc := geminiReq.ExtraBody.Google.ThinkingConfig
				if tc.ThinkingBudget == nil || tc.ThinkingBudget.IntValue == nil || *tc.ThinkingBudget.IntValue != 1024 {
					return false
				}

				if tc.ThinkingLevel != "" {
					return false
				}

				return tc.IncludeThoughts
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq, err := tt.transformer.TransformRequest(nil, tt.request)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, httpReq)

				if tt.validate != nil {
					require.True(t, tt.validate(httpReq), "validation failed")
				}
			}
		})
	}
}

func TestTransformRequestWithExtraBody(t *testing.T) {
	tests := []struct {
		name              string
		request           *llm.Request
		expectError       bool
		expectedReasoning string
		expectedExtraBody bool
		description       string
	}{
		{
			name: "request with extra body thinking_budget integer 8192 converts to medium",
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
			expectError:       false,
			expectedReasoning: "",
			expectedExtraBody: true,
			description:       "Should convert thinking_budget 8192 to reasoning_effort medium",
		},
		{
			name: "request with extra body thinking_budget integer 1024 converts to low",
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
							"thinking_budget": 1024,
							"include_thoughts": true
						}
					}
				}`),
			},
			expectError:       false,
			expectedReasoning: "",
			expectedExtraBody: true,
			description:       "Should convert thinking_budget 1024 to reasoning_effort low",
		},
		{
			name: "request with extra body thinking_budget integer 24576 converts to high",
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
							"thinking_budget": 24576,
							"include_thoughts": true
						}
					}
				}`),
			},
			expectError:       false,
			expectedReasoning: "",
			expectedExtraBody: true,
			description:       "Should convert thinking_budget 24576 to reasoning_effort high",
		},
		{
			name: "request with extra body thinking_budget string converts directly",
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
			expectError:       false,
			expectedReasoning: "",
			expectedExtraBody: true,
			description:       "Should convert string thinking_budget to reasoning_effort",
		},
		{
			name: "request with extra body thinking_level takes priority",
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
							"thinking_level": "high",
							"thinking_budget": 1024,
							"include_thoughts": true
						}
					}
				}`),
			},
			expectError:       false,
			expectedReasoning: "",
			expectedExtraBody: true,
			description:       "ThinkingLevel should take priority over ThinkingBudget",
		},
		{
			name: "request with extra body and reasoning effort (extra body overrides)",
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
			expectError:       false,
			expectedReasoning: "",
			expectedExtraBody: true,
			description:       "Extra body thinking_config should override reasoning_effort",
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
			expectError:       false,
			expectedReasoning: "",
			expectedExtraBody: false,
			description:       "Should handle invalid extra body gracefully",
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
			expectError:       false,
			expectedReasoning: "",
			expectedExtraBody: false,
			description:       "Should handle empty extra body gracefully",
		},
		{
			name: "request with only reasoning_effort (no extra body)",
			request: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
				ReasoningEffort: "medium",
			},
			expectError:       false,
			expectedReasoning: "medium",
			expectedExtraBody: false,
			description:       "Should preserve reasoning_effort when no extra body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer("https://ai.google.dev/v1beta/openai", "test-key")
			require.NoError(t, err)
			require.NotNil(t, transformer)

			httpReq, err := transformer.TransformRequest(nil, tt.request)
			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, httpReq)
			} else {
				require.NoError(t, err)
				require.NotNil(t, httpReq)

				// Verify the request body
				var geminiReq Request

				err = json.Unmarshal(httpReq.Body, &geminiReq)
				require.NoError(t, err)

				// Verify reasoning_effort is set correctly
				require.Equal(t, tt.expectedReasoning, geminiReq.ReasoningEffort)

				// Verify ExtraBody behavior
				if tt.expectedExtraBody {
					require.NotNil(t, geminiReq.ExtraBody)
					require.NotNil(t, geminiReq.ExtraBody.Google)
					require.NotNil(t, geminiReq.ExtraBody.Google.ThinkingConfig)

					tc := geminiReq.ExtraBody.Google.ThinkingConfig
					require.True(t, tc.IncludeThoughts)
				} else {
					require.Nil(t, geminiReq.ExtraBody)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformError_ParsesGeminiOpenAIArray(t *testing.T) {
	transformerInterface, err := NewOutboundTransformer("https://ai.google.dev/v1beta/openai", "test-key")
	require.NoError(t, err)

	tr := transformerInterface.(*OutboundTransformer)

	respErr := tr.TransformError(nil, &httpclient.Error{
		StatusCode: 400,
		Body: []byte("[" +
			"{\"error\":{\"code\":400,\"message\":\"Expected one of either `reasoning_effort` or custom `thinking_config`; found both.\",\"status\":\"INVALID_ARGUMENT\"}}" +
			"]"),
	})

	require.NotNil(t, respErr)
	require.Equal(t, 400, respErr.StatusCode)
	require.Equal(t, "INVALID_ARGUMENT", respErr.Detail.Type)
	require.Equal(t, "400", respErr.Detail.Code)
	require.Contains(t, respErr.Detail.Message, "found both")
}

func TestParseExtraBody(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected *ExtraBody
	}{
		{
			name:     "empty input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty json",
			input:    json.RawMessage(`{}`),
			expected: &ExtraBody{},
		},
		{
			name: "valid thinking_config with int budget",
			input: json.RawMessage(`{
				"google": {
					"thinking_config": {
						"thinking_budget": 1024,
						"thinking_level": "low",
						"include_thoughts": true
					}
				}
			}`),
			expected: &ExtraBody{
				Google: &GoogleExtraBody{
					ThinkingConfig: &ThinkingConfig{
						ThinkingBudget:  NewThinkingBudgetInt(1024),
						ThinkingLevel:   "low",
						IncludeThoughts: true,
					},
				},
			},
		},
		{
			name: "valid thinking_config with string budget",
			input: json.RawMessage(`{
				"google": {
					"thinking_config": {
						"thinking_budget": "high",
						"include_thoughts": true
					}
				}
			}`),
			expected: &ExtraBody{
				Google: &GoogleExtraBody{
					ThinkingConfig: &ThinkingConfig{
						ThinkingBudget:  NewThinkingBudgetString("high"),
						IncludeThoughts: true,
					},
				},
			},
		},
		{
			name:     "invalid json",
			input:    json.RawMessage(`{invalid`),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseExtraBody(tt.input)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)

			if tt.expected.Google == nil {
				assert.Nil(t, result.Google)
				return
			}

			require.NotNil(t, result.Google)

			if tt.expected.Google.ThinkingConfig == nil {
				assert.Nil(t, result.Google.ThinkingConfig)
				return
			}

			require.NotNil(t, result.Google.ThinkingConfig)

			tc := result.Google.ThinkingConfig
			expectedTC := tt.expected.Google.ThinkingConfig

			assert.Equal(t, expectedTC.ThinkingLevel, tc.ThinkingLevel)
			assert.Equal(t, expectedTC.IncludeThoughts, tc.IncludeThoughts)

			if expectedTC.ThinkingBudget != nil {
				require.NotNil(t, tc.ThinkingBudget)

				if expectedTC.ThinkingBudget.IntValue != nil {
					require.NotNil(t, tc.ThinkingBudget.IntValue)
					assert.Equal(t, *expectedTC.ThinkingBudget.IntValue, *tc.ThinkingBudget.IntValue)
				}

				if expectedTC.ThinkingBudget.StringValue != nil {
					require.NotNil(t, tc.ThinkingBudget.StringValue)
					assert.Equal(t, *expectedTC.ThinkingBudget.StringValue, *tc.ThinkingBudget.StringValue)
				}
			}
		})
	}
}
