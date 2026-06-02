package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// TestOutboundTransformer_ServerToolUse_NoPanic replays the SSE trace from the
// production panic report: a server_tool_use content block followed by
// input_json_delta chunks. Before the fix this tripped a nil-pointer
// dereference inside transformStreamChunk.
func TestOutboundTransformer_ServerToolUse_NoPanic(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		Type:           PlatformDirect,
		BaseURL:        "https://api.anthropic.com",
		APIKeyProvider: auth.NewStaticKeyProvider("dummy"),
	})
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		sseEvent(t, "message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    "msg_01AEsGpin3gJumakZWMTyQp3",
				"type":  "message",
				"role":  "assistant",
				"model": "claude-opus-4-7",
				"content": []any{},
				"usage": map[string]any{
					"input_tokens":  6,
					"output_tokens": 0,
				},
			},
		}),
		sseEvent(t, "content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": 1,
			"content_block": map[string]any{
				"type":  "server_tool_use",
				"id":    "srvtoolu_01U9uNSdDhJHvxz2mBp8qtdv",
				"name":  "web_search",
				"input": map[string]any{},
			},
		}),
		sseEvent(t, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": 1,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": ""},
		}),
		sseEvent(t, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": 1,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": "{\"query\": \"K"},
		}),
		sseEvent(t, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": 1,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": "Win\"}"},
		}),
		sseEvent(t, "content_block_stop", map[string]any{
			"type": "content_block_stop", "index": 1,
		}),
		sseEvent(t, "message_delta", map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": "tool_use"},
			"usage": map[string]any{"output_tokens": 10},
		}),
		sseEvent(t, "message_stop", map[string]any{"type": "message_stop"}),
	}

	mockStream := streams.SliceStream(events)

	ctx := t.Context()

	transformed, err := transformer.TransformStream(ctx, nil, mockStream)
	require.NoError(t, err)

	var combinedArgs strings.Builder

	var (
		toolCallName    string
		toolCallID      string
		anthropicType   string
		responseCount   int
		sawDoneResponse bool
	)

	for transformed.Next() {
		resp := transformed.Current()
		if resp == nil {
			continue
		}

		responseCount++
		if resp == llm.DoneResponse {
			sawDoneResponse = true
			continue
		}

		for _, choice := range resp.Choices {
			if choice.Delta == nil {
				continue
			}

			for _, tc := range choice.Delta.ToolCalls {
				if tc.ID != "" {
					toolCallID = tc.ID
				}
				if tc.Function.Name != "" {
					toolCallName = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					combinedArgs.WriteString(tc.Function.Arguments)
				}
				if at := getAnthropicType(tc.TransformerMetadata); at != "" {
					anthropicType = at
				}
			}
		}
	}

	require.NoError(t, transformed.Err())
	require.Greater(t, responseCount, 0)
	require.True(t, sawDoneResponse)
	require.Equal(t, "srvtoolu_01U9uNSdDhJHvxz2mBp8qtdv", toolCallID)
	require.Equal(t, "web_search", toolCallName)
	require.Equal(t, "server_tool_use", anthropicType)
	require.Equal(t, `{"query": "KWin"}`, combinedArgs.String())
	require.True(t, json.Valid([]byte(combinedArgs.String())))
}

// TestOutboundTransformer_WebSearchToolResult_InlineResult ensures a
// web_search_tool_result content block is surfaced on the assistant message
// as an InlineToolResult with the original content bytes preserved.
func TestOutboundTransformer_WebSearchToolResult_InlineResult(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		Type:           PlatformDirect,
		BaseURL:        "https://api.anthropic.com",
		APIKeyProvider: auth.NewStaticKeyProvider("dummy"),
	})
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		sseEvent(t, "message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id": "msg_abc", "type": "message", "role": "assistant",
				"model":   "claude-opus-4-7",
				"content": []any{},
				"usage":   map[string]any{"input_tokens": 1, "output_tokens": 0},
			},
		}),
		sseEvent(t, "content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type":        "web_search_tool_result",
				"tool_use_id": "srvtoolu_abc",
				"content": []any{
					map[string]any{
						"type":              "web_search_result",
						"url":               "https://example.com",
						"title":             "Example",
						"encrypted_content": "EqgfCio...",
					},
				},
			},
		}),
		sseEvent(t, "content_block_stop", map[string]any{
			"type": "content_block_stop", "index": 0,
		}),
		sseEvent(t, "message_delta", map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": "end_turn"},
			"usage": map[string]any{"output_tokens": 5},
		}),
		sseEvent(t, "message_stop", map[string]any{"type": "message_stop"}),
	}

	ctx := t.Context()

	transformed, err := transformer.TransformStream(ctx, nil, streams.SliceStream(events))
	require.NoError(t, err)

	var inline *llm.InlineToolResult

	for transformed.Next() {
		resp := transformed.Current()
		if resp == nil || resp == llm.DoneResponse {
			continue
		}

		for _, choice := range resp.Choices {
			if choice.Delta != nil && len(choice.Delta.InlineToolResults) > 0 {
				copy := choice.Delta.InlineToolResults[0]
				inline = &copy
			}
		}
	}

	require.NoError(t, transformed.Err())
	require.NotNil(t, inline, "expected an InlineToolResult on the assistant delta")
	require.Equal(t, "srvtoolu_abc", inline.ToolCallID)
	require.Equal(t, "web_search_tool_result", getAnthropicType(inline.TransformerMetadata))
	require.False(t, inline.IsError)

	// The preserved content bytes should still parse.
	raw := getAnthropicToolResultContent(inline.TransformerMetadata)
	require.NotEmpty(t, raw)
	require.Equal(t, "web_search_result", gjson.GetBytes(raw, "0.type").String())
}

func sseEvent(t *testing.T, typ string, payload map[string]any) *httpclient.StreamEvent {
	t.Helper()

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	return &httpclient.StreamEvent{Type: typ, Data: data}
}
