package anthropic

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
)

// TestServerToolUse_InboundRoundTrip ensures that a multi-turn Anthropic
// request whose assistant turn contains a server_tool_use + web_search_tool_result
// pair is preserved through the inbound→outbound conversion.
func TestServerToolUse_InboundRoundTrip(t *testing.T) {
	assistantInput := json.RawMessage(`{"query":"hello"}`)
	caller := json.RawMessage(`{"type":"direct"}`)
	resultContent := json.RawMessage(`[{"type":"web_search_result","url":"https://example.com","title":"Example","encrypted_content":"abc"}]`)

	// Start from an Anthropic-native MessageRequest shape.
	resultBlock := MessageContentBlock{
		Type:      "web_search_tool_result",
		ToolUseID: lo.ToPtr("srvtoolu_abc"),
		Caller:    caller,
	}
	resultBlock.Content = &MessageContent{}
	resultBlock.Content.SetRaw(resultContent)

	anthropicReq := &MessageRequest{
		Model:     "claude-opus-4-7",
		MaxTokens: 1024,
		Messages: []MessageParam{
			{
				Role: "user",
				Content: MessageContent{
					Content: lo.ToPtr("Please search for something."),
				},
			},
			{
				Role: "assistant",
				Content: MessageContent{
					MultipleContent: []MessageContentBlock{
						{Type: "text", Text: lo.ToPtr("Let me search.")},
						{
							Type:   "server_tool_use",
							ID:     "srvtoolu_abc",
							Name:   lo.ToPtr("web_search"),
							Input:  assistantInput,
							Caller: caller,
						},
						resultBlock,
					},
				},
			},
		},
	}

	// Inbound: Anthropic -> llm.Request
	req, err := convertToLLMRequest(anthropicReq)
	require.NoError(t, err)

	// Find the assistant message.
	var assistantMsg *llm.Message
	for i := range req.Messages {
		if req.Messages[i].Role == "assistant" {
			assistantMsg = &req.Messages[i]
			break
		}
	}

	require.NotNil(t, assistantMsg)
	require.Len(t, assistantMsg.ToolCalls, 1, "server_tool_use should become a tool call")
	require.Equal(t, "server_tool_use", getAnthropicType(assistantMsg.ToolCalls[0].TransformerMetadata))
	require.JSONEq(t, string(caller), string(getAnthropicCaller(assistantMsg.ToolCalls[0].TransformerMetadata)))

	require.Len(t, assistantMsg.InlineToolResults, 1, "web_search_tool_result should become an inline tool result")

	ir := assistantMsg.InlineToolResults[0]
	require.Equal(t, "srvtoolu_abc", ir.ToolCallID)
	require.Equal(t, "web_search_tool_result", getAnthropicType(ir.TransformerMetadata))
	require.JSONEq(t, string(caller), string(getAnthropicCaller(ir.TransformerMetadata)))

	raw := getAnthropicToolResultContent(ir.TransformerMetadata)
	require.NotEmpty(t, raw)
	require.Equal(t, "web_search_result", gjson.GetBytes(raw, "0.type").String())

	// Outbound restore: use toolUseBlockFromLLM and toolResultBlockFromInline to
	// rebuild Anthropic content blocks and confirm the original type + caller survive.
	useBlock := toolUseBlockFromLLM(assistantMsg.ToolCalls[0])
	require.Equal(t, "server_tool_use", useBlock.Type)
	require.Equal(t, "srvtoolu_abc", useBlock.ID)
	require.JSONEq(t, string(caller), string(useBlock.Caller))

	resultRebuilt, ok := toolResultBlockFromInline(ir)
	require.True(t, ok)
	require.Equal(t, "web_search_tool_result", resultRebuilt.Type)
	require.NotNil(t, resultRebuilt.ToolUseID)
	require.Equal(t, "srvtoolu_abc", *resultRebuilt.ToolUseID)
	require.JSONEq(t, string(caller), string(resultRebuilt.Caller))

	// The rebuilt content should serialize to the same bytes as the original
	// (byte-identical round-trip).
	require.NotNil(t, resultRebuilt.Content)

	marshaled, err := json.Marshal(resultRebuilt.Content)
	require.NoError(t, err)
	require.JSONEq(t, string(resultContent), string(marshaled))
}

// TestConvertToLlmResponse_ServerToolUse covers the non-streaming path:
// a Message payload with a server_tool_use + web_search_tool_result is
// surfaced as a ToolCall + InlineToolResult with preserved metadata.
func TestConvertToLlmResponse_ServerToolUse(t *testing.T) {
	caller := json.RawMessage(`{"type":"direct"}`)
	resultContent := json.RawMessage(`[{"type":"web_search_result","url":"https://example.com","title":"Example"}]`)

	resultBlock := MessageContentBlock{
		Type:      "web_search_tool_result",
		ToolUseID: lo.ToPtr("srvtoolu_abc"),
		Caller:    caller,
	}
	resultBlock.Content = &MessageContent{}
	resultBlock.Content.SetRaw(resultContent)

	anthropicResp := &Message{
		ID:    "msg_1",
		Role:  "assistant",
		Type:  "message",
		Model: "claude-opus-4-7",
		Content: []MessageContentBlock{
			{
				Type:   "server_tool_use",
				ID:     "srvtoolu_abc",
				Name:   lo.ToPtr("web_search"),
				Input:  json.RawMessage(`{"query":"hello"}`),
				Caller: caller,
			},
			resultBlock,
		},
	}

	resp := convertToLlmResponse(anthropicResp, PlatformDirect)
	require.NotNil(t, resp)
	require.Len(t, resp.Choices, 1)

	msg := resp.Choices[0].Message
	require.NotNil(t, msg)
	require.Len(t, msg.ToolCalls, 1)
	require.Equal(t, "server_tool_use", getAnthropicType(msg.ToolCalls[0].TransformerMetadata))

	require.Len(t, msg.InlineToolResults, 1)
	require.Equal(t, "web_search_tool_result", getAnthropicType(msg.InlineToolResults[0].TransformerMetadata))
}

// TestAnthropicResponse_RoundTrip_ServerToolUse exercises the non-streaming
// response round-trip against the real-world web_search fixture:
// Anthropic Message → llm.Response → Anthropic Message. Text-block
// interleaving and citations are deferred (handled in a later spec), so this
// test asserts only the fields that must survive today:
//   - server_tool_use block with id / name / input / caller
//   - web_search_tool_result block with tool_use_id / caller / result entries
//   - total assistant text content (concatenated across all text blocks)
func TestAnthropicResponse_RoundTrip_ServerToolUse(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test")

	var original Message

	require.NoError(t, xtest.LoadTestData(t, "anthropic-server-tool.response.json", &original))

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	require.NoError(t, enc.Encode(original))

	chatResp, err := outboundTransformer.TransformResponse(t.Context(), &httpclient.Response{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    buf.Bytes(),
	})
	require.NoError(t, err)

	inboundResp, err := inboundTransformer.TransformResponse(t.Context(), chatResp)
	require.NoError(t, err)

	var got Message

	require.NoError(t, json.Unmarshal(inboundResp.Body, &got))

	wantUse, wantResult := findServerToolBlocks(t, original.Content)
	gotUse, gotResult := findServerToolBlocks(t, got.Content)

	require.Equal(t, wantUse.Type, gotUse.Type, "tool_use block type")
	require.Equal(t, wantUse.ID, gotUse.ID, "tool_use id")
	require.Equal(t, lo.FromPtr(wantUse.Name), lo.FromPtr(gotUse.Name), "tool_use name")
	require.JSONEq(t, string(wantUse.Input), string(gotUse.Input), "tool_use input")

	require.Equal(t, wantResult.Type, gotResult.Type, "tool_result block type")
	require.Equal(t, lo.FromPtr(wantResult.ToolUseID), lo.FromPtr(gotResult.ToolUseID), "tool_result tool_use_id")
	if len(wantResult.Caller) > 0 {
		require.JSONEq(t, string(wantResult.Caller), string(gotResult.Caller), "tool_result caller")
	}

	wantRaw, _ := json.Marshal(wantResult.Content)
	gotRaw, _ := json.Marshal(gotResult.Content)
	require.JSONEq(t, string(wantRaw), string(gotRaw), "tool_result content")

	require.Equal(t,
		concatText(original.Content),
		concatText(got.Content),
		"concatenated text content should match")

	// Ordering sanity: the server_tool_use and web_search_tool_result blocks
	// must stay BEFORE any assistant text blocks, because the text narrates
	// the tool's search results. Emitting text first would be nonsensical.
	useIdx, resultIdx, firstTextIdx := -1, -1, -1
	for i, b := range got.Content {
		switch {
		case b.Type == "server_tool_use" && useIdx < 0:
			useIdx = i
		case b.Type == "web_search_tool_result" && resultIdx < 0:
			resultIdx = i
		case b.Type == "text" && firstTextIdx < 0:
			firstTextIdx = i
		}
	}
	require.GreaterOrEqual(t, useIdx, 0, "server_tool_use must be emitted")
	require.GreaterOrEqual(t, resultIdx, 0, "web_search_tool_result must be emitted")
	require.GreaterOrEqual(t, firstTextIdx, 0, "at least one text block must be emitted")
	require.Less(t, useIdx, firstTextIdx, "server_tool_use must come before assistant text")
	require.Less(t, resultIdx, firstTextIdx, "web_search_tool_result must come before assistant text")

	// Wire-level check: the individual web_search_result entries inside the
	// tool_result content must carry their original fields (url, title,
	// encrypted_content, …). Those are not modeled on MessageContentBlock,
	// so they must survive via the content byte-passthrough.
	results := gjson.GetBytes(inboundResp.Body, `content.#(type=="web_search_tool_result").content`)
	require.True(t, results.Exists(), "web_search_tool_result.content missing in wire body")
	require.True(t, results.IsArray(), "web_search_tool_result.content should be an array")
	require.Greater(t, len(results.Array()), 0, "web_search_tool_result.content should not be empty")

	first := results.Array()[0]
	require.Equal(t, "web_search_result", first.Get("type").String(),
		"nested result type preserved")
	require.NotEmpty(t, first.Get("url").String(),
		"nested web_search_result must preserve url")
	require.NotEmpty(t, first.Get("title").String(),
		"nested web_search_result must preserve title")
	require.NotEmpty(t, first.Get("encrypted_content").String(),
		"nested web_search_result must preserve encrypted_content")
}

func findServerToolBlocks(t *testing.T, blocks []MessageContentBlock) (MessageContentBlock, MessageContentBlock) {
	t.Helper()

	var use, result MessageContentBlock

	for _, b := range blocks {
		if b.Type == "server_tool_use" {
			use = b
		}
		if b.Type == "web_search_tool_result" {
			result = b
		}
	}

	require.Equal(t, "server_tool_use", use.Type, "server_tool_use block should be present")
	require.Equal(t, "web_search_tool_result", result.Type, "web_search_tool_result block should be present")

	return use, result
}

func concatText(blocks []MessageContentBlock) string {
	var sb strings.Builder

	for _, b := range blocks {
		if b.Type == "text" && b.Text != nil {
			sb.WriteString(*b.Text)
		}
	}

	return sb.String()
}

// Silence linter for gjson import reuse across tests in this file.
var _ = gjson.Parse
