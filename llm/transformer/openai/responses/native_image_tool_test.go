package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResponsesNativeImageToolPreserved(t *testing.T) {
	// Given a client-native Responses request, without an Images request or bridge.
	body := `{"model":"gpt-5.5","input":"synthetic","tools":[{"type":"image_generation","quality":"low","size":"1024x1024","output_format":"png"}],"tool_choice":{"type":"image_generation"}}`
	inbound := NewInboundTransformer()
	request, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Body: []byte(body), Headers: http.Header{"Content-Type": {"application/json"}},
	})
	require.NoError(t, err)
	require.Nil(t, request.Image)
	outbound, err := NewOutboundTransformer("https://example.invalid/v1", "synthetic-key")
	require.NoError(t, err)
	// When the native request is serialized through the Responses outbound.
	encoded, err := outbound.TransformRequest(context.Background(), request)
	require.NoError(t, err)
	// Then its native tool parameters and tool choice survive and it stays Responses.
	var expected, actual map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(body), &expected))
	require.NoError(t, json.Unmarshal(encoded.Body, &actual))
	require.JSONEq(t, string(expected["tools"]), string(actual["tools"]))
	require.JSONEq(t, string(expected["tool_choice"]), string(actual["tool_choice"]))
	require.Equal(t, "https://example.invalid/v1/responses", encoded.URL)
}
