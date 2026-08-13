package responses

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
)

func attachOpenAIResponsesRequestExtensions(chatReq *llm.Request, req *Request, rawBody []byte) {
	if chatReq == nil || req == nil {
		return
	}

	raw := parseRawRequestFragments(rawBody)
	reasoningContext := ""
	if req.Reasoning != nil {
		reasoningContext = req.Reasoning.Context
	}
	requestExt := &llm.OpenAIResponsesRequestExtensions{
		ReasoningContext: reasoningContext,
		RawTools:         buildRawOnlyToolFragments(req.Tools, raw.Tools),
		ToolSignatures:   buildRepresentedToolSignatures(req.Tools),
		RawToolChoice:    rawUnsupportedToolChoice(req.ToolChoice, raw.ToolChoice),
		RawInputItems:    buildRawOnlyInputFragments(req.Input, raw.InputItems),
		RawInputMessages: replayMessageSignatures(chatReq.Messages),
		RawInputTools:    replayInputToolSignatures(chatReq.Tools),
	}

	if requestExt.ReasoningContext == "" && len(requestExt.RawTools) == 0 && len(requestExt.RawToolChoice) == 0 && len(requestExt.RawInputItems) == 0 {
		return
	}

	ext := llm.EnsureOpenAIResponsesProviderExtensions(chatReq)
	if ext == nil {
		return
	}
	ext.Request = requestExt
}

type rawRequestFragments struct {
	Tools      []json.RawMessage
	ToolChoice json.RawMessage
	InputItems []json.RawMessage
}

func parseRawRequestFragments(rawBody []byte) rawRequestFragments {
	if len(rawBody) == 0 {
		return rawRequestFragments{}
	}

	var raw struct {
		Tools      []json.RawMessage `json:"tools"`
		ToolChoice json.RawMessage   `json:"tool_choice"`
		Input      json.RawMessage   `json:"input"`
	}
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return rawRequestFragments{}
	}

	var inputItems []json.RawMessage
	if len(raw.Input) > 0 && json.Unmarshal(raw.Input, &inputItems) != nil {
		inputItems = nil
	}

	return rawRequestFragments{
		Tools:      raw.Tools,
		ToolChoice: raw.ToolChoice,
		InputItems: inputItems,
	}
}

func buildRepresentedToolSignatures(tools []Tool) []string {
	if len(tools) == 0 {
		return nil
	}

	signatures := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool.Type == "namespace" {
			for _, subTool := range tool.Tools {
				if subTool.Type == "function" {
					signatures = append(signatures, responseToolSignature(Tool{
						Type: "function",
						Name: namespaceFunctionName(tool.Name, subTool.Name),
					}))
				}
			}
			continue
		}
		if !isStructurallyRepresentedToolType(tool.Type) {
			continue
		}
		signatures = append(signatures, responseToolSignature(tool))
	}

	return signatures
}

func buildRawOnlyToolFragments(tools []Tool, rawTools []json.RawMessage) []llm.OpenAIResponsesRawFragment {
	if len(tools) == 0 {
		return nil
	}

	fragments := make([]llm.OpenAIResponsesRawFragment, 0, len(tools))
	for i := range tools {
		if i >= len(rawTools) || len(rawTools[i]) == 0 || (isStructurallyRepresentedToolType(tools[i].Type) && tools[i].Type != "tool_search") {
			continue
		}

		fragments = append(fragments, llm.OpenAIResponsesRawFragment{
			Type:                 tools[i].Type,
			Name:                 tools[i].Name,
			OriginalIndex:        i,
			RepresentedToolCount: representedRawToolCount(tools[i]),
			Raw:                  cloneRaw(rawTools[i]),
		})
	}

	return fragments
}

// representedRawToolCount counts tool declarations represented by one preserved raw item.
func representedRawToolCount(tool Tool) int {
	if tool.Type == "tool_search" {
		return 1
	}
	if tool.Type != "namespace" {
		return 0
	}

	count := 0
	for _, subTool := range tool.Tools {
		if subTool.Type == "function" {
			count++
		}
	}

	return count
}

func isStructurallyRepresentedToolType(toolType string) bool {
	switch toolType {
	case "function", "image_generation", "web_search", "custom", "tool_search":
		return true
	default:
		return false
	}
}

func responseToolSignature(tool Tool) string {
	switch tool.Type {
	case "function", "custom":
		return tool.Type + ":" + tool.Name
	default:
		return tool.Type
	}
}

func rawUnsupportedToolChoice(choice *ToolChoice, rawChoice json.RawMessage) json.RawMessage {
	if choice == nil || len(rawChoice) == 0 {
		return nil
	}

	var mode string
	if json.Unmarshal(rawChoice, &mode) == nil {
		return nil
	}

	var rawObject map[string]json.RawMessage
	if json.Unmarshal(rawChoice, &rawObject) != nil {
		return cloneRaw(rawChoice)
	}
	if rawToolChoiceObjectFullyRepresented(choice, rawObject) {
		return nil
	}

	return cloneRaw(rawChoice)
}

// rawToolChoiceObjectFullyRepresented reports whether the common ToolChoice IR
// can reproduce every selector field without relying on the raw extension.
func rawToolChoiceObjectFullyRepresented(choice *ToolChoice, rawObject map[string]json.RawMessage) bool {
	if choice == nil || rawObject == nil {
		return false
	}

	if choice.Type != nil && *choice.Type == "allowed_tools" {
		if !rawObjectHasOnlyFields(rawObject, "type", "mode", "tools") {
			return false
		}
		if _, ok := rawObject["type"]; !ok {
			return false
		}
		rawTools, ok := rawObject["tools"]
		if !ok {
			return false
		}
		var tools []map[string]json.RawMessage
		if json.Unmarshal(rawTools, &tools) != nil || tools == nil {
			return false
		}
		for _, tool := range tools {
			if !rawObjectHasOnlyFields(tool, "type", "name") {
				return false
			}
			if _, hasType := tool["type"]; !hasType {
				return false
			}
			if _, hasName := tool["name"]; !hasName {
				return false
			}
		}
		return true
	}

	if choice.Mode != nil && choice.Type == nil && choice.Name == nil && len(choice.Tools) == 0 {
		return rawObjectHasOnlyFields(rawObject, "mode")
	}

	if choice.Mode == nil && choice.Type != nil && choice.Name != nil && len(choice.Tools) == 0 {
		if !rawObjectHasOnlyFields(rawObject, "type", "name") {
			return false
		}
		_, hasType := rawObject["type"]
		_, hasName := rawObject["name"]
		return hasType && hasName
	}

	return false
}

func rawObjectHasOnlyFields(object map[string]json.RawMessage, allowed ...string) bool {
	if object == nil {
		return false
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedSet[field] = struct{}{}
	}
	for field := range object {
		if _, ok := allowedSet[field]; !ok {
			return false
		}
	}
	return true
}

func buildRawOnlyInputFragments(input Input, rawItems []json.RawMessage) []llm.OpenAIResponsesRawFragment {
	if len(input.Items) == 0 {
		return nil
	}

	fragments := make([]llm.OpenAIResponsesRawFragment, 0)
	for i := range input.Items {
		item := input.Items[i]
		preserveRepresented := item.Type == "tool_search_call" || item.Type == "tool_search_output" || item.Type == "agent_message"
		if i >= len(rawItems) || len(rawItems[i]) == 0 || (isStructurallyRepresentedInputItem(item.Type) && !preserveRepresented) {
			continue
		}

		fragments = append(fragments, llm.OpenAIResponsesRawFragment{
			Type:                 item.Type,
			Name:                 item.Name,
			CallID:               item.CallID,
			OriginalIndex:        i,
			RepresentedToolCount: lo.Ternary(preserveRepresented, 1, 0),
			Raw:                  cloneRaw(rawItems[i]),
		})
	}

	return fragments
}

func isStructurallyRepresentedInputItem(itemType string) bool {
	switch itemType {
	case "", "message", "input_text", "input_image", "function_call", "function_call_output",
		"custom_tool_call", "custom_tool_call_output", "tool_search_call", "tool_search_output",
		"reasoning", "compaction", "compaction_summary", "agent_message":
		return true
	default:
		return false
	}
}

func openAIResponsesRequestExtensions(llmReq *llm.Request) *llm.OpenAIResponsesRequestExtensions {
	if llmReq == nil || llmReq.ProviderExtensions == nil || llmReq.ProviderExtensions.OpenAIResponses == nil {
		return nil
	}
	requestExt := llmReq.ProviderExtensions.OpenAIResponses.Request

	return requestExt
}

func marshalRequestPayload(payload Request, llmReq *llm.Request) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	requestExt := openAIResponsesRequestExtensions(llmReq)
	if requestExt == nil {
		return body, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}

	if tools, ok := mergeRawOnlyTools(obj["tools"], requestExt, llmReq.Messages, llmReq.Tools); ok {
		toolsRaw, err := json.Marshal(tools)
		if err != nil {
			return nil, err
		}
		obj["tools"] = toolsRaw
	}

	if len(requestExt.RawToolChoice) > 0 && rawToolChoiceMatchesCurrentTools(requestExt.RawToolChoice, payload.ToolChoice) {
		obj["tool_choice"] = cloneRaw(requestExt.RawToolChoice)
	}

	if input, ok := mergeRawOnlyInputItems(obj["input"], requestExt, llmReq.Messages, llmReq.Tools); ok {
		inputRaw, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		obj["input"] = inputRaw
	}

	return json.Marshal(obj)
}

func mergeRawOnlyInputItems(
	structuredRaw json.RawMessage,
	requestExt *llm.OpenAIResponsesRequestExtensions,
	currentMessages []llm.Message,
	currentTools []llm.Tool,
) ([]json.RawMessage, bool) {
	if requestExt == nil || len(requestExt.RawInputItems) == 0 {
		return nil, false
	}
	if !rawInputReplayMatchesCurrent(requestExt, currentMessages, currentTools) {
		return nil, false
	}
	rawFragments := requestExt.RawInputItems

	var structuredItems []json.RawMessage
	if len(structuredRaw) > 0 {
		if err := json.Unmarshal(structuredRaw, &structuredItems); err != nil {
			return nil, false
		}
	}

	representedCount := 0
	for _, fragment := range rawFragments {
		representedCount += fragment.RepresentedToolCount
	}
	if representedCount > len(structuredItems) {
		return nil, false
	}
	total := len(structuredItems) - representedCount + len(rawFragments)
	items := make([]json.RawMessage, 0, total)
	structuredIndex := 0
	rawByIndex := make(map[int]llm.OpenAIResponsesRawFragment, len(rawFragments))
	for _, fragment := range rawFragments {
		if len(fragment.Raw) == 0 || fragment.OriginalIndex < 0 {
			return nil, false
		}
		rawByIndex[fragment.OriginalIndex] = fragment
	}

	for i := 0; i < total; i++ {
		if fragment, ok := rawByIndex[i]; ok {
			items = append(items, cloneRaw(fragment.Raw))
			structuredIndex += fragment.RepresentedToolCount
			continue
		}
		if structuredIndex >= len(structuredItems) {
			return nil, false
		}
		items = append(items, cloneRaw(structuredItems[structuredIndex]))
		structuredIndex++
	}

	if structuredIndex != len(structuredItems) {
		return nil, false
	}

	return items, true
}

func mergeRawOnlyTools(
	structuredRaw json.RawMessage,
	requestExt *llm.OpenAIResponsesRequestExtensions,
	currentMessages []llm.Message,
	currentTools []llm.Tool,
) ([]json.RawMessage, bool) {
	if requestExt == nil || len(requestExt.RawTools) == 0 {
		return nil, false
	}

	var structuredTools []json.RawMessage
	if len(structuredRaw) > 0 {
		if err := json.Unmarshal(structuredRaw, &structuredTools); err != nil {
			return nil, false
		}
	}
	type rawToolGroup struct {
		fragment   llm.OpenAIResponsesRawFragment
		signatures []string
		used       bool
	}
	groups := make([]rawToolGroup, 0, len(requestExt.RawTools))
	for _, fragment := range requestExt.RawTools {
		if len(fragment.Raw) == 0 {
			continue
		}
		var rawTool Tool
		if json.Unmarshal(fragment.Raw, &rawTool) != nil {
			continue
		}
		converted, err := convertToolsToLLM([]Tool{rawTool})
		if err != nil || len(converted) == 0 {
			continue
		}
		for i := range converted {
			converted[i].ResponsesRawID = fmt.Sprintf("tools:%d", fragment.OriginalIndex)
		}
		groups = append(groups, rawToolGroup{fragment: fragment, signatures: replayToolSignatures(converted)})
	}

	currentSignatures := replayToolSignatures(currentTools)
	replayRawInput := rawInputReplayMatchesCurrent(requestExt, currentMessages, currentTools)
	tools := make([]json.RawMessage, 0, len(structuredTools)+len(groups))
	structuredIndex := 0
	replayed := false
	for toolIndex := 0; toolIndex < len(currentTools); {
		matchedGroup := -1
		for groupIndex := range groups {
			group := &groups[groupIndex]
			end := toolIndex + len(group.signatures)
			if group.used || end > len(currentSignatures) ||
				!slices.Equal(currentSignatures[toolIndex:end], group.signatures) {
				continue
			}
			matchedGroup = groupIndex
			break
		}

		if matchedGroup >= 0 {
			group := &groups[matchedGroup]
			group.used = true
			tools = append(tools, cloneRaw(group.fragment.Raw))
			for i := 0; i < len(group.signatures); i++ {
				if responsesToolEmitsStructured(currentTools[toolIndex+i], replayRawInput) {
					structuredIndex++
				}
			}
			if structuredIndex > len(structuredTools) {
				return nil, false
			}
			toolIndex += len(group.signatures)
			replayed = true
			continue
		}

		if responsesToolEmitsStructured(currentTools[toolIndex], replayRawInput) {
			if structuredIndex >= len(structuredTools) {
				return nil, false
			}
			tools = append(tools, cloneRaw(structuredTools[structuredIndex]))
			structuredIndex++
		}
		toolIndex++
	}

	if structuredIndex != len(structuredTools) {
		return nil, false
	}

	return tools, replayed
}

func responsesOriginToolEmitsTopLevel(tool llm.Tool, replayRawInput bool) bool {
	switch tool.ResponsesOrigin {
	case "":
		return true
	case "raw_tool":
		return true
	case "additional_tools":
		return !replayRawInput
	case "tool_search_output":
		return false
	default:
		return false
	}
}

func responsesToolEmitsStructured(tool llm.Tool, replayRawInput bool) bool {
	if !responsesOriginToolEmitsTopLevel(tool, replayRawInput) {
		return false
	}
	switch tool.Type {
	case llm.ToolTypeFunction, llm.ToolTypeImageGeneration, llm.ToolTypeWebSearch, llm.ToolTypeGoogleSearch,
		llm.ToolTypeResponsesCustomTool, llm.ToolTypeResponsesToolSearch:
		return true
	default:
		return false
	}
}

func rawInputReplayMatchesCurrent(
	requestExt *llm.OpenAIResponsesRequestExtensions,
	currentMessages []llm.Message,
	currentTools []llm.Tool,
) bool {
	return requestExt != nil && len(requestExt.RawInputItems) > 0 &&
		slices.Equal(requestExt.RawInputMessages, replayMessageSignatures(currentMessages)) &&
		slices.Equal(requestExt.RawInputTools, replayInputToolSignatures(currentTools))
}

func replayToolSignatures(tools []llm.Tool) []string {
	signatures := make([]string, len(tools))
	for i := range tools {
		signatures[i] = replayToolSignature(tools[i])
	}
	return signatures
}

func replayToolSignature(tool llm.Tool) string {
	data, err := json.Marshal(struct {
		Tool         llm.Tool `json:"tool"`
		Origin       string   `json:"origin,omitempty"`
		SourceType   string   `json:"source_type,omitempty"`
		RawID        string   `json:"raw_id,omitempty"`
		OriginCallID string   `json:"origin_call_id,omitempty"`
	}{
		Tool: tool, Origin: tool.ResponsesOrigin, SourceType: tool.ResponsesSourceType,
		RawID: tool.ResponsesRawID, OriginCallID: tool.ResponsesOriginCallID,
	})
	if err != nil {
		return "\x00invalid"
	}
	return string(data)
}

func replayMessageSignatures(messages []llm.Message) []string {
	signatures := make([]string, len(messages))
	for i := range messages {
		data, err := json.Marshal(messages[i])
		if err != nil {
			signatures[i] = "\x00invalid"
			continue
		}
		signatures[i] = string(data)
	}
	return signatures
}

func replayInputToolSignatures(tools []llm.Tool) []string {
	signatures := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool.ResponsesOrigin == "additional_tools" || tool.ResponsesOrigin == "tool_search_output" {
			signatures = append(signatures, replayToolSignature(tool))
		}
	}
	return signatures
}

func rawToolChoiceMatchesCurrentTools(raw json.RawMessage, current *ToolChoice) bool {
	if current == nil {
		return false
	}

	var rawChoice ToolChoice
	if err := json.Unmarshal(raw, &rawChoice); err != nil {
		return false
	}

	return toolChoiceSignature(&rawChoice) == toolChoiceSignature(current)
}

func toolChoiceSignature(choice *ToolChoice) string {
	if choice == nil {
		return ""
	}

	if choice.Type != nil && *choice.Type == "allowed_tools" {
		data, err := json.Marshal(choice)
		if err != nil {
			return ""
		}
		return "allowed:" + string(data)
	}

	if choice.Mode != nil {
		return "mode:" + *choice.Mode
	}

	if choice.Type != nil && choice.Name != nil {
		return "named:" + *choice.Type + ":" + *choice.Name
	}

	return ""
}

func cloneRaw(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	return append(json.RawMessage(nil), src...)
}
