package anthropic

import (
	"encoding/json"
	"sort"
	"strings"
)

// TransformerMetadata keys used to round-trip Anthropic server-side tool blocks
// through the unified llm.Request/llm.Response shape. Plain tool_use /
// tool_result blocks are NOT tagged — absence of TransformerMetadataKeyAnthropicType
// means a vanilla function-call pair that can be handled by the existing code
// paths.
const (
	// TransformerMetadataKeyAnthropicType stores the original Anthropic block
	// type (e.g. "server_tool_use", "web_search_tool_result") for special
	// (non-vanilla) tool blocks.
	TransformerMetadataKeyAnthropicType = "anthropic_type"

	// TransformerMetadataKeyAnthropicCaller stores the optional Anthropic
	// `caller` object as json.RawMessage so it round-trips without the proxy
	// needing to know the full shape
	// (direct / code_execution_20250825 / code_execution_20260120 / ...).
	TransformerMetadataKeyAnthropicCaller = "anthropic_caller"

	// TransformerMetadataKeyAnthropicToolResultContent stores the original
	// *_tool_result content object as json.RawMessage so it round-trips
	// byte-identical.
	TransformerMetadataKeyAnthropicToolResultContent = "anthropic_tool_result_content"

	// TransformerMetadataKeyAnthropicBlockIndex stores the ordinal position
	// (int) of a content block inside the original Anthropic assistant turn.
	// Used to restore interleaving (text / tool_use / tool_result) on the
	// non-streaming path, where llm.Message otherwise flattens order.
	TransformerMetadataKeyAnthropicBlockIndex = "anthropic_block_index"
)

// isAnthropicSpecialToolUseBlock reports whether a content block type is an
// Anthropic server-side tool invocation block (e.g. "server_tool_use",
// "mcp_tool_use"). Plain "tool_use" returns false — it is a vanilla function
// call handled by the existing code paths without metadata tagging.
func isAnthropicSpecialToolUseBlock(blockType string) bool {
	return blockType != "tool_use" && strings.HasSuffix(blockType, "_tool_use")
}

// isAnthropicSpecialToolResultBlock reports whether a content block type is an
// Anthropic server-side tool result block (e.g. "web_search_tool_result",
// "code_execution_tool_result", "mcp_tool_result"). Plain "tool_result"
// returns false.
func isAnthropicSpecialToolResultBlock(blockType string) bool {
	return blockType != "tool_result" && strings.HasSuffix(blockType, "_tool_result")
}

// isAnthropicToolUseLike is true for any *_tool_use, including plain tool_use.
func isAnthropicToolUseLike(blockType string) bool {
	return blockType == "tool_use" || strings.HasSuffix(blockType, "_tool_use")
}

// isAnthropicToolResultLike is true for any *_tool_result, including plain tool_result.
func isAnthropicToolResultLike(blockType string) bool {
	return blockType == "tool_result" || strings.HasSuffix(blockType, "_tool_result")
}

func ensureMetaMap(m *map[string]any) {
	if *m == nil {
		*m = make(map[string]any)
	}
}

// setAnthropicSpecialMeta writes anthropic_type (+ optional anthropic_caller)
// into dst when blockType identifies a special (non-vanilla) tool block. It is
// a no-op for plain "tool_use" / "tool_result".
func setAnthropicSpecialMeta(dst *map[string]any, blockType string, caller json.RawMessage) {
	if !isAnthropicSpecialToolUseBlock(blockType) && !isAnthropicSpecialToolResultBlock(blockType) {
		return
	}

	ensureMetaMap(dst)
	(*dst)[TransformerMetadataKeyAnthropicType] = blockType

	if len(caller) > 0 {
		(*dst)[TransformerMetadataKeyAnthropicCaller] = caller
	}
}

// setAnthropicToolResultContent stores the raw JSON bytes of a *_tool_result
// content object so it can be emitted back byte-identical.
func setAnthropicToolResultContent(dst *map[string]any, content json.RawMessage) {
	if len(content) == 0 {
		return
	}

	ensureMetaMap(dst)
	(*dst)[TransformerMetadataKeyAnthropicToolResultContent] = content
}

func getAnthropicType(src map[string]any) string {
	v, _ := src[TransformerMetadataKeyAnthropicType].(string)
	return v
}

func asJSONRawMessage(v any) json.RawMessage {
	switch raw := v.(type) {
	case nil:
		return nil
	case json.RawMessage:
		return raw
	case []byte:
		return raw
	case string:
		return json.RawMessage(raw)
	default:
		b, err := json.Marshal(raw)
		if err != nil {
			return nil
		}

		return b
	}
}

func getAnthropicCaller(src map[string]any) json.RawMessage {
	return asJSONRawMessage(src[TransformerMetadataKeyAnthropicCaller])
}

func getAnthropicToolResultContent(src map[string]any) json.RawMessage {
	return asJSONRawMessage(src[TransformerMetadataKeyAnthropicToolResultContent])
}

// orderedContentBlock pairs a MessageContentBlock with its original Anthropic
// block index and an insertion-order tiebreaker. sortOrderedContentBlocks
// stably sorts a slice of these so blocks with a known block index lead
// (ascending) and blocks without a known index trail in natural order.
type orderedContentBlock struct {
	idx   int
	order int
	block MessageContentBlock
}

func sortOrderedContentBlocks(blocks []orderedContentBlock) []orderedContentBlock {
	sort.SliceStable(blocks, func(i, j int) bool {
		a, b := blocks[i], blocks[j]

		aKnown := a.idx >= 0
		bKnown := b.idx >= 0

		if aKnown && bKnown {
			if a.idx != b.idx {
				return a.idx < b.idx
			}

			return a.order < b.order
		}

		if aKnown != bKnown {
			return aKnown
		}

		return a.order < b.order
	})

	return blocks
}

// setAnthropicBlockIndex records a content block's original ordinal position so
// it can be restored during round-trip emission.
func setAnthropicBlockIndex(dst *map[string]any, idx int) {
	ensureMetaMap(dst)
	(*dst)[TransformerMetadataKeyAnthropicBlockIndex] = idx
}

// getAnthropicBlockIndex returns the original content-block ordinal position,
// or -1 when absent.
func getAnthropicBlockIndex(src map[string]any) int {
	v, ok := src[TransformerMetadataKeyAnthropicBlockIndex]
	if !ok {
		return -1
	}

	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return -1
	}
}
