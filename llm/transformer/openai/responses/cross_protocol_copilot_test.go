package responses_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/copilot"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// stubCopilotTokenProvider is a stand-in copilot.TokenProvider for tests.
type stubCopilotTokenProvider struct{}

func (stubCopilotTokenProvider) GetToken(context.Context) (string, error) {
	return "test-token", nil
}

func copilotMustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// TestCrossProtocol_CopilotResponseMetadataRoundTrip verifies that copilot —
// which builds its own chat-completions TransformRequest and TransformResponse
// (not delegating to the chat base for the chat path) — propagates
// TransformerMetadata on both the request and the response so a responses
// client's namespace tool map survives the cross-protocol round-trip.
//
// Regression guard: copilot's self-built chat path previously did not call
// shared.PropagateRequestMetadata (request) nor shared.MergeResponseMetadata
// (response), so the namespace map was lost before reaching the upstream and
// again on the way back, breaking the responses inbound restoration.
//
// This test lives in package responses_test (not the internal responses test
// package) because copilot imports responses, so an internal responses test
// importing copilot would form an import cycle.
func TestCrossProtocol_CopilotResponseMetadataRoundTrip(t *testing.T) {
	responsesInbound := responses.NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: copilotMustMarshal(t, map[string]any{
			"model": "gpt-4o",
			"input": "use the tool",
			"tools": []map[string]any{
				{
					"type": "namespace",
					"name": "mcp__node_repl",
					"tools": []map[string]any{
						{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
					},
				},
			},
		}),
	}
	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	require.NotEmpty(t, llmReq.TransformerMetadata, "responses inbound must record TransformerMetadata (namespace map)")

	copilotOut, err := copilot.NewOutboundTransformer(copilot.OutboundTransformerParams{
		TokenProvider: stubCopilotTokenProvider{},
	})
	require.NoError(t, err)
	// gpt-4o routes through the chat-completions path, not the responses path.
	llmReq.Model = "gpt-4o"
	httpReq, err := copilotOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	t.Run("copilot request carries TransformerMetadata", func(t *testing.T) {
		require.NotNil(t, httpReq.TransformerMetadata, "copilot outbound must propagate TransformerMetadata on request")
		require.NotEmpty(t, httpReq.TransformerMetadata, "copilot outbound must carry the namespace tool map")
	})

	chatRespBody := copilotMustMarshal(t, map[string]any{
		"id":      "chatcompl_1",
		"object":  "chat.completion",
		"model":   "gpt-4o",
		"created": 1700000000,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role": "assistant",
					"tool_calls": []map[string]any{
						{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      "mcp__node_repl__run",
								"arguments": `{"x":1}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
	})
	httpResp := &httpclient.Response{
		StatusCode: 200,
		Body:       chatRespBody,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Request:    httpReq,
	}

	llmResp, err := copilotOut.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)

	t.Run("copilot response carries namespace map", func(t *testing.T) {
		require.NotNil(t, llmResp.TransformerMetadata, "copilot outbound must clone request TransformerMetadata to response")
		require.NotEmpty(t, llmResp.TransformerMetadata, "copilot outbound response must carry the namespace tool map")
	})

	clientResp, err := responsesInbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)
	var respPayload responses.Response
	require.NoError(t, json.Unmarshal(clientResp.Body, &respPayload))
	var fcItem *responses.Item
	for i := range respPayload.Output {
		if respPayload.Output[i].Type == "function_call" {
			fcItem = &respPayload.Output[i]
			break
		}
	}
	require.NotNil(t, fcItem, "function_call item must exist")
	require.Equal(t, "run", fcItem.Name, "name must be restored to leaf")
	require.Equal(t, "mcp__node_repl", fcItem.Namespace, "namespace must be restored to group")
}
