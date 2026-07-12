package responses

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/transformer/shared"
)

// G13a: public same-protocol seam for actual reasoning object + include values.
// Hub must preserve client-supplied fields; must not inject Codex "always send"
// reasoning / reasoning.encrypted_content policy when the client omitted them.
func TestG13a_ReasoningAndEncryptedInclude_SameProtocolPreserved(t *testing.T) {
	body, err := os.ReadFile("testdata/g13a-reasoning-include.request.json")
	require.NoError(t, err)

	// Source request carries a non-empty reasoning object and an include list
	// that contains reasoning.encrypted_content among other values (order matters).
	var source map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.Contains(t, source, "reasoning")
	require.Contains(t, source, "include")

	var sourceReasoning map[string]any
	require.NoError(t, json.Unmarshal(source["reasoning"], &sourceReasoning))
	require.NotEmpty(t, sourceReasoning)

	var sourceInclude []string
	require.NoError(t, json.Unmarshal(source["include"], &sourceInclude))
	require.Equal(t, []string{
		"file_search_call.results",
		"reasoning.encrypted_content",
		"message.input_image.image_url",
	}, sourceInclude)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	// Canonical captures typed reasoning + include metadata without inventing values.
	require.Equal(t, "high", llmReq.ReasoningEffort)
	require.NotNil(t, llmReq.ReasoningSummary)
	require.Equal(t, "auto", *llmReq.ReasoningSummary)
	includeMeta, ok := llmReq.TransformerMetadata[shared.MetadataKeyInclude].([]string)
	require.True(t, ok, "include must ride TransformerMetadata on the canonical request")
	require.Equal(t, sourceInclude, includeMeta, "include order and values must be preserved as supplied")

	// Outbound Responses body re-emits the same reasoning subfields and include values.
	require.Contains(t, payload, "reasoning")
	require.Contains(t, payload, "include")

	var outboundReasoning map[string]any
	require.NoError(t, json.Unmarshal(payload["reasoning"], &outboundReasoning))
	require.Equal(t, "high", outboundReasoning["effort"])
	require.Equal(t, "auto", outboundReasoning["summary"])

	var outboundInclude []string
	require.NoError(t, json.Unmarshal(payload["include"], &outboundInclude))
	require.Equal(t, sourceInclude, outboundInclude,
		"include must be preserved (no sort/dedupe/drop/injection of constants)")
}

// G13a absence: Hub Responses same-protocol path must not inject Codex client
// defaults (always send reasoning + include reasoning.encrypted_content).
func TestG13a_DefaultOmission_NoHubInjectionOfReasoningOrInclude(t *testing.T) {
	body, err := os.ReadFile("testdata/g13a-default-omission.request.json")
	require.NoError(t, err)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	require.Empty(t, llmReq.ReasoningEffort)
	require.Nil(t, llmReq.ReasoningSummary)
	require.Nil(t, llmReq.ReasoningBudget)
	if llmReq.TransformerMetadata != nil {
		_, hasInclude := llmReq.TransformerMetadata[shared.MetadataKeyInclude]
		require.False(t, hasInclude, "Hub must not invent include metadata for omitted requests")
	}

	_, hasReasoning := payload["reasoning"]
	require.False(t, hasReasoning, "Hub must not inject reasoning when client omitted it")
	_, hasInclude := payload["include"]
	require.False(t, hasInclude, "Hub must not inject include when client omitted it")
}
