package shared

import (
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

// DowngradeResponsesChatToolLifecycle removes Responses-only tool lifecycle
// state that a provider-specific Chat codec cannot encode and restore.
func DowngradeResponsesChatToolLifecycle(request *llm.Request) (*llm.Request, error) {
	if request == nil {
		return nil, nil
	}

	removedSourceTools := make(map[string]struct{})
	retainedFunctionNames := make(map[string]struct{})
	cloned := *request
	removedFunctionNames := make(map[string]struct{})
	cloned.Tools = make([]llm.Tool, 0, len(request.Tools))
	for _, tool := range request.Tools {
		if requiresResponsesChatToolLifecycle(tool) {
			if tool.ResponsesSourceType != "" {
				name := tool.Function.Name
				if name == "" {
					return nil, fmt.Errorf("%w: invalid responses source tool: function name is required", transformer.ErrInvalidRequest)
				}
				if _, duplicate := removedSourceTools[name]; duplicate {
					return nil, fmt.Errorf("%w: ambiguous responses source tool %q: duplicate removed definition", transformer.ErrInvalidRequest, name)
				}
				removedSourceTools[name] = struct{}{}
				removedFunctionNames[name] = struct{}{}
			} else if tool.Function.Namespace != "" {
				removedFunctionNames[tool.Function.Name] = struct{}{}
				memberName, err := llm.NamespaceFunctionMemberName(tool.Function)
				if err != nil {
					return nil, fmt.Errorf("%w: %w", transformer.ErrInvalidRequest, err)
				}
				removedFunctionNames[memberName] = struct{}{}
			}
			continue
		}
		if tool.Type == llm.ToolTypeFunction {
			retainedFunctionNames[tool.Function.Name] = struct{}{}
		}
		cloned.Tools = append(cloned.Tools, tool)
	}
	for name := range removedSourceTools {
		if _, conflict := retainedFunctionNames[name]; conflict {
			return nil, fmt.Errorf("%w: ambiguous responses source tool %q: conflicts with retained function", transformer.ErrInvalidRequest, name)
		}
	}
	cloned.Messages = filterOutToolLifecycleMessages(request.Messages, func(toolCall llm.ToolCall) bool {
		_, removedSourceCall := removedSourceTools[toolCall.Function.Name]
		return requiresResponsesChatToolLifecycleCall(toolCall) ||
			(toolCall.Type == llm.ToolTypeFunction && toolCall.Function.Namespace == "" &&
				toolCall.Function.Name != "" && removedSourceCall)
	})
	cloned.Tools, cloned.ToolChoice = filterResponsesToolChoiceForPlainFunctions(
		cloned.Tools,
		request.ToolChoice,
		removedFunctionNames,
	)
	if len(cloned.Tools) == 0 {
		cloned.ParallelToolCalls = nil
	}

	return &cloned, nil
}

func requiresResponsesChatToolLifecycleCall(toolCall llm.ToolCall) bool {
	return toolCall.Type == llm.ToolTypeResponsesCustomTool ||
		toolCall.ResponseCustomToolCall != nil ||
		toolCall.Type == llm.ToolTypeResponsesToolSearch ||
		toolCall.ResponseToolSearchCall != nil ||
		toolCall.Function.Namespace != ""
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
