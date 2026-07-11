package responses

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResponsesReasoningContextSameProtocolRoundTrip(t *testing.T) {
	body, err := os.ReadFile("testdata/reasoning-context.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	// effort/summary stay on common fields; context is Responses-native.
	require.Equal(t, "high", llmReq.ReasoningEffort)
	require.NotNil(t, llmReq.ReasoningSummary)
	require.Equal(t, "auto", *llmReq.ReasoningSummary)
	ctxVal, ok := llmReq.TransformerMetadata[responsesReasoningContextTransformerMetadataKey].(string)
	require.True(t, ok, "reasoning.context must be preserved outside common effort/summary fields")
	require.Equal(t, "current_turn", ctxVal)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	require.Contains(t, outboundBody, "reasoning")

	var reasoning map[string]any
	require.NoError(t, json.Unmarshal(outboundBody["reasoning"], &reasoning))
	require.Equal(t, "high", reasoning["effort"])
	require.Equal(t, "auto", reasoning["summary"])
	require.Equal(t, "current_turn", reasoning["context"])
}
