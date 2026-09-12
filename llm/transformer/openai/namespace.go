package openai

import (
	"fmt"
	"slices"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// namespaceToolMapping is private to one Chat outbound attempt. Unified
// requests and responses carry the original name and namespace as fields.
type namespaceToolMapping map[string]llm.ToolFunction

const namespaceToolMappingMetadataKey = "openai_chat_namespace_tool_mapping"

// PrepareNamespaceRequest prepares a private Chat request and the metadata that
// must accompany its HTTP request. Chat-compatible adapters that override the
// base request builder use this same boundary for naming and restoration.
func PrepareNamespaceRequest(src *llm.Request) (*llm.Request, map[string]any, error) {
	request, mapping, err := prepareNamespaceRequest(src)
	if err != nil {
		return nil, nil, err
	}
	if len(mapping) == 0 {
		return request, nil, nil
	}
	return request, map[string]any{namespaceToolMappingMetadataKey: mapping}, nil
}

// prepareNamespaceRequest flattens identities only on a copy for Chat. The
// original request remains reusable by retries and Responses channel switches.
func prepareNamespaceRequest(src *llm.Request) (*llm.Request, namespaceToolMapping, error) {
	if err := transformer.ValidateFlatFunctionNames(src); err != nil {
		return nil, nil, err
	}
	req := *src
	req.Tools = slices.Clone(src.Tools)
	mapping := make(namespaceToolMapping)
	register := func(namespace, name string) string {
		flat := llm.FlattenFunctionName(namespace, name)
		if namespace != "" {
			mapping[flat] = llm.ToolFunction{Namespace: namespace, Name: name}
		}
		return flat
	}
	for i := range req.Tools {
		tool := &req.Tools[i]
		if tool.Type != llm.ToolTypeFunction {
			continue
		}
		tool.Function.Name = register(tool.Function.Namespace, tool.Function.Name)
		tool.Function.Namespace = ""
	}
	req.Messages = slices.Clone(src.Messages)
	for i := range req.Messages {
		req.Messages[i].ToolCalls = slices.Clone(src.Messages[i].ToolCalls)
		for j := range req.Messages[i].ToolCalls {
			tc := &req.Messages[i].ToolCalls[j]
			if tc.Type != "function" && tc.Type != "" {
				continue
			}
			tc.Function.Name = register(tc.Function.Namespace, tc.Function.Name)
			tc.Function.Namespace = ""
		}
	}
	choice, err := chatNamespaceToolChoice(src.ToolChoice, src.Tools)
	if err != nil {
		return nil, nil, err
	}
	req.ToolChoice = choice
	if len(mapping) == 0 {
		mapping = nil
	}
	return &req, mapping, nil
}

// chatNamespaceToolChoice rejects constraints that cannot select exactly one
// Chat function; this validation must not run in the shared inbound path.
func chatNamespaceToolChoice(src *llm.ToolChoice, tools []llm.Tool) (*llm.ToolChoice, error) {
	src = llm.ResolveSingleFunctionChoice(src, tools)
	if src == nil {
		return nil, nil
	}
	result := *src
	if len(src.Tools) > 0 {
		return nil, fmt.Errorf("%w: tool_choice.tools cannot be represented as a single OpenAI Chat Completions function", transformer.ErrInvalidRequest)
	}
	if src.NamedToolChoice == nil {
		return &result, nil
	}
	named := *src.NamedToolChoice
	if named.Type != llm.ToolTypeFunction {
		return nil, fmt.Errorf("%w: tool choice %q of type %q cannot be represented as a single OpenAI Chat Completions function", transformer.ErrInvalidRequest, named.Function.Name, named.Type)
	}
	if named.Function.Namespace != "" {
		found := false
		for _, tool := range tools {
			if tool.Type == llm.ToolTypeFunction && tool.Function.Name == named.Function.Name && tool.Function.Namespace == named.Function.Namespace {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: unknown namespace function choice %q in %q", transformer.ErrInvalidRequest, named.Function.Name, named.Function.Namespace)
		}
		named.Function.Name = llm.FlattenFunctionName(named.Function.Namespace, named.Function.Name)
		named.Function.Namespace = ""
	}
	result.NamedToolChoice = &named
	return &result, nil
}

// RestoreNamespaceResponse restores standard tool identities for adapters with
// their own Chat response decoder. Pass the HTTP request from the same attempt.
func RestoreNamespaceResponse(req *httpclient.Request, resp *llm.Response) {
	restoreNamespaceResponse(resp, extractNamespaceToolMapping(req))
}

// restoreNamespaceResponse resolves exact names, including identity that first
// appears in a later stream delta. It never guesses by splitting underscores.
func restoreNamespaceResponse(resp *llm.Response, mapping namespaceToolMapping) {
	if resp == nil || resp == llm.DoneResponse || len(mapping) == 0 {
		return
	}
	for i := range resp.Choices {
		for _, message := range []*llm.Message{resp.Choices[i].Message, resp.Choices[i].Delta} {
			if message == nil {
				continue
			}
			for j := range message.ToolCalls {
				fc := &message.ToolCalls[j].Function
				if ref, ok := mapping[fc.Name]; ok {
					fc.Name = ref.Name
					fc.Namespace = ref.Namespace
				}
			}
		}
	}
}
