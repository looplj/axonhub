package responses

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

func protocolExtensionsFromRequest(req *Request) *llm.ProtocolExtensions {
	if req == nil {
		return nil
	}

	ext := &llm.OpenAIResponsesExtensions{
		RequestExtra: cloneRawMap(req.Extra),
	}
	if shouldPreserveInput(req.Input) {
		ext.InputItems = rawItemsFromInput(req.Input)
	}
	if shouldPreserveTools(req.Tools) {
		ext.Tools = rawToolsFromTools(req.Tools)
	}
	if shouldPreserveToolChoice(req.ToolChoice) {
		ext.ToolChoice = rawToolChoiceFromToolChoice(req.ToolChoice)
	}

	if len(ext.RequestExtra) == 0 && len(ext.InputItems) == 0 && len(ext.Tools) == 0 && len(ext.ToolChoice) == 0 {
		return nil
	}

	return &llm.ProtocolExtensions{OpenAIResponses: ext}
}

func cloneRawMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]json.RawMessage, len(src))
	for key, value := range src {
		dst[key] = cloneRaw(value)
	}
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}

	return dst
}

func rawItemsFromInput(input Input) []llm.OpenAIResponsesRawItem {
	if len(input.Items) == 0 {
		return nil
	}

	items := make([]llm.OpenAIResponsesRawItem, 0, len(input.Items))
	for _, item := range input.Items {
		items = append(items, rawItemFromItem(item))
	}
	return items
}

func rawToolsFromTools(tools []Tool) []llm.OpenAIResponsesRawItem {
	if len(tools) == 0 {
		return nil
	}

	items := make([]llm.OpenAIResponsesRawItem, 0, len(tools))
	for _, tool := range tools {
		raw := tool.Raw
		if len(raw) == 0 {
			raw, _ = json.Marshal(tool)
		}
		items = append(items, llm.OpenAIResponsesRawItem{
			Type: tool.Type,
			Raw:  cloneRaw(raw),
		})
	}
	return items
}

func rawToolChoiceFromToolChoice(toolChoice *ToolChoice) json.RawMessage {
	if toolChoice == nil {
		return nil
	}
	if len(toolChoice.Raw) > 0 {
		return cloneRaw(toolChoice.Raw)
	}

	raw, err := json.Marshal(toolChoice)
	if err != nil {
		return nil
	}
	return cloneRaw(raw)
}

func rawItemFromItem(item Item) llm.OpenAIResponsesRawItem {
	raw := item.Raw
	if len(raw) == 0 {
		raw, _ = json.Marshal(item)
	}
	return llm.OpenAIResponsesRawItem{
		Type: item.Type,
		ID:   item.ID,
		Raw:  cloneRaw(raw),
	}
}

func protocolExtensionsForItem(item *Item) *llm.ProtocolExtensions {
	if item == nil {
		return nil
	}
	if !shouldPreserveItem(*item) {
		return nil
	}
	rawItem := rawItemFromItem(*item)
	if len(rawItem.Raw) == 0 {
		return nil
	}
	return &llm.ProtocolExtensions{
		OpenAIResponses: &llm.OpenAIResponsesExtensions{
			InputItems: []llm.OpenAIResponsesRawItem{rawItem},
		},
	}
}

func protocolExtensionsForTool(tool Tool) *llm.ProtocolExtensions {
	rawTool := rawToolsFromTools([]Tool{tool})
	if len(rawTool) == 0 {
		return nil
	}
	return &llm.ProtocolExtensions{
		OpenAIResponses: &llm.OpenAIResponsesExtensions{
			Tools: rawTool,
		},
	}
}

func protocolExtensionsForOutput(output []Item) *llm.ProtocolExtensions {
	if len(output) == 0 {
		return nil
	}
	if !shouldPreserveOutput(output) {
		return nil
	}

	items := make([]llm.OpenAIResponsesRawItem, 0, len(output))
	for _, item := range output {
		items = append(items, rawItemFromItem(item))
	}
	return &llm.ProtocolExtensions{
		OpenAIResponses: &llm.OpenAIResponsesExtensions{
			OutputItems: items,
		},
	}
}

func protocolExtensionsForResponse(resp *Response) *llm.ProtocolExtensions {
	if resp == nil {
		return nil
	}

	// Store response-level fields that would otherwise disappear in the semantic llm.Response view.
	ext := &llm.OpenAIResponsesExtensions{
		ResponseRaw:      cloneRaw(resp.Raw),
		ResponseExtra:    cloneRawMap(resp.Extra),
		ResponseMetadata: cloneStringMap(resp.Metadata),
	}

	if shouldPreserveOutput(resp.Output) {
		ext.OutputItems = rawItemsFromInput(Input{Items: resp.Output})
	}

	if len(ext.ResponseRaw) == 0 &&
		len(ext.ResponseExtra) == 0 &&
		len(ext.ResponseMetadata) == 0 &&
		len(ext.OutputItems) == 0 {
		return nil
	}

	return &llm.ProtocolExtensions{OpenAIResponses: ext}
}

func shouldPreserveInput(input Input) bool {
	if len(input.Items) == 0 {
		return false
	}
	for _, item := range input.Items {
		if shouldPreserveItem(item) {
			return true
		}
	}
	return false
}

func shouldPreserveOutput(output []Item) bool {
	for _, item := range output {
		if shouldPreserveItem(item) {
			return true
		}
	}
	return false
}

func shouldPreserveTools(tools []Tool) bool {
	for _, tool := range tools {
		if len(tool.Extra) > 0 {
			return true
		}
		switch tool.Type {
		case "function", "image_generation", "custom":
			continue
		default:
			return true
		}
	}
	return false
}

func shouldPreserveToolChoice(toolChoice *ToolChoice) bool {
	if toolChoice == nil {
		return false
	}
	if len(toolChoice.Tools) > 0 {
		return true
	}
	return len(toolChoice.Extra) > 0
}

func shouldPreserveItem(item Item) bool {
	if len(item.Extra) > 0 {
		return true
	}
	if len(item.Annotations) > 0 {
		return true
	}
	if item.Content != nil && shouldPreserveInput(*item.Content) {
		return true
	}
	if item.Output != nil && shouldPreserveInput(*item.Output) {
		return true
	}
	switch item.Type {
	case "message", "input_text", "input_image", "output_text", "function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output", "reasoning", "image_generation_call", "compaction", "compaction_summary", "":
		return false
	default:
		return true
	}
}

func rawEventProtocolExtensions(ev *StreamEvent) *llm.ProtocolExtensions {
	return rawEventProtocolExtensionsFromRaw(ev, nil)
}

func rawEventProtocolExtensionsFromRaw(ev *StreamEvent, rawData []byte) *llm.ProtocolExtensions {
	if ev == nil {
		return nil
	}
	raw := json.RawMessage(rawData)
	if len(raw) == 0 {
		raw, _ = json.Marshal(ev)
	}
	if len(raw) == 0 {
		return nil
	}

	seq := ev.SequenceNumber
	return &llm.ProtocolExtensions{
		OpenAIResponses: &llm.OpenAIResponsesExtensions{
			RawEvent: &llm.OpenAIResponsesRawEvent{
				Type:           string(ev.Type),
				SequenceNumber: &seq,
				Raw:            cloneRaw(raw),
			},
		},
	}
}

func itemResultJSON(result any) string {
	if result == nil {
		return ""
	}
	if s := itemResultString(result); s != "" {
		return s
	}
	data, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	return string(data)
}

func openAIResponsesExtensions(ext *llm.ProtocolExtensions) *llm.OpenAIResponsesExtensions {
	if ext == nil {
		return nil
	}
	return ext.OpenAIResponses
}

func toolsFromRawItems(rawItems []llm.OpenAIResponsesRawItem) []Tool {
	if len(rawItems) == 0 {
		return nil
	}

	tools := make([]Tool, 0, len(rawItems))
	for _, rawItem := range rawItems {
		if len(rawItem.Raw) == 0 {
			continue
		}
		var tool Tool
		if err := json.Unmarshal(rawItem.Raw, &tool); err != nil {
			continue
		}
		// Preserve the exact JSON so a later round trip can keep provider-private fields.
		tool.Raw = cloneRaw(rawItem.Raw)
		tools = append(tools, tool)
	}
	return tools
}

func toolChoiceFromRaw(raw json.RawMessage) *ToolChoice {
	if len(raw) == 0 {
		return nil
	}

	var toolChoice ToolChoice
	if err := json.Unmarshal(raw, &toolChoice); err != nil {
		return nil
	}
	toolChoice.Raw = cloneRaw(raw)
	toolChoice.PreferRaw = true
	return &toolChoice
}

func inputFromRawItems(rawItems []llm.OpenAIResponsesRawItem) Input {
	if len(rawItems) == 0 {
		return Input{}
	}

	items := make([]Item, 0, len(rawItems))
	for _, rawItem := range rawItems {
		if len(rawItem.Raw) == 0 {
			continue
		}
		var item Item
		if err := json.Unmarshal(rawItem.Raw, &item); err != nil {
			continue
		}
		// Keep Raw attached after decoding so terminal/event preservation can replay the original shape.
		item.Raw = cloneRaw(rawItem.Raw)
		items = append(items, item)
	}
	if len(items) == 0 {
		return Input{}
	}

	return Input{Items: items}
}

func outputFromRawItems(rawItems []llm.OpenAIResponsesRawItem) []Item {
	return inputFromRawItems(rawItems).Items
}

func rawEventFromResponse(resp *llm.Response) *llm.OpenAIResponsesRawEvent {
	if resp == nil || resp.ProtocolExtensions == nil || resp.ProtocolExtensions.OpenAIResponses == nil {
		return nil
	}
	rawEvent := resp.ProtocolExtensions.OpenAIResponses.RawEvent
	if rawEvent == nil || len(rawEvent.Raw) == 0 {
		return nil
	}
	return rawEvent
}

func applyResponseExtensions(resp *Response, ext *llm.OpenAIResponsesExtensions) {
	if resp == nil || ext == nil {
		return
	}

	resp.Raw = cloneRaw(ext.ResponseRaw)
	resp.Extra = cloneRawMap(ext.ResponseExtra)
	if len(ext.ResponseMetadata) > 0 && len(resp.Metadata) == 0 {
		// Semantic metadata wins when it was already rebuilt from current response state.
		resp.Metadata = cloneStringMap(ext.ResponseMetadata)
	}
}
