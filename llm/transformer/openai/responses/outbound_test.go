package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestNewOutboundTransformer(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		baseURL     string
		expectError bool
	}{
		{
			name:        "valid parameters",
			apiKey:      "test-api-key",
			baseURL:     "https://api.openai.com",
			expectError: false,
		},
		{
			name:        "empty api key",
			apiKey:      "",
			baseURL:     "https://api.openai.com",
			expectError: true,
		},
		{
			name:        "empty base url",
			apiKey:      "test-api-key",
			baseURL:     "",
			expectError: true,
		},
		{
			name:        "base url with trailing slash",
			apiKey:      "test-api-key",
			baseURL:     "https://api.openai.com/",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer(tt.baseURL, tt.apiKey)
			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, transformer)
			} else {
				require.NoError(t, err)
				require.NotNil(t, transformer)
				require.Equal(t, tt.apiKey, transformer.config.APIKeyProvider.Get(context.Background()))
				// Base URL should be normalized with v1 version
				require.Equal(t, "https://api.openai.com/v1", transformer.config.BaseURL)
			}
		})
	}
}

func TestOutboundTransformer_TransformResponse_CanceledFinishReason(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	result, err := transformer.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"id":"resp_canceled","object":"response","created_at":1700000000,"status":"canceled","model":"gpt-5","output":[]}`),
	})
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	require.NotNil(t, result.Choices[0].FinishReason)
	require.Equal(t, "cancelled", *result.Choices[0].FinishReason)
}

func TestOutboundTransformer_buildFullRequestURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		rawURL   bool
		expected string
	}{
		{
			name:     "no v1 prefix",
			baseURL:  "https://api.openai.com",
			rawURL:   false,
			expected: "https://api.openai.com/v1/responses",
		},
		{
			name:     "with v1 suffix",
			baseURL:  "https://api.openai.com/v1",
			rawURL:   false,
			expected: "https://api.openai.com/v1/responses",
		},
		{
			name:     "with v1 in path",
			baseURL:  "https://api.openai.com/v1/custom",
			rawURL:   false,
			expected: "https://api.openai.com/v1/custom/responses",
		},
		{
			name:     "raw url with # suffix",
			baseURL:  "https://api.openai.com/custom#",
			rawURL:   true,
			expected: "https://api.openai.com/custom/responses",
		},
		{
			name:     "websocket codex base with # suffix",
			baseURL:  "wss://chatgpt.com/backend-api/codex#",
			rawURL:   true,
			expected: "wss://chatgpt.com/backend-api/codex/responses",
		},
		{
			name:     "raw url with explicit config",
			baseURL:  "https://api.openai.com/custom#",
			rawURL:   true,
			expected: "https://api.openai.com/custom/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				transformer *OutboundTransformer
				err         error
			)

			if tt.rawURL && strings.HasSuffix(tt.baseURL, "#") {
				transformer, err = NewOutboundTransformer(tt.baseURL, "test-key")
			} else {
				transformer, err = NewOutboundTransformerWithConfig(&Config{
					BaseURL:        tt.baseURL,
					APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
					RawURL:         tt.rawURL,
				})
			}

			require.NoError(t, err)

			url, err := transformer.buildFullRequestURL(nil)
			require.NoError(t, err)
			require.Equal(t, tt.expected, url)
		})
	}
}

func TestOutboundTransformer_APIFormat(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.Equal(t, llm.APIFormatOpenAIResponse, transformer.APIFormat())
}

func TestOutboundTransformer_TransformRequest_AccountIdentity(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.openai.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	})
	require.NoError(t, err)

	req := &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
		},
	}

	hreq, err := transformer.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.Nil(t, hreq.Metadata)
}

func TestOutboundTransformer_TransformRequest_OmitsMetadataWhenEmpty(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.openai.com",
		APIKeyProvider: auth.NewStaticKeyProvider(""),
	})
	require.NoError(t, err)

	req := &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
		},
	}

	hreq, err := transformer.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.Nil(t, hreq.Metadata)
}

func TestOutboundTransformer_TransformRequest_WebSearchRequiredToolChoice(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model: "gpt-4o-search-preview",
		Messages: []llm.Message{{
			Role: "user",
			Content: llm.MessageContent{
				Content: lo.ToPtr("latest ai news"),
			},
		}},
		Tools: []llm.Tool{{
			Type: llm.ToolTypeWebSearch,
		}},
		ToolChoice: &llm.ToolChoice{
			ToolChoice: lo.ToPtr("required"),
		},
	}

	hreq, err := transformer.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(hreq.Body, &payload)
	require.NoError(t, err)
	require.Equal(t, "required", payload["tool_choice"])
}

func roundTripResponsesRequestPayload(t *testing.T, body string, mutate func(*llm.Request)) (map[string]any, *llm.Request) {
	t.Helper()

	llmReq, httpReq := roundTripResponsesRequest(t, body, mutate)

	var payload map[string]any
	err := json.Unmarshal(httpReq.Body, &payload)
	require.NoError(t, err)
	return payload, llmReq
}

func roundTripResponsesRawPayload(t *testing.T, body string, mutate func(*llm.Request)) (map[string]json.RawMessage, *llm.Request) {
	t.Helper()

	llmReq, httpReq := roundTripResponsesRequest(t, body, mutate)

	var payload map[string]json.RawMessage
	err := json.Unmarshal(httpReq.Body, &payload)
	require.NoError(t, err)
	return payload, llmReq
}

func roundTripResponsesRequest(t *testing.T, body string, mutate func(*llm.Request)) (*llm.Request, *httpclient.Request) {
	t.Helper()

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(body)})
	require.NoError(t, err)

	if mutate != nil {
		mutate(llmReq)
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	return llmReq, httpReq
}

func TestOutboundTransformer_TransformRequest_RoundTripsBasicResponsesRequest(t *testing.T) {
	payload, _ := roundTripResponsesRequestPayload(t, `{
		"model": "gpt-4o",
		"input": "hello",
		"stream": true
	}`, func(llmReq *llm.Request) {
		llmReq.Model = "mapped-model"
	})

	require.Equal(t, "mapped-model", payload["model"])
	require.Equal(t, "hello", payload["input"])
	require.Equal(t, true, payload["stream"])
}

func TestOutboundTransformer_TransformRequest_PreservesUnknownTopLevelResponsesFields(t *testing.T) {
	payload, _ := roundTripResponsesRawPayload(t, `{
		"model": "gpt-4o",
		"input": "hello",
		"stream": true,
		"x_future_response_field": {"enabled": true, "limit": 3}
	}`, func(llmReq *llm.Request) {
		llmReq.Model = "mapped-model"
	})

	require.JSONEq(t, `{"enabled": true, "limit": 3}`, string(payload["x_future_response_field"]))
	require.JSONEq(t, `"mapped-model"`, string(payload["model"]))
}

func TestOutboundTransformer_TransformRequest_KnownFieldsOverrideRawTopLevelFallback(t *testing.T) {
	payload, _ := roundTripResponsesRawPayload(t, `{
		"model": "gpt-4o",
		"input": "hello",
		"stream": true,
		"x_future_response_field": {"enabled": true}
	}`, func(llmReq *llm.Request) {
		llmReq.Model = "mapped-model"
	})

	require.JSONEq(t, `"mapped-model"`, string(payload["model"]))
	require.JSONEq(t, `{"enabled": true}`, string(payload["x_future_response_field"]))
}

func TestOutboundTransformer_TransformRequest_PreservesClientMetadataSeparatelyFromMetadata(t *testing.T) {
	payload, llmReq := roundTripResponsesRawPayload(t, `{
		"model": "gpt-4o",
		"input": "hello",
		"metadata": {"trace": "model-visible"},
		"client_metadata": {"codex_version": "1.2.3", "session_id": "session-123"}
	}`, nil)

	require.NotNil(t, llmReq.ProviderExtensions)
	require.NotNil(t, llmReq.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, llmReq.ProviderExtensions.OpenAIResponses.Request)
	require.Equal(t, map[string]string{
		"codex_version": "1.2.3",
		"session_id":    "session-123",
	}, llmReq.ProviderExtensions.OpenAIResponses.Request.ClientMetadata)
	require.JSONEq(t, `{"trace":"model-visible"}`, string(payload["metadata"]))
	require.JSONEq(t, `{"codex_version":"1.2.3","session_id":"session-123"}`, string(payload["client_metadata"]))
}

func TestOutboundTransformer_TransformRequest_PreservesNamespaceToolStructure(t *testing.T) {
	payload, _ := roundTripResponsesRequestPayload(t, `{
		"model": "gpt-4o",
		"input": "use the docs search",
		"tools": [
			{
				"type": "namespace",
				"name": "docs",
				"description": "Documentation tools",
				"tools": [
					{"type": "function", "name": "search", "description": "Search docs", "parameters": {"type": "object", "properties": {}}}
				]
			}
		]
	}`, nil)

	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)

	namespaceTool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "namespace", namespaceTool["type"])
	require.Equal(t, "docs", namespaceTool["name"])

	childTools, ok := namespaceTool["tools"].([]any)
	require.True(t, ok)
	require.Len(t, childTools, 1)
	child, ok := childTools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", child["type"])
	require.Equal(t, "search", child["name"])
	require.NotContains(t, child["name"], "docs__")
}

func TestOutboundTransformer_TransformRequest_PreservesToolSearchDeclaration(t *testing.T) {
	payload, _ := roundTripResponsesRequestPayload(t, `{
		"model": "gpt-4o",
		"input": "search docs",
		"tools": [
			{
				"type": "tool_search",
				"name": "search_docs",
				"namespace": "docs",
				"description": "Search documentation",
				"execution": {"type": "server"},
				"parameters": {"type": "object", "properties": {"query": {"type": "string"}}},
				"x_vendor_hint": "keep-me"
			}
		]
	}`, nil)

	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_search", tool["type"])
	require.Equal(t, "search_docs", tool["name"])
	require.Equal(t, "docs", tool["namespace"])
	require.Equal(t, "Search documentation", tool["description"])
	require.Equal(t, "keep-me", tool["x_vendor_hint"])
	require.Contains(t, tool, "execution")
	require.Contains(t, tool, "parameters")
}

func TestOutboundTransformer_TransformRequest_PreservesDeferLoadingOnFunctionTools(t *testing.T) {
	payload, _ := roundTripResponsesRequestPayload(t, `{
		"model": "gpt-4o",
		"input": "load tools lazily",
		"tools": [
			{"type": "function", "name": "get_weather", "defer_loading": true, "parameters": {"type": "object", "properties": {}}}
		]
	}`, nil)

	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, tool["defer_loading"])
}

func TestOutboundTransformer_TransformRequest_PreservesDeferLoadingOnNamespaceChildTools(t *testing.T) {
	payload, _ := roundTripResponsesRequestPayload(t, `{
		"model": "gpt-4o",
		"input": "load namespace tools lazily",
		"tools": [
			{"type": "namespace", "name": "docs", "tools": [
				{"type": "function", "name": "search", "defer_loading": true, "parameters": {"type": "object", "properties": {}}}
			]}
		]
	}`, nil)

	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	namespaceTool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	children, ok := namespaceTool["tools"].([]any)
	require.True(t, ok)
	require.Len(t, children, 1)
	child, ok := children[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, child["defer_loading"])
}

func TestOutboundTransformer_TransformRequest_PreservesAdditionalToolsInputItems(t *testing.T) {
	payload, llmReq := roundTripResponsesRequestPayload(t, `{
		"model": "gpt-4o",
		"input": [
			{
				"type": "additional_tools",
				"x_reason": "lazy-load",
				"tools": [
					{"type": "namespace", "name": "docs", "tools": [{"type": "function", "name": "search", "parameters": {"type": "object", "properties": {}}}]},
					{"type": "tool_search", "name": "search_docs", "namespace": "docs"}
				]
			},
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "hello"}]}
		]
	}`, nil)

	require.NotNil(t, llmReq.ProviderExtensions)
	require.NotNil(t, llmReq.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, llmReq.ProviderExtensions.OpenAIResponses.Request)
	require.Len(t, llmReq.ProviderExtensions.OpenAIResponses.Request.AdditionalTools, 1)
	require.JSONEq(t, `{"type":"additional_tools","x_reason":"lazy-load","tools":[{"type":"namespace","name":"docs","tools":[{"type":"function","name":"search","parameters":{"type":"object","properties":{}}}]},{"type":"tool_search","name":"search_docs","namespace":"docs"}]}`, string(llmReq.ProviderExtensions.OpenAIResponses.Request.AdditionalTools[0].Raw))
	require.Empty(t, llmReq.ProviderExtensions.OpenAIResponses.Request.RawInputItems)

	input, ok := payload["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)
	additionalTools, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "additional_tools", additionalTools["type"])
	require.Equal(t, "lazy-load", additionalTools["x_reason"])
	nestedTools, ok := additionalTools["tools"].([]any)
	require.True(t, ok)
	require.Len(t, nestedTools, 2)
	namespaceTool, ok := nestedTools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "namespace", namespaceTool["type"])
	toolSearch, ok := nestedTools[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_search", toolSearch["type"])
}

func TestOutboundTransformer_TransformRequest_PreservesUnknownToolVariants(t *testing.T) {
	payload, _ := roundTripResponsesRawPayload(t, `{
		"model": "gpt-4o",
		"input": "use future tool",
		"tools": [
			{"type": "future_tool", "name": "future", "nested": {"enabled": true}, "list": [1, 2]}
		]
	}`, nil)

	var tools []json.RawMessage
	err := json.Unmarshal(payload["tools"], &tools)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.JSONEq(t, `{"type":"future_tool","name":"future","nested":{"enabled":true},"list":[1,2]}`, string(tools[0]))
}

func TestOutboundTransformer_TransformRequest_PreservesAdditionalToolsAlongsideRawOnlyInputItems(t *testing.T) {
	payload, llmReq := roundTripResponsesRequestPayload(t, `{
		"model": "gpt-4o",
		"input": [
			{"type": "additional_tools", "tools": [{"type": "tool_search", "name": "search_docs"}]},
			{"type": "tool_search_call", "id": "ts_1", "status": "completed", "queries": ["docs"]},
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "hello"}]}
		]
	}`, nil)

	requestExt := llmReq.ProviderExtensions.OpenAIResponses.Request
	require.Len(t, requestExt.AdditionalTools, 1)
	require.Len(t, requestExt.RawInputItems, 1)
	require.Equal(t, "tool_search_call", requestExt.RawInputItems[0].Type)

	input, ok := payload["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 3)
	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "additional_tools", first["type"])
	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_search_call", second["type"])
	third, ok := input[2].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "message", third["type"])
}

func TestOutboundTransformer_TransformRequest_PreservesUnknownInputItemVariants(t *testing.T) {
	payload, _ := roundTripResponsesRawPayload(t, `{
		"model": "gpt-4o",
		"input": [
			{"type": "future_input_item", "id": "item_1", "payload": {"enabled": true}, "list": ["a", "b"]},
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "hello"}]}
		]
	}`, nil)

	var input []json.RawMessage
	err := json.Unmarshal(payload["input"], &input)
	require.NoError(t, err)
	require.Len(t, input, 2)
	require.JSONEq(t, `{"type":"future_input_item","id":"item_1","payload":{"enabled":true},"list":["a","b"]}`, string(input[0]))
}

func TestOutboundTransformer_TransformRequest_PreservesComplexToolChoiceRawForm(t *testing.T) {
	payload, _ := roundTripResponsesRawPayload(t, `{
		"model": "gpt-4o",
		"input": "use future choice",
		"tools": [
			{"type": "function", "name": "get_weather", "parameters": {"type": "object", "properties": {}}}
		],
		"tool_choice": {
			"type": "future_choice",
			"name": "get_weather",
			"mode": "auto",
			"x_policy": {"allow": ["get_weather"]}
		}
	}`, nil)

	require.JSONEq(t, `{"type":"future_choice","name":"get_weather","mode":"auto","x_policy":{"allow":["get_weather"]}}`, string(payload["tool_choice"]))
}

func TestOutboundTransformer_TransformRequest_EmitsClientMetadataPreservationDiagnostics(t *testing.T) {
	_, httpReq := roundTripResponsesRequest(t, `{
		"model": "gpt-4o",
		"input": "hello",
		"client_metadata": {"codex_version": "1.2.3"}
	}`, nil)

	require.NotNil(t, httpReq.TransformerMetadata)
	diagnostics, ok := httpReq.TransformerMetadata[responsesRequestPreservationDiagnosticsTransformerMetadataKey].(requestPreservationDiagnostics)
	require.True(t, ok)
	require.True(t, diagnostics.NativePreservation)
	require.Equal(t, 1, diagnostics.ClientMetadataCount)
}

func TestOutboundTransformer_TransformRequest_EmitsRequestPreservationDiagnostics(t *testing.T) {
	_, httpReq := roundTripResponsesRequest(t, `{
		"model": "gpt-4o",
		"input": [
			{"type": "additional_tools", "tools": [{"type": "tool_search", "name": "search_docs"}]},
			{"type": "message", "role": "user", "content": [{"type":"input_text", "text":"hello"}]}
		],
		"tools": [
			{"type": "tool_search", "name": "search_docs", "namespace": "docs"}
		],
		"tool_choice": {"type": "tool_search", "tools": [{"type": "tool_search", "name": "search_docs"}]},
		"x_future_response_field": true
	}`, nil)

	require.NotNil(t, httpReq.TransformerMetadata)
	diagnostics, ok := httpReq.TransformerMetadata[responsesRequestPreservationDiagnosticsTransformerMetadataKey].(requestPreservationDiagnostics)
	require.True(t, ok)
	require.True(t, diagnostics.NativePreservation)
	require.Equal(t, 1, diagnostics.UnknownTopLevelFieldCount)
	require.Equal(t, 1, diagnostics.NativeToolCount)
	require.Equal(t, 1, diagnostics.RawOnlyToolCount)
	require.Equal(t, 1, diagnostics.AdditionalToolsCount)
	require.Equal(t, 0, diagnostics.RawInputItemCount)
	require.True(t, diagnostics.RawToolChoicePreserved)
}

func TestOutboundTransformer_TransformRequest_EmitsDetailedNativePreservationDiagnostics(t *testing.T) {
	_, httpReq := roundTripResponsesRequest(t, `{
		"model": "gpt-4o",
		"input": [
			{"type": "future_input_item", "payload": {"enabled": true}},
			{"type": "message", "role": "user", "content": [{"type":"input_text", "text":"hello"}]}
		],
		"tools": [
			{"type": "namespace", "name": "docs", "tools": [{"type":"function", "name":"search", "parameters":{"type":"object", "properties":{}}}]},
			{"type": "tool_search", "name": "search_docs", "namespace": "docs"},
			{"type": "future_tool", "name": "future", "payload": {"enabled": true}}
		],
		"x_future_response_field": true
	}`, nil)

	require.NotNil(t, httpReq.TransformerMetadata)
	diagnostics, ok := httpReq.TransformerMetadata[responsesRequestPreservationDiagnosticsTransformerMetadataKey].(requestPreservationDiagnostics)
	require.True(t, ok)
	require.True(t, diagnostics.NativePreservation)
	require.Equal(t, 1, diagnostics.UnknownTopLevelFieldCount)
	require.Equal(t, 3, diagnostics.NativeToolCount)
	require.Equal(t, 1, diagnostics.NamespaceToolCount)
	require.Equal(t, 1, diagnostics.ToolSearchToolCount)
	require.Equal(t, 1, diagnostics.UnknownToolCount)
	require.Equal(t, 1, diagnostics.RawInputItemCount)
	require.Equal(t, 1, diagnostics.UnknownInputItemCount)
}

func TestOutboundTransformer_TransformRequest_ReplaysProviderRawToolsAndToolChoice(t *testing.T) {
	inbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": "Search and run shell.",
			"tools": [
				{
					"type": "tool_search",
					"name": "search_docs",
					"namespace": "docs"
				},
				{
					"type": "function",
					"name": "get_weather",
					"parameters": {"type": "object", "properties": {}}
				}
			],
			"tool_choice": {
				"type": "tool_search",
				"tools": [
					{"type": "tool_search", "name": "search_docs"}
				]
			}
		}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	llmReq.Model = "mapped-model"

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(httpReq.Body, &payload)
	require.NoError(t, err)
	require.Equal(t, "mapped-model", payload["model"])

	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 2)
	rawTool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_search", rawTool["type"])
	require.Equal(t, "docs", rawTool["namespace"])

	toolChoice, ok := payload["tool_choice"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_search", toolChoice["type"])
	require.Len(t, toolChoice["tools"], 1)
}

func TestOutboundTransformer_TransformRequest_ReplaysProviderRawInputItems(t *testing.T) {
	inbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": [
				{
					"type": "tool_search_call",
					"call_id": "call_search",
					"status": "completed",
					"arguments": {"query":"image generation","limit":10}
				},
				{
					"type": "message",
					"role": "user",
					"content": [{"type":"input_text","text":"hello"}]
				}
			]
		}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(httpReq.Body, &payload)
	require.NoError(t, err)

	input, ok := payload["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)

	rawItem, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_search_call", rawItem["type"])
	arguments, ok := rawItem["arguments"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "image generation", arguments["query"])
	require.Equal(t, float64(10), arguments["limit"])

	message, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "message", message["type"])
}

func TestOutboundTransformer_TransformRequest_DoesNotReplayRawToolWhenStructuredToolChanged(t *testing.T) {
	payload, _ := roundTripResponsesRequestPayload(t, `{
		"model": "gpt-4o",
		"input": "call weather",
		"tools": [
			{"type": "function", "name": "get_weather", "description": "old", "parameters": {"type": "object", "properties": {}}}
		]
	}`, func(llmReq *llm.Request) {
		require.Len(t, llmReq.Tools, 1)
		llmReq.Tools[0].Function.Description = "new"
		llmReq.Tools[0].Function.Parameters = json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)
	})

	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "new", tool["description"])
	parameters, ok := tool["parameters"].(map[string]any)
	require.True(t, ok)
	properties, ok := parameters["properties"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, properties, "city")
}

func TestOutboundTransformer_TransformRequest_DoesNotReplayRawToolWhenToolsChanged(t *testing.T) {
	inbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": "Search and run shell.",
			"tools": [
				{"type": "tool_search", "name": "search_docs", "namespace": "docs"},
				{"type": "function", "name": "get_weather", "parameters": {"type": "object", "properties": {}}}
			]
		}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	llmReq.Tools = []llm.Tool{{
		Type: "function",
		Function: llm.Function{
			Name:       "different_tool",
			Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(httpReq.Body, &payload)
	require.NoError(t, err)

	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", tool["type"])
	require.Equal(t, "different_tool", tool["name"])
}

func TestProviderExtensions_NotSerializedWithLLMRequest(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("hi")},
		}},
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Request: &llm.OpenAIResponsesRequestExtensions{
					ClientMetadata:    map[string]string{"secret": "client metadata"},
					RawTopLevelFields: map[string]json.RawMessage{"secret_top": json.RawMessage(`{"secret":"top level"}`)},
					NativeTools: &llm.OpenAIResponsesNativeTools{
						Raw:        []json.RawMessage{json.RawMessage(`{"secret":"native tool"}`)},
						Signatures: []string{"function:get_weather"},
					},
					RawTools: []llm.OpenAIResponsesRawFragment{{
						Type: "tool_search",
						Raw:  json.RawMessage(`{"secret":"raw prompt"}`),
					}},
					RawToolChoice: json.RawMessage(`{"secret":"raw choice"}`),
				},
			},
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)
	serialized := string(data)
	require.NotContains(t, serialized, "client metadata")
	require.NotContains(t, serialized, "top level")
	require.NotContains(t, serialized, "native tool")
	require.NotContains(t, serialized, "raw prompt")
	require.NotContains(t, serialized, "raw choice")
	require.NotContains(t, serialized, "provider_extensions")
}

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")

	tests := []struct {
		name        string
		chatReq     *llm.Request
		expectError bool
		validate    func(t *testing.T, result *httpclient.Request, chatReq *llm.Request)
	}{
		{
			name:        "nil request",
			chatReq:     nil,
			expectError: true,
		},
		{
			name: "simple text request",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				require.Equal(t, http.MethodPost, result.Method)
				require.Equal(t, "https://api.openai.com/v1/responses", result.URL)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.Equal(t, "application/json", result.Headers.Get("Accept"))
				require.NotNil(t, result.Auth)
				require.Equal(t, "bearer", result.Auth.Type)
				require.Equal(t, "test-api-key", result.Auth.APIKey)

				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Equal(t, chatReq.Model, req.Model)
				require.Equal(t, chatReq.Messages[0].Content.Content, req.Input.Text)
			},
		},
		{
			name: "request with system message",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "system",
						Content: llm.MessageContent{
							Content: lo.ToPtr("You are a helpful assistant."),
						},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello!"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Equal(t, "You are a helpful assistant.", req.Instructions)
			},
		},
		{
			name: "request with multimodal content",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("What's in this image?"),
								},
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "data:image/jpeg;base64,/9j/4AAQSkZJRg...",
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request with image generation tool",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Generate an image of a cat"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: llm.ToolTypeImageGeneration,
						ImageGeneration: &llm.ImageGeneration{
							Quality:           "high",
							Size:              "1024x1024",
							OutputFormat:      "png",
							OutputCompression: func() *int64 { v := int64(80); return &v }(),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Len(t, req.Tools, 1)
				require.Equal(t, llm.ToolTypeImageGeneration, req.Tools[0].Type)
				require.Equal(t, "high", req.Tools[0].Quality)
				require.Equal(t, "1024x1024", req.Tools[0].Size)
				require.Equal(t, "png", req.Tools[0].OutputFormat)
				require.Equal(t, int64(80), *req.Tools[0].OutputCompression)
			},
		},
		{
			name: "request with web search tool",
			chatReq: &llm.Request{
				Model: "gpt-4o-search-preview",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("latest ai news"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: llm.ToolTypeWebSearch,
						WebSearch: &llm.WebSearch{
							AllowedDomains: []string{"openai.com"},
							UserLocation: llm.WebSearchToolUserLocation{
								Type:    "approximate",
								Country: "US",
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Equal(t, []Tool{
					{
						Type: "web_search",
						Filters: &WebSearchFilters{
							AllowedDomains: []string{"openai.com"},
						},
						UserLocation: &WebSearchUserLocation{
							Type:    "approximate",
							Country: "US",
						},
					},
				}, req.Tools)
			},
		},
		{
			name: "request with google search tool maps to web_search",
			chatReq: &llm.Request{
				Model: "gpt-5.4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Search the web for the latest AI announcement."),
						},
					},
				},
				Tools: []llm.Tool{{
					Type: llm.ToolTypeGoogleSearch,
					Google: &llm.GoogleTools{
						Search: &llm.GoogleSearch{},
					},
				}},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var raw map[string]any

				err := json.Unmarshal(result.Body, &raw)
				require.NoError(t, err)

				tools, ok := raw["tools"].([]any)
				require.True(t, ok)
				require.Len(t, tools, 1)

				tool, ok := tools[0].(map[string]any)
				require.True(t, ok)
				require.Equal(t, llm.ToolTypeWebSearch, tool["type"])
			},
		},
		{
			name: "request with unsupported tool type is skipped",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "unsupported_tool",
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				// Unsupported tools should be skipped
				require.Len(t, req.Tools, 0)
			},
		},
		{
			name: "request with function tool",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("What's the weather?"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "function",
						Function: llm.Function{
							Name:        "get_weather",
							Description: "Get weather information",
							Parameters:  []byte(`{"type":"object","properties":{"location":{"type":"string"}}}`),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Len(t, req.Tools, 1)
				require.Equal(t, "function", req.Tools[0].Type)
				require.Equal(t, "get_weather", req.Tools[0].Name)
				require.Equal(t, "Get weather information", req.Tools[0].Description)
			},
		},
		{
			name: "request with zero-arg function tool normalizes empty object schema",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Run the tool"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "function",
						Function: llm.Function{
							Name:        "ping",
							Description: "Ping tool",
							Parameters:  []byte(`{"type":"object"}`),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Len(t, req.Tools, 1)
				require.Equal(t, "object", req.Tools[0].Parameters["type"])
				require.Equal(t, map[string]any{}, req.Tools[0].Parameters["properties"])
			},
		},
		{
			name: "request with reasoning effort and budget - budget preserved for round-trip",
			chatReq: &llm.Request{
				Model:           "o3",
				ReasoningEffort: "high",
				ReasoningBudget: lo.ToPtr(int64(5000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Solve this problem"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Reasoning)
				require.Equal(t, "high", req.Reasoning.Effort)
				// effort present alongside budget: effort wins, max_tokens omitted
				require.Nil(t, req.Reasoning.MaxTokens)
			},
		},
		{
			name: "request with reasoning effort only",
			chatReq: &llm.Request{
				Model:           "o3",
				ReasoningEffort: "medium",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Solve this problem"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Reasoning)
				require.Equal(t, "medium", req.Reasoning.Effort)
				require.Nil(t, req.Reasoning.MaxTokens)
			},
		},
		{
			name: "request with reasoning budget only",
			chatReq: &llm.Request{
				Model:           "o3",
				ReasoningBudget: lo.ToPtr(int64(3000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Solve this problem"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Reasoning)
				require.Empty(t, req.Reasoning.Effort)
				require.NotNil(t, req.Reasoning.MaxTokens)
				require.Equal(t, int64(3000), *req.Reasoning.MaxTokens)
			},
		},
		{
			name: "request with tool choice auto",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				ToolChoice: &llm.ToolChoice{
					ToolChoice: lo.ToPtr("auto"),
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.ToolChoice)
				require.NotNil(t, req.ToolChoice.Mode)
				require.Equal(t, "auto", *req.ToolChoice.Mode)
			},
		},
		{
			name: "request with top_p and top_logprobs",
			chatReq: &llm.Request{
				Model:       "gpt-4o",
				TopP:        lo.ToPtr(0.9),
				TopLogprobs: lo.ToPtr(int64(5)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.TopP)
				require.Equal(t, 0.9, *req.TopP)
				require.NotNil(t, req.TopLogprobs)
				require.Equal(t, int64(5), *req.TopLogprobs)
			},
		},
		{
			name: "request with temperature",
			chatReq: &llm.Request{
				Model:       "gpt-4o",
				Temperature: lo.ToPtr(0.7),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Temperature)
				require.Equal(t, 0.7, *req.Temperature)
			},
		},
		{
			name: "request with modalities",
			chatReq: &llm.Request{
				Model:      "gpt-4o",
				Modalities: []string{"text", "audio"},
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Equal(t, []string{"text", "audio"}, req.Modalities)
			},
		},
		{
			name: "request with background mode",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				TransformerMetadata: map[string]any{
					"background": true,
				},
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Background)
				require.True(t, *req.Background)
			},
		},
		{
			name: "request with frequency_penalty and presence_penalty",
			chatReq: &llm.Request{
				Model:            "gpt-4o",
				FrequencyPenalty: lo.ToPtr(0.5),
				PresencePenalty:  lo.ToPtr(0.3),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.FrequencyPenalty)
				require.Equal(t, 0.5, *req.FrequencyPenalty)
				require.NotNil(t, req.PresencePenalty)
				require.Equal(t, 0.3, *req.PresencePenalty)
			},
		},
		{
			name: "request with streaming enabled",
			chatReq: &llm.Request{
				Model:  "gpt-4o",
				Stream: func() *bool { v := true; return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Stream)
				require.True(t, *req.Stream)
			},
		},
		{
			name: "request with parallel tool calls",
			chatReq: &llm.Request{
				Model:             "gpt-4o",
				ParallelToolCalls: lo.ToPtr(false),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "function",
						Function: llm.Function{
							Name:        "test_function",
							Description: "Test function",
							Parameters:  []byte(`{"type":"object","properties":{}}`),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.ParallelToolCalls)
				require.False(t, *req.ParallelToolCalls)
			},
		},
		{
			name: "request with parallel tool calls but no tools",
			chatReq: &llm.Request{
				Model:             "gpt-4o",
				ParallelToolCalls: lo.ToPtr(true),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				// No tools provided
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Nil(t, req.ParallelToolCalls, "ParallelToolCalls should be nil when no tools are provided")
			},
		},
		{
			name: "request with text options",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				ResponseFormat: &llm.ResponseFormat{
					Type: "json_object",
				},
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Return JSON"; return &s }(),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Text)
			},
		},
		{
			name: "request with include field",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				TransformerMetadata: map[string]any{
					"include": []string{"file_search_call.results", "reasoning.encrypted_content"},
				},
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Include)
				require.Equal(t, []string{"file_search_call.results", "reasoning.encrypted_content"}, req.Include)
			},
		},
		{
			name: "request with previous_response_id",
			chatReq: &llm.Request{
				Model:              "gpt-5.4",
				PreviousResponseID: lo.ToPtr("resp_prev_123"),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Continue"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.PreviousResponseID)
				require.Equal(t, "resp_prev_123", *req.PreviousResponseID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformRequest(context.Background(), tt.chatReq)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result, tt.chatReq)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest_UsesSharedSessionIDAsPromptCacheKeyFallback(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	ctx := shared.WithSessionID(context.Background(), "shared-session-123")

	req := &llm.Request{
		Model: "gpt-5.4",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello"),
				},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(ctx, req)
	require.NoError(t, err)

	var payload Request

	err = json.Unmarshal(httpReq.Body, &payload)
	require.NoError(t, err)
	require.NotNil(t, payload.PromptCacheKey)
	require.Equal(t, "shared-session-123", *payload.PromptCacheKey)
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")

	tests := []struct {
		name        string
		httpResp    *httpclient.Response
		expectError bool
		validate    func(t *testing.T, result *llm.Response)
	}{
		{
			name:        "nil response",
			httpResp:    nil,
			expectError: true,
		},
		{
			name: "HTTP error status",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusBadRequest,
				Body:       []byte(`{"error": {"message": "Bad request"}}`),
			},
			expectError: true,
		},
		{
			name: "empty response body",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte{},
			},
			expectError: true,
		},
		{
			name: "invalid JSON response",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(`{invalid json}`),
			},
			expectError: true,
		},
		{
			name: "valid response with text output",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "resp_123",
					"object": "response",
					"created_at": 1759161016,
					"status": "completed",
					"model": "gpt-4o",
					"output": [
						{
							"id": "msg_123",
							"type": "message",
							"status": "completed",
							"content": [
								{
									"type": "output_text",
									"text": "Hello! How can I help you?"
								}
							],
							"role": "assistant"
						}
					],
					"usage": {
						"input_tokens": 10,
						"output_tokens": 20,
						"total_tokens": 30
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Response) {
				require.Equal(t, "chat.completion", result.Object)
				require.Equal(t, "resp_123", result.ID)
				require.Equal(t, "gpt-4o", result.Model)
				require.Len(t, result.Choices, 1)
				require.Equal(t, "assistant", result.Choices[0].Message.Role)
				require.NotNil(t, result.Choices[0].Message.Content.Content)
				require.Equal(t, "Hello! How can I help you?", *result.Choices[0].Message.Content.Content)
				require.NotNil(t, result.Usage)
				require.Equal(t, int64(10), result.Usage.PromptTokens)
				require.Equal(t, int64(20), result.Usage.CompletionTokens)
				require.Equal(t, int64(30), result.Usage.TotalTokens)
			},
		},
		{
			name: "response with image generation result",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "resp_456",
					"object": "response",
					"created_at": 1759161016,
					"status": "completed",
					"model": "gpt-4o",
					"output": [
						{
							"id": "img_123",
							"type": "image_generation_call",
							"status": "completed",
							"result": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Response) {
				require.Equal(t, "chat.completion", result.Object)
				require.Equal(t, "resp_456", result.ID)
				require.Len(t, result.Choices, 1)
				require.Equal(t, "assistant", result.Choices[0].Message.Role)
				require.Len(t, result.Choices[0].Message.Content.MultipleContent, 1)
				require.Equal(t, "image_url", result.Choices[0].Message.Content.MultipleContent[0].Type)
				require.NotNil(t, result.Choices[0].Message.Content.MultipleContent[0].ImageURL)
				require.Contains(t, result.Choices[0].Message.Content.MultipleContent[0].ImageURL.URL, "data:image/png;base64,")
			},
		},
		{
			name: "response with encrypted reasoning",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "resp_789",
					"object": "response",
					"created_at": 1759161016,
					"status": "completed",
					"model": "gpt-4o",
					"output": [
						{
							"id": "rs_123",
							"type": "reasoning",
							"summary": [],
							"encrypted_content": "encrypted_data_here"
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Response) {
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].Message)
				require.NotNil(t, result.Choices[0].Message.ReasoningSignature)
				require.Equal(t, "encrypted_data_here", *result.Choices[0].Message.ReasoningSignature)
			},
		},
		{
			name: "response with previous_response_id",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "resp_456",
					"object": "response",
					"created_at": 1759161016,
					"status": "completed",
					"model": "gpt-5.4",
					"previous_response_id": "resp_prev_123",
					"output": [
						{
							"id": "msg_456",
							"type": "message",
							"status": "completed",
							"content": [
								{
									"type": "output_text",
									"text": "Continued response"
								}
							],
							"role": "assistant"
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Response) {
				require.NotNil(t, result.PreviousResponseID)
				require.Equal(t, "resp_prev_123", *result.PreviousResponseID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(context.Background(), tt.httpResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformResponse_ServiceTierAndError(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	result, err := transformer.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body: []byte(`{
			"id": "resp_meta_123",
			"object": "response",
			"created_at": 1759161016,
			"status": "failed",
			"model": "gpt-5.4",
			"service_tier": "priority",
			"error": {
				"type": "server_error",
				"code": "upstream_failed",
				"message": "upstream provider error"
			},
			"output": []
		}`),
	})
	require.NoError(t, err)
	require.Equal(t, "priority", result.ServiceTier)
	require.NotNil(t, result.Error)
	require.Equal(t, "server_error", result.Error.Detail.Type)
	require.Equal(t, "upstream_failed", result.Error.Detail.Code)
	require.Equal(t, "upstream provider error", result.Error.Detail.Message)
}

func TestOutboundTransformer_TransformRequest_WithTestData(t *testing.T) {
	tests := []struct {
		name        string
		requestFile string
		validate    func(t *testing.T, result *httpclient.Request, expectedReq *llm.Request)
	}{
		{
			name:        "image generation request transformation",
			requestFile: "image-generation.request.json",
			validate: func(t *testing.T, result *httpclient.Request, expectedReq *llm.Request) {
				// Verify basic HTTP request properties
				require.Equal(t, http.MethodPost, result.Method)
				require.Equal(t, "https://api.openai.com/v1/responses", result.URL)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.Equal(t, "application/json", result.Headers.Get("Accept"))
				require.NotEmpty(t, result.Body)

				// Verify auth
				require.NotNil(t, result.Auth)
				require.Equal(t, "bearer", result.Auth.Type)
				require.Equal(t, "test-api-key", result.Auth.APIKey)

				// Parse the transformed request
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)

				// Verify model
				require.Equal(t, expectedReq.Model, req.Model)

				// Verify tools transformation
				if len(expectedReq.Tools) > 0 {
					require.NotNil(t, req.Tools)
					require.Len(t, req.Tools, len(expectedReq.Tools))

					for i, tool := range expectedReq.Tools {
						require.Equal(t, tool.Type, req.Tools[i].Type)

						if tool.ImageGeneration != nil {
							require.Equal(t, tool.ImageGeneration.Quality, req.Tools[i].Quality)
							require.Equal(t, tool.ImageGeneration.Size, req.Tools[i].Size)
							require.Equal(t, tool.ImageGeneration.OutputFormat, req.Tools[i].OutputFormat)
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the test request data
			var expectedReq llm.Request

			err := xtest.LoadTestData(t, tt.requestFile, &expectedReq)
			if err != nil {
				t.Skipf("Test data file %s not found, skipping test", tt.requestFile)
				return
			}

			// Create transformer
			transformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)

			// Transform the request
			result, err := transformer.TransformRequest(context.Background(), &expectedReq)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Run validation
			tt.validate(t, result, &expectedReq)
		})
	}
}

func TestOutboundTransformer_TransformResponse_WithTestData(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")

	tests := []struct {
		name         string
		responseFile string
		validate     func(t *testing.T, result *llm.Response)
	}{
		{
			name:         "stop response transformation",
			responseFile: "stop.response.json",
			validate: func(t *testing.T, result *llm.Response) {
				require.Equal(t, "chat.completion", result.Object)
				require.NotEmpty(t, result.ID)
				require.Equal(t, "gpt-4o", result.Model)
				require.Len(t, result.Choices, 1)
				require.Equal(t, "assistant", result.Choices[0].Message.Role)
				require.NotNil(t, result.Choices[0].Message.Content.Content)
				require.Contains(t, *result.Choices[0].Message.Content.Content, "weather")
				require.NotNil(t, result.Usage)
				require.Greater(t, result.Usage.TotalTokens, int64(0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var responseData json.RawMessage
			// Load the test response data
			err := xtest.LoadTestData(t, tt.responseFile, &responseData)
			if err != nil {
				t.Errorf("Test data file %s not found, skipping test", tt.responseFile)
				return
			}

			// Create HTTP response
			httpResp := &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       responseData,
			}

			// Transform the response
			result, err := transformer.TransformResponse(context.Background(), httpResp)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Run validation
			tt.validate(t, result)
		})
	}
}

// TestOutboundTransformer_TransformRequest_NamespaceDoesNotStarveRawTools covers #3:
// a namespace container tool expands into N canonical functions, but the old
// buildRepresentedToolSignatures skipped namespace and buildRawOnlyToolFragments
// kept it as raw. That made structuredToolSignaturesMatch see len(canonical) >
// len(signatures) -> false, so co-resident raw-only tools (file_search/mcp) were
// dropped on Responses->Responses pass-through.
func TestOutboundTransformer_TransformRequest_NamespaceDoesNotStarveRawTools(t *testing.T) {
	inbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": "use the docs search then file_search",
			"tools": [
				{
					"type": "namespace",
					"name": "docs",
					"tools": [
						{"type": "function", "name": "search", "parameters": {"type": "object", "properties": {}}}
					]
				},
				{
					"type": "file_search",
					"name": "file_search",
					"vector_store_ids": ["vs_123"]
				}
			]
		}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	llmReq.Model = "mapped-model"

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(httpReq.Body, &payload)
	require.NoError(t, err)

	tools, ok := payload["tools"].([]any)
	require.True(t, ok)

	// Expect both the native namespace container AND the raw-only file_search
	// to survive the Responses->Responses pass-through.
	typesByName := map[string]string{}
	for _, raw := range tools {
		tm, ok := raw.(map[string]any)
		require.True(t, ok)
		typesByName[fmt.Sprintf("%v", tm["type"])] = fmt.Sprintf("%v", tm["name"])
	}
	// native namespace container present; it must not be flattened here
	require.Contains(t, typesByName, "namespace")
	require.Equal(t, "docs", typesByName["namespace"])
	// raw-only file_search must NOT be starved
	require.Contains(t, typesByName, "file_search")
}

// TestConvertToLLMRequest_Prompt covers F19: the stored prompt template
// reference (prompt{ id, version, variables }) must survive
// Responses->canonical->Responses via TransformerMetadata, mirroring the
// F15 background / F16-F18 passthrough family. Before the fix the Prompt
// field was commented out (// TODO) and a client's prompt body was silently
// dropped by lenient unmarshal.
func TestConvertToLLMRequest_Prompt(t *testing.T) {
	t.Run("inbound preserves prompt into metadata", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4o",
			Prompt: &Prompt{
				ID:        "pmpt_abc",
				Version:   lo.ToPtr("2"),
				Variables: map[string]string{"topic": "cats"},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		v, ok := result.TransformerMetadata["prompt"]
		require.True(t, ok)
		p, ok := v.(*Prompt)
		require.True(t, ok)
		require.Equal(t, "pmpt_abc", p.ID)
		require.NotNil(t, p.Version)
		require.Equal(t, "2", *p.Version)
		require.Equal(t, "cats", p.Variables["topic"])
	})

	t.Run("outbound restores prompt from metadata", func(t *testing.T) {
		llmReq := &llm.Request{
			Model: "gpt-4o",
			Messages: []llm.Message{{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("hi")},
			}},
			TransformerMetadata: map[string]any{
				"prompt": &Prompt{
					ID:        "pmpt_xyz",
					Version:   lo.ToPtr("3"),
					Variables: map[string]string{"topic": "dogs"},
				},
			},
		}

		outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
		require.NoError(t, err)

		httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
		require.NoError(t, err)

		var got Request
		require.NoError(t, json.Unmarshal(httpReq.Body, &got))
		require.NotNil(t, got.Prompt)
		require.Equal(t, "pmpt_xyz", got.Prompt.ID)
		require.NotNil(t, got.Prompt.Version)
		require.Equal(t, "3", *got.Prompt.Version)
		require.Equal(t, "dogs", got.Prompt.Variables["topic"])
	})

	t.Run("prompt absent stays absent", func(t *testing.T) {
		req := &Request{Model: "gpt-4o"}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		_, ok := result.TransformerMetadata["prompt"]
		require.False(t, ok)
	})
}

// TestOutboundTransformer_TransformRequest_RawInputItemsSurvivePromptPrepend covers #12:
// when a non-system prompt is prepended to the canonical messages, the outbound
// merge of RawInputItems must keep raw-only items (e.g. tool_search_call) in
// their original position relative to the user's structured items, not shove
// them ahead of the injected prepend message.
func TestOutboundTransformer_TransformRequest_RawInputItemsSurvivePromptPrepend(t *testing.T) {
	inbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": [
				{
					"type": "tool_search_call",
					"call_id": "call_search",
					"status": "completed",
					"arguments": {"query":"image generation","limit":10}
				},
				{
					"type": "message",
					"role": "user",
					"content": [{"type":"input_text","text":"hello"}]
				}
			]
		}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)

	// Simulate a prepended user prompt (injected by the prompt pipeline between
	// inbound and outbound). It must not displace the raw-only input item.
	// The prompt pipeline (injectPrompts) records the prepend count on the
	// OpenAI Responses provider extensions so the outbound merge can offset
	// raw-only items accordingly.
	llmReq.Messages = append([]llm.Message{{
		Role: "user",
		Content: llm.MessageContent{
			Content: lo.ToPtr("INJECTED"),
		},
	}}, llmReq.Messages...)
	if ext := llm.EnsureOpenAIResponsesProviderExtensions(llmReq); ext != nil && ext.Request != nil {
		ext.Request.PrependCount = 1
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(httpReq.Body, &payload)
	require.NoError(t, err)

	input, ok := payload["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 3)

	// Expected order: [INJECTED message, tool_search_call, hello message].
	// The bug produced [tool_search_call, INJECTED, hello].
	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "message", first["type"])
	// prepended user message content is "INJECTED"
	content, ok := first["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	ctext, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "INJECTED", ctext["text"])

	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_search_call", second["type"])

	third, ok := input[2].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "message", third["type"])
}

// TestOutboundTransformer_TransformRequest_RawInputItemsSurvivePromptAppend covers #12
// append regression guard: an appended prompt must not displace raw-only input
// items. Append grows the tail only, so raw items keep their original position.
func TestOutboundTransformer_TransformRequest_RawInputItemsSurvivePromptAppend(t *testing.T) {
	inbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": [
				{
					"type": "tool_search_call",
					"call_id": "call_search",
					"status": "completed",
					"arguments": {"query":"image generation","limit":10}
				},
				{
					"type": "message",
					"role": "user",
					"content": [{"type":"input_text","text":"hello"}]
				}
			]
		}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)

	// Simulate an appended prompt. It must sit at the tail; raw-only item keeps
	// its original first position.
	llmReq.Messages = append(llmReq.Messages, llm.Message{
		Role: "user",
		Content: llm.MessageContent{
			Content: lo.ToPtr("APPENDED"),
		},
	})

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(httpReq.Body, &payload)
	require.NoError(t, err)

	input, ok := payload["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 3)

	// Expected order: [tool_search_call, hello message, APPENDED message].
	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_search_call", first["type"])

	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "message", second["type"])

	third, ok := input[2].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "message", third["type"])
}
