package responses

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// Forward-compat: unknown Responses reasoning.effort must survive same-protocol
// inbound → canonical → outbound without reject/downgrade/replace.
func TestResponsesUnknownReasoningEffortSameProtocolRoundTrip(t *testing.T) {
	const futureEffort = "future-effort"
	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "o3",
			"input": "Solve this",
			"reasoning": {
				"effort": "` + futureEffort + `"
			}
		}`),
	})
	require.NoError(t, err)
	require.Equal(t, futureEffort, llmReq.ReasoningEffort, "unknown reasoning.effort must be captured verbatim")

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	require.Contains(t, outboundBody, "reasoning")

	var reasoning map[string]any
	require.NoError(t, json.Unmarshal(outboundBody["reasoning"], &reasoning))
	require.Equal(t, futureEffort, reasoning["effort"], "unknown reasoning.effort must re-emit on Responses wire unchanged")
}

// convertReasoning must copy non-empty unknown effort strings as-is (no enum filter).
func TestConvertReasoning_UnknownEffortPreserved(t *testing.T) {
	const futureEffort = "future-effort"
	result := convertReasoning(&llm.Request{ReasoningEffort: futureEffort})
	require.NotNil(t, result)
	require.Equal(t, futureEffort, result.Effort)
}
