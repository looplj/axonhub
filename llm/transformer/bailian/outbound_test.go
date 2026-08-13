package bailian

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
	responsesapi "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestBailianResponsesToolLifecycle_StreamsThroughWrapper(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://example.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)
	bailian := outbound.(*OutboundTransformer)
	require.True(t, bailian.ResponsesRequestCapabilities(&llm.Request{}).ChatToolLifecycle)
	require.False(t, bailian.ResponsesRequestCapabilities(&llm.Request{RequestType: llm.RequestTypeCompact}).ChatToolLifecycle)

	request := &llm.Request{
		Model:     "qwen-max",
		APIFormat: llm.APIFormatOpenAIResponse,
		Stream:    lo.ToPtr(true),
		Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("patch")}}},
		Tools: []llm.Tool{{
			Type: llm.ToolTypeResponsesCustomTool,
			ResponseCustomTool: &llm.ResponseCustomTool{
				Name: "apply_patch", Description: "Apply patch text",
			},
		}},
	}
	httpRequest, err := bailian.TransformRequest(t.Context(), request)
	require.NoError(t, err)
	require.Contains(t, httpRequest.TransformerMetadata, "openai_responses_chat_tool_mappings")
	require.Contains(t, httpRequest.TransformerMetadata, "openai_responses_chat_tool_catalog")
	require.Equal(t, llm.ToolTypeResponsesCustomTool, request.Tools[0].Type)

	var wire openai.Request
	require.NoError(t, json.Unmarshal(httpRequest.Body, &wire))
	require.Len(t, wire.Tools, 1)
	require.Equal(t, llm.ToolTypeFunction, wire.Tools[0].Type)
	require.Equal(t, "apply_patch", wire.Tools[0].Function.Name)
	require.JSONEq(t, `{"type":"object","properties":{"input":{"type":"string","description":"Exact raw custom-tool input. Escape quotes, backslashes, and control characters only as required for the outer JSON string; do not add another object or serialization layer."}},"required":["input"],"additionalProperties":false}`, string(wire.Tools[0].Function.Parameters))

	providerStream, err := bailian.TransformStream(t.Context(), httpRequest, streams.SliceStream([]*httpclient.StreamEvent{
		{Data: []byte(`{"id":"chatcmpl_1","object":"chat.completion.chunk","model":"qwen-max","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"apply_patch","arguments":"{\"input\":\"patch\"}"}}]},"finish_reason":"tool_calls"}]}`)},
		{Data: []byte(`[DONE]`)},
	}))
	require.NoError(t, err)
	stream, err := responsesapi.NewInboundTransformer().TransformStream(t.Context(), providerStream)
	require.NoError(t, err)

	var customDone *responsesapi.Item
	for stream.Next() {
		var event responsesapi.StreamEvent
		require.NoError(t, json.Unmarshal(stream.Current().Data, &event))
		if event.Type == responsesapi.StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "custom_tool_call" {
			customDone = event.Item
		}
	}
	require.NoError(t, stream.Err())
	require.NotNil(t, customDone)
	require.Equal(t, "apply_patch", customDone.Name)
	require.Equal(t, "patch", lo.FromPtr(customDone.Input))
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
