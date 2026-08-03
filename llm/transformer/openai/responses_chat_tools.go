package openai

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/looplj/axonhub/llm"
)

const (
	responsesChatToolMappingsMetadataKey = "openai_responses_chat_tool_mappings"
	responsesChatToolWarningsMetadataKey = "openai_responses_chat_tool_warnings"
	responsesChatToolCatalogMetadataKey  = "openai_responses_chat_tool_catalog"
)

type responsesChatToolKind string

const (
	responsesChatToolCustom    responsesChatToolKind = "custom"
	responsesChatToolSearch    responsesChatToolKind = "tool_search"
	responsesChatToolNamespace responsesChatToolKind = "namespace"
)

type responsesChatToolMapping struct {
	Kind        responsesChatToolKind
	ChatName    string
	Name        string
	Namespace   string
	Execution   string
	HistoryOnly bool
}

type responsesChatToolAdapter struct {
	byChatName        map[string]responsesChatToolMapping
	byIdentity        map[string]responsesChatToolMapping
	usedNames         map[string]struct{}
	availableNames    map[string]struct{}
	plainDefinitions  map[string]string
	mappingSignatures map[string]string
	emittedPlain      map[string]struct{}
	err               error
	warnings          []string
}

// newResponsesChatToolAdapter indexes existing names and definitions before conversion.
func newResponsesChatToolAdapter(tools []llm.Tool) *responsesChatToolAdapter {
	a := &responsesChatToolAdapter{
		byChatName:        make(map[string]responsesChatToolMapping),
		byIdentity:        make(map[string]responsesChatToolMapping),
		usedNames:         make(map[string]struct{}),
		availableNames:    make(map[string]struct{}),
		plainDefinitions:  make(map[string]string),
		mappingSignatures: make(map[string]string),
		emittedPlain:      make(map[string]struct{}),
	}
	for _, tool := range tools {
		if tool.Type == llm.ToolTypeFunction && tool.Function.Namespace == "" {
			if tool.Function.Name == "" {
				a.addWarning("unsupported_tool_type: dropped function tool without a name")
				continue
			}
			signature, err := functionDefinitionSignature(tool.Function)
			if err != nil {
				a.addWarning("invalid function tool %q was dropped: %v", tool.Function.Name, err)
				continue
			}
			if previous, exists := a.plainDefinitions[tool.Function.Name]; exists && previous != signature {
				a.addWarning("tool_name_conflict: kept first definition of function %q", tool.Function.Name)
				continue
			}
			a.plainDefinitions[tool.Function.Name] = signature
			a.usedNames[tool.Function.Name] = struct{}{}
		}
	}
	return a
}

// convertTools translates callable Responses primitives into Chat function definitions.
func (a *responsesChatToolAdapter) convertTools(tools []llm.Tool) []Tool {
	result := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		switch tool.Type {
		case llm.ToolTypeFunction:
			if tool.Function.Name == "" {
				if tool.Function.Namespace != "" {
					a.addWarning("unsupported_tool_type: dropped namespace function tool without a name")
				}
				continue
			}
			signature, err := functionDefinitionSignature(tool.Function)
			if err != nil {
				if tool.Function.Namespace != "" {
					a.addWarning("invalid namespace tool %q was dropped: %v", tool.Function.Name, err)
				}
				continue
			}
			chatTool := ToolFromLLM(tool)
			if tool.Function.Namespace != "" {
				name := strings.TrimPrefix(tool.Function.Name, tool.Function.Namespace+"__")
				identity := mappingIdentity(responsesChatToolNamespace, name, tool.Function.Namespace)
				mapping := responsesChatToolMapping{
					Kind: responsesChatToolNamespace, Name: name, Namespace: tool.Function.Namespace,
				}
				chatName, duplicate := a.registerMapping(identity, tool.Function.Name, "axonhub_namespace_tool", signature, mapping)
				if duplicate {
					continue
				}
				chatTool.Function.Name = chatName
			} else {
				if selected, ok := a.plainDefinitions[tool.Function.Name]; !ok || selected != signature {
					continue
				}
				if _, emitted := a.emittedPlain[tool.Function.Name]; emitted {
					continue
				}
				a.emittedPlain[tool.Function.Name] = struct{}{}
				a.availableNames[tool.Function.Name] = struct{}{}
			}
			result = append(result, chatTool)

		case llm.ToolTypeResponsesCustomTool:
			if tool.ResponseCustomTool == nil {
				a.addWarning("unsupported_tool_type: dropped custom tool with missing definition")
				continue
			}
			if tool.ResponseCustomTool.Name == "" {
				a.addWarning("unsupported_tool_type: dropped custom tool without a name")
				continue
			}
			identity := mappingIdentity(responsesChatToolCustom, tool.ResponseCustomTool.Name, "")
			signature, err := json.Marshal(tool.ResponseCustomTool)
			if err != nil {
				a.addWarning("invalid custom tool %q was dropped: %v", tool.ResponseCustomTool.Name, err)
				continue
			}
			mapping := responsesChatToolMapping{Kind: responsesChatToolCustom, Name: tool.ResponseCustomTool.Name}
			chatName, duplicate := a.registerMapping(identity, tool.ResponseCustomTool.Name, "axonhub_custom_tool", string(signature), mapping)
			if duplicate {
				continue
			}
			if format := tool.ResponseCustomTool.Format; format != nil &&
				(strings.EqualFold(strings.TrimSpace(format.Type), "grammar") || format.Definition != "") {
				a.addWarning("grammar_constraint_degraded: custom tool %q grammar is advisory after conversion to Chat Completions", tool.ResponseCustomTool.Name)
			}
			description := responsesChatCustomToolDescription(tool.ResponseCustomTool)
			result = append(result, Tool{
				Type: llm.ToolTypeFunction,
				Function: Function{
					Name: chatName, Description: description,
					Parameters: json.RawMessage(`{"type":"object","properties":{"input":{"type":"string","description":"Exact raw custom-tool input. Escape quotes, backslashes, and control characters only as required for the outer JSON string; do not add another object or serialization layer."}},"required":["input"],"additionalProperties":false}`),
				},
			})
		case llm.ToolTypeResponsesToolSearch:
			if tool.ResponseToolSearch == nil {
				a.addWarning("unsupported_tool_type: dropped tool_search with missing definition")
				continue
			}
			if tool.ResponseToolSearch.Execution != "client" {
				a.addWarning("unsupported_execution_owner: dropped tool_search with execution %q", tool.ResponseToolSearch.Execution)
				continue
			}
			identity := mappingIdentity(responsesChatToolSearch, "tool_search", "")
			signature, err := json.Marshal(tool.ResponseToolSearch)
			if err != nil {
				a.addWarning("invalid tool_search definition was dropped: %v", err)
				continue
			}
			mapping := responsesChatToolMapping{
				Kind: responsesChatToolSearch, Name: "tool_search", Execution: tool.ResponseToolSearch.Execution,
			}
			chatName, duplicate := a.registerMapping(identity, "tool_search", "axonhub_tool_search", string(signature), mapping)
			if duplicate {
				continue
			}
			parameters := tool.ResponseToolSearch.Parameters
			if len(parameters) == 0 || string(parameters) == "null" {
				parameters = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			result = append(result, Tool{
				Type: llm.ToolTypeFunction,
				Function: Function{
					Name: chatName, Description: tool.ResponseToolSearch.Description, Parameters: parameters,
				},
			})

		case llm.ToolTypeResponsesOpaqueTool:
			if tool.ResponseOpaqueTool == nil {
				a.addWarning("unsupported_tool_type: dropped opaque Responses tool with missing definition")
				continue
			}
			a.addWarning("unsupported_tool_type: dropped Responses tool %q (%s) without a Chat lifecycle codec", tool.ResponseOpaqueTool.Name, tool.ResponseOpaqueTool.SourceType)

		case llm.ToolTypeImageGeneration, llm.ToolTypeWebSearch, llm.ToolTypeGoogleSearch,
			llm.ToolTypeGoogleCodeExecution, llm.ToolTypeGoogleUrlContext:
			// These common-model tools already predate the Responses-to-Chat bridge.
			// Chat Completions cannot carry them, so retain the legacy silent filter
			// instead of reporting a Responses compatibility degradation.
			continue

		default:
			a.addWarning("unsupported_tool_type: dropped %s tool that cannot be translated to Chat Completions", tool.Type)
		}
	}
	return result
}

// responsesChatCustomToolDescription explains the JSON function wrapper and
// preserves original instructions while marking grammar as advisory.
func responsesChatCustomToolDescription(tool *llm.ResponseCustomTool) string {
	const wrapperInstructions = "This Responses custom tool is represented as a Chat Completions function. " +
		"Pass the exact raw custom-tool input as the string value of the `input` argument. " +
		"The outer function arguments must be valid JSON: escape quotes, backslashes, and control characters " +
		"only as required for that JSON string. Do not add Markdown code fences. " +
		"Any original instruction below about not wrapping input in JSON applies to the `input` string itself; " +
		"do not add another object or serialization layer inside it."

	var description strings.Builder
	description.WriteString(wrapperInstructions)
	if original := strings.TrimSpace(tool.Description); original != "" {
		description.WriteString("\n\nOriginal custom-tool instructions (apply to the `input` string):\n")
		description.WriteString(original)
	}
	if format := tool.Format; format != nil && format.Definition != "" {
		description.WriteString("\n\nChat Completions cannot enforce the original custom-tool grammar during sampling. " +
			"Treat this grammar as required guidance for the `input` string:\n")
		if syntax := strings.TrimSpace(format.Syntax); syntax != "" {
			description.WriteString(syntax)
			description.WriteString(" grammar:\n")
		} else {
			description.WriteString("grammar:\n")
		}
		description.WriteString(format.Definition)
	}
	return description.String()
}

// convertMessage translates specialized Responses calls in message history into Chat calls.
func (a *responsesChatToolAdapter) convertMessage(message llm.Message, reasoningField ReasoningField) Message {
	converted := MessageFromLLMWithConfig(message, reasoningField)
	if len(message.ToolCalls) == 0 {
		return converted
	}

	converted.ToolCalls = make([]ToolCall, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		switch {
		case call.ResponseCustomToolCall != nil:
			mapping, ok := a.findMapping(responsesChatToolCustom, call.ResponseCustomToolCall.Name, "")
			if !ok {
				mapping = a.registerHistoryMapping(
					mappingIdentity(responsesChatToolCustom, call.ResponseCustomToolCall.Name, ""),
					call.ResponseCustomToolCall.Name,
					"axonhub_custom_tool_history",
					responsesChatToolMapping{Kind: responsesChatToolCustom, Name: call.ResponseCustomToolCall.Name},
				)
			}
			arguments, _ := json.Marshal(map[string]string{"input": call.ResponseCustomToolCall.Input})
			converted.ToolCalls = append(converted.ToolCalls, chatFunctionCall(call, a.specialCallID(call, call.ResponseCustomToolCall.CallID), mapping.ChatName, string(arguments)))

		case call.ResponseToolSearchCall != nil:
			mapping, ok := a.findMapping(responsesChatToolSearch, "tool_search", "")
			if !ok {
				mapping = a.registerHistoryMapping(
					mappingIdentity(responsesChatToolSearch, "tool_search", ""),
					"tool_search",
					"axonhub_tool_search_history",
					responsesChatToolMapping{
						Kind: responsesChatToolSearch, Name: "tool_search", Execution: call.ResponseToolSearchCall.Execution,
					},
				)
			}
			converted.ToolCalls = append(converted.ToolCalls, chatFunctionCall(call, a.specialCallID(call, call.ResponseToolSearchCall.CallID), mapping.ChatName, call.ResponseToolSearchCall.Arguments))

		default:
			name := call.Function.Name
			if call.Function.Namespace != "" {
				name = call.Function.Namespace + "__" + call.Function.Name
				if mapping, ok := a.findMapping(responsesChatToolNamespace, call.Function.Name, call.Function.Namespace); ok {
					name = mapping.ChatName
				}
			}
			convertedCall := ToolCallFromLLM(call)
			convertedCall.Type = llm.ToolTypeFunction
			convertedCall.Function.Name = name
			converted.ToolCalls = append(converted.ToolCalls, convertedCall)
		}
	}
	return converted
}

// hasChatAssistantPayload reports whether a converted message contains data
// accepted as assistant history by Chat Completions providers. Non-assistant
// roles use different validation rules and pass through unchanged.
func hasChatAssistantPayload(message Message) bool {
	if message.Role != "assistant" {
		return true
	}
	if len(message.ToolCalls) > 0 ||
		(message.ReasoningContent != nil && strings.TrimSpace(*message.ReasoningContent) != "") ||
		(message.Reasoning != nil && strings.TrimSpace(*message.Reasoning) != "") ||
		strings.TrimSpace(message.Refusal) != "" || message.Audio != nil {
		return true
	}
	if message.Content.Content != nil && strings.TrimSpace(*message.Content.Content) != "" {
		return true
	}
	for _, part := range message.Content.MultipleContent {
		if (part.Text != nil && strings.TrimSpace(*part.Text) != "") ||
			part.ImageURL != nil || part.VideoURL != nil || part.InputAudio != nil {
			return true
		}
	}
	return false
}

// chatFunctionCall builds a Chat function call while preserving common call metadata.
func chatFunctionCall(call llm.ToolCall, callID, name, arguments string) ToolCall {
	return ToolCall{
		ID: callID, Type: llm.ToolTypeFunction, Index: call.Index,
		Function: FunctionCall{Name: name, Arguments: arguments},
	}
}

// convertToolChoice maps Responses tool selection onto the converted Chat catalog.
func (a *responsesChatToolAdapter) convertToolChoice(choice *llm.ToolChoice) *ToolChoice {
	if choice == nil {
		return nil
	}
	result := &ToolChoice{ToolChoice: choice.ToolChoice}
	if choice.NamedToolChoice == nil {
		return result
	}
	name := choice.NamedToolChoice.Function.Name
	kind := choiceMappingKind(choice.NamedToolChoice.Type)
	switch {
	case kind != "":
		mapping, ok := a.findToolChoiceMapping(kind, name)
		if !ok {
			a.setError(fmt.Errorf("unsupported_tool_choice: named tool %q is unavailable after Responses-to-Chat conversion", name))
			return nil
		}
		name = mapping.ChatName
	case choice.NamedToolChoice.Type == llm.ToolTypeFunction:
		if mapping, ok := a.findNamespaceToolChoiceMapping(name); ok {
			if _, plain := a.emittedPlain[name]; plain && mapping.ChatName != name {
				a.setError(fmt.Errorf("tool_name_conflict: function tool choice %q is ambiguous between plain and namespace tools", name))
				return nil
			}
			name = mapping.ChatName
		}
	case choice.NamedToolChoice.Type != "" && choice.NamedToolChoice.Type != llm.ToolTypeFunction:
		a.setError(fmt.Errorf("unsupported_tool_choice: named %s tool %q cannot be translated to Chat Completions", choice.NamedToolChoice.Type, name))
		return nil
	}
	if _, ok := a.availableNames[name]; !ok {
		a.setError(fmt.Errorf("unsupported_tool_choice: named tool %q is unavailable after Responses-to-Chat conversion", choice.NamedToolChoice.Function.Name))
		return nil
	}
	result.NamedToolChoice = &NamedToolChoice{Type: llm.ToolTypeFunction, Function: ToolFunction{Name: name}}
	return result
}

// reserveName returns a collision-free Chat function name.
func (a *responsesChatToolAdapter) reserveName(preferred, fallback string) string {
	if preferred == "" {
		preferred = fallback
	}
	if _, exists := a.usedNames[preferred]; !exists {
		a.usedNames[preferred] = struct{}{}
		return preferred
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_%d", fallback, i)
		if _, exists := a.usedNames[candidate]; !exists {
			a.usedNames[candidate] = struct{}{}
			return candidate
		}
	}
}

// registerMapping records the reversible identity of a converted tool definition.
func (a *responsesChatToolAdapter) registerMapping(identity, preferred, fallback, signature string, mapping responsesChatToolMapping) (string, bool) {
	if existing, ok := a.byIdentity[identity]; ok {
		if existingSignature := a.mappingSignatures[identity]; existingSignature != signature {
			a.addWarning("tool_name_conflict: kept first definition of tool %q", mapping.Name)
		}
		return existing.ChatName, true
	}
	chatName := a.reserveName(preferred, fallback)
	mapping.ChatName = chatName
	a.byIdentity[identity] = mapping
	a.byChatName[chatName] = mapping
	a.availableNames[chatName] = struct{}{}
	a.mappingSignatures[identity] = signature
	return chatName, false
}

// registerHistoryMapping records a non-callable mapping used only to replay history.
func (a *responsesChatToolAdapter) registerHistoryMapping(
	identity, preferred, fallback string,
	mapping responsesChatToolMapping,
) responsesChatToolMapping {
	if existing, ok := a.byIdentity[identity]; ok {
		return existing
	}
	mapping.ChatName = a.reserveName(preferred, fallback)
	mapping.HistoryOnly = true
	a.byIdentity[identity] = mapping
	a.byChatName[mapping.ChatName] = mapping
	return mapping
}

// findMapping looks up a converted tool by its Responses identity.
func (a *responsesChatToolAdapter) findMapping(kind responsesChatToolKind, name, namespace string) (responsesChatToolMapping, bool) {
	mapping, ok := a.byIdentity[mappingIdentity(kind, name, namespace)]
	return mapping, ok
}

// findToolChoiceMapping resolves a named choice to one unambiguous converted tool.
func (a *responsesChatToolAdapter) findToolChoiceMapping(kind responsesChatToolKind, name string) (responsesChatToolMapping, bool) {
	if kind == responsesChatToolSearch {
		return a.findMapping(kind, "tool_search", "")
	}
	if kind == responsesChatToolCustom {
		return a.findMapping(kind, name, "")
	}
	if kind == responsesChatToolNamespace {
		return a.findNamespaceToolChoiceMapping(name)
	}
	return responsesChatToolMapping{}, false
}

// findNamespaceToolChoiceMapping resolves member or flattened namespace names.
func (a *responsesChatToolAdapter) findNamespaceToolChoiceMapping(name string) (responsesChatToolMapping, bool) {
	var found responsesChatToolMapping
	matched := false
	for _, mapping := range a.byIdentity {
		if mapping.Kind != responsesChatToolNamespace ||
			(mapping.ChatName != name && mapping.Name != name && mapping.Namespace+"__"+mapping.Name != name) {
			continue
		}
		if matched {
			a.setError(fmt.Errorf("tool_name_conflict: namespace tool choice %q is ambiguous", name))
			return responsesChatToolMapping{}, false
		}
		found = mapping
		matched = true
	}
	return found, matched
}

// specialCallID selects the protocol-specific call ID and reports inconsistencies.
func (a *responsesChatToolAdapter) specialCallID(call llm.ToolCall, specialized string) string {
	if specialized != "" {
		if call.ID != "" && specialized != call.ID {
			a.addWarning("tool_call_id_conflict: used specialized call ID %q instead of outer call ID %q", specialized, call.ID)
		}
		return specialized
	}
	return call.ID
}

// addWarning records a non-fatal conversion degradation.
func (a *responsesChatToolAdapter) addWarning(format string, args ...any) {
	a.warnings = append(a.warnings, fmt.Sprintf(format, args...))
}

// setError retains the first fatal conversion error.
func (a *responsesChatToolAdapter) setError(err error) {
	if a.err == nil && err != nil {
		a.err = err
	}
}

// mappingIdentity returns the stable internal key for a Responses tool identity.
func mappingIdentity(kind responsesChatToolKind, name, namespace string) string {
	return string(kind) + "\x00" + namespace + "\x00" + name
}

// choiceMappingKind maps a Responses tool-choice type to its adapter category.
func choiceMappingKind(toolType string) responsesChatToolKind {
	switch toolType {
	case "custom", llm.ToolTypeResponsesCustomTool:
		return responsesChatToolCustom
	case "tool_search", llm.ToolTypeResponsesToolSearch:
		return responsesChatToolSearch
	case "namespace":
		return responsesChatToolNamespace
	default:
		return ""
	}
}

// functionDefinitionSignature normalizes a function definition for deduplication.
func functionDefinitionSignature(function llm.Function) (string, error) {
	var parameters any
	if len(function.Parameters) > 0 {
		if err := json.Unmarshal(function.Parameters, &parameters); err != nil {
			return "", err
		}
	}
	normalized, err := json.Marshal(struct {
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters"`
		Strict      *bool  `json:"strict,omitempty"`
	}{Description: function.Description, Parameters: parameters, Strict: function.Strict})
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

// mappings returns an isolated Chat-name lookup table for response restoration.
func (a *responsesChatToolAdapter) mappings() map[string]responsesChatToolMapping {
	if len(a.byChatName) == 0 {
		return nil
	}
	result := make(map[string]responsesChatToolMapping, len(a.byChatName))
	for name, mapping := range a.byChatName {
		result[name] = mapping
	}
	return result
}

// catalog returns the sorted callable Chat tool names used to recognize name fragments.
func (a *responsesChatToolAdapter) catalog() []string {
	if len(a.availableNames) == 0 {
		return nil
	}
	result := make([]string, 0, len(a.availableNames))
	for name := range a.availableNames {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// restoreResponsesChatToolCalls restores specialized Responses calls in a full response.
func restoreResponsesChatToolCalls(response *llm.Response, mappings map[string]responsesChatToolMapping) {
	if response == nil || len(mappings) == 0 {
		return
	}
	for i := range response.Choices {
		if response.Choices[i].Message != nil {
			restoreResponsesChatMessage(response.Choices[i].Message, mappings, true)
		}
		if response.Choices[i].Delta != nil {
			restoreResponsesChatMessage(response.Choices[i].Delta, mappings, true)
		}
	}
}

type responsesChatToolStreamRestorer struct {
	mappings map[string]responsesChatToolMapping
	catalog  map[string]struct{}
	byIndex  map[[2]int]responsesChatToolMapping
	pending  map[[2]int]llm.ToolCall
	plain    map[[2]int]struct{}
}

// newResponsesChatToolStreamRestorer creates state for restoring fragmented Chat calls.
func newResponsesChatToolStreamRestorer(
	mappings map[string]responsesChatToolMapping,
	catalogs ...[]string,
) *responsesChatToolStreamRestorer {
	restorer := &responsesChatToolStreamRestorer{
		mappings: mappings,
		catalog:  map[string]struct{}{},
		byIndex:  map[[2]int]responsesChatToolMapping{},
		pending:  map[[2]int]llm.ToolCall{},
		plain:    map[[2]int]struct{}{},
	}
	for _, catalog := range catalogs {
		for _, name := range catalog {
			restorer.catalog[name] = struct{}{}
		}
	}
	return restorer
}

// restore restores specialized calls and tracks their names across stream chunks.
func (r *responsesChatToolStreamRestorer) restore(response *llm.Response) {
	if response == nil || (len(r.mappings) == 0 && len(r.catalog) == 0) {
		return
	}
	for i := range response.Choices {
		choice := &response.Choices[i]
		message := choice.Delta
		if message == nil {
			message = choice.Message
		}
		if message != nil {
			restored := make([]llm.ToolCall, 0, len(message.ToolCalls))
			for j := range message.ToolCalls {
				call := message.ToolCalls[j]
				key := [2]int{choice.Index, call.Index}
				if mapping, ok := r.byIndex[key]; ok {
					call.Function.Name = mapping.ChatName
					restored = append(restored, call)
					continue
				}
				if _, ok := r.plain[key]; ok {
					restored = append(restored, call)
					continue
				}

				currentName := call.Function.Name
				if pending, ok := r.pending[key]; ok {
					call = mergeResponsesChatToolCallFragments(pending, call)
					if !r.isKnownName(call.Function.Name) && r.isKnownName(currentName) {
						call.Function.Name = currentName
					}
				}
				potentialLongerName := r.isPotentialKnownName(call.Function.Name)
				if mapping, ok := r.mappings[call.Function.Name]; ok && !mapping.HistoryOnly {
					if call.ID == "" || potentialLongerName {
						r.pending[key] = call
						continue
					}
					r.byIndex[key] = mapping
					delete(r.pending, key)
					restored = append(restored, call)
					continue
				}
				if _, exact := r.catalog[call.Function.Name]; exact && call.ID != "" && !potentialLongerName {
					delete(r.pending, key)
					r.plain[key] = struct{}{}
					restored = append(restored, call)
					continue
				}
				if call.ID == "" || potentialLongerName {
					r.pending[key] = call
					continue
				}
				delete(r.pending, key)
				r.plain[key] = struct{}{}
				restored = append(restored, call)
			}
			message.ToolCalls = restored
		}

		if choice.FinishReason != nil {
			if message == nil {
				message = &llm.Message{}
				choice.Delta = message
			}
			pendingIndexes := make([]int, 0)
			for key := range r.pending {
				if key[0] == choice.Index {
					pendingIndexes = append(pendingIndexes, key[1])
				}
			}
			sort.Ints(pendingIndexes)
			for _, index := range pendingIndexes {
				key := [2]int{choice.Index, index}
				call := r.pending[key]
				if mapping, ok := r.mappings[call.Function.Name]; ok && !mapping.HistoryOnly {
					r.byIndex[key] = mapping
				} else {
					r.plain[key] = struct{}{}
				}
				message.ToolCalls = append(message.ToolCalls, call)
				delete(r.pending, key)
			}
		}
		if message != nil {
			restoreResponsesChatMessage(message, r.mappings, false)
		}
	}
}

// isKnownName reports whether a name exactly identifies a declared callable tool.
func (r *responsesChatToolStreamRestorer) isKnownName(name string) bool {
	if mapping, ok := r.mappings[name]; ok && !mapping.HistoryOnly {
		return true
	}
	_, ok := r.catalog[name]
	return ok
}

// isPotentialKnownName reports whether a partial name can become a declared tool name.
func (r *responsesChatToolStreamRestorer) isPotentialKnownName(name string) bool {
	for knownName := range r.catalog {
		if knownName != name && strings.HasPrefix(knownName, name) {
			return true
		}
	}
	for chatName, mapping := range r.mappings {
		if !mapping.HistoryOnly && chatName != name && strings.HasPrefix(chatName, name) {
			return true
		}
	}
	return false
}

// mergeResponsesChatToolCallFragments accumulates identity, name, and arguments deltas.
func mergeResponsesChatToolCallFragments(pending, current llm.ToolCall) llm.ToolCall {
	merged := pending
	if current.ID != "" {
		merged.ID = current.ID
	}
	if current.Type != "" {
		merged.Type = current.Type
	}
	merged.Index = current.Index
	merged.Function.Name += current.Function.Name
	merged.Function.Arguments += current.Function.Arguments
	if current.Function.Namespace != "" {
		merged.Function.Namespace = current.Function.Namespace
	}
	if len(current.TransformerMetadata) > 0 {
		if merged.TransformerMetadata == nil {
			merged.TransformerMetadata = map[string]any{}
		}
		for key, value := range current.TransformerMetadata {
			merged.TransformerMetadata[key] = value
		}
	}
	return merged
}

// restoreResponsesChatMessage restores specialized calls in one message or delta.
func restoreResponsesChatMessage(message *llm.Message, mappings map[string]responsesChatToolMapping, unwrapCustom bool) {
	for i := range message.ToolCalls {
		call := &message.ToolCalls[i]
		mapping, ok := mappings[call.Function.Name]
		if !ok || mapping.HistoryOnly {
			continue
		}
		switch mapping.Kind {
		case responsesChatToolCustom:
			input := call.Function.Arguments
			if unwrapCustom {
				var wrapper struct {
					Input *string `json:"input"`
				}
				if json.Unmarshal([]byte(call.Function.Arguments), &wrapper) == nil && wrapper.Input != nil {
					input = *wrapper.Input
				}
			}
			call.Type = llm.ToolTypeResponsesCustomTool
			call.ResponseCustomToolCall = &llm.ResponseCustomToolCall{CallID: call.ID, Name: mapping.Name, Input: input}
			if call.TransformerMetadata == nil {
				call.TransformerMetadata = map[string]any{}
			}
			call.TransformerMetadata["openai_responses_chat_wrapped_custom"] = true
		case responsesChatToolSearch:
			call.Type = llm.ToolTypeResponsesToolSearch
			call.ResponseToolSearchCall = &llm.ResponseToolSearchCall{CallID: call.ID, Execution: mapping.Execution, Arguments: call.Function.Arguments}
		case responsesChatToolNamespace:
			call.Function.Name = mapping.Name
			call.Function.Namespace = mapping.Namespace
		}
	}
}
