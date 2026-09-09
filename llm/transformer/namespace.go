package transformer

import (
	"fmt"

	"github.com/looplj/axonhub/llm"
)

func ValidateFlatFunctionNames(request *llm.Request) error {
	identities := make(map[string]llm.ToolFunction)
	register := func(namespace, name string) error {
		flat := llm.FlattenFunctionName(namespace, name)
		identity := llm.ToolFunction{Namespace: namespace, Name: name}
		if previous, exists := identities[flat]; exists && previous != identity {
			return fmt.Errorf("%w: namespace tool flat name %q conflicts with another function", ErrInvalidRequest, flat)
		}
		identities[flat] = identity
		return nil
	}
	declared := make(map[string]bool)
	for _, tool := range request.Tools {
		if tool.Type != llm.ToolTypeFunction {
			continue
		}
		if err := register(tool.Function.Namespace, tool.Function.Name); err != nil {
			return err
		}
		flat := llm.FlattenFunctionName(tool.Function.Namespace, tool.Function.Name)
		if declared[flat] {
			return fmt.Errorf("%w: duplicate function tool name %q", ErrInvalidRequest, flat)
		}
		declared[flat] = true
	}
	for _, message := range request.Messages {
		for _, call := range message.ToolCalls {
			if call.Type != llm.ToolTypeFunction && call.Type != "" {
				continue
			}
			if err := register(call.Function.Namespace, call.Function.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func ValidateNamespaceToolChoice(choice *llm.ToolChoice, tools []llm.Tool) error {
	resolved := llm.ResolveSingleFunctionChoice(choice, tools)
	if resolved == nil {
		return nil
	}
	if len(resolved.Tools) > 0 {
		return fmt.Errorf("%w: tool_choice.tools cannot be represented", ErrInvalidRequest)
	}
	if named := resolved.NamedToolChoice; named != nil && named.Type == "namespace" {
		return fmt.Errorf("%w: namespace tool choice %q cannot be represented as a single function", ErrInvalidRequest, named.Function.Name)
	}
	return nil
}
