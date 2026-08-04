package longcat

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestOutboundTransformer_ResponsesToolLifecycle_NonStreamRestore(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.example.com", "test-key")
	require.NoError(t, err)
	longcat := outbound.(*OutboundTransformer)
	require.True(t, longcat.ResponsesRequestCapabilities(&llm.Request{}).ChatToolLifecycle)
	require.False(t, longcat.ResponsesRequestCapabilities(&llm.Request{RequestType: llm.RequestTypeCompact}).ChatToolLifecycle)

	request := &llm.Request{
		Model:     "LongCat-Flash-Omni-2603",
		APIFormat: llm.APIFormatOpenAIResponse,
		Stream:    lo.ToPtr(false),
		Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("patch")}}},
		Tools: []llm.Tool{{
			Type:               llm.ToolTypeResponsesCustomTool,
			ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"},
		}},
	}
	httpRequest, err := longcat.TransformRequest(t.Context(), request)
	require.NoError(t, err)
	require.Contains(t, httpRequest.TransformerMetadata, "openai_responses_chat_tool_mappings")
	require.Contains(t, httpRequest.TransformerMetadata, "openai_responses_chat_tool_catalog")
	require.Equal(t, llm.ToolTypeResponsesCustomTool, request.Tools[0].Type)

	var wire openai.Request
	require.NoError(t, json.Unmarshal(httpRequest.Body, &wire))
	require.Len(t, wire.Tools, 1)
	require.Equal(t, llm.ToolTypeFunction, wire.Tools[0].Type)
	require.Equal(t, "apply_patch", wire.Tools[0].Function.Name)

	response, err := longcat.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Request:    httpRequest,
		Body: []byte(`{
			"id":"chatcmpl_1","object":"chat.completion","model":"LongCat-Flash-Omni-2603",
			"choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{
				"id":"call_1","type":"function","function":{"name":"apply_patch","arguments":"{\"input\":\"patch\"}"}
			}]},"finish_reason":"tool_calls"}]
		}`),
	})
	require.NoError(t, err)
	require.Len(t, response.Choices, 1)
	require.NotNil(t, response.Choices[0].Message)
	require.Len(t, response.Choices[0].Message.ToolCalls, 1)
	call := response.Choices[0].Message.ToolCalls[0]
	require.Equal(t, llm.ToolTypeResponsesCustomTool, call.Type)
	require.NotNil(t, call.ResponseCustomToolCall)
	require.Equal(t, "apply_patch", call.ResponseCustomToolCall.Name)
	require.Equal(t, "patch", call.ResponseCustomToolCall.Input)
}

func TestOutboundTransformer_TransformRequest_ForceMultipleContent(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.example.com", "test-key")
	require.NoError(t, err)

	tests := []struct {
		name     string
		content  llm.MessageContent
		wantText string
	}{
		{
			name:     "plain string content is converted to array",
			content:  llm.MessageContent{Content: lo.ToPtr("Hello!")},
			wantText: "Hello!",
		},
		{
			name:     "empty string content is converted to array",
			content:  llm.MessageContent{Content: lo.ToPtr("")},
			wantText: "",
		},
		{
			name:     "nil content gets empty text array",
			content:  llm.MessageContent{},
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &llm.Request{
				Model:  "LongCat-Flash-Omni-2603",
				Stream: lo.ToPtr(false),
				Messages: []llm.Message{
					{Role: "user", Content: tt.content},
				},
			}

			httpReq, err := tr.TransformRequest(context.Background(), req)
			require.NoError(t, err)
			if tt.content.Content == nil && len(tt.content.MultipleContent) == 0 {
				require.Nil(t, req.Messages[0].Content.Content)
				require.Empty(t, req.Messages[0].Content.MultipleContent)
			}

			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(httpReq.Body, &body))

			var messages []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body["messages"], &messages))

			// Content must be an array, not a string
			contentRaw := messages[0]["content"]
			require.True(t, len(contentRaw) > 0 && contentRaw[0] == '[',
				"expected array content, got: %s", string(contentRaw))

			var parts []map[string]string
			require.NoError(t, json.Unmarshal(contentRaw, &parts))
			require.Len(t, parts, 1)
			require.Equal(t, "text", parts[0]["type"])
			require.Equal(t, tt.wantText, parts[0]["text"])
		})
	}
}

func TestOutboundTransformer_TransformRequest_MultipleContentPreserved(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.example.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:  "LongCat-Flash-Omni-2603",
		Stream: lo.ToPtr(false),
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("What is this?")},
						{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/img.png"}},
					},
				},
			},
		},
	}

	httpReq, err := tr.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(httpReq.Body, &body))

	var messages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body["messages"], &messages))

	// Content must remain an array
	contentRaw := messages[0]["content"]
	require.True(t, len(contentRaw) > 0 && contentRaw[0] == '[',
		"expected array content, got: %s", string(contentRaw))
}
