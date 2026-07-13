package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

// R3: an official hosted output item that has no canonical owner must survive
// Responses -> llm -> Responses through the Responses response sidecar.
func TestR3_FileSearchCallOutput_SameProtocolRawRoundTrip(t *testing.T) {
	source := []byte(`{
		"id":"resp_file_search_1",
		"object":"response",
		"created_at":1700000003,
		"model":"gpt-5",
		"status":"completed",
		"output":[
			{"id":"fs_1","type":"file_search_call","status":"completed","queries":["policy"],"results":[{"file_id":"file_1","filename":"policy.md","score":0.9,"future_result":{"x":1}}]},
			{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Found it","annotations":[]}]}
		]
	}`)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{StatusCode: http.StatusOK, Body: source})
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	httpResp, err := inbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)

	var sourceRoot, gotRoot map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source, &sourceRoot))
	require.NoError(t, json.Unmarshal(httpResp.Body, &gotRoot))
	var sourceOutput, gotOutput []json.RawMessage
	require.NoError(t, json.Unmarshal(sourceRoot["output"], &sourceOutput))
	require.NoError(t, json.Unmarshal(gotRoot["output"], &gotOutput))
	require.Len(t, gotOutput, 2)
	require.JSONEq(t, string(sourceOutput[0]), string(gotOutput[0]))
	require.JSONEq(t, string(sourceOutput[1]), string(gotOutput[1]))
}

// R3: a response containing only a raw-native output item must not gain a
// synthetic assistant message during same-protocol replay.
func TestR3_RawOnlyOutput_DoesNotInventEmptyMessage(t *testing.T) {
	source := []byte(`{
		"id":"resp_file_search_only",
		"object":"response",
		"created_at":1700000004,
		"model":"gpt-5",
		"status":"completed",
		"output":[
			{"id":"fs_only","type":"file_search_call","status":"completed","queries":["policy"]}
		]
	}`)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{StatusCode: http.StatusOK, Body: source})
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	httpResp, err := inbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)

	var sourceRoot, gotRoot map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source, &sourceRoot))
	require.NoError(t, json.Unmarshal(httpResp.Body, &gotRoot))
	var sourceOutput, gotOutput []json.RawMessage
	require.NoError(t, json.Unmarshal(sourceRoot["output"], &sourceOutput))
	require.NoError(t, json.Unmarshal(gotRoot["output"], &gotOutput))
	require.Len(t, gotOutput, 1)
	require.JSONEq(t, string(sourceOutput[0]), string(gotOutput[0]))
}
