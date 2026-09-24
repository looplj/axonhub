package responses

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

// Names are encoded only when entering the unified model from native data.
// A native local name may itself begin with the namespace prefix.
func flatFunctionName(namespace, name string) string {
	if namespace == "" || name == "" {
		return name
	}
	return namespace + "__" + name
}

func localFunctionName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	// Trim exactly once. Both the namespace and local name may contain "__".
	return strings.TrimPrefix(name, namespace+"__")
}

// flattenRequestFunctionNames encodes native local names in declarations and
// history once, validating name conflicts without modifying src.
func flattenRequestFunctionNames(src *llm.Request) (*llm.Request, error) {
	req := *src
	req.Tools = slices.Clone(src.Tools)
	req.Messages = slices.Clone(src.Messages)
	type functionIdentity struct {
		namespace string
		name      string
	}
	identities := make(map[string]functionIdentity)
	register := func(namespace, name string) (string, error) {
		if namespace != "" && name == "" {
			return "", fmt.Errorf("%w: namespace function name is empty", transformer.ErrInvalidRequest)
		}
		flat := flatFunctionName(namespace, name)
		identity := functionIdentity{namespace: namespace, name: name}
		if previous, exists := identities[flat]; exists && previous != identity {
			return "", fmt.Errorf("%w: namespace tool flat name %q conflicts with another function", transformer.ErrInvalidRequest, flat)
		}
		identities[flat] = identity
		return flat, nil
	}
	declared := make(map[string]bool)
	for i := range req.Tools {
		tool := &req.Tools[i]
		if tool.Type != llm.ToolTypeFunction {
			continue
		}
		flat, err := register(tool.Function.Namespace, tool.Function.Name)
		if err != nil {
			return nil, err
		}
		if declared[flat] {
			return nil, fmt.Errorf("%w: duplicate function tool name %q", transformer.ErrInvalidRequest, flat)
		}
		declared[flat] = true
		tool.Function.Name = flat
	}
	for i := range req.Messages {
		req.Messages[i].ToolCalls = slices.Clone(src.Messages[i].ToolCalls)
		for j := range req.Messages[i].ToolCalls {
			call := &req.Messages[i].ToolCalls[j]
			if call.Type != llm.ToolTypeFunction && call.Type != "" {
				continue
			}
			flat, err := register(call.Function.Namespace, call.Function.Name)
			if err != nil {
				return nil, err
			}
			call.Function.Name = flat
		}
	}
	return &req, nil
}

const namespaceMappingMetadataKey = "namespace_mapping"

// namespaceMetadata records the inbound identities used to restore Responses output.
// The mapping is immutable after creation; pipeline transports it as opaque metadata.
func namespaceMetadata(src *llm.Request) map[string]any {
	if src == nil {
		return nil
	}
	var mapping map[string]string
	add := func(name, namespace string) {
		if name == "" || namespace == "" {
			return
		}
		if mapping == nil {
			mapping = make(map[string]string)
		}
		mapping[name] = namespace
	}
	for _, tool := range src.Tools {
		if tool.Type == llm.ToolTypeFunction {
			add(tool.Function.Name, tool.Function.Namespace)
		}
	}
	for _, message := range src.Messages {
		for _, call := range message.ToolCalls {
			if call.Type == llm.ToolTypeFunction || call.Type == "" {
				add(call.Function.Name, call.Function.Namespace)
			}
		}
	}
	metadata := maps.Clone(src.TransformerMetadata)
	delete(metadata, namespaceMappingMetadataKey)
	if len(mapping) > 0 {
		if metadata == nil {
			metadata = make(map[string]any)
		}
		metadata[namespaceMappingMetadataKey] = mapping
	}
	return metadata
}

// namespaceForName accepts both the original map and its JSON-decoded form.
func namespaceForName(metadata map[string]any, name string) string {
	switch mapping := metadata[namespaceMappingMetadataKey].(type) {
	case map[string]string:
		return mapping[name]
	case map[string]any:
		namespace, _ := mapping[name].(string)
		return namespace
	default:
		return ""
	}
}

// mapResponseFunctionNames copies only the edited portions of a response. Stream
// decoders may retain their own tool calls or return the same Current repeatedly.
func mapResponseFunctionNames(src *llm.Response, nativeToUnified bool) *llm.Response {
	if src == nil || src == llm.DoneResponse {
		return src
	}
	metadata := src.TransformerMetadata
	convert := func(src *llm.Message) *llm.Message {
		if src == nil {
			return nil
		}
		result := src
		for i, call := range src.ToolCalls {
			if call.Type != "function" && call.Type != "" {
				continue
			}
			function := call.Function
			if nativeToUnified {
				function.Name = flatFunctionName(function.Namespace, function.Name)
			} else {
				if function.Namespace == "" {
					function.Namespace = namespaceForName(metadata, function.Name)
				}
				function.Name = localFunctionName(function.Namespace, function.Name)
			}
			if function == call.Function {
				continue
			}
			if result == src {
				copied := *src
				copied.ToolCalls = slices.Clone(src.ToolCalls)
				result = &copied
			}
			result.ToolCalls[i].Function = function
		}
		return result
	}
	result := src
	for i, choice := range src.Choices {
		message, delta := convert(choice.Message), convert(choice.Delta)
		if message == choice.Message && delta == choice.Delta {
			continue
		}
		if result == src {
			copied := *src
			copied.Choices = slices.Clone(src.Choices)
			result = &copied
		}
		result.Choices[i].Message, result.Choices[i].Delta = message, delta
	}
	return result
}
