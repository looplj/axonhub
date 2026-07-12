package responses

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// G15c: Responses request input reasoning items carry their own item.id, independent
// of any following assistant message id or tool ResponseItemID. Same-protocol
// round-trip must preserve supplied non-empty reasoning ids and omit when absent.
// Must not use Message.ID for reasoning identity (message merge needs that slot).
//
// ResponseReasoningItemID is a *string presence marker for Responses-native
// reasoning items: non-nil means the message came from a type=reasoning input
// item (even when the source omitted id); nil means ordinary cross-protocol
// ReasoningContent and must not force a Responses reasoning item.
func TestG15c_StandaloneReasoningItemIdentity_PreservesSuppliedIDsAndAbsence(t *testing.T) {
	body, err := os.ReadFile("testdata/g15c-standalone-reasoning-item-identity.request.json")
	require.NoError(t, err)

	var source map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	var sourceInput []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["input"], &sourceInput))
	require.Len(t, sourceInput, 5)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	// user + (reasoning+assistant message) + (reasoning+assistant message) => 3
	require.Len(t, llmReq.Messages, 3)

	require.Equal(t, "user", llmReq.Messages[0].Role)

	// reasoning id must not be stored on Message.ID; following message owns Message.ID.
	require.Equal(t, "assistant", llmReq.Messages[1].Role)
	require.Equal(t, "msg_after_standalone_reason", llmReq.Messages[1].ID)
	require.NotNil(t, llmReq.Messages[1].ResponseReasoningItemID)
	require.Equal(t, "rs_standalone_legacy", *llmReq.Messages[1].ResponseReasoningItemID)
	require.NotNil(t, llmReq.Messages[1].ReasoningSignature)
	require.Equal(t, "gAAAA_standalone_reason", *llmReq.Messages[1].ReasoningSignature)

	// Source reasoning omitted id → carrier is present (Responses origin) but empty.
	require.Equal(t, "assistant", llmReq.Messages[2].Role)
	require.Empty(t, llmReq.Messages[2].ID)
	require.NotNil(t, llmReq.Messages[2].ResponseReasoningItemID,
		"Responses reasoning item origin must be marked even when id was omitted")
	require.Empty(t, *llmReq.Messages[2].ResponseReasoningItemID)
	require.NotNil(t, llmReq.Messages[2].ReasoningSignature)
	require.Equal(t, "gAAAA_no_id_reason", *llmReq.Messages[2].ReasoningSignature)

	inputRaw, ok := payload["input"]
	require.True(t, ok, "outbound must emit input array")
	var outbound []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(inputRaw, &outbound))
	require.Len(t, outbound, 5, "reasoning/message order and cardinality must be preserved")

	assertOutboundItemType(t, outbound[0], "message")
	require.JSONEq(t, `"user"`, string(outbound[0]["role"]))

	assertOutboundItemType(t, outbound[1], "reasoning")
	require.JSONEq(t, `"rs_standalone_legacy"`, string(outbound[1]["id"]),
		"standalone reasoning item id must round-trip independently of following message id")
	require.JSONEq(t, `"gAAAA_standalone_reason"`, string(outbound[1]["encrypted_content"]))

	assertOutboundItemType(t, outbound[2], "message")
	require.JSONEq(t, `"msg_after_standalone_reason"`, string(outbound[2]["id"]),
		"following assistant message id must remain on the message item, not the reasoning item")
	require.JSONEq(t, `"assistant"`, string(outbound[2]["role"]))

	assertOutboundItemType(t, outbound[3], "reasoning")
	_, hasReasoningID := outbound[3]["id"]
	require.False(t, hasReasoningID, "source reasoning had no id; outbound must omit id (no synthesis)")

	assertOutboundItemType(t, outbound[4], "message")
	require.JSONEq(t, `"assistant"`, string(outbound[4]["role"]))
	_, hasMessageID := outbound[4]["id"]
	require.False(t, hasMessageID, "source assistant message had no id; outbound must omit id")
}

// Pure standalone: reasoning is the last input item (no following assistant/tool).
// Supplied reasoning item.id must still round-trip at the public request-body seam.
func TestG15c_PureStandaloneReasoningItemIdentity_PreservesSuppliedID(t *testing.T) {
	body, err := os.ReadFile("testdata/g15c-pure-standalone-reasoning-item-identity.request.json")
	require.NoError(t, err)

	var source map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	var sourceInput []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["input"], &sourceInput))
	require.Len(t, sourceInput, 2)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	require.Len(t, llmReq.Messages, 2)
	require.Equal(t, "user", llmReq.Messages[0].Role)

	require.Equal(t, "assistant", llmReq.Messages[1].Role)
	require.Empty(t, llmReq.Messages[1].ID, "pure standalone reasoning has no following message id")
	require.NotNil(t, llmReq.Messages[1].ResponseReasoningItemID)
	require.Equal(t, "rs_pure_standalone", *llmReq.Messages[1].ResponseReasoningItemID)
	require.NotNil(t, llmReq.Messages[1].ReasoningSignature)
	require.Equal(t, "gAAAA_pure_standalone", *llmReq.Messages[1].ReasoningSignature)
	require.Empty(t, llmReq.Messages[1].ToolCalls)

	inputRaw, ok := payload["input"]
	require.True(t, ok)
	var outbound []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(inputRaw, &outbound))
	require.Len(t, outbound, 2, "pure standalone must not drop the reasoning item")

	assertOutboundItemType(t, outbound[0], "message")
	require.JSONEq(t, `"user"`, string(outbound[0]["role"]))

	assertOutboundItemType(t, outbound[1], "reasoning")
	require.JSONEq(t, `"rs_pure_standalone"`, string(outbound[1]["id"]),
		"pure standalone reasoning item id must round-trip with no following message/tool")
	require.JSONEq(t, `"gAAAA_pure_standalone"`, string(outbound[1]["encrypted_content"]))
}

// Summary-only reasoning (no encrypted_content) must still emit type=reasoning and
// preserve/omit item.id. Gate must not depend on encrypted_content alone.
func TestG15c_SummaryOnlyReasoningItemIdentity_PreservesPresenceAndID(t *testing.T) {
	body, err := os.ReadFile("testdata/g15c-summary-only-reasoning-item-identity.request.json")
	require.NoError(t, err)

	var source map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	var sourceInput []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["input"], &sourceInput))
	require.Len(t, sourceInput, 3)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	require.Len(t, llmReq.Messages, 3)
	require.Equal(t, "user", llmReq.Messages[0].Role)

	// With id, no encrypted_content.
	require.Equal(t, "assistant", llmReq.Messages[1].Role)
	require.NotNil(t, llmReq.Messages[1].ResponseReasoningItemID)
	require.Equal(t, "rs_summary_only_with_id", *llmReq.Messages[1].ResponseReasoningItemID)
	require.Nil(t, llmReq.Messages[1].ReasoningSignature)
	require.NotNil(t, llmReq.Messages[1].ReasoningContent)
	require.Equal(t, "summary with item id, no encrypted_content", *llmReq.Messages[1].ReasoningContent)

	// Without id, no encrypted_content: still a Responses reasoning origin.
	require.Equal(t, "assistant", llmReq.Messages[2].Role)
	require.NotNil(t, llmReq.Messages[2].ResponseReasoningItemID,
		"summary-only Responses reasoning without id must still mark origin")
	require.Empty(t, *llmReq.Messages[2].ResponseReasoningItemID)
	require.Nil(t, llmReq.Messages[2].ReasoningSignature)
	require.NotNil(t, llmReq.Messages[2].ReasoningContent)
	require.Equal(t, "summary without item id and without encrypted_content", *llmReq.Messages[2].ReasoningContent)

	inputRaw, ok := payload["input"]
	require.True(t, ok)
	var outbound []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(inputRaw, &outbound))
	require.Len(t, outbound, 3, "summary-only reasoning items must not be dropped")

	assertOutboundItemType(t, outbound[0], "message")
	require.JSONEq(t, `"user"`, string(outbound[0]["role"]))

	assertOutboundItemType(t, outbound[1], "reasoning")
	require.JSONEq(t, `"rs_summary_only_with_id"`, string(outbound[1]["id"]),
		"summary-only reasoning with id must preserve id without encrypted_content")
	_, hasEnc1 := outbound[1]["encrypted_content"]
	require.False(t, hasEnc1, "source had no encrypted_content; outbound must omit it")
	require.Contains(t, string(outbound[1]["summary"]), "summary with item id, no encrypted_content")

	assertOutboundItemType(t, outbound[2], "reasoning")
	_, hasID2 := outbound[2]["id"]
	require.False(t, hasID2, "source summary-only reasoning had no id; outbound must omit id")
	_, hasEnc2 := outbound[2]["encrypted_content"]
	require.False(t, hasEnc2)
	require.Contains(t, string(outbound[2]["summary"]), "summary without item id and without encrypted_content")
}

// G15c: when reasoning is merged with a following function_call / custom_tool_call,
// the reasoning item id remains independent of tool ResponseItemID / call_id.
func TestG15c_ReasoningFollowingToolIdentity_PreservesReasoningIDs(t *testing.T) {
	body, err := os.ReadFile("testdata/g15c-reasoning-following-tool-item-identity.request.json")
	require.NoError(t, err)

	var source map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	var sourceInput []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["input"], &sourceInput))
	require.Len(t, sourceInput, 7)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	require.Len(t, llmReq.Messages, 5)

	require.Equal(t, "assistant", llmReq.Messages[1].Role)
	require.NotNil(t, llmReq.Messages[1].ResponseReasoningItemID)
	require.Equal(t, "rs_before_function", *llmReq.Messages[1].ResponseReasoningItemID)
	require.Empty(t, llmReq.Messages[1].ID, "no following message item; Message.ID must stay empty")
	require.Len(t, llmReq.Messages[1].ToolCalls, 1)
	require.Equal(t, "fc_after_reason", llmReq.Messages[1].ToolCalls[0].ResponseItemID)
	require.Equal(t, "call_weather_after_reason", llmReq.Messages[1].ToolCalls[0].ID)

	require.Equal(t, "assistant", llmReq.Messages[3].Role)
	require.NotNil(t, llmReq.Messages[3].ResponseReasoningItemID)
	require.Equal(t, "rs_before_custom", *llmReq.Messages[3].ResponseReasoningItemID)
	require.Empty(t, llmReq.Messages[3].ID)
	require.Len(t, llmReq.Messages[3].ToolCalls, 1)
	ctc := llmReq.Messages[3].ToolCalls[0]
	require.Equal(t, llm.ToolTypeResponsesCustomTool, ctc.Type)
	require.Equal(t, "ctc_after_reason", ctc.ResponseItemID)
	require.Equal(t, "call_patch_after_reason", ctc.ID)

	inputRaw, ok := payload["input"]
	require.True(t, ok)
	var outbound []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(inputRaw, &outbound))
	require.Len(t, outbound, 7)

	assertOutboundItemType(t, outbound[1], "reasoning")
	require.JSONEq(t, `"rs_before_function"`, string(outbound[1]["id"]),
		"reasoning id before function_call must round-trip and not be confused with tool item id")
	assertOutboundItemType(t, outbound[2], "function_call")
	require.JSONEq(t, `"fc_after_reason"`, string(outbound[2]["id"]))
	require.JSONEq(t, `"call_weather_after_reason"`, string(outbound[2]["call_id"]))

	assertOutboundItemType(t, outbound[4], "reasoning")
	require.JSONEq(t, `"rs_before_custom"`, string(outbound[4]["id"]),
		"reasoning id before custom_tool_call must round-trip and not be confused with tool item id")
	assertOutboundItemType(t, outbound[5], "custom_tool_call")
	require.JSONEq(t, `"ctc_after_reason"`, string(outbound[5]["id"]))
	require.JSONEq(t, `"call_patch_after_reason"`, string(outbound[5]["call_id"]))
}

// Ordinary ReasoningContent without a Responses origin marker must not invent a
// type=reasoning input item on same-family Responses outbound. This protects the
// *string presence semantics of ResponseReasoningItemID.
func TestG15c_CrossProtocolReasoningContent_DoesNotInventResponsesReasoningItem(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": [
			{
				"type": "message",
				"role": "user",
				"content": [{"type": "input_text", "text": "hi"}]
			}
		]
	}`

	payload, _ := roundTripResponsesRawPayload(t, body, func(req *llm.Request) {
		req.Messages = append(req.Messages, llm.Message{
			Role:             "assistant",
			ReasoningContent: lo.ToPtr("chat-style reasoning only"),
			Content:          llm.MessageContent{Content: lo.ToPtr("answer")},
		})
	})

	inputRaw, ok := payload["input"]
	require.True(t, ok)
	var outbound []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(inputRaw, &outbound))

	for i, item := range outbound {
		if rawType, has := item["type"]; has {
			require.NotEqual(t, `"reasoning"`, string(rawType),
				"item %d must not invent a Responses reasoning item from bare ReasoningContent", i)
		}
	}
}
