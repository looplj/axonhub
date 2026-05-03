package responses

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/looplj/axonhub/llm"
)

const (
	inputKindString = "string"
	inputKindArray  = "array"
)

var safeTopLevelExtraKeys = map[string]struct{}{
	"client_metadata": {},
	"debug":           {},
	"debug_options":   {},
	"trace_id":        {},
	"request_id":      {},
}

func attachOpenAIResponsesRequestExtensions(chatReq *llm.Request, req *Request, rawBody []byte) {
	if chatReq == nil || req == nil {
		return
	}

	providerExt := llm.EnsureOpenAIResponsesProviderExtensions(chatReq)
	if providerExt == nil {
		return
	}

	requestExt := &llm.OpenAIResponsesRequestExtensions{
		RawBody:         cloneRaw(rawBody),
		TopLevelExtra:   map[string]json.RawMessage{},
		MetadataRaw:     cloneRaw(req.MetadataRaw),
		MetadataExtra:   metadataExtra(req.MetadataRaw),
		InputKind:       req.Input.Kind,
		InputRaw:        cloneRaw(req.Input.Raw),
		InstructionsRaw: rawField(req.Raw, "instructions"),
		ToolChoiceRaw:   toolChoiceRaw(req.ToolChoice),
		InputItems:      buildInputRawItems(req.Input),
		Tools:           buildToolRawItems(req.Tools),
	}

	if requestExt.InputKind == "" {
		if req.Input.Text != nil {
			requestExt.InputKind = inputKindString
		} else if req.Input.Items != nil {
			requestExt.InputKind = inputKindArray
		}
	}

	for key, value := range req.Extra {
		if isSafeTopLevelExtra(key) {
			requestExt.TopLevelExtra[key] = cloneRaw(value)
			continue
		}
		if requestExt.TopLevelSemanticExtra == nil {
			requestExt.TopLevelSemanticExtra = map[string]json.RawMessage{}
		}
		requestExt.TopLevelSemanticExtra[key] = cloneRaw(value)
	}

	if len(requestExt.TopLevelExtra) == 0 {
		requestExt.TopLevelExtra = nil
	}

	requestExt.ProtectableFragments = buildProtectableFragments(requestExt.InputItems)
	providerExt.Request = requestExt
}

func isSafeTopLevelExtra(key string) bool {
	_, ok := safeTopLevelExtraKeys[key]

	return ok
}

func toolChoiceRaw(choice *ToolChoice) json.RawMessage {
	if choice == nil {
		return nil
	}

	return cloneRaw(choice.Raw)
}

func buildInputRawItems(input Input) []llm.OpenAIResponsesRawItem {
	if len(input.Items) == 0 {
		return nil
	}

	items := make([]llm.OpenAIResponsesRawItem, 0, len(input.Items))
	consumedSpans := consumedSpansByInputIndex(input.Items)

	for i := range input.Items {
		item := input.Items[i]
		path := fmt.Sprintf("input[%d]", i)
		rawItem := llm.OpenAIResponsesRawItem{
			Type:          item.Type,
			ID:            item.ID,
			OriginalIndex: ptrInt(i),
			Path:          path,
			SemanticKey:   inputItemSemanticKey(item, i),
			CallID:        item.CallID,
			Raw:           cloneRaw(item.Raw),
			Extra:         cloneRawMap(item.Extra),
			ConsumedSpan:  consumedSpans[i],
		}

		if isRawOnlyInputItemType(item.Type) {
			scope := rawOnlyInputScope(item.Type)
			textPaths := extractProtectableTextPaths(item.Raw, path)
			rawItem.Protection = llm.OpenAIResponsesRawProtection{
				Status:        llm.OpenAIResponsesProtectionNotSupported,
				Scanned:       false,
				TextExtracted: len(textPaths) > 0,
				ReplayAllowed: false,
				Scope:         scope,
				TextPaths:     textPaths,
			}
		}

		items = append(items, rawItem)
	}

	return items
}

func consumedSpansByInputIndex(items []Item) map[int]*llm.OpenAIResponsesConsumedSpan {
	spans := make(map[int]*llm.OpenAIResponsesConsumedSpan)

	for i := 0; i < len(items); i++ {
		if items[i].Type != "reasoning" {
			continue
		}

		_, consumed, err := convertReasoningWithFollowing(items, i)
		if err != nil || consumed <= 1 {
			continue
		}

		span := &llm.OpenAIResponsesConsumedSpan{Start: i, End: i + consumed}
		for j := i; j < i+consumed && j < len(items); j++ {
			spans[j] = span
		}
		i += consumed - 1
	}

	return spans
}

func inputItemSemanticKey(item Item, index int) string {
	if isRawOnlyInputItemType(item.Type) {
		return ""
	}

	switch item.Type {
	case "message", "input_text", "", "input_image", "function_call", "custom_tool_call",
		"function_call_output", "custom_tool_call_output", "reasoning", "compaction", "compaction_summary":
		return fmt.Sprintf("input:%d:%s", index, item.Type)
	default:
		return ""
	}
}

func isRawOnlyInputItemType(itemType string) bool {
	switch itemType {
	case "shell_call_output", "tool_search_output", "mcp_call", "mcp_tool_call_output", "local_shell_call_output":
		return true
	}

	return strings.HasSuffix(itemType, "_tool_call_output") &&
		itemType != "function_call_output" &&
		itemType != "custom_tool_call_output"
}

func rawOnlyInputScope(itemType string) string {
	switch itemType {
	case "mcp_call", "mcp_tool_call_output", "shell_call_output", "local_shell_call_output", "tool_search_output":
		return "tool"
	default:
		return "tool"
	}
}

func buildToolRawItems(tools []Tool) []llm.OpenAIResponsesRawItem {
	if len(tools) == 0 {
		return nil
	}

	items := make([]llm.OpenAIResponsesRawItem, 0, len(tools))
	for i := range tools {
		tool := tools[i]
		items = append(items, llm.OpenAIResponsesRawItem{
			Type:          tool.Type,
			ID:            tool.Name,
			OriginalIndex: ptrInt(i),
			Path:          fmt.Sprintf("tools[%d]", i),
			SemanticKey:   toolSemanticKey(tool, i),
			Raw:           cloneRaw(tool.Raw),
			Extra:         cloneRawMap(tool.Extra),
		})
	}

	return items
}

func toolSemanticKey(tool Tool, index int) string {
	switch tool.Type {
	case "function", "image_generation", "custom":
		return fmt.Sprintf("tool:%d:%s:%s", index, tool.Type, tool.Name)
	default:
		return ""
	}
}

func buildProtectableFragments(items []llm.OpenAIResponsesRawItem) []llm.OpenAIResponsesProtectableFragment {
	var fragments []llm.OpenAIResponsesProtectableFragment
	for _, item := range items {
		if len(item.Protection.TextPaths) == 0 {
			continue
		}

		textByPath := extractProtectableTexts(item.Raw, item.Path)
		for _, textPath := range item.Protection.TextPaths {
			text := textByPath[textPath]
			if text == "" {
				continue
			}

			fragments = append(fragments, llm.OpenAIResponsesProtectableFragment{
				Path:        textPath,
				Scope:       item.Protection.Scope,
				Text:        text,
				RewriteMode: "drop_on_change",
			})
		}
	}

	return fragments
}

func extractProtectableTextPaths(raw json.RawMessage, basePath string) []string {
	textByPath := extractProtectableTexts(raw, basePath)
	if len(textByPath) == 0 {
		return nil
	}

	paths := make([]string, 0, len(textByPath))
	for path := range textByPath {
		paths = append(paths, path)
	}
	slices.Sort(paths)

	return paths
}

func extractProtectableTexts(raw json.RawMessage, basePath string) map[string]string {
	if len(raw) == 0 {
		return nil
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}

	result := map[string]string{}
	if obj, ok := value.(map[string]any); ok {
		for _, key := range []string{"output", "content", "text", "input", "result", "payload"} {
			if child, ok := obj[key]; ok {
				collectStringLeaves(child, basePath+"."+key, result)
			}
		}
	}

	if len(result) == 0 {
		collectStringLeaves(value, basePath, result)
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func collectStringLeaves(value any, path string, result map[string]string) {
	switch typed := value.(type) {
	case string:
		if typed != "" && isProtectableLeafPath(path) {
			result[path] = typed
		}
	case []any:
		for i, item := range typed {
			collectStringLeaves(item, fmt.Sprintf("%s[%d]", path, i), result)
		}
	case map[string]any:
		for key, item := range typed {
			if isUnprotectableMetadataKey(key) {
				continue
			}
			collectStringLeaves(item, path+"."+key, result)
		}
	}
}

func isProtectableLeafPath(path string) bool {
	return strings.Contains(path, ".output") ||
		strings.Contains(path, ".content") ||
		strings.Contains(path, ".text") ||
		strings.Contains(path, ".input") ||
		strings.Contains(path, ".result") ||
		strings.Contains(path, ".payload")
}

func isUnprotectableMetadataKey(key string) bool {
	switch key {
	case "id", "type", "call_id", "name", "status":
		return true
	default:
		return false
	}
}

func ptrInt(value int) *int {
	return &value
}
