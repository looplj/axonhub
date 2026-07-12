package responses

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// G15a: OpenAI Responses same-protocol public seam must preserve supplied non-empty
// input item identities (message id + function/tool item id / call_id) and must not
// invent ids when the source omitted them. Codex msg_/fc_ prefix policy is out of
// scope; generateItemID() is only for Hub-generated response output items.
func TestG15a_InputItemIdentity_PreservesSuppliedIDsAndOrder(t *testing.T) {
	body, err := os.ReadFile("testdata/g15a-input-item-identity.request.json")
	require.NoError(t, err)

	var source map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	var sourceInput []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["input"], &sourceInput))
	require.Len(t, sourceInput, 5)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	// Canonical: message ids land on Message.ID; function_call keeps ResponseItemID
	// separate from call_id (ToolCall.ID).
	require.Len(t, llmReq.Messages, 5)
	require.Equal(t, "legacy-id", llmReq.Messages[0].ID)
	require.Equal(t, "user", llmReq.Messages[0].Role)

	require.Equal(t, "assistant", llmReq.Messages[1].Role)
	require.Len(t, llmReq.Messages[1].ToolCalls, 1)
	require.Equal(t, "call_weather_1", llmReq.Messages[1].ToolCalls[0].ID)
	require.Equal(t, "fc_item_legacy", llmReq.Messages[1].ToolCalls[0].ResponseItemID)

	require.Equal(t, "tool", llmReq.Messages[2].Role)
	require.Equal(t, "fc_out_legacy", llmReq.Messages[2].ID)
	require.NotNil(t, llmReq.Messages[2].ToolCallID)
	require.Equal(t, "call_weather_1", *llmReq.Messages[2].ToolCallID)

	// Source assistant message had no id → canonical must not invent one.
	require.Equal(t, "assistant", llmReq.Messages[3].Role)
	require.Empty(t, llmReq.Messages[3].ID)
	require.NotNil(t, llmReq.Messages[3].Content.Content)
	require.Equal(t, "It is sunny in SF.", *llmReq.Messages[3].Content.Content)

	require.Equal(t, "user", llmReq.Messages[4].Role)
	require.Equal(t, "any-string-not-codex-prefix", llmReq.Messages[4].ID)

	inputRaw, ok := payload["input"]
	require.True(t, ok, "outbound must emit input array")
	var outbound []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(inputRaw, &outbound))
	require.Len(t, outbound, 5, "item order and cardinality must be preserved")

	assertOutboundItemType(t, outbound[0], "message")
	require.JSONEq(t, `"legacy-id"`, string(outbound[0]["id"]),
		"supplied non-empty message id must be preserved verbatim (any string, not Codex prefix)")
	require.JSONEq(t, `"user"`, string(outbound[0]["role"]))

	assertOutboundItemType(t, outbound[1], "function_call")
	require.JSONEq(t, `"fc_item_legacy"`, string(outbound[1]["id"]),
		"function_call item id must be preserved and not confused with call_id")
	require.JSONEq(t, `"call_weather_1"`, string(outbound[1]["call_id"]))
	require.JSONEq(t, `"get_weather"`, string(outbound[1]["name"]))

	assertOutboundItemType(t, outbound[2], "function_call_output")
	require.JSONEq(t, `"fc_out_legacy"`, string(outbound[2]["id"]),
		"function_call_output item id must be preserved and not confused with call_id")
	require.JSONEq(t, `"call_weather_1"`, string(outbound[2]["call_id"]))

	assertOutboundItemType(t, outbound[3], "message")
	require.JSONEq(t, `"assistant"`, string(outbound[3]["role"]))
	_, hasAssistantID := outbound[3]["id"]
	require.False(t, hasAssistantID, "source assistant message had no id; outbound must omit id")

	assertOutboundItemType(t, outbound[4], "message")
	require.JSONEq(t, `"any-string-not-codex-prefix"`, string(outbound[4]["id"]),
		"non-Codex-prefix message id must still round-trip verbatim")
	require.JSONEq(t, `"user"`, string(outbound[4]["role"]))
}

// G15a absence: when source input items omit id, same-protocol outbound must also
// omit id. Hub must not synthesize item_* / msg_* for request input replay.
func TestG15a_DefaultOmission_NoSyntheticInputItemIDs(t *testing.T) {
	body, err := os.ReadFile("testdata/g15a-default-omission.request.json")
	require.NoError(t, err)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	for _, msg := range llmReq.Messages {
		require.Empty(t, msg.ID, "canonical must not invent message ids when source omitted them")
		for _, tc := range msg.ToolCalls {
			require.Empty(t, tc.ResponseItemID, "canonical must not invent function_call item ids")
			require.NotEmpty(t, tc.ID, "call_id still arrives as ToolCall.ID")
		}
	}

	inputRaw, ok := payload["input"]
	require.True(t, ok)
	var outbound []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(inputRaw, &outbound))
	require.Len(t, outbound, 3)

	for i, item := range outbound {
		if rawID, hasID := item["id"]; hasID {
			var id string
			require.NoError(t, json.Unmarshal(rawID, &id))
			require.False(t, strings.HasPrefix(id, "item_"),
				"item %d: must not synthesize item_* for request input (got %q)", i, id)
			require.False(t, strings.HasPrefix(id, "msg_"),
				"item %d: must not synthesize msg_* for request input (got %q)", i, id)
			require.Failf(t, "unexpected id", "item %d: source had no id but outbound emitted id=%q", i, id)
		}
	}

	assertOutboundItemType(t, outbound[0], "message")
	assertOutboundItemType(t, outbound[1], "function_call")
	require.JSONEq(t, `"call_no_item_id"`, string(outbound[1]["call_id"]))
	assertOutboundItemType(t, outbound[2], "function_call_output")
	require.JSONEq(t, `"call_no_item_id"`, string(outbound[2]["call_id"]))
}

func assertOutboundItemType(t *testing.T, item map[string]json.RawMessage, want string) {
	t.Helper()
	rawType, ok := item["type"]
	require.True(t, ok, "item missing type")
	require.JSONEq(t, `"`+want+`"`, string(rawType))
}
