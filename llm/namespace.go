package llm

// FlattenFunctionName encodes a namespaced function for protocols that only
// accept flat names. Decoding requires an exact request-local mapping.
func FlattenFunctionName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "__" + name
}

// ResolveSingleFunctionChoice resolves a namespace or a one-entry tool list
// when it identifies exactly one declared function. Unresolved constraints are
// preserved so that each outbound can apply its own capability checks.
func ResolveSingleFunctionChoice(src *ToolChoice, tools []Tool) *ToolChoice {
	if src == nil || src.ToolChoice != nil {
		return src
	}
	var option ToolOption
	switch {
	case len(src.Tools) == 1 && src.NamedToolChoice == nil:
		option = src.Tools[0]
	case len(src.Tools) == 0 && src.NamedToolChoice != nil && src.NamedToolChoice.Type == "namespace":
		option = ToolOption{Type: "namespace", Name: src.NamedToolChoice.Function.Name}
	default:
		return src
	}
	var function ToolFunction
	switch option.Type {
	case "namespace":
		count := 0
		for _, tool := range tools {
			if tool.Type == ToolTypeFunction && tool.Function.Namespace == option.Name {
				function = ToolFunction{Name: tool.Function.Name, Namespace: tool.Function.Namespace}
				count++
			}
		}
		if count != 1 {
			return src
		}
	case "function":
		function = ToolFunction{Name: option.Name, Namespace: option.Namespace}
		if option.Namespace != "" {
			found := false
			for _, tool := range tools {
				if tool.Type == ToolTypeFunction && tool.Function.Name == option.Name && tool.Function.Namespace == option.Namespace {
					found = true
					break
				}
			}
			if !found {
				return src
			}
		}
	default:
		return src
	}
	return &ToolChoice{NamedToolChoice: &NamedToolChoice{Type: ToolTypeFunction, Function: function}}
}
