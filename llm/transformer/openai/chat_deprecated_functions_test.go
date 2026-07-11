package openai

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestOpenAIChatRequestDeprecatedFunctionsRawRoundTrip(t *testing.T) {
	body, err := os.ReadFile("testdata/openai-deprecated-functions.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	// Deprecated functions must not be silently rewritten into modern tools unless
	// an explicit bridge is proven; same-protocol raw preserve is the default.
	require.Empty(t, llmReq.Tools)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var source, outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	require.Contains(t, outboundBody, "functions")
	require.JSONEq(t, string(source["functions"]), string(outboundBody["functions"]))
	require.NotContains(t, outboundBody, "tools")

	requestType := reflect.TypeOf(*llmReq)
	_, has := requestType.FieldByName("Functions")
	require.False(t, has, "deprecated functions must not widen llm.Request")
}

func TestOpenAIChatRequestDeprecatedFunctionCallRawRoundTrip(t *testing.T) {
	body, err := os.ReadFile("testdata/openai-deprecated-function-call.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	require.Nil(t, llmReq.ToolChoice)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var source, outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	require.Contains(t, outboundBody, "function_call")
	require.JSONEq(t, string(source["function_call"]), string(outboundBody["function_call"]))
	require.Contains(t, outboundBody, "functions")
	require.JSONEq(t, string(source["functions"]), string(outboundBody["functions"]))
	require.NotContains(t, outboundBody, "tool_choice")
	require.NotContains(t, outboundBody, "tools")

	requestType := reflect.TypeOf(*llmReq)
	_, has := requestType.FieldByName("FunctionCall")
	require.False(t, has, "deprecated function_call must not widen llm.Request")
}

func TestOpenAIChatRequestDeprecatedAndModernToolsPrecedence(t *testing.T) {
	body, err := os.ReadFile("testdata/openai-deprecated-and-modern-tools.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	// Modern tools path remains the common abstraction.
	require.Len(t, llmReq.Tools, 1)
	require.Equal(t, "get_weather", llmReq.Tools[0].Function.Name)
	require.NotNil(t, llmReq.ToolChoice)
	require.NotNil(t, llmReq.ToolChoice.ToolChoice)
	require.Equal(t, "auto", *llmReq.ToolChoice.ToolChoice)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var source, outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))

	// Legacy raw fields are preserved for same-protocol replay.
	require.JSONEq(t, string(source["functions"]), string(outboundBody["functions"]))
	require.JSONEq(t, string(source["function_call"]), string(outboundBody["function_call"]))

	// Modern fields remain driven by the common abstraction path.
	require.Contains(t, outboundBody, "tools")
	require.Contains(t, outboundBody, "tool_choice")

	var tools []map[string]any
	require.NoError(t, json.Unmarshal(outboundBody["tools"], &tools))
	require.Len(t, tools, 1)
	fn, ok := tools[0]["function"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "get_weather", fn["name"])

	var toolChoice any
	require.NoError(t, json.Unmarshal(outboundBody["tool_choice"], &toolChoice))
	require.Equal(t, "auto", toolChoice)
}

func TestOpenAIChatRequestDeprecatedFunctionsNotSynthesizedForResponses(t *testing.T) {
	for _, path := range []string{
		"testdata/openai-deprecated-functions.request.json",
		"testdata/openai-deprecated-function-call.request.json",
		"testdata/openai-deprecated-and-modern-tools.request.json",
	} {
		t.Run(path, func(t *testing.T) {
			body, err := os.ReadFile(path)
			require.NoError(t, err)

			inbound := NewInboundTransformer()
			llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{"application/json"}},
				Body:    body,
			})
			require.NoError(t, err)

			outbound, err := responses.NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)
			upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
			require.NoError(t, err)

			var outboundBody map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
			require.NotContains(t, outboundBody, "functions")
			require.NotContains(t, outboundBody, "function_call")
		})
	}
}

func TestOpenAIChatResponseDeprecatedMessageFunctionCallBridge(t *testing.T) {
	body, err := os.ReadFile("testdata/openai-deprecated-message-function-call.response.json")
	require.NoError(t, err)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	})
	require.NoError(t, err)
	require.Len(t, llmResp.Choices, 1)
	require.NotNil(t, llmResp.Choices[0].Message)
	require.NotNil(t, llmResp.Choices[0].FinishReason)
	require.Equal(t, "function_call", *llmResp.Choices[0].FinishReason)

	// Bridge deprecated message.function_call into the modern tool-call lifecycle
	// so downstream consumers that only understand tool_calls still work.
	require.Len(t, llmResp.Choices[0].Message.ToolCalls, 1)
	tc := llmResp.Choices[0].Message.ToolCalls[0]
	require.Equal(t, "function", tc.Type)
	require.Equal(t, "get_weather", tc.Function.Name)
	require.JSONEq(t, `{"location":"New York City"}`, tc.Function.Arguments)

	// Client-facing Chat response should still re-emit the deprecated shape.
	inbound := NewInboundTransformer()
	httpResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var clientBody map[string]any
	require.NoError(t, json.Unmarshal(httpResp.Body, &clientBody))
	choices, ok := clientBody["choices"].([]any)
	require.True(t, ok)
	require.Len(t, choices, 1)
	choice, ok := choices[0].(map[string]any)
	require.True(t, ok)
	message, ok := choice["message"].(map[string]any)
	require.True(t, ok)
	functionCall, ok := message["function_call"].(map[string]any)
	require.True(t, ok, "client response must preserve message.function_call")
	require.Equal(t, "get_weather", functionCall["name"])
	require.JSONEq(t, `{"location":"New York City"}`, functionCall["arguments"].(string))
	require.Equal(t, "function_call", choice["finish_reason"])
}

func TestOpenAIChatModernToolPathUnaffectedByDeprecatedBridge(t *testing.T) {
	body, err := os.ReadFile("testdata/openai-tool.response.json")
	require.NoError(t, err)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	})
	require.NoError(t, err)
	require.Len(t, llmResp.Choices, 1)
	require.NotNil(t, llmResp.Choices[0].Message)
	require.Len(t, llmResp.Choices[0].Message.ToolCalls, 1)
	require.Equal(t, "get_weather", llmResp.Choices[0].Message.ToolCalls[0].Function.Name)
	require.Equal(t, "tool_calls", *llmResp.Choices[0].FinishReason)

	inbound := NewInboundTransformer()
	httpResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var clientBody map[string]any
	require.NoError(t, json.Unmarshal(httpResp.Body, &clientBody))
	choices := clientBody["choices"].([]any)
	choice := choices[0].(map[string]any)
	message := choice["message"].(map[string]any)
	_, hasFunctionCall := message["function_call"]
	require.False(t, hasFunctionCall, "modern tool_calls response must not invent deprecated function_call")
	require.Contains(t, message, "tool_calls")
}
