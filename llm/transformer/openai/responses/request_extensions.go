package responses

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
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

func buildRawOnlyToolFragments(tools []Tool, rawTools []json.RawMessage) []llm.OpenAIResponsesRawFragment {
	if len(tools) == 0 {
		return nil
	}

	fragments := make([]llm.OpenAIResponsesRawFragment, 0, len(tools))
	for i := range tools {
		if i >= len(rawTools) || len(rawTools[i]) == 0 || (isStructurallyRepresentedToolType(tools[i].Type) && tools[i].Type != ToolTypeToolSearch) {
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
	if tool.Type == ToolTypeToolSearch {
		return 1
	}
	if tool.Type != ToolTypeNamespace {
		return 0
	}

	count := 0
	for _, subTool := range tool.Tools {
		if namespaceCallableToolType(subTool) {
			count++
		}
	}

	return count
}

func isStructurallyRepresentedToolType(toolType string) bool {
	switch toolType {
	case llm.ToolTypeFunction, "image_generation", "web_search", ToolTypeCustom, ToolTypeToolSearch:
		return true
	default:
		return false
	}
}

func responseToolSignature(tool Tool) string {
	switch tool.Type {
	case llm.ToolTypeFunction, ToolTypeCustom:
		return tool.Type + ":" + tool.Name
	default:
		return tool.Type
	}
}

func rawUnsupportedToolChoice(choice *ToolChoice, rawChoice json.RawMessage) json.RawMessage {
	if choice == nil || len(rawChoice) == 0 {
		return nil
	}

	classification := ClassifyRawToolChoice(rawChoice)
	if classification.FullyRepresented {
		var decodedRaw ToolChoice
		if err := json.Unmarshal(rawChoice, &decodedRaw); err != nil {
			return cloneRaw(rawChoice)
		}
		if toolChoiceSignature(&decodedRaw) == toolChoiceSignature(choice) {
			return nil
		}
	}

	return cloneRaw(rawChoice)
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

	// Compute once: replay matching re-marshals every message and tool, so
	// both raw merges share the same verdict.
	replayRawInput := rawInputReplayMatchesCurrent(requestExt, llmReq.Messages, llmReq.Tools)

	tools, replayed, err := mergeRawOnlyTools(obj["tools"], requestExt, llmReq.Tools, replayRawInput)
	if err != nil {
		return nil, err
	}
	if replayed {
		toolsRaw, err := json.Marshal(tools)
		if err != nil {
			return nil, err
		}
		obj["tools"] = toolsRaw
	}

	if len(requestExt.RawToolChoice) > 0 && rawToolChoiceMatchesCurrentTools(requestExt.RawToolChoice, payload.ToolChoice) {
		obj["tool_choice"] = cloneRaw(requestExt.RawToolChoice)
	}

	if input, ok := mergeRawOnlyInputItems(obj["input"], requestExt, replayRawInput); ok {
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
	replayRawInput bool,
) ([]json.RawMessage, bool) {
	if requestExt == nil || len(requestExt.RawInputItems) == 0 {
		return nil, false
	}
	if !replayRawInput {
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

type rawToolGroup struct {
	fragment   llm.OpenAIResponsesRawFragment
	namespace  string
	lossy      bool
	signatures []string
}

type rawReplayPlan struct {
	steps    []json.RawMessage
	replayed bool
}

func mergeRawOnlyTools(
	structuredRaw json.RawMessage,
	requestExt *llm.OpenAIResponsesRequestExtensions,
	currentTools []llm.Tool,
	replayRawInput bool,
) ([]json.RawMessage, bool, error) {
	if requestExt == nil || len(requestExt.RawTools) == 0 {
		return nil, false, nil
	}

	var structuredTools []json.RawMessage
	if len(structuredRaw) > 0 {
		if err := json.Unmarshal(structuredRaw, &structuredTools); err != nil {
			return nil, false, err
		}
	}

	rawGroups := buildRawToolGroups(requestExt.RawTools)
	plan, err := buildReplayPlan(currentTools, rawGroups, structuredTools, replayRawInput)
	if err != nil {
		return nil, false, err
	}
	if plan == nil {
		return nil, false, nil
	}

	tools := make([]json.RawMessage, 0, len(plan.steps))
	for _, step := range plan.steps {
		tools = append(tools, cloneRaw(step))
	}

	return tools, plan.replayed, nil
}

func buildRawToolGroups(fragments []llm.OpenAIResponsesRawFragment) []rawToolGroup {
	groups := make([]rawToolGroup, 0, len(fragments))
	for _, fragment := range fragments {
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
		namespace := ""
		lossy := false
		if rawTool.Type == ToolTypeNamespace {
			namespace = rawTool.Name
			for _, subTool := range rawTool.Tools {
				if !namespaceCallableToolType(subTool) {
					lossy = true
					break
				}
			}
		}
		groups = append(groups, rawToolGroup{
			fragment: fragment, namespace: namespace, lossy: lossy, signatures: replayToolSignatures(converted),
		})
	}
	return groups
}

func buildReplayPlan(
	currentTools []llm.Tool,
	groups []rawToolGroup,
	structuredTools []json.RawMessage,
	replayRawInput bool,
) (*rawReplayPlan, error) {
	lossyNamespaces := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if group.lossy {
			lossyNamespaces[group.namespace] = struct{}{}
		}
	}
	currentUnsupportedNamespaces := make(map[string]struct{})
	for _, tool := range currentTools {
		if tool.Type == llm.ToolTypeResponsesOpaqueTool && tool.ResponseOpaqueTool != nil &&
			tool.ResponseOpaqueTool.Namespace != "" {
			currentUnsupportedNamespaces[tool.ResponseOpaqueTool.Namespace] = struct{}{}
		}
	}

	currentSignatures := replayToolSignatures(currentTools)
	plan := &rawReplayPlan{steps: make([]json.RawMessage, 0, len(structuredTools)+len(groups))}
	usedGroups := make([]bool, len(groups))
	structuredIndex := 0
	activeNamespace := ""
	for toolIndex := 0; toolIndex < len(currentTools); {
		matchedGroup := -1
		for groupIndex := range groups {
			group := &groups[groupIndex]
			end := toolIndex + len(group.signatures)
			if usedGroups[groupIndex] || end > len(currentSignatures) ||
				!slices.Equal(currentSignatures[toolIndex:end], group.signatures) {
				continue
			}
			// A raw namespace can match only the complete current group. A
			// prefix match means an intermediate transform added a member;
			// replaying the old fragment would silently drop that addition.
			if group.namespace != "" && end < len(currentTools) &&
				namespaceOfResponsesTool(currentTools[end]) == group.namespace {
				continue
			}
			matchedGroup = groupIndex
			break
		}

		if matchedGroup >= 0 {
			group := groups[matchedGroup]
			usedGroups[matchedGroup] = true
			plan.steps = append(plan.steps, group.fragment.Raw)
			if group.namespace != "" {
				if namespaceGroupEmitsStructured(currentTools[toolIndex:toolIndex+len(group.signatures)], replayRawInput) &&
					group.namespace != activeNamespace {
					if !namespaceWrapperMatches(structuredTools, structuredIndex, group.namespace) {
						return nil, nil
					}
					structuredIndex++
					activeNamespace = group.namespace
				}
			} else {
				for i := range len(group.signatures) {
					if !responsesToolEmitsStructured(currentTools[toolIndex+i], replayRawInput) {
						continue
					}
					if structuredIndex >= len(structuredTools) {
						return nil, nil
					}
					structuredIndex++
				}
				activeNamespace = ""
			}
			if structuredIndex > len(structuredTools) {
				return nil, nil
			}
			toolIndex += len(group.signatures)
			plan.replayed = true
			continue
		}

		if namespace := namespaceOfResponsesTool(currentTools[toolIndex]); namespace != "" {
			_, lossy := lossyNamespaces[namespace]
			_, stillUnsupported := currentUnsupportedNamespaces[namespace]
			if stillUnsupported &&
				(lossy || namespaceGroupModified(groups, currentTools, currentSignatures, toolIndex, namespace)) {
				return nil, fmt.Errorf(
					"%w: unsupported_namespace_replay: namespace %q was modified and contains member type(s) without a structural Responses codec",
					transformer.ErrInvalidRequest,
					namespace,
				)
			}
		}

		if responsesToolEmitsStructured(currentTools[toolIndex], replayRawInput) {
			if namespace := namespaceOfResponsesTool(currentTools[toolIndex]); namespace != "" {
				if namespace == activeNamespace {
					toolIndex++
					continue
				}
				if !namespaceWrapperMatches(structuredTools, structuredIndex, namespace) {
					return nil, nil
				}
				plan.steps = append(plan.steps, structuredTools[structuredIndex])
				structuredIndex++
				activeNamespace = namespace
			} else {
				if structuredIndex >= len(structuredTools) {
					return nil, nil
				}
				plan.steps = append(plan.steps, structuredTools[structuredIndex])
				structuredIndex++
				activeNamespace = ""
			}
		}
		toolIndex++
	}

	if structuredIndex != len(structuredTools) {
		return nil, nil
	}

	return plan, nil
}

func namespaceGroupModified(
	groups []rawToolGroup,
	currentTools []llm.Tool,
	currentSignatures []string,
	toolIndex int,
	namespace string,
) bool {
	for i := range groups {
		if groups[i].namespace != namespace {
			continue
		}
		end := toolIndex + len(groups[i].signatures)
		complete := end <= len(currentSignatures) &&
			slices.Equal(currentSignatures[toolIndex:end], groups[i].signatures) &&
			(end == len(currentTools) || namespaceOfResponsesTool(currentTools[end]) != namespace)
		return !complete
	}
	return false
}

func namespaceGroupEmitsStructured(tools []llm.Tool, replayRawInput bool) bool {
	for _, tool := range tools {
		if responsesToolEmitsStructured(tool, replayRawInput) {
			return true
		}
	}
	return false
}

func namespaceWrapperMatches(
	structuredTools []json.RawMessage,
	structuredIndex int,
	namespace string,
) bool {
	if structuredIndex >= len(structuredTools) {
		return false
	}
	var structuredTool Tool
	if json.Unmarshal(structuredTools[structuredIndex], &structuredTool) != nil ||
		structuredTool.Type != ToolTypeNamespace || structuredTool.Name != namespace {
		return false
	}
	return true
}

func namespaceOfResponsesTool(tool llm.Tool) string {
	switch {
	case tool.Type == llm.ToolTypeFunction && tool.Function.Namespace != "":
		return tool.Function.Namespace
	case tool.Type == llm.ToolTypeResponsesCustomTool && tool.ResponseCustomTool != nil:
		return tool.ResponseCustomTool.Namespace
	case tool.Type == llm.ToolTypeResponsesOpaqueTool && tool.ResponseOpaqueTool != nil:
		return tool.ResponseOpaqueTool.Namespace
	default:
		return ""
	}
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
		Tool                 llm.Tool `json:"tool"`
		Origin               string   `json:"origin,omitempty"`
		SourceType           string   `json:"source_type,omitempty"`
		RawID                string   `json:"raw_id,omitempty"`
		OriginCallID         string   `json:"origin_call_id,omitempty"`
		NamespaceDescription string   `json:"namespace_description,omitempty"`
	}{
		Tool: tool, Origin: tool.ResponsesOrigin, SourceType: tool.ResponsesSourceType,
		RawID: tool.ResponsesRawID, OriginCallID: tool.ResponsesOriginCallID,
		NamespaceDescription: tool.ResponsesNamespaceDescription,
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

	if choice.Type != nil && *choice.Type == ToolChoiceTypeAllowedTools {
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
