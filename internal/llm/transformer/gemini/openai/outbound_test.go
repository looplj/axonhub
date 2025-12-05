package geminioai

import (
	"context"
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

func TestReasoningEffortToThinkingConfig(t *testing.T) {
	tests := []struct {
		name           string
		effort         string
		expectedLevel  string
		expectedBudget int
		expectedNil    bool
	}{
		{
			name:           "none",
			effort:         "none",
			expectedBudget: 0,
		},
		{
			name:           "minimal",
			effort:         "minimal",
			expectedLevel:  "low",
			expectedBudget: 1024,
		},
		{
			name:           "low",
			effort:         "low",
			expectedLevel:  "low",
			expectedBudget: 1024,
		},
		{
			name:           "medium",
			effort:         "medium",
			expectedLevel:  "high",
			expectedBudget: 8192,
		},
		{
			name:           "high",
			effort:         "high",
			expectedLevel:  "high",
			expectedBudget: 24576,
		},
		{
			name:        "unknown",
			effort:      "unknown",
			expectedNil: true,
		},
		{
			name:        "empty",
			effort:      "",
			expectedNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := reasoningEffortToThinkingConfig(tt.effort)

			if tt.expectedNil {
				assert.Nil(t, config)
				return
			}

			require.NotNil(t, config)
			assert.Equal(t, tt.expectedLevel, config.ThinkingLevel)
			require.NotNil(t, config.ThinkingBudget)
			require.NotNil(t, config.ThinkingBudget.IntValue)
			assert.Equal(t, tt.expectedBudget, *config.ThinkingBudget.IntValue)
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
			name:        "request with reasoning_effort converts to thinking_config",
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

				// reasoning_effort should be cleared
				if geminiReq.ReasoningEffort != "" {
					return false
				}

				// extra_body should have thinking_config
				if geminiReq.ExtraBody == nil || geminiReq.ExtraBody.Google == nil || geminiReq.ExtraBody.Google.ThinkingConfig == nil {
					return false
				}

				tc := geminiReq.ExtraBody.Google.ThinkingConfig

				return tc.ThinkingLevel == "high" &&
					tc.ThinkingBudget != nil &&
					tc.ThinkingBudget.IntValue != nil &&
					*tc.ThinkingBudget.IntValue == 8192 &&
					tc.IncludeThoughts
			},
		},
		{
			name:        "extra_body has higher priority than reasoning_effort",
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
				ReasoningEffort: "high", // This should be ignored
				ExtraBody: json.RawMessage(`{
					"google": {
						"thinking_config": {
							"thinking_budget": 2048,
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

				// reasoning_effort should be cleared
				if geminiReq.ReasoningEffort != "" {
					return false
				}

				// extra_body should use the provided thinking_config, not the one from reasoning_effort
				if geminiReq.ExtraBody == nil || geminiReq.ExtraBody.Google == nil || geminiReq.ExtraBody.Google.ThinkingConfig == nil {
					return false
				}

				tc := geminiReq.ExtraBody.Google.ThinkingConfig

				// Should be 2048 from extra_body, not 24576 from reasoning_effort="high"
				return tc.ThinkingBudget != nil &&
					tc.ThinkingBudget.IntValue != nil &&
					*tc.ThinkingBudget.IntValue == 2048 &&
					tc.IncludeThoughts
			},
		},
		{
			name:        "extra_body with string thinking_budget",
			transformer: createTransformer("https://generativelanguage.googleapis.com", "test-api-key"),
			request: &llm.Request{
				Model: "gemini-3.0-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Explain AI"),
						},
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
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var geminiReq Request

				err := json.Unmarshal(req.Body, &geminiReq)
				if err != nil {
					return false
				}

				if geminiReq.ExtraBody == nil || geminiReq.ExtraBody.Google == nil || geminiReq.ExtraBody.Google.ThinkingConfig == nil {
					return false
				}

				tc := geminiReq.ExtraBody.Google.ThinkingConfig

				// Should be "low" string value
				return tc.ThinkingBudget != nil &&
					tc.ThinkingBudget.StringValue != nil &&
					*tc.ThinkingBudget.StringValue == "low" &&
					tc.IncludeThoughts
			},
		},
		{
			name:        "nil request",
			transformer: createTransformer("https://generativelanguage.googleapis.com", "test-api-key"),
			request:     nil,
			wantErr:     true,
			errContains: "chat completion request is nil",
		},
		{
			name:        "missing model",
			transformer: createTransformer("https://generativelanguage.googleapis.com", "test-api-key"),
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr:     true,
			errContains: "model is required",
		},
		{
			name:        "empty messages",
			transformer: createTransformer("https://generativelanguage.googleapis.com", "test-api-key"),
			request: &llm.Request{
				Model:    "gemini-2.5-flash",
				Messages: []llm.Message{},
			},
			wantErr:     true,
			errContains: "messages are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := tt.transformer.TransformRequest(context.Background(), tt.request)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, req)

			if tt.validate != nil {
				assert.True(t, tt.validate(req), "validation failed")
			}
		})
	}
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
