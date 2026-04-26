package deepseek

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestOutboundTransformer_TransformRequest_ResponseFormat(t *testing.T) {
	config := &Config{
		BaseURL:        "https://api.deepseek.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	tests := []struct {
		name                  string
		request               *llm.Request
		expectedType          string
		expectedJSONSchemaNil bool
	}{
		{
			name: "json_schema converted to json_object",
			request: &llm.Request{
				Model: "deepseek-chat",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type:       "json_schema",
					JSONSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
				},
			},
			expectedType:          "json_object",
			expectedJSONSchemaNil: true,
		},
		{
			name: "json_object remains unchanged",
			request: &llm.Request{
				Model: "deepseek-chat",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type: "json_object",
				},
			},
			expectedType:          "json_object",
			expectedJSONSchemaNil: true,
		},
		{
			name: "text remains unchanged",
			request: &llm.Request{
				Model: "deepseek-chat",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type: "text",
				},
			},
			expectedType:          "text",
			expectedJSONSchemaNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := transformer.TransformRequest(ctx, tt.request)

			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, http.MethodPost, got.Method)

			var dsReq Request

			err = json.Unmarshal(got.Body, &dsReq)
			require.NoError(t, err)

			assert.NotNil(t, dsReq.ResponseFormat)
			assert.Equal(t, tt.expectedType, dsReq.ResponseFormat.Type)

			if tt.expectedJSONSchemaNil {
				assert.Nil(t, dsReq.ResponseFormat.JSONSchema)
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest_Thinking(t *testing.T) {
	config := &Config{
		BaseURL:        "https://api.deepseek.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	tests := []struct {
		name            string
		reasoningEffort string
		expectedThinking string
	}{
		{
			name:            "reasoning effort high enables thinking",
			reasoningEffort: "high",
			expectedThinking: "enabled",
		},
		{
			name:            "reasoning effort medium enables thinking",
			reasoningEffort: "medium",
			expectedThinking: "enabled",
		},
		{
			name:            "reasoning effort none disables thinking",
			reasoningEffort: "none",
			expectedThinking: "disabled",
		},
		{
			name:            "empty reasoning effort enables thinking by default",
			reasoningEffort: "",
			expectedThinking: "enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &llm.Request{
				Model:           "deepseek-reasoner",
				ReasoningEffort: tt.reasoningEffort,
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			}

			ctx := context.Background()
			got, err := transformer.TransformRequest(ctx, request)

			require.NoError(t, err)
			assert.NotNil(t, got)

			var dsReq Request

			err = json.Unmarshal(got.Body, &dsReq)
			require.NoError(t, err)

			require.NotNil(t, dsReq.Thinking)
			assert.Equal(t, tt.expectedThinking, dsReq.Thinking.Type)
		})
	}
}

func TestOutboundTransformer_TransformRequest_URL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		expectedURL string
	}{
		{
			name:        "base URL ending with /v1",
			baseURL:     "https://api.deepseek.com/v1",
			expectedURL: "https://api.deepseek.com/v1/chat/completions",
		},
		{
			name:        "base URL without /v1 suffix",
			baseURL:     "https://api.deepseek.com",
			expectedURL: "https://api.deepseek.com/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				BaseURL:        tt.baseURL,
				APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
			}

			transformer, err := NewOutboundTransformerWithConfig(config)
			require.NoError(t, err)

			request := &llm.Request{
				Model: "deepseek-chat",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			}

			ctx := context.Background()
			got, err := transformer.TransformRequest(ctx, request)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedURL, got.URL)
		})
	}
}

func TestOutboundTransformer_TransformRequest_ReasoningContentFill(t *testing.T) {
	config := &Config{
		BaseURL:        "https://api.deepseek.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	}

	tr, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	tests := []struct {
		name               string
		reasoningEffort    string
		messages           []llm.Message
		expectedReasoning  []map[string]any // per assistant message: {"reasoning_content": "<value>"} or nil
		expectThinking     bool
	}{
		{
			name:            "thinking enabled fills empty reasoning_content for assistant messages",
			reasoningEffort: "high",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hi")}},
			},
			expectThinking: true,
			expectedReasoning: []map[string]any{
				{"reasoning_content": ""},
			},
		},
		{
			name:            "thinking enabled preserves existing reasoning_content",
			reasoningEffort: "high",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hi")}, ReasoningContent: lo.ToPtr("Let me think...")},
			},
			expectThinking: true,
			expectedReasoning: []map[string]any{
				{"reasoning_content": "Let me think..."},
			},
		},
		{
			name:            "default thinking fills reasoning_content when effort is empty",
			reasoningEffort: "",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hi")}},
			},
			expectThinking: true,
			expectedReasoning: []map[string]any{
				{"reasoning_content": ""},
			},
		},
		{
			name:            "thinking disabled does not fill reasoning_content",
			reasoningEffort: "none",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hi")}},
			},
			expectThinking: false,
			expectedReasoning: []map[string]any{
				nil,
			},
		},
		{
			name:            "multiple assistant messages all get filled",
			reasoningEffort: "medium",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hi")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("How are you?")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("I'm fine")}, ReasoningContent: lo.ToPtr("thinking")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Great")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Thanks")}},
			},
			expectThinking: true,
			expectedReasoning: []map[string]any{
				{"reasoning_content": ""},
				{"reasoning_content": "thinking"},
				{"reasoning_content": ""},
			},
		},
		{
			name:            "non-assistant messages are not affected",
			reasoningEffort: "high",
			messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("You are helpful")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hi")}},
			},
			expectThinking: true,
			expectedReasoning: []map[string]any{
				{"reasoning_content": ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &llm.Request{
				Model:           "deepseek-reasoner",
				ReasoningEffort: tt.reasoningEffort,
				Messages:        tt.messages,
			}

			ctx := context.Background()
			got, err := tr.TransformRequest(ctx, request)

			require.NoError(t, err)
			assert.NotNil(t, got)

			var dsReq Request

			err = json.Unmarshal(got.Body, &dsReq)
			require.NoError(t, err)

			require.NotNil(t, dsReq.Thinking)
			if tt.expectThinking {
				assert.Equal(t, "enabled", dsReq.Thinking.Type)
			} else {
				assert.Equal(t, "disabled", dsReq.Thinking.Type)
			}

			// Collect assistant messages in order
			var assistantMsgs []openai.Message
			for _, msg := range dsReq.Messages {
				if msg.Role == "assistant" {
					assistantMsgs = append(assistantMsgs, msg)
				}
			}

			require.Equal(t, len(tt.expectedReasoning), len(assistantMsgs), "number of assistant messages")

			for i, expected := range tt.expectedReasoning {
				if expected == nil {
					assert.Nil(t, assistantMsgs[i].ReasoningContent, "assistant msg %d should have nil ReasoningContent", i)
				} else {
					require.NotNil(t, assistantMsgs[i].ReasoningContent, "assistant msg %d should have ReasoningContent", i)
					assert.Equal(t, expected["reasoning_content"], *assistantMsgs[i].ReasoningContent, "assistant msg %d ReasoningContent", i)
				}
			}
		})
	}
}

// Verify Request struct embeds openai.Request correctly.
func TestRequest_EmbeddedOpenAIRequest(t *testing.T) {
	dsReq := Request{
		Request: openai.Request{
			Model: "deepseek-chat",
		},
		Thinking: &Thinking{
			Type: "enabled",
		},
	}

	data, err := json.Marshal(dsReq)
	require.NoError(t, err)

	var parsed map[string]any

	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "deepseek-chat", parsed["model"])
	thinking, ok := parsed["thinking"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "enabled", thinking["type"])
}
