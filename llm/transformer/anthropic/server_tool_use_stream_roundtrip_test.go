package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm/streams"
)

// TestAnthropicStreamRoundTrip_ServerToolUse exercises the
// anthropic SSE → llm.Response chunks → anthropic SSE pipeline with the
// web_search (server_tool_use + web_search_tool_result) fixture.
//
// Structural expectations (the SSE ordering may normalize slightly, but the
// key fields must survive):
//
//   - server_tool_use content_block_start appears with same id / name / caller
//   - its input (across input_json_delta chunks) concatenates to a JSON-valid
//     object matching the original
//   - web_search_tool_result content_block_start appears with same tool_use_id /
//     caller / content bytes
//   - text content blocks produce identical concatenated text per block
//   - signature_delta values survive (subject to scope encoding — ignored here)
func TestAnthropicStreamRoundTrip_ServerToolUse(t *testing.T) {
	originalEvents, err := xtest.LoadStreamChunks(t, "anthropic-server-tool.stream.jsonl")
	require.NoError(t, err)

	// 1) outbound: Anthropic SSE → []*llm.Response chunks
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		Type:           PlatformDirect,
		BaseURL:        "https://api.anthropic.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test"),
	})
	require.NoError(t, err)

	ctx := t.Context()

	outboundStream, err := outbound.TransformStream(ctx, nil, streams.SliceStream(originalEvents))
	require.NoError(t, err)

	var llmResponses []*llm.Response
	for outboundStream.Next() {
		resp := outboundStream.Current()
		if resp != nil {
			llmResponses = append(llmResponses, resp)
		}
	}
	require.NoError(t, outboundStream.Err())
	require.NotEmpty(t, llmResponses)

	// 2) inbound: []*llm.Response → Anthropic SSE
	inbound := NewInboundTransformer()
	inboundStream, err := inbound.TransformStream(ctx, streams.SliceStream(llmResponses))
	require.NoError(t, err)

	var emittedEvents []*httpclient.StreamEvent
	for inboundStream.Next() {
		ev := inboundStream.Current()
		if ev != nil {
			emittedEvents = append(emittedEvents, ev)
		}
	}
	require.NoError(t, inboundStream.Err())
	require.NotEmpty(t, emittedEvents)

	original := summarizeStream(t, originalEvents)
	emitted := summarizeStream(t, emittedEvents)

	// --- server_tool_use: id / name / input / caller must survive ---
	require.Len(t, emitted.toolUses, len(original.toolUses),
		"server_tool_use block count should survive round-trip")
	for i, want := range original.toolUses {
		got := emitted.toolUses[i]
		require.Equal(t, want.blockType, got.blockType, "tool use [%d]: block type", i)
		require.Equal(t, want.id, got.id, "tool use [%d]: id", i)
		require.Equal(t, want.name, got.name, "tool use [%d]: name", i)
		require.JSONEq(t, want.input, got.input, "tool use [%d]: input JSON", i)
		if len(want.caller) > 0 {
			require.JSONEq(t, string(want.caller), string(got.caller),
				"tool use [%d]: caller", i)
		}
	}

	// --- *_tool_result: tool_use_id / caller / content must survive ---
	require.Len(t, emitted.toolResults, len(original.toolResults),
		"*_tool_result block count should survive round-trip")
	for i, want := range original.toolResults {
		got := emitted.toolResults[i]
		require.Equal(t, want.blockType, got.blockType, "tool result [%d]: block type", i)
		require.Equal(t, want.toolUseID, got.toolUseID, "tool result [%d]: tool_use_id", i)
		if len(want.caller) > 0 {
			require.JSONEq(t, string(want.caller), string(got.caller),
				"tool result [%d]: caller", i)
		}
		require.JSONEq(t, string(want.content), string(got.content),
			"tool result [%d]: content", i)
	}

	// --- text content: concatenated text across all text blocks must match.
	// We don't assert per-block boundaries here because Anthropic splits text
	// around citations (out of scope for this change; handled in a later
	// spec). Concatenation is enough to catch dropped/duplicated tokens.
	require.Equal(t,
		strings.Join(original.textBlocks, ""),
		strings.Join(emitted.textBlocks, ""),
		"concatenated text across all text blocks should match")

	// --- ordering: server_tool_use and web_search_tool_result must appear
	// BEFORE the first text block, because the text narrates the tool result.
	// Emitting text first would deliver the answer before the tool call,
	// which breaks downstream clients that interpret SSE in arrival order.
	require.GreaterOrEqual(t, emitted.firstUseOrder, 0, "server_tool_use must appear in the emitted stream")
	require.GreaterOrEqual(t, emitted.firstResultOrder, 0, "web_search_tool_result must appear in the emitted stream")
	require.GreaterOrEqual(t, emitted.firstTextOrder, 0, "text content must appear in the emitted stream")
	require.Less(t, emitted.firstUseOrder, emitted.firstTextOrder,
		"server_tool_use must be emitted before assistant text")
	require.Less(t, emitted.firstResultOrder, emitted.firstTextOrder,
		"web_search_tool_result must be emitted before assistant text")

	// Wire-level check: the individual web_search_result entries inside the
	// tool_result content must carry their original fields (url, title,
	// encrypted_content, …). These are not modeled on MessageContentBlock,
	// so they must survive via the content byte-passthrough.
	var resultEventData []byte
	for _, ev := range emittedEvents {
		if !bytes.Contains(ev.Data, []byte(`"type":"web_search_tool_result"`)) {
			continue
		}
		if bytes.Contains(ev.Data, []byte(`"content_block_start"`)) {
			resultEventData = ev.Data
			break
		}
	}
	require.NotEmpty(t, resultEventData, "expected a web_search_tool_result content_block_start event")

	first := gjson.GetBytes(resultEventData, `content_block.content.0`)
	require.True(t, first.Exists(), "nested web_search_result entry should be present")
	require.Equal(t, "web_search_result", first.Get("type").String())
	require.NotEmpty(t, first.Get("url").String(), "url must survive the stream round-trip")
	require.NotEmpty(t, first.Get("title").String(), "title must survive the stream round-trip")
	require.NotEmpty(t, first.Get("encrypted_content").String(),
		"encrypted_content must survive the stream round-trip")
}

type streamSummary struct {
	toolUses         []toolUseInfo
	toolResults      []toolResultInfo
	textBlocks       []string
	firstUseOrder    int
	firstResultOrder int
	firstTextOrder   int
}

type toolUseInfo struct {
	blockType string
	id        string
	name      string
	input     string
	caller    json.RawMessage
}

type toolResultInfo struct {
	blockType string
	toolUseID string
	content   json.RawMessage
	caller    json.RawMessage
}

func summarizeStream(t *testing.T, events []*httpclient.StreamEvent) streamSummary {
	t.Helper()

	summary := streamSummary{
		firstUseOrder:    -1,
		firstResultOrder: -1,
		firstTextOrder:   -1,
	}

	var (
		currentBlockType = map[int64]string{}
		toolInputByIdx   = map[int64]*strings.Builder{}
		toolUseByIdx     = map[int64]*toolUseInfo{}
		textByIdx        = map[int64]*strings.Builder{}
		textIndexOrder   []int64
		nextArrivalOrder int
	)

	for _, ev := range events {
		var se StreamEvent
		if err := json.Unmarshal(ev.Data, &se); err != nil {
			continue
		}

		switch se.Type {
		case "content_block_start":
			if se.ContentBlock == nil || se.Index == nil {
				continue
			}
			idx := *se.Index
			cb := se.ContentBlock
			currentBlockType[idx] = cb.Type

			arrivalOrder := nextArrivalOrder
			nextArrivalOrder++

			switch {
			case isAnthropicToolUseLike(cb.Type):
				if summary.firstUseOrder < 0 {
					summary.firstUseOrder = arrivalOrder
				}
				name := ""
				if cb.Name != nil {
					name = *cb.Name
				}
				info := &toolUseInfo{
					blockType: cb.Type,
					id:        cb.ID,
					name:      name,
					caller:    cb.Caller,
				}
				toolUseByIdx[idx] = info
				toolInputByIdx[idx] = &strings.Builder{}
				// Anthropic typically ships empty {} input in start; defer to deltas.
				if len(cb.Input) > 0 && string(cb.Input) != "{}" {
					toolInputByIdx[idx].Write(cb.Input)
				}
			case isAnthropicToolResultLike(cb.Type):
				if summary.firstResultOrder < 0 {
					summary.firstResultOrder = arrivalOrder
				}
				tri := toolResultInfo{blockType: cb.Type, caller: cb.Caller}
				if cb.ToolUseID != nil {
					tri.toolUseID = *cb.ToolUseID
				}
				if cb.Content != nil {
					raw, _ := json.Marshal(cb.Content)
					tri.content = raw
				}
				summary.toolResults = append(summary.toolResults, tri)
			case cb.Type == "text":
				if summary.firstTextOrder < 0 {
					summary.firstTextOrder = arrivalOrder
				}
				textByIdx[idx] = &strings.Builder{}
				textIndexOrder = append(textIndexOrder, idx)
			}

		case "content_block_delta":
			if se.Delta == nil || se.Index == nil || se.Delta.Type == nil {
				continue
			}
			idx := *se.Index
			switch *se.Delta.Type {
			case "input_json_delta":
				if b, ok := toolInputByIdx[idx]; ok && se.Delta.PartialJSON != nil {
					b.WriteString(*se.Delta.PartialJSON)
				}
			case "text_delta":
				if b, ok := textByIdx[idx]; ok && se.Delta.Text != nil {
					b.WriteString(*se.Delta.Text)
				}
			}
		}
	}

	// Finalize tool uses in index order for stable comparison.
	indices := make([]int64, 0, len(toolUseByIdx))
	for i := range toolUseByIdx {
		indices = append(indices, i)
	}
	// simple ascending sort
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[j] < indices[i] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}
	for _, idx := range indices {
		info := toolUseByIdx[idx]
		if b := toolInputByIdx[idx]; b != nil {
			if b.Len() == 0 {
				info.input = "{}"
			} else {
				info.input = b.String()
			}
		} else {
			info.input = "{}"
		}
		summary.toolUses = append(summary.toolUses, *info)
	}

	for _, idx := range textIndexOrder {
		if b := textByIdx[idx]; b != nil {
			summary.textBlocks = append(summary.textBlocks, b.String())
		}
	}

	// Sanity: surface the tally when debugging.
	_ = fmt.Sprintf("tool_uses=%d tool_results=%d text_blocks=%d",
		len(summary.toolUses), len(summary.toolResults), len(summary.textBlocks))

	return summary
}
