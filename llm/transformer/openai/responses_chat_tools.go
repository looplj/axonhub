package openai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	responsesapi "github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// Metadata keys shared with provider contract tests.
const (
	ResponsesChatToolMappingsMetadataKey = "openai_responses_chat_tool_mappings"
	ResponsesChatToolCatalogMetadataKey  = "openai_responses_chat_tool_catalog"

	responsesChatToolWarningsMetadataKey = "openai_responses_chat_tool_warnings"
	responsesChatStrictFinishMetadataKey = "openai_responses_chat_strict_finish"
)

const (
	responsesChatToolSearchName                   = responsesapi.ToolTypeToolSearch
	responsesChatClientToolFallbackName           = "axonhub_client_tool"
	responsesChatToolSearchFallbackName           = "axonhub_tool_search"
	responsesChatCustomToolHistoryFallbackName    = "axonhub_custom_tool_history"
	responsesChatToolSearchHistoryFallbackName    = "axonhub_tool_search_history"
	responsesChatNamespaceToolHistoryFallbackName = "axonhub_namespace_tool_history"
)

// HasResponsesChatToolMappings reports whether request metadata carries the
// reversible Responses-to-Chat tool mapping state.
func HasResponsesChatToolMappings(req *httpclient.Request) bool {
	return len(responsesChatToolMappings(req)) > 0
}

type responsesChatToolKind string

const (
	responsesChatToolFunction  responsesChatToolKind = "function"
	responsesChatToolCustom    responsesChatToolKind = "custom"
	responsesChatToolSearch    responsesChatToolKind = "tool_search"
	responsesChatToolNamespace responsesChatToolKind = "namespace"
	responsesChatToolClient    responsesChatToolKind = "client_tool"
)

type responsesChatToolMapping struct {
	Kind        responsesChatToolKind
	ChatName    string
	Name        string
	Namespace   string
	SourceType  string
	Execution   string
	HistoryOnly bool
}

type toolIdentity struct {
	Kind      responsesChatToolKind
	Name      string
	Namespace string
}

type flattenedIdentityKey struct {
	Kind responsesChatToolKind
	Name string
}

type sourceIdentityKey struct {
	SourceType string
	Name       string
}

type responsesChatToolAdapter struct {
	names                 *responsesChatToolNameAllocator
	registry              *responsesChatToolMappingRegistry
	availableNames        map[string]struct{}
	unsupportedNamespaces map[string][]string
	emittedPlain          map[string]struct{}
	err                   error
	warnings              []string
}

var chatFunctionNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// newResponsesChatToolAdapter indexes existing names and definitions before conversion.
func newResponsesChatToolAdapter(tools []llm.Tool) *responsesChatToolAdapter {
	a := &responsesChatToolAdapter{
		names:                 newResponsesChatToolNameAllocator(),
		registry:              newResponsesChatToolMappingRegistry(),
		availableNames:        make(map[string]struct{}),
		unsupportedNamespaces: make(map[string][]string),
		emittedPlain:          make(map[string]struct{}),
	}
	for _, tool := range tools {
		if tool.Type == llm.ToolTypeFunction && tool.Function.Namespace == "" && tool.ResponsesSourceType == "" {
			if tool.Function.Name == "" {
				a.addWarningf("unsupported_tool_type: dropped function tool without a name")
				continue
			}
			signature, err := functionDefinitionSignature(tool.Function)
			if err != nil {
				a.addWarningf("invalid function tool %q was dropped: %v", tool.Function.Name, err)
				continue
			}
			identity := toolIdentity{Kind: responsesChatToolFunction, Name: tool.Function.Name}
			if previous, exists := a.registry.signature(identity); exists && previous != signature {
				a.setError(conflictingDefinitionError(identity))
				continue
			}
			if err := a.names.reserveExact(tool.Function.Name, identity); err != nil {
				a.setError(err)
			}
			a.registry.saveSignature(identity, signature)
		}
	}
	for _, tool := range tools {
		if tool.Type != llm.ToolTypeResponsesCustomTool || tool.ResponseCustomTool == nil || tool.ResponseCustomTool.Name == "" {
			continue
		}
		identity := customToolIdentity(tool.ResponseCustomTool.Name, tool.ResponseCustomTool.Namespace)
		chatName := namespaceChatName(tool.ResponseCustomTool.Name, tool.ResponseCustomTool.Namespace)
		signature, err := json.Marshal(tool.ResponseCustomTool)
		if err != nil {
			a.setError(err)
			continue
		}
		if previous, exists := a.registry.signature(identity); exists && previous != string(signature) {
			a.setError(conflictingDefinitionError(identity))
			continue
		}
		if err := a.names.reserveExact(chatName, identity); err != nil {
			a.setError(err)
			continue
		}
		a.registry.saveSignature(identity, string(signature))
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
					_, err := llm.NamespaceFunctionMemberName(tool.Function)
					a.setError(err)
				} else {
					a.addWarningf("unsupported_tool_type: dropped function tool without a name")
				}
				continue
			}
			memberName := tool.Function.Name
			if tool.Function.Namespace != "" {
				var err error
				memberName, err = llm.NamespaceFunctionMemberName(tool.Function)
				if err != nil {
					a.setError(err)
					continue
				}
			}
			normalizedFunction, err := normalizeChatFunctionDefinition(tool.Function)
			if err != nil {
				if tool.Function.Namespace != "" {
					a.addWarningf("invalid namespace tool %q was dropped: %v", tool.Function.Name, err)
				}
				continue
			}
			tool.Function = normalizedFunction
			if tool.Function.DeferLoading {
				a.setError(fmt.Errorf("unsupported_function_feature: function tool %q uses defer_loading, which Chat Completions cannot represent", tool.Function.Name))
				continue
			}
			signature, err := functionDefinitionSignature(tool.Function)
			if err != nil {
				if tool.Function.Namespace != "" {
					a.addWarningf("invalid namespace tool %q was dropped: %v", tool.Function.Name, err)
				}
				continue
			}
			chatTool := ToolFromLLM(tool)
			if tool.Function.Namespace != "" {
				identity := toolIdentity{Kind: responsesChatToolNamespace, Name: memberName, Namespace: tool.Function.Namespace}
				mapping := responsesChatToolMapping{
					Kind: responsesChatToolNamespace, Name: memberName, Namespace: tool.Function.Namespace,
					SourceType: tool.ResponsesSourceType,
				}
				chatName, duplicate := a.registerStrictMapping(identity, namespaceChatName(memberName, tool.Function.Namespace), signature, mapping)
				if duplicate {
					continue
				}
				chatTool.Function.Name = chatName
				if tool.ResponsesSourceType != "" {
					a.addWarningf("client_tool_output_degraded: %s tool %q returns as a function_call after Chat conversion", tool.ResponsesSourceType, memberName)
				}
			} else if tool.ResponsesSourceType != "" {
				identity := clientToolIdentity(tool.ResponsesSourceType, tool.Function.Name)
				mapping := responsesChatToolMapping{
					Kind: responsesChatToolClient, Name: tool.Function.Name, SourceType: tool.ResponsesSourceType,
				}
				chatName, duplicate := a.registerMapping(identity, tool.Function.Name, responsesChatClientToolFallbackName, signature, mapping)
				if duplicate {
					continue
				}
				chatTool.Function.Name = chatName
				a.addWarningf("client_tool_output_degraded: %s tool %q returns as a function_call after Chat conversion", tool.ResponsesSourceType, tool.Function.Name)
			} else {
				identity := toolIdentity{Kind: responsesChatToolFunction, Name: tool.Function.Name}
				if selected, ok := a.registry.signature(identity); !ok || selected != signature {
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
				a.addWarningf("unsupported_tool_type: dropped custom tool with missing definition")
				continue
			}
			if tool.ResponseCustomTool.Name == "" {
				a.addWarningf("unsupported_tool_type: dropped custom tool without a name")
				continue
			}
			namespace := tool.ResponseCustomTool.Namespace
			identity := customToolIdentity(tool.ResponseCustomTool.Name, namespace)
			signature, err := json.Marshal(tool.ResponseCustomTool)
			if err != nil {
				a.addWarningf("invalid custom tool %q was dropped: %v", tool.ResponseCustomTool.Name, err)
				continue
			}
			mapping := responsesChatToolMapping{
				Kind: responsesChatToolCustom, Name: tool.ResponseCustomTool.Name, Namespace: namespace,
			}
			chatName, duplicate := a.registerStrictMapping(identity, namespaceChatName(tool.ResponseCustomTool.Name, namespace), string(signature), mapping)
			if duplicate {
				continue
			}
			if format := tool.ResponseCustomTool.Format; format != nil &&
				(strings.EqualFold(strings.TrimSpace(format.Type), "grammar") || format.Definition != "") {
				a.addWarningf("grammar_constraint_degraded: custom tool %q grammar is advisory after conversion to Chat Completions", tool.ResponseCustomTool.Name)
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
				a.addWarningf("unsupported_tool_type: dropped tool_search with missing definition")
				continue
			}
			if tool.ResponseToolSearch.Execution != "client" {
				a.addWarningf("unsupported_execution_owner: dropped tool_search with execution %q", tool.ResponseToolSearch.Execution)
				continue
			}
			parameters, err := normalizeChatFunctionParameters(tool.ResponseToolSearch.Parameters)
			if err != nil {
				a.addWarningf("invalid tool_search definition was dropped: %v", err)
				continue
			}
			normalizedToolSearch := *tool.ResponseToolSearch
			normalizedToolSearch.Parameters = parameters
			identity := toolIdentity{Kind: responsesChatToolSearch, Name: responsesChatToolSearchName}
			signature, err := json.Marshal(&normalizedToolSearch)
			if err != nil {
				a.addWarningf("invalid tool_search definition was dropped: %v", err)
				continue
			}
			mapping := responsesChatToolMapping{
				Kind: responsesChatToolSearch, Name: responsesChatToolSearchName, Execution: tool.ResponseToolSearch.Execution,
			}
			chatName, duplicate := a.registerMapping(identity, responsesChatToolSearchName, responsesChatToolSearchFallbackName, string(signature), mapping)
			if duplicate {
				continue
			}
			result = append(result, Tool{
				Type: llm.ToolTypeFunction,
				Function: Function{
					Name: chatName, Description: normalizedToolSearch.Description, Parameters: parameters,
				},
			})

		case llm.ToolTypeResponsesOpaqueTool:
			if tool.ResponseOpaqueTool == nil {
				a.addWarningf("unsupported_tool_type: dropped opaque Responses tool with missing definition")
				continue
			}
			if namespace := tool.ResponseOpaqueTool.Namespace; namespace != "" {
				a.unsupportedNamespaces[namespace] = append(
					a.unsupportedNamespaces[namespace], tool.ResponseOpaqueTool.SourceType,
				)
			}
			a.addWarningf("unsupported_tool_type: dropped Responses tool %q (%s) without a Chat lifecycle codec", tool.ResponseOpaqueTool.Name, tool.ResponseOpaqueTool.SourceType)

		case llm.ToolTypeImageGeneration, llm.ToolTypeWebSearch, llm.ToolTypeGoogleSearch,
			llm.ToolTypeGoogleCodeExecution, llm.ToolTypeGoogleUrlContext:
			// These common-model tools already predate the Responses-to-Chat bridge.
			// Chat Completions cannot carry them, so retain the legacy silent filter
			// instead of reporting a Responses compatibility degradation.
			continue

		default:
			a.addWarningf("unsupported_tool_type: dropped %s tool that cannot be translated to Chat Completions", tool.Type)
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
	for index, call := range message.ToolCalls {
		// Responses calls are independent output items and commonly all carry
		// index zero. Once grouped into one Chat assistant message, indexes must
		// be unique and follow their array positions.
		call.Index = index
		switch {
		case call.ResponseCustomToolCall != nil:
			customNamespace := call.ResponseCustomToolCall.Namespace
			mapping, ok := a.findMapping(responsesChatToolCustom, call.ResponseCustomToolCall.Name, customNamespace)
			if !ok {
				mapping, ok = a.registerHistoryMapping(
					customToolIdentity(call.ResponseCustomToolCall.Name, customNamespace),
					namespaceChatName(call.ResponseCustomToolCall.Name, customNamespace),
					responsesChatCustomToolHistoryFallbackName,
					responsesChatToolMapping{
						Kind: responsesChatToolCustom, Name: call.ResponseCustomToolCall.Name, Namespace: customNamespace,
					},
				)
				if !ok {
					continue
				}
			}
			arguments, _ := json.Marshal(map[string]string{"input": call.ResponseCustomToolCall.Input})
			converted.ToolCalls = append(converted.ToolCalls, chatFunctionCall(call, a.specialCallID(call, call.ResponseCustomToolCall.CallID), mapping.ChatName, string(arguments)))

		case call.ResponseToolSearchCall != nil:
			mapping, ok := a.findMapping(responsesChatToolSearch, responsesChatToolSearchName, "")
			if !ok {
				mapping, ok = a.registerHistoryMapping(
					toolIdentity{Kind: responsesChatToolSearch, Name: responsesChatToolSearchName},
					responsesChatToolSearchName,
					responsesChatToolSearchHistoryFallbackName,
					responsesChatToolMapping{
						Kind: responsesChatToolSearch, Name: responsesChatToolSearchName, Execution: call.ResponseToolSearchCall.Execution,
					},
				)
				if !ok {
					continue
				}
			}
			arguments := sanitizeToolSearchArguments(call.ResponseToolSearchCall.Arguments)
			converted.ToolCalls = append(converted.ToolCalls, chatFunctionCall(call, a.specialCallID(call, call.ResponseToolSearchCall.CallID), mapping.ChatName, arguments))

		default:
			name := call.Function.Name
			if call.Function.Namespace != "" {
				fullName := llm.JoinNamespaceFunctionName(call.Function.Namespace, call.Function.Name)
				if mapping, ok := a.findMapping(responsesChatToolNamespace, call.Function.Name, call.Function.Namespace); ok {
					name = mapping.ChatName
				} else {
					mapping, ok = a.registerHistoryMapping(
						toolIdentity{Kind: responsesChatToolNamespace, Name: call.Function.Name, Namespace: call.Function.Namespace},
						fullName,
						responsesChatNamespaceToolHistoryFallbackName,
						responsesChatToolMapping{
							Kind: responsesChatToolNamespace, Name: call.Function.Name, Namespace: call.Function.Namespace,
						},
					)
					if !ok {
						continue
					}
					name = mapping.ChatName
				}
			} else if mapping, ok := a.findClientHistoryMapping(name); ok {
				name = mapping.ChatName
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
	return shared.HasChatCompatibleAssistantPayload(message.ToLLMMessage())
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
		if choice.ToolChoice == nil {
			return nil
		}
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
		mapping, namespace := a.findNamespaceFunctionChoiceMapping(name)
		_, plain := a.emittedPlain[name]
		if namespace {
			name = mapping.ChatName
		} else if !plain {
			a.setError(fmt.Errorf("unsupported_tool_choice: named tool %q is unavailable after Responses-to-Chat conversion", name))
			return nil
		}
	case choice.NamedToolChoice.Type != "" && choice.NamedToolChoice.Type != llm.ToolTypeFunction:
		mapping, ok := a.findSourceTypeToolChoiceMapping(choice.NamedToolChoice.Type, name)
		if !ok {
			a.setError(fmt.Errorf("unsupported_tool_choice: named %s tool %q cannot be translated to Chat Completions", choice.NamedToolChoice.Type, name))
			return nil
		}
		name = mapping.ChatName
	}
	if _, ok := a.availableNames[name]; !ok {
		a.setError(fmt.Errorf("unsupported_tool_choice: named tool %q is unavailable after Responses-to-Chat conversion", choice.NamedToolChoice.Function.Name))
		return nil
	}
	result.NamedToolChoice = &NamedToolChoice{Type: llm.ToolTypeFunction, Function: ToolFunction{Name: name}}
	return result
}

// degradeUnsupportedRawToolSelector detects a Responses selector whose
// semantics cannot be inferred safely, records the loss, and asks the caller
// to use Chat Completions' default automatic selection.
func (a *responsesChatToolAdapter) degradeUnsupportedRawToolSelector(request *llm.Request) bool {
	if request == nil || request.ProviderExtensions == nil || request.ProviderExtensions.OpenAIResponses == nil ||
		request.ProviderExtensions.OpenAIResponses.Request == nil {
		return false
	}

	rawChoice := request.ProviderExtensions.OpenAIResponses.Request.RawToolChoice
	if len(rawChoice) == 0 {
		return false
	}
	if !rawChatToolChoiceMatchesCurrent(rawChoice, request.ToolChoice) {
		return false
	}
	selectorType, unsupported := unsupportedRawChatToolSelector(rawChoice)
	if !unsupported {
		return false
	}

	if selectorType == "" {
		selectorType = "unknown"
	}
	a.addWarningf(
		"unsupported_tool_choice_degraded: %s selector cannot be represented in Chat Completions; using auto",
		selectorType,
	)
	return true
}

// rawChatToolChoiceMatchesCurrent prevents a preserved raw selector from
// overriding a selector that an intermediate request transform cleared or
// replaced after the Responses request was decoded.
func rawChatToolChoiceMatchesCurrent(rawChoice json.RawMessage, current *llm.ToolChoice) bool {
	currentSignature, ok := llmToolChoiceSignature(current)
	if !ok {
		return false
	}
	rawSignature, ok := rawChatToolChoiceSemanticSignature(rawChoice)
	return ok && rawSignature == currentSignature
}

func llmToolChoiceSignature(choice *llm.ToolChoice) (string, bool) {
	if choice == nil {
		return "", false
	}
	if choice.AllowedToolsSet {
		tools := choice.AllowedTools
		if tools == nil {
			tools = []llm.ToolOption{}
		}
		data, err := json.Marshal(struct {
			Mode  *string          `json:"mode,omitempty"`
			Tools []llm.ToolOption `json:"tools"`
		}{Mode: choice.ToolChoice, Tools: tools})
		return "allowed:" + string(data), err == nil
	}
	if choice.ToolChoice != nil {
		return "mode:" + *choice.ToolChoice, true
	}
	if choice.NamedToolChoice != nil {
		return "named:" + choice.NamedToolChoice.Type + ":" + choice.NamedToolChoice.Function.Name, true
	}
	return "empty", true
}

func rawChatToolChoiceSemanticSignature(rawChoice json.RawMessage) (string, bool) {
	classification := responsesapi.ClassifyRawToolChoice(rawChoice)
	var mode string
	if json.Unmarshal(rawChoice, &mode) == nil {
		return "mode:" + mode, true
	}

	var selector struct {
		Type  *string         `json:"type"`
		Name  *string         `json:"name"`
		Mode  *string         `json:"mode"`
		Tools json.RawMessage `json:"tools"`
	}
	if json.Unmarshal(rawChoice, &selector) != nil {
		return "", false
	}
	switch classification.Kind {
	case responsesapi.RawToolChoiceKindString, responsesapi.RawToolChoiceKindModeObject:
		if selector.Mode == nil {
			return "", false
		}
		return "mode:" + *selector.Mode, true
	case responsesapi.RawToolChoiceKindNamed:
		if selector.Type == nil {
			return "", false
		}
		name := ""
		if selector.Name != nil {
			name = *selector.Name
		}
		return "named:" + *selector.Type + ":" + name, true
	case responsesapi.RawToolChoiceKindAllowedTools:
		var tools []llm.ToolOption
		if len(selector.Tools) > 0 && string(selector.Tools) != "null" && json.Unmarshal(selector.Tools, &tools) != nil {
			return "", false
		}
		return llmToolChoiceSignature(&llm.ToolChoice{
			ToolChoice: selector.Mode, AllowedTools: tools, AllowedToolsSet: true,
		})
	}

	// Unknown selector shapes may still contain a decodable subset that
	// matches the current IR, so preserve the legacy precedence for them.
	if selector.Mode != nil {
		return "mode:" + *selector.Mode, true
	}
	if selector.Type != nil {
		name := ""
		if selector.Name != nil {
			name = *selector.Name
		}
		return "named:" + *selector.Type + ":" + name, true
	}
	return "empty", true
}

// unsupportedRawChatToolSelector protects callers that may carry a raw
// extension created by an older transformer: fully represented string,
// named, and allowed-tools selectors must retain their normal Chat mapping.
func unsupportedRawChatToolSelector(rawChoice json.RawMessage) (string, bool) {
	classification := responsesapi.ClassifyRawToolChoice(rawChoice)
	if classification.FullyRepresented {
		if classification.Kind == responsesapi.RawToolChoiceKindAllowedTools {
			return classification.SelectorType, false
		}
		return "", false
	}

	return classification.SelectorType, true
}

// filterAllowedTools applies a Responses allowed_tools constraint after all
// identities have been mapped, leaving history mappings intact.
func (a *responsesChatToolAdapter) filterAllowedTools(tools []Tool, choice *llm.ToolChoice) []Tool {
	if choice == nil || !choice.AllowedToolsSet {
		return tools
	}

	allowedNames := make(map[string]struct{}, len(choice.AllowedTools))
	for _, option := range choice.AllowedTools {
		names, ok := a.resolveAllowedTool(option)
		if !ok {
			a.addWarningf("unsupported_allowed_tool: dropped unavailable %s tool %q from allowed_tools", option.Type, option.Name)
			continue
		}
		for _, name := range names {
			allowedNames[name] = struct{}{}
		}
	}

	filtered := make([]Tool, 0, len(allowedNames))
	a.availableNames = make(map[string]struct{}, len(allowedNames))
	for _, tool := range tools {
		if _, ok := allowedNames[tool.Function.Name]; !ok {
			continue
		}
		filtered = append(filtered, tool)
		a.availableNames[tool.Function.Name] = struct{}{}
	}
	a.registry.markActive(allowedNames)
	return filtered
}

// resolveAllowedTool maps one type-aware Responses identity to emitted Chat names.
func (a *responsesChatToolAdapter) resolveAllowedTool(option llm.ToolOption) ([]string, bool) {
	if option.Type == responsesapi.ToolTypeNamespace {
		if err := a.unsupportedNamespaceError(option.Name); err != nil {
			a.setError(err)
			return nil, false
		}
		matched := make([]string, 0)
		for _, identity := range a.registry.namespace(option.Name) {
			mapping, ok := a.registry.find(identity)
			if !ok {
				continue
			}
			if isCallableNamespaceMember(mapping, option.Name) {
				matched = append(matched, mapping.ChatName)
			}
		}
		if len(matched) > 0 {
			sort.Strings(matched)
			return matched, true
		}
	}
	kind := choiceMappingKind(option.Type)
	if kind != "" {
		mapping, ok := a.findToolChoiceMapping(kind, option.Name)
		return []string{mapping.ChatName}, ok
	}
	if option.Type == llm.ToolTypeFunction {
		mapping, namespace := a.findNamespaceFunctionChoiceMapping(option.Name)
		_, plain := a.emittedPlain[option.Name]
		if plain == namespace {
			return nil, false
		}
		if namespace {
			return []string{mapping.ChatName}, true
		}
		return []string{option.Name}, true
	}
	mapping, ok := a.findSourceTypeToolChoiceMapping(option.Type, option.Name)
	return []string{mapping.ChatName}, ok
}

func validateChatFunctionName(name string) error {
	if !chatFunctionNamePattern.MatchString(name) {
		return fmt.Errorf("invalid_tool_name: Chat function name %q must match %s", name, chatFunctionNamePattern.String())
	}
	return nil
}

// registerStrictMapping records a tool whose Chat name is its protocol identity.
func (a *responsesChatToolAdapter) registerStrictMapping(identity toolIdentity, chatName, signature string, mapping responsesChatToolMapping) (string, bool) {
	if existing, ok := a.registry.find(identity); ok {
		existingSignature, _ := a.registry.signature(identity)
		if existingSignature != signature || existing.SourceType != mapping.SourceType {
			a.setError(conflictingDefinitionError(identity))
			return "", true
		}
		return existing.ChatName, true
	}
	if err := a.names.reserveExact(chatName, identity); err != nil {
		a.setError(err)
		return "", true
	}
	a.registry.add(identity, chatName, signature, mapping, false)
	a.availableNames[chatName] = struct{}{}
	return chatName, false
}

func namespaceChatName(name, namespace string) string {
	if namespace == "" {
		return name
	}
	return llm.JoinNamespaceFunctionName(namespace, name)
}

func customToolIdentity(name, namespace string) toolIdentity {
	return toolIdentity{Kind: responsesChatToolCustom, Name: name, Namespace: namespace}
}

func toolIdentityLabel(identity toolIdentity) string {
	if identity.Namespace != "" {
		return string(identity.Kind) + " " + identity.Namespace + "." + identity.Name
	}
	return string(identity.Kind) + " " + identity.Name
}

func toolNameCollisionError(name string, existing, requested toolIdentity) error {
	return fmt.Errorf("tool_name_conflict: Chat name %q is required by %s and %s", name, toolIdentityLabel(existing), toolIdentityLabel(requested))
}

func conflictingDefinitionError(identity toolIdentity) error {
	return fmt.Errorf("tool_definition_conflict: %s has multiple different definitions", toolIdentityLabel(identity))
}

// registerMapping records a reversible definition with a generated collision-free name.
func (a *responsesChatToolAdapter) registerMapping(identity toolIdentity, preferred, fallback, signature string, mapping responsesChatToolMapping) (string, bool) {
	if existing, ok := a.registry.find(identity); ok {
		existingSignature, _ := a.registry.signature(identity)
		if existingSignature != signature {
			a.setError(conflictingDefinitionError(identity))
			return "", true
		}
		return existing.ChatName, true
	}
	chatName, err := a.names.reserve(preferred, fallback, identity)
	if err != nil {
		a.setError(err)
		return "", true
	}
	a.registry.add(identity, chatName, signature, mapping, false)
	a.availableNames[chatName] = struct{}{}
	return chatName, false
}

// registerHistoryMapping records a non-callable mapping used only to replay history.
func (a *responsesChatToolAdapter) registerHistoryMapping(
	identity toolIdentity, preferred, fallback string,
	mapping responsesChatToolMapping,
) (responsesChatToolMapping, bool) {
	if existing, ok := a.registry.find(identity); ok {
		return existing, true
	}
	chatName, err := a.names.reserve(preferred, fallback, identity)
	if err != nil {
		a.setError(err)
		return responsesChatToolMapping{}, false
	}
	a.registry.add(identity, chatName, "", mapping, true)
	mapping.ChatName = chatName
	mapping.HistoryOnly = true
	return mapping, true
}

// findMapping looks up a converted tool by its Responses identity.
func (a *responsesChatToolAdapter) findMapping(kind responsesChatToolKind, name, namespace string) (responsesChatToolMapping, bool) {
	return a.registry.find(toolIdentity{Kind: kind, Name: name, Namespace: namespace})
}

// findToolChoiceMapping resolves a named choice to one unambiguous converted tool.
func (a *responsesChatToolAdapter) findToolChoiceMapping(kind responsesChatToolKind, name string) (responsesChatToolMapping, bool) {
	if kind == responsesChatToolSearch {
		return a.findMapping(kind, responsesChatToolSearchName, "")
	}
	if kind == responsesChatToolCustom {
		return a.findFlattenedMapping(kind, name)
	}
	if kind == responsesChatToolNamespace {
		return a.findNamespaceSelectorMapping(name)
	}
	return responsesChatToolMapping{}, false
}

// findFlattenedMapping resolves a canonical namespace__name identity.
func (a *responsesChatToolAdapter) findFlattenedMapping(kind responsesChatToolKind, name string) (responsesChatToolMapping, bool) {
	var found responsesChatToolMapping
	matched := false
	for _, identity := range a.registry.flattened(kind, name) {
		mapping, ok := a.registry.find(identity)
		if !ok || mapping.Kind != kind {
			continue
		}
		if matched {
			a.setError(fmt.Errorf("tool_name_conflict: %s tool choice %q is ambiguous", kind, name))
			return responsesChatToolMapping{}, false
		}
		found = mapping
		matched = true
	}
	return found, matched
}

// findNamespaceFunctionChoiceMapping resolves function/functions__name while
// keeping a plain function and a namespace function strictly separate.
func (a *responsesChatToolAdapter) findNamespaceFunctionChoiceMapping(name string) (responsesChatToolMapping, bool) {
	if _, plain := a.emittedPlain[name]; plain {
		return responsesChatToolMapping{}, false
	}
	return a.findFlattenedMapping(responsesChatToolNamespace, name)
}

// findNamespaceSelectorMapping maps a namespace selector to its sole member
// for named tool choice. Chat cannot force a group when a namespace has more
// than one callable member.
func (a *responsesChatToolAdapter) findNamespaceSelectorMapping(namespace string) (responsesChatToolMapping, bool) {
	if err := a.unsupportedNamespaceError(namespace); err != nil {
		a.setError(err)
		return responsesChatToolMapping{}, false
	}
	found := false
	var result responsesChatToolMapping
	for _, identity := range a.registry.namespace(namespace) {
		mapping, ok := a.registry.find(identity)
		if !ok {
			continue
		}
		if !isCallableNamespaceMember(mapping, namespace) {
			continue
		}
		if found {
			a.setError(fmt.Errorf("unsupported_tool_choice: namespace %q has multiple callable tools and cannot map to one Chat named choice", namespace))
			return responsesChatToolMapping{}, false
		}
		result = mapping
		found = true
	}
	return result, found
}

func (a *responsesChatToolAdapter) unsupportedNamespaceError(namespace string) error {
	unsupported, exists := a.unsupportedNamespaces[namespace]
	if !exists {
		return nil
	}
	return fmt.Errorf(
		"unsupported_tool_choice: namespace %q contains %s tool(s) without a Chat lifecycle codec",
		namespace, strings.Join(unsupported, ", "),
	)
}

func isCallableNamespaceMember(mapping responsesChatToolMapping, namespace string) bool {
	return mapping.Namespace == namespace && !mapping.HistoryOnly &&
		(mapping.Kind == responsesChatToolNamespace || mapping.Kind == responsesChatToolCustom)
}

// findSourceTypeToolChoiceMapping resolves a promoted future client tool by
// its original Responses type and name.
func (a *responsesChatToolAdapter) findSourceTypeToolChoiceMapping(sourceType, name string) (responsesChatToolMapping, bool) {
	var found responsesChatToolMapping
	matched := false
	key := sourceIdentityKey{SourceType: sourceType, Name: name}
	for _, identity := range a.registry.source(key) {
		mapping, ok := a.registry.find(identity)
		if !ok || mapping.SourceType != sourceType || mapping.Name != name {
			continue
		}
		if matched {
			a.setError(fmt.Errorf("tool_name_conflict: %s tool choice %q is ambiguous", sourceType, name))
			return responsesChatToolMapping{}, false
		}
		found = mapping
		matched = true
	}
	return found, matched
}

// findClientHistoryMapping resolves a historical plain function call name to
// the renamed Chat name of a client tool definition. When the original name
// no longer matches an emitted Chat tool (for example a same-named plain
// definition won the name reservation and was later filtered out), the call
// must follow the definition's renamed Chat name; otherwise strict providers
// reject a call that references no declared tool.
func (a *responsesChatToolAdapter) findClientHistoryMapping(name string) (responsesChatToolMapping, bool) {
	if _, available := a.availableNames[name]; available {
		return responsesChatToolMapping{}, false
	}

	var found responsesChatToolMapping
	matched := false
	for _, identity := range a.registry.client(name) {
		mapping, ok := a.registry.find(identity)
		if !ok {
			continue
		}
		if mapping.Kind != responsesChatToolClient || mapping.Name != name {
			continue
		}
		if matched {
			// Ambiguous: several client tools share the original name.
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
			a.addWarningf("tool_call_id_conflict: used specialized call ID %q instead of outer call ID %q", specialized, call.ID)
		}
		return specialized
	}
	return call.ID
}

// addWarningf records a non-fatal conversion degradation.
func (a *responsesChatToolAdapter) addWarningf(format string, args ...any) {
	a.warnings = append(a.warnings, fmt.Sprintf(format, args...))
}

// setError retains the first fatal conversion error.
func (a *responsesChatToolAdapter) setError(err error) {
	if a.err == nil && err != nil {
		a.err = err
	}
}

func clientToolIdentity(sourceType, name string) toolIdentity {
	return toolIdentity{Kind: responsesChatToolClient, Name: name, Namespace: sourceType}
}

// choiceMappingKind maps a Responses tool-choice type to its adapter category.
func choiceMappingKind(toolType string) responsesChatToolKind {
	switch toolType {
	case responsesapi.ToolTypeCustom, llm.ToolTypeResponsesCustomTool:
		return responsesChatToolCustom
	case responsesapi.ToolTypeToolSearch, llm.ToolTypeResponsesToolSearch:
		return responsesChatToolSearch
	case responsesapi.ToolTypeNamespace:
		return responsesChatToolNamespace
	default:
		return ""
	}
}

// normalizeChatFunctionDefinition ensures the parameter root satisfies the
// object-only contract required by Chat Completions function tools.
func normalizeChatFunctionDefinition(function llm.Function) (llm.Function, error) {
	parameters, err := normalizeChatFunctionParameters(function.Parameters)
	if err != nil {
		return llm.Function{}, err
	}
	function.Parameters = parameters
	return function, nil
}

func normalizeChatFunctionParameters(parameters json.RawMessage) (json.RawMessage, error) {
	const emptyObjectSchema = `{"type":"object","properties":{}}`

	trimmed := strings.TrimSpace(string(parameters))
	if trimmed == "" || trimmed == "null" {
		return json.RawMessage(emptyObjectSchema), nil
	}

	var schema map[string]json.RawMessage
	if err := json.Unmarshal(parameters, &schema); err != nil {
		return nil, fmt.Errorf("parameters must be a JSON object: %w", err)
	}
	if schema == nil {
		return json.RawMessage(emptyObjectSchema), nil
	}

	rawType, hasType := schema["type"]
	if !hasType {
		schema["type"] = json.RawMessage(`"object"`)
		normalized, err := json.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize parameters schema: %w", err)
		}
		return normalized, nil
	}

	var schemaType string
	if err := json.Unmarshal(rawType, &schemaType); err != nil || schemaType != "object" {
		return nil, fmt.Errorf("parameters schema type is required and must be %q", "object")
	}
	return parameters, nil
}

// functionDefinitionSignature normalizes a function definition for deduplication.
func functionDefinitionSignature(function llm.Function) (string, error) {
	function, err := normalizeChatFunctionDefinition(function)
	if err != nil {
		return "", err
	}
	var parameters any
	if len(function.Parameters) > 0 {
		if err := json.Unmarshal(function.Parameters, &parameters); err != nil {
			return "", err
		}
	}
	normalized, err := json.Marshal(struct {
		Description  string `json:"description,omitempty"`
		Parameters   any    `json:"parameters"`
		Strict       *bool  `json:"strict,omitempty"`
		DeferLoading bool   `json:"defer_loading,omitempty"`
	}{Description: function.Description, Parameters: parameters, Strict: function.Strict, DeferLoading: function.DeferLoading})
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

// mappings returns an isolated Chat-name lookup table for response restoration.
func (a *responsesChatToolAdapter) mappings() map[string]responsesChatToolMapping {
	return a.registry.mappings()
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
			call.ResponseCustomToolCall = &llm.ResponseCustomToolCall{
				CallID: call.ID, Name: mapping.Name, Namespace: mapping.Namespace, Input: input,
			}
			if call.TransformerMetadata == nil {
				call.TransformerMetadata = map[string]any{}
			}
			call.TransformerMetadata[responsesapi.ChatWrappedCustomMetadataKey] = true
		case responsesChatToolSearch:
			call.Type = llm.ToolTypeResponsesToolSearch
			call.ResponseToolSearchCall = &llm.ResponseToolSearchCall{CallID: call.ID, Execution: mapping.Execution, Arguments: call.Function.Arguments}
		case responsesChatToolNamespace:
			call.Function.Name = mapping.Name
			call.Function.Namespace = mapping.Namespace
		case responsesChatToolClient:
			call.Function.Name = mapping.Name
		}
	}
}

func sanitizeToolSearchArguments(arguments string) string {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return "{}"
	}
	if json.Valid([]byte(arguments)) {
		return arguments
	}
	return "{}"
}
