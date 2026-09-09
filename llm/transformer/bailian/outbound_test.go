package bailian

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestBailianTransformRequest_PreservesOpenAIEnableThinking(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://example.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		apiFormat llm.APIFormat
		rawBody   string
		expected  string
	}{
		{
			name:      "false",
			apiFormat: llm.APIFormatOpenAIChatCompletion,
			rawBody:   `{"model":"qwen-max","messages":[{"role":"user","content":"hi"}],"enable_thinking":false}`,
			expected:  `false`,
		},
		{
			name:      "true",
			apiFormat: llm.APIFormatOpenAIChatCompletion,
			rawBody:   `{"model":"qwen-max","messages":[{"role":"user","content":"hi"}],"enable_thinking":true}`,
			expected:  `true`,
		},
		{
			name:      "omitted",
			apiFormat: llm.APIFormatOpenAIChatCompletion,
			rawBody:   `{"model":"qwen-max","messages":[{"role":"user","content":"hi"}]}`,
		},
		{
			name:      "null",
			apiFormat: llm.APIFormatOpenAIChatCompletion,
			rawBody:   `{"model":"qwen-max","messages":[{"role":"user","content":"hi"}],"enable_thinking":null}`,
		},
		{
			name:      "string",
			apiFormat: llm.APIFormatOpenAIChatCompletion,
			rawBody:   `{"model":"qwen-max","messages":[{"role":"user","content":"hi"}],"enable_thinking":"false"}`,
		},
		{
			name:      "non OpenAI inbound",
			apiFormat: llm.APIFormatAnthropicMessage,
			rawBody:   `{"model":"qwen-max","messages":[{"role":"user","content":"hi"}],"enable_thinking":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userContent := "hi"
			req := &llm.Request{
				Model:     "qwen-max",
				APIFormat: tt.apiFormat,
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: &userContent}},
				},
				RawRequest: &httpclient.Request{Body: []byte(tt.rawBody)},
			}

			httpReq, err := transformer.TransformRequest(context.Background(), req)
			require.NoError(t, err)

			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(httpReq.Body, &body))

			enableThinking, ok := body["enable_thinking"]
			if tt.expected == "" {
				require.False(t, ok)
				return
			}

			require.True(t, ok)
			require.JSONEq(t, tt.expected, string(enableThinking))
		})
	}
}

func TestBailianTransformRequest_MergeConsecutiveToolCalls(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://example.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	userContent := "hi"
	toolOneArgs := "{}"
	toolTwoArgs := "{}"
	out1 := "out1"
	out2 := "out2"
	callOne := "call_1"
	callTwo := "call_2"

	req := &llm.Request{
		Model: "qwen-max",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &userContent}},
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   callOne,
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "tool_one",
							Arguments: toolOneArgs,
						},
					},
				},
			},
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   callTwo,
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "tool_two",
							Arguments: toolTwoArgs,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: &callOne,
				Content:    llm.MessageContent{Content: &out1},
			},
			{
				Role:       "tool",
				ToolCallID: &callTwo,
				Content:    llm.MessageContent{Content: &out2},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	var oaiReq openai.Request
	require.NoError(t, json.Unmarshal(httpReq.Body, &oaiReq))
	require.Len(t, oaiReq.Messages, 4)
	require.Equal(t, "user", oaiReq.Messages[0].Role)
	require.Equal(t, "assistant", oaiReq.Messages[1].Role)
	require.Len(t, oaiReq.Messages[1].ToolCalls, 2)
	require.Equal(t, callOne, oaiReq.Messages[1].ToolCalls[0].ID)
	require.Equal(t, callTwo, oaiReq.Messages[1].ToolCalls[1].ID)
	require.Equal(t, "tool", oaiReq.Messages[2].Role)
	require.NotNil(t, oaiReq.Messages[2].ToolCallID)
	require.Equal(t, callOne, *oaiReq.Messages[2].ToolCallID)
	require.Equal(t, "tool", oaiReq.Messages[3].Role)
	require.NotNil(t, oaiReq.Messages[3].ToolCallID)
	require.Equal(t, callTwo, *oaiReq.Messages[3].ToolCallID)
}
