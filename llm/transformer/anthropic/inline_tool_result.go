package anthropic

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
)

// inlineToolResultFromBlock converts a *_tool_result MessageContentBlock to
// llm.InlineToolResult, preserving the Anthropic-specific type, optional
// caller, and the original content bytes via TransformerMetadata.
func inlineToolResultFromBlock(block *MessageContentBlock) llm.InlineToolResult {
	if block == nil {
		return llm.InlineToolResult{}
	}

	ir := llm.InlineToolResult{}
	if block.ToolUseID != nil {
		ir.ToolCallID = *block.ToolUseID
	}

	rawContent := marshalToolResultContent(block.Content)
	if len(rawContent) > 0 {
		ir.Output = string(rawContent)
	}

	if block.IsError != nil && *block.IsError {
		ir.IsError = true
	} else if hasErrorTypeSuffix(rawContent) {
		ir.IsError = true
	}

	setAnthropicSpecialMeta(&ir.TransformerMetadata, block.Type, block.Caller)
	setAnthropicToolResultContent(&ir.TransformerMetadata, rawContent)

	return ir
}

// marshalToolResultContent returns the JSON bytes of the tool-result content,
// preserving the original shape (array, object, string).
func marshalToolResultContent(content *MessageContent) json.RawMessage {
	if content == nil {
		return nil
	}

	data, err := json.Marshal(content)
	if err != nil {
		return nil
	}

	return data
}

// hasErrorTypeSuffix reports whether content's top-level `type` field ends
// with "_error" (e.g. "web_search_tool_result_error"). Works on both object
// and array roots.
func hasErrorTypeSuffix(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}

	result := gjson.GetBytes(raw, "type")
	if !result.Exists() {
		return false
	}

	return strings.HasSuffix(result.String(), "_error")
}
