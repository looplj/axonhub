package responses

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// G14a: public same-protocol seam for supplied reasoning.summary and
// stream_options.reasoning_summary_delivery (Codex summary-delivery option).
// Hub must preserve client-supplied values and keep typed + raw nested
// stream_options coexisting without overwrite; it must not invent Codex model
// capability gates.
func TestG14a_SummaryAndSummaryDelivery_SameProtocolPreserved(t *testing.T) {
	body, err := os.ReadFile("testdata/g14a-summary-stream-options.request.json")
	require.NoError(t, err)

	var source map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.Contains(t, source, "reasoning")
	require.Contains(t, source, "stream_options")

	var sourceReasoning map[string]any
	require.NoError(t, json.Unmarshal(source["reasoning"], &sourceReasoning))
	require.Equal(t, "detailed", sourceReasoning["summary"])
	require.Equal(t, "high", sourceReasoning["effort"])

	var sourceStreamOptions map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["stream_options"], &sourceStreamOptions))
	require.JSONEq(t, `false`, string(sourceStreamOptions["include_obfuscation"]))
	require.JSONEq(t, `"sequential_cutoff"`, string(sourceStreamOptions["reasoning_summary_delivery"]))
	require.JSONEq(t, `{"x":1}`, string(sourceStreamOptions["future_nested"]))

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	// Canonical captures supplied summary verbatim.
	require.NotNil(t, llmReq.ReasoningSummary)
	require.Equal(t, "detailed", *llmReq.ReasoningSummary)
	require.Equal(t, "high", llmReq.ReasoningEffort)

	// Outbound reasoning.summary is the supplied value (not rewritten / dropped).
	require.Contains(t, payload, "reasoning")
	var outboundReasoning map[string]any
	require.NoError(t, json.Unmarshal(payload["reasoning"], &outboundReasoning))
	require.Equal(t, "detailed", outboundReasoning["summary"])
	require.Equal(t, "high", outboundReasoning["effort"])

	// stream_options must re-emit typed known field, summary-delivery option, and
	// unknown nested keys together (G9 raw merge semantics).
	require.Contains(t, payload, "stream_options")
	var outboundStreamOptions map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload["stream_options"], &outboundStreamOptions))

	require.Contains(t, outboundStreamOptions, "include_obfuscation")
	require.JSONEq(t, `false`, string(outboundStreamOptions["include_obfuscation"]),
		"typed include_obfuscation must survive")
	require.Contains(t, outboundStreamOptions, "reasoning_summary_delivery")
	require.JSONEq(t, `"sequential_cutoff"`, string(outboundStreamOptions["reasoning_summary_delivery"]),
		"summary-delivery stream option must be preserved as supplied")
	require.Contains(t, outboundStreamOptions, "future_nested")
	require.JSONEq(t, `{"x":1}`, string(outboundStreamOptions["future_nested"]),
		"unknown nested stream_options keys must coexist with typed fields")
}

// G14a absence: Hub Responses same-protocol path must not inject reasoning.summary
// or summary-delivery stream_options from any Codex model capability policy.
func TestG14a_DefaultOmission_NoHubInjectionOfSummaryOrSummaryDelivery(t *testing.T) {
	body, err := os.ReadFile("testdata/g14a-default-omission.request.json")
	require.NoError(t, err)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	require.Empty(t, llmReq.ReasoningEffort)
	require.Nil(t, llmReq.ReasoningSummary)
	require.Nil(t, llmReq.StreamOptions)

	_, hasReasoning := payload["reasoning"]
	require.False(t, hasReasoning, "Hub must not inject reasoning when client omitted it")
	_, hasStreamOptions := payload["stream_options"]
	require.False(t, hasStreamOptions, "Hub must not inject stream_options when client omitted it")
}

// G14a / G9 boundary: when stream_options has only non-typed nested keys (no
// include_obfuscation), Hub must still re-emit the raw object on same-protocol
// replay instead of dropping the whole key because convertStreamOptions returned nil.
func TestG14a_RawOnlyStreamOptionsWithoutTypedField(t *testing.T) {
	payload, llmReq := roundTripResponsesRawPayload(t, `{
		"model": "o3",
		"input": "hello",
		"stream": true,
		"stream_options": {
			"reasoning_summary_delivery": "sequential_cutoff",
			"future_only": {"y": 2}
		}
	}`, nil)

	require.Nil(t, convertStreamOptions(llmReq.StreamOptions, llmReq.TransformerMetadata))

	streamOptionsRaw, ok := payload["stream_options"]
	require.True(t, ok, "stream_options must be present even without typed field")
	var so map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(streamOptionsRaw, &so))
	require.JSONEq(t, `"sequential_cutoff"`, string(so["reasoning_summary_delivery"]))
	require.JSONEq(t, `{"y":2}`, string(so["future_only"]))
	_, hasInclude := so["include_obfuscation"]
	require.False(t, hasInclude)
}

// G14a / G9 precedence: raw sidecar values win a shared nested key over a
// later canonical typed mutation, while raw-only summary-delivery survives.
func TestG14a_StreamOptionsRawValuesWinSharedTypedKey(t *testing.T) {
	payload, _ := roundTripResponsesRawPayload(t, `{
		"model": "o3",
		"input": "hello",
		"stream": true,
		"stream_options": {
			"include_obfuscation": false,
			"reasoning_summary_delivery": "sequential_cutoff"
		}
	}`, func(req *llm.Request) {
		// This simulates a typed-side update after inbound capture. The raw
		// Responses sidecar must retain the original wire value on replay.
		typedValue := true
		req.TransformerMetadata[responsesIncludeObfuscationTransformerMetadataKey] = &typedValue
	})

	var streamOptions map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload["stream_options"], &streamOptions))
	require.JSONEq(t, `false`, string(streamOptions["include_obfuscation"]))
	require.JSONEq(t, `"sequential_cutoff"`, string(streamOptions["reasoning_summary_delivery"]))
}
