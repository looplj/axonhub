package shared

import (
	"strings"

	"github.com/looplj/axonhub/llm"
)

// DowngradeResponsesChatToolLifecycle removes Responses-only tool lifecycle
// state that a provider-specific Chat codec cannot encode and restore.
func DowngradeResponsesChatToolLifecycle(request *llm.Request) *llm.Request {
	if request == nil {
		return nil
	}

	cloned := *request
	cloned.Messages = FilterOutResponsesChatToolLifecycleMessages(request.Messages)
	removedFunctionNames := make(map[string]struct{})
	cloned.Tools = make([]llm.Tool, 0, len(request.Tools))
	for _, tool := range request.Tools {
		if requiresResponsesChatToolLifecycle(tool) {
			if tool.Function.Namespace != "" {
				removedFunctionNames[tool.Function.Name] = struct{}{}
				removedFunctionNames[strings.TrimPrefix(tool.Function.Name, tool.Function.Namespace+"__")] = struct{}{}
			}
			continue
		}
		cloned.Tools = append(cloned.Tools, tool)
	}
	cloned.Tools, cloned.ToolChoice = filterResponsesToolChoiceForPlainFunctions(
		cloned.Tools,
		request.ToolChoice,
		removedFunctionNames,
	)
	if len(cloned.Tools) == 0 {
		cloned.ParallelToolCalls = nil
	}

	return &cloned
}

func requiresResponsesChatToolLifecycle(tool llm.Tool) bool {
	return tool.Type == llm.ToolTypeResponsesCustomTool || tool.ResponseCustomTool != nil ||
		tool.Type == llm.ToolTypeResponsesToolSearch || tool.ResponseToolSearch != nil ||
		tool.Type == llm.ToolTypeResponsesOpaqueTool || tool.ResponseOpaqueTool != nil ||
		tool.Function.Namespace != "" || tool.ResponsesSourceType != ""
}

func filterResponsesToolChoiceForPlainFunctions(
	tools []llm.Tool,
	choice *llm.ToolChoice,
	removedFunctionNames map[string]struct{},
) ([]llm.Tool, *llm.ToolChoice) {
	if choice == nil {
		return tools, nil
	}

	if choice.AllowedToolsSet {
		allowedFunctions := make(map[string]struct{}, len(choice.AllowedTools))
		for _, option := range choice.AllowedTools {
			if option.Type != llm.ToolTypeFunction {
				continue
			}
			if _, ambiguous := removedFunctionNames[option.Name]; ambiguous {
				continue
			}
			allowedFunctions[option.Name] = struct{}{}
		}
		filtered := make([]llm.Tool, 0, len(tools))
		for _, tool := range tools {
			_, allowed := allowedFunctions[tool.Function.Name]
			if tool.Type == llm.ToolTypeFunction && allowed {
				filtered = append(filtered, tool)
			}
		}
		if len(filtered) == 0 || choice.ToolChoice == nil {
			return filtered, nil
		}
		return filtered, &llm.ToolChoice{ToolChoice: choice.ToolChoice}
	}

	if choice.NamedToolChoice == nil {
		if len(tools) == 0 {
			return tools, nil
		}
		return tools, choice
	}
	if choice.NamedToolChoice.Type != llm.ToolTypeFunction {
		return tools, nil
	}
	if _, ambiguous := removedFunctionNames[choice.NamedToolChoice.Function.Name]; ambiguous {
		return tools, nil
	}
	for _, tool := range tools {
		if tool.Type == llm.ToolTypeFunction && tool.Function.Name == choice.NamedToolChoice.Function.Name {
			return tools, choice
		}
	}
	return tools, nil
}
