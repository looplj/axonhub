package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	openaiTransformer "github.com/looplj/axonhub/llm/transformer/openai"
)

// A1: same-protocol public seam must preserve unknown request content blocks and
// unknown tool_result nested children. Do not drop them in closed switches.
func TestA1_UnknownRequestContentBlock_SameProtocolRoundTrip(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 64,
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "before"},
					{
						"type": "search_result",
						"source": "web",
						"title": "Example",
						"url": "https://example.com/a",
						"future_nested": {"keep": true, "n": 2}
					},
					{"type": "text", "text": "after"}
				]
			}
		]
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	requestExt := llmReq.ProviderExtensions.Anthropic.Request
	require.Len(t, requestExt.RawContentFragments, 1)
	fragment := requestExt.RawContentFragments[0]
	require.Equal(t, 0, fragment.MessageIndex)
	require.Equal(t, 1, fragment.PartIndex)
	require.False(t, fragment.NestedInToolResult)
	require.JSONEq(t, `{"type":"search_result","source":"web","title":"Example","url":"https://example.com/a","future_nested":{"keep":true,"n":2}}`, string(fragment.Raw))

	require.Len(t, llmReq.Messages, 1)
	require.Len(t, llmReq.Messages[0].Content.MultipleContent, 3)
	rawPart := llmReq.Messages[0].Content.MultipleContent[1]
	require.Equal(t, "anthropic_raw_block", rawPart.Type)
	require.Equal(t, 1, getAnthropicBlockIndex(rawPart.TransformerMetadata))
	require.NotContains(t, rawPart.TransformerMetadata, "anthropic_raw_block", "canonical raw placeholder must not retain provider JSON")

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	upstream, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var source, out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(upstream.Body, &out))

	var sourceMsgs, outMsgs []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["messages"], &sourceMsgs))
	require.NoError(t, json.Unmarshal(out["messages"], &outMsgs))
	require.Len(t, outMsgs, 1)

	var sourceBlocks, outBlocks []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(sourceMsgs[0]["content"], &sourceBlocks))
	require.NoError(t, json.Unmarshal(outMsgs[0]["content"], &outBlocks))
	require.Len(t, outBlocks, 3, "unknown block between known text blocks must survive same-protocol replay")

	require.JSONEq(t, `"text"`, string(outBlocks[0]["type"]))
	require.JSONEq(t, `"before"`, string(outBlocks[0]["text"]))

	require.JSONEq(t, string(sourceBlocks[1]["type"]), string(outBlocks[1]["type"]))
	require.JSONEq(t, string(sourceBlocks[1]["source"]), string(outBlocks[1]["source"]))
	require.JSONEq(t, string(sourceBlocks[1]["title"]), string(outBlocks[1]["title"]))
	require.JSONEq(t, string(sourceBlocks[1]["url"]), string(outBlocks[1]["url"]))
	require.JSONEq(t, string(sourceBlocks[1]["future_nested"]), string(outBlocks[1]["future_nested"]))

	require.JSONEq(t, `"text"`, string(outBlocks[2]["type"]))
	require.JSONEq(t, `"after"`, string(outBlocks[2]["text"]))
}

// A1: Anthropic-only raw placeholders are sidecars for Anthropic replay, not
// OpenAI Chat content parts. A cross-protocol request must drop the placeholder
// rather than serialize an invalid provider-internal type.
func TestA1_UnknownRequestContentBlock_DoesNotLeakToOpenAIChat(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"max_tokens": 64,
		"messages": [{
			"role": "user",
			"content": [
				{"type": "text", "text": "before"},
				{"type": "search_result", "source": "web", "title": "Example"},
				{"type": "text", "text": "after"}
			]
		}]
	}`)

	anthropicInbound := NewInboundTransformer()
	llmReq, err := anthropicInbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	require.NotEmpty(t, llmReq.ProviderExtensions.Anthropic.Request.RawContentFragments)

	chatOutbound, err := openaiTransformer.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	chatReq, err := chatOutbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(chatReq.Body, &got))
	var messages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got["messages"], &messages))
	require.Len(t, messages, 1)
	require.NotContains(t, string(messages[0]["content"]), "anthropic_raw_block")
	require.JSONEq(t, `[{"type":"text","text":"before"},{"type":"text","text":"after"}]`, string(messages[0]["content"]))
}

func TestA1_ToolResultUnknownChild_SameProtocolRoundTrip(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 64,
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_01",
						"content": [
							{"type": "text", "text": "ok"},
							{
								"type": "search_result",
								"source": "web",
								"title": "Nested",
								"url": "https://example.com/nested",
								"provider_native": {"rank": 1}
							}
						]
					},
					{"type": "text", "text": "continue"}
				]
			}
		]
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	requestExt := llmReq.ProviderExtensions.Anthropic.Request
	require.Len(t, requestExt.RawContentFragments, 1)
	fragment := requestExt.RawContentFragments[0]
	require.Equal(t, 0, fragment.MessageIndex)
	require.Equal(t, 1, fragment.PartIndex)
	require.True(t, fragment.NestedInToolResult)
	require.JSONEq(t, `{"type":"search_result","source":"web","title":"Nested","url":"https://example.com/nested","provider_native":{"rank":1}}`, string(fragment.Raw))

	require.Len(t, llmReq.Messages, 2)
	require.Len(t, llmReq.Messages[0].Content.MultipleContent, 2)
	rawPart := llmReq.Messages[0].Content.MultipleContent[1]
	require.Equal(t, "anthropic_raw_block", rawPart.Type)
	require.Equal(t, 1, getAnthropicBlockIndex(rawPart.TransformerMetadata))
	require.NotContains(t, rawPart.TransformerMetadata, "anthropic_raw_block", "canonical raw placeholder must not retain provider JSON")

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	upstream, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstream.Body, &out))
	var outMsgs []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out["messages"], &outMsgs))
	require.NotEmpty(t, outMsgs)

	// tool_result and following user text may be merged into one user message.
	var foundToolResult bool
	for _, msg := range outMsgs {
		var blocks []map[string]json.RawMessage
		if err := json.Unmarshal(msg["content"], &blocks); err != nil {
			continue
		}
		for _, block := range blocks {
			if string(block["type"]) != `"tool_result"` {
				continue
			}
			foundToolResult = true
			var children []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(block["content"], &children))
			require.GreaterOrEqual(t, len(children), 2, "unknown tool_result child must not be dropped")

			require.JSONEq(t, `"text"`, string(children[0]["type"]))
			require.JSONEq(t, `"ok"`, string(children[0]["text"]))

			require.JSONEq(t, `"search_result"`, string(children[1]["type"]))
			require.JSONEq(t, `"https://example.com/nested"`, string(children[1]["url"]))
			require.JSONEq(t, `{"rank":1}`, string(children[1]["provider_native"]))
		}
	}
	require.True(t, foundToolResult, "tool_result block must be present after round-trip")
}

func TestA1_ConvertibleMediaBeforeUnknownBlock_PreservesExactOrder(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "image",
			content: `[
				{
					"type": "image",
					"source": {
						"type": "base64",
						"media_type": "image/png",
						"data": "iVBORw0KGgo="
					}
				},
				{
					"type": "search_result",
					"source": "web",
					"title": "After image",
					"url": "https://example.com/image-result",
					"future_nested": {"keep": true}
				}
			]`,
		},
		{
			name: "document",
			content: `[
				{
					"type": "document",
					"source": {
						"type": "base64",
						"media_type": "application/pdf",
						"data": "JVBERi0xLjQ="
					}
				},
				{
					"type": "search_result",
					"source": "web",
					"title": "After document",
					"url": "https://example.com/document-result",
					"future_nested": {"keep": true}
				}
			]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 64,
				"messages": [{
					"role": "user",
					"content": ` + tt.content + `
				}]
			}`)

			inbound := NewInboundTransformer()
			llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{"application/json"}},
				Body:    body,
			})
			require.NoError(t, err)

			outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
			require.NoError(t, err)
			upstream, err := outbound.TransformRequest(t.Context(), llmReq)
			require.NoError(t, err)

			var out map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(upstream.Body, &out))
			var outMessages []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(out["messages"], &outMessages))
			require.Len(t, outMessages, 1)
			require.JSONEq(t, tt.content, string(outMessages[0]["content"]), "content block array order must survive the public same-protocol request round-trip")
		})
	}
}

func TestA1_ToolResultWithTopLevelUnknownBlock_SameProtocolRoundTrip(t *testing.T) {
	// Public Anthropic request -> canonical -> Anthropic request seam.
	// When a single user content array mixes tool_result with a future top-level
	// block (and a typed text block), grouping must not drop the raw block.
	body := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 64,
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_a1_group",
						"content": "tool output"
					},
					{"type": "text", "text": "typed-after-tool"},
					{
						"type": "search_result",
						"source": "web",
						"title": "Grouped future block",
						"url": "https://example.com/grouped-future",
						"future_nested": {"keep": true, "via": "tool_result_group"}
					}
				]
			}
		]
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	requestExt := llmReq.ProviderExtensions.Anthropic.Request
	require.Len(t, requestExt.RawContentFragments, 1)
	fragment := requestExt.RawContentFragments[0]
	require.Equal(t, 1, fragment.MessageIndex, "sidecar owner is the canonical user message, not the grouped tool message")
	require.Equal(t, 2, fragment.PartIndex)
	require.False(t, fragment.NestedInToolResult)
	require.JSONEq(t, `{"type":"search_result","source":"web","title":"Grouped future block","url":"https://example.com/grouped-future","future_nested":{"keep":true,"via":"tool_result_group"}}`, string(fragment.Raw))

	require.Len(t, llmReq.Messages, 2)
	rawPart := llmReq.Messages[1].Content.MultipleContent[1]
	require.Equal(t, "anthropic_raw_block", rawPart.Type)
	require.Equal(t, 2, getAnthropicBlockIndex(rawPart.TransformerMetadata))
	require.NotContains(t, rawPart.TransformerMetadata, "anthropic_raw_block", "canonical raw placeholder must not retain provider JSON")

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	upstream, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var source, out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(upstream.Body, &out))

	var sourceMsgs, outMsgs []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["messages"], &sourceMsgs))
	require.NoError(t, json.Unmarshal(out["messages"], &outMsgs))
	require.NotEmpty(t, outMsgs)

	var sourceBlocks []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(sourceMsgs[0]["content"], &sourceBlocks))
	require.Len(t, sourceBlocks, 3)

	// tool_result grouping may collapse into one user message; collect all blocks.
	var outBlocks []map[string]json.RawMessage
	for _, msg := range outMsgs {
		var blocks []map[string]json.RawMessage
		if err := json.Unmarshal(msg["content"], &blocks); err != nil {
			continue
		}
		outBlocks = append(outBlocks, blocks...)
	}
	require.Len(t, outBlocks, 3, "tool_result + typed text + future top-level block must all survive grouping")

	require.JSONEq(t, `"tool_result"`, string(outBlocks[0]["type"]))
	require.JSONEq(t, `"toolu_a1_group"`, string(outBlocks[0]["tool_use_id"]))
	require.JSONEq(t, `"tool output"`, string(outBlocks[0]["content"]))

	require.JSONEq(t, `"text"`, string(outBlocks[1]["type"]))
	require.JSONEq(t, `"typed-after-tool"`, string(outBlocks[1]["text"]))

	require.JSONEq(t, string(sourceBlocks[2]["type"]), string(outBlocks[2]["type"]))
	require.JSONEq(t, string(sourceBlocks[2]["source"]), string(outBlocks[2]["source"]))
	require.JSONEq(t, string(sourceBlocks[2]["title"]), string(outBlocks[2]["title"]))
	require.JSONEq(t, string(sourceBlocks[2]["url"]), string(outBlocks[2]["url"]))
	require.JSONEq(t, string(sourceBlocks[2]["future_nested"]), string(outBlocks[2]["future_nested"]))
}
