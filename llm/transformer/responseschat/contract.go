// Package responseschat contains reusable provider acceptance helpers for the
// Responses-to-Chat tool lifecycle.
package responseschat

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
	responsesapi "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// Outbound is the subset needed to verify a provider's real Responses-to-Chat
// tool lifecycle rather than only its capability bit.
type Outbound interface {
	TransformRequest(context.Context, *llm.Request) (*httpclient.Request, error)
	TransformResponse(context.Context, *httpclient.Response) (*llm.Response, error)
	TransformStream(context.Context, *httpclient.Request, streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error)
}

// RequireToolLifecycle verifies request conversion, mapping metadata, non-stream
// restoration, and fragmented stream restoration for namespace and custom tools.
func RequireToolLifecycle(t *testing.T, outbound Outbound, model string) {
	t.Helper()

	httpRequest := transformLifecycleRequest(t, outbound, model, false)
	requireLifecycleWire(t, httpRequest)
	requireNonStreamRestoration(t, outbound, httpRequest, model)

	streamRequest := transformLifecycleRequest(t, outbound, model, true)
	requireLifecycleWire(t, streamRequest)
	requireFragmentedStreamRestoration(t, outbound, streamRequest, model)
}

func transformLifecycleRequest(t *testing.T, outbound Outbound, model string, stream bool) *httpclient.Request {
	t.Helper()

	request := &llm.Request{
		Model:     model,
		APIFormat: llm.APIFormatOpenAIResponse,
		Stream:    lo.ToPtr(stream),
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("use tools")}},
		},
		Tools: []llm.Tool{
			{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name: "functions__lookup", Namespace: "functions", Parameters: json.RawMessage("{\"type\":\"object\"}"),
				},
			},
			{
				Type: llm.ToolTypeResponsesCustomTool,
				ResponseCustomTool: &llm.ResponseCustomTool{
					Name: "apply_patch", Description: "Apply a patch",
				},
			},
		},
	}

	httpRequest, err := outbound.TransformRequest(t.Context(), request)
	require.NoError(t, err)
	require.NotNil(t, httpRequest)
	require.Equal(t, string(llm.APIFormatOpenAIChatCompletion), httpRequest.APIFormat)
	require.Contains(t, httpRequest.TransformerMetadata, openai.ResponsesChatToolMappingsMetadataKey)
	require.Contains(t, httpRequest.TransformerMetadata, openai.ResponsesChatToolCatalogMetadataKey)
	require.Equal(t, llm.ToolTypeFunction, request.Tools[0].Type)
	require.Equal(t, "functions", request.Tools[0].Function.Namespace)
	require.Equal(t, llm.ToolTypeResponsesCustomTool, request.Tools[1].Type)
	return httpRequest
}

func requireLifecycleWire(t *testing.T, httpRequest *httpclient.Request) {
	t.Helper()

	var wire openai.Request
	require.NoError(t, json.Unmarshal(httpRequest.Body, &wire))
	require.Len(t, wire.Tools, 2)
	require.Equal(t, llm.ToolTypeFunction, wire.Tools[0].Type)
	require.Equal(t, "functions__lookup", wire.Tools[0].Function.Name)
	require.Equal(t, llm.ToolTypeFunction, wire.Tools[1].Type)
	require.Equal(t, "apply_patch", wire.Tools[1].Function.Name)
}

func requireNonStreamRestoration(t *testing.T, outbound Outbound, httpRequest *httpclient.Request, model string) {
	t.Helper()

	responseBody := []byte("{\"id\":\"chatcmpl_1\",\"object\":\"chat.completion\",\"model\":\"" + model + "\",\"choices\":[{\"index\":0,\"message\":{\"role\":\"assistant\",\"tool_calls\":[{\"id\":\"call_lookup\",\"type\":\"function\",\"function\":{\"name\":\"functions__lookup\",\"arguments\":\"{\\\"query\\\":\\\"axon\\\"}\"}},{\"id\":\"call_patch\",\"type\":\"function\",\"function\":{\"name\":\"apply_patch\",\"arguments\":\"{\\\"input\\\":\\\"patch\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}")
	response, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Request:    httpRequest,
		Body:       responseBody,
	})
	require.NoError(t, err)
	require.Len(t, response.Choices, 1)
	require.NotNil(t, response.Choices[0].Message)
	require.Len(t, response.Choices[0].Message.ToolCalls, 2)

	lookup := response.Choices[0].Message.ToolCalls[0]
	require.Equal(t, llm.ToolTypeFunction, lookup.Type)
	require.Equal(t, "lookup", lookup.Function.Name)
	require.Equal(t, "functions", lookup.Function.Namespace)

	patch := response.Choices[0].Message.ToolCalls[1]
	require.Equal(t, llm.ToolTypeResponsesCustomTool, patch.Type)
	require.NotNil(t, patch.ResponseCustomToolCall)
	require.Equal(t, "apply_patch", patch.ResponseCustomToolCall.Name)
	require.Equal(t, "patch", patch.ResponseCustomToolCall.Input)
}

func requireFragmentedStreamRestoration(t *testing.T, outbound Outbound, httpRequest *httpclient.Request, model string) {
	t.Helper()

	streamEvents := []*httpclient.StreamEvent{
		{Data: marshalLifecycleStreamEvent(t, model, lifecycleStreamToolCall{
			Index: 0, ID: "call_patch", Type: "function",
			Function: lifecycleStreamFunction{Name: "apply_patch", Arguments: `{"input":"pat`},
		}, "")},
		{Data: marshalLifecycleStreamEvent(t, model, lifecycleStreamToolCall{
			Index: 0, Function: lifecycleStreamFunction{Arguments: `ch"}`},
		}, "")},
		{Data: marshalLifecycleStreamEvent(t, model, lifecycleStreamToolCall{
			Index: 1, ID: "call_lookup", Type: "function",
			Function: lifecycleStreamFunction{Name: "functions__lookup", Arguments: `{"query":"axon"}`},
		}, "tool_calls")},
		{Data: []byte("[DONE]")},
	}
	providerStream, err := outbound.TransformStream(t.Context(), httpRequest, streams.SliceStream(streamEvents))
	require.NoError(t, err)
	var providerResponses []*llm.Response
	for providerStream.Next() {
		current := providerStream.Current()
		raw, _ := json.Marshal(current)
		t.Logf("provider response: %s", raw)
		providerResponses = append(providerResponses, current)
	}
	require.NoError(t, providerStream.Err())
	providerStream = streams.SliceStream(providerResponses)

	responseStream, err := responsesapi.NewInboundTransformer().TransformStream(t.Context(), providerStream)
	require.NoError(t, err)

	var customItem, functionItem *responsesapi.Item
	for responseStream.Next() {
		var event responsesapi.StreamEvent
		require.NoError(t, json.Unmarshal(responseStream.Current().Data, &event))
		if event.Type != responsesapi.StreamEventTypeOutputItemDone || event.Item == nil {
			continue
		}
		switch event.Item.Type {
		case "custom_tool_call":
			customItem = event.Item
		case "function_call":
			functionItem = event.Item
		}
	}
	require.NoError(t, responseStream.Err())
	require.NotNil(t, customItem)
	require.Equal(t, "apply_patch", customItem.Name)
	require.Equal(t, "patch", lo.FromPtr(customItem.Input))
	require.NotNil(t, functionItem)
	require.Equal(t, "lookup", functionItem.Name)
	require.Equal(t, "functions", functionItem.Namespace)
}

type lifecycleStreamFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type lifecycleStreamToolCall struct {
	Index    int                     `json:"index"`
	ID       string                  `json:"id,omitempty"`
	Type     string                  `json:"type,omitempty"`
	Function lifecycleStreamFunction `json:"function"`
}

func marshalLifecycleStreamEvent(
	t *testing.T,
	model string,
	toolCall lifecycleStreamToolCall,
	finishReason string,
) []byte {
	t.Helper()

	choice := map[string]any{
		"index": 0,
		"delta": map[string]any{"tool_calls": []lifecycleStreamToolCall{toolCall}},
	}
	if finishReason != "" {
		choice["finish_reason"] = finishReason
	}
	data, err := json.Marshal(map[string]any{
		"id":      "chatcmpl_1",
		"object":  "chat.completion.chunk",
		"model":   model,
		"choices": []any{choice},
	})
	require.NoError(t, err)
	require.True(t, json.Valid(data))
	return data
}
