package bailian

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestBailianTransformRequest_ReasoningEffort(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://example.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		apiFormat llm.APIFormat
		effort    string
		rawBody   string
	}{
		{name: "none", effort: "none"},
		{name: "unspecified"},
		{name: "minimal", effort: "minimal"},
		{name: "low", effort: "low"},
		{name: "medium", effort: "medium"},
		{name: "high", effort: "high"},
		{name: "xhigh", effort: "xhigh"},
		{name: "other value", effort: "provider-specific"},
		{name: "none ignores raw true", effort: "none", apiFormat: llm.APIFormatOpenAIChatCompletion, rawBody: `{"enable_thinking":true}`},
		{name: "high ignores raw false", effort: "high", apiFormat: llm.APIFormatOpenAIChatCompletion, rawBody: `{"enable_thinking":false}`},
		{name: "unspecified ignores raw false", apiFormat: llm.APIFormatOpenAIChatCompletion, rawBody: `{"enable_thinking":false}`},
		{name: "cross inbound none", effort: "none", apiFormat: llm.APIFormatAnthropicMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a unified request, optionally carrying conflicting native input.
			userContent := "hi"
			req := &llm.Request{
				Model:           "qwen-max",
				ReasoningEffort: tt.effort,
				APIFormat:       tt.apiFormat,
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: &userContent}},
				},
				RawRequest: &httpclient.Request{Body: []byte(tt.rawBody)},
			}
			original, err := json.Marshal(req)
			require.NoError(t, err)

			// When Bailian transforms the request.
			httpReq, err := transformer.TransformRequest(context.Background(), req)
			require.NoError(t, err)

			// Then the serialized reasoning options follow only the unified effort.
			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(httpReq.Body, &body))
			if tt.effort == "none" {
				require.JSONEq(t, `false`, string(body["enable_thinking"]))
				require.NotContains(t, body, "reasoning_effort")
			} else {
				require.NotContains(t, body, "enable_thinking")
				if tt.effort == "" {
					require.NotContains(t, body, "reasoning_effort")
				} else {
					expected, err := json.Marshal(tt.effort)
					require.NoError(t, err)
					require.JSONEq(t, string(expected), string(body["reasoning_effort"]))
				}
			}
			unchanged, err := json.Marshal(req)
			require.NoError(t, err)
			require.JSONEq(t, string(original), string(unchanged))
		})
	}
}

func TestBailianTransformRequest_OpenAIInboundReasoningNone(t *testing.T) {
	// Given a standard OpenAI request with a conflicting native flag.
	ctx := context.Background()
	input := &httpclient.Request{
		Headers: http.Header{"Content-Type": {"application/json"}},
		Body:    []byte(`{"model":"qwen-max","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"none","enable_thinking":true}`),
	}
	req, err := openai.NewInboundTransformer().TransformRequest(ctx, input)
	require.NoError(t, err)
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL: "https://example.com", APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// When the parsed request goes through Bailian outbound.
	output, err := transformer.TransformRequest(ctx, req)
	require.NoError(t, err)

	// Then standard none disables thinking without forwarding reasoning_effort.
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(output.Body, &body))
	require.JSONEq(t, `false`, string(body["enable_thinking"]))
	require.NotContains(t, body, "reasoning_effort")
	require.Equal(t, "none", req.ReasoningEffort)
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
