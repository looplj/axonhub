package responses

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// G15b residual coverage: custom_tool_call / custom_tool_call_output use the same
// public inbound→canonical→outbound seam as function_call variants. Item.id and
// call_id must stay independent, no id synthesis, order preserved. Codex prefix
// policy and output-stream paths are out of scope.
func TestG15b_CustomToolItemIdentity_PreservesSuppliedIDsAndOrder(t *testing.T) {
	body, err := os.ReadFile("testdata/g15b-custom-tool-item-identity.request.json")
	require.NoError(t, err)

	var source map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	var sourceInput []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["input"], &sourceInput))
	require.Len(t, sourceInput, 6)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	// Canonical: custom tool call keeps ResponseItemID separate from call_id;
	// custom tool output stores item id on Message.ID and call_id on ToolCallID.
	require.Len(t, llmReq.Messages, 6)

	require.Equal(t, "msg_user_custom", llmReq.Messages[0].ID)
	require.Equal(t, "user", llmReq.Messages[0].Role)

	require.Equal(t, "assistant", llmReq.Messages[1].Role)
	require.Len(t, llmReq.Messages[1].ToolCalls, 1)
	tcWithID := llmReq.Messages[1].ToolCalls[0]
	require.Equal(t, llm.ToolTypeResponsesCustomTool, tcWithID.Type)
	require.Equal(t, "call_patch_legacy", tcWithID.ID)
	require.Equal(t, "ctc_item_legacy", tcWithID.ResponseItemID)
	require.NotNil(t, tcWithID.ResponseCustomToolCall)
	require.Equal(t, "call_patch_legacy", tcWithID.ResponseCustomToolCall.CallID)
	require.Equal(t, "apply_patch", tcWithID.ResponseCustomToolCall.Name)

	require.Equal(t, "tool", llmReq.Messages[2].Role)
	require.Equal(t, "ctc_out_legacy", llmReq.Messages[2].ID)
	require.NotNil(t, llmReq.Messages[2].ToolCallID)
	require.Equal(t, "call_patch_legacy", *llmReq.Messages[2].ToolCallID)

	require.Equal(t, "assistant", llmReq.Messages[3].Role)
	require.Len(t, llmReq.Messages[3].ToolCalls, 1)
	tcNoID := llmReq.Messages[3].ToolCalls[0]
	require.Equal(t, llm.ToolTypeResponsesCustomTool, tcNoID.Type)
	require.Equal(t, "call_patch_no_item_id", tcNoID.ID)
	require.Empty(t, tcNoID.ResponseItemID, "canonical must not invent custom_tool_call item ids")
	require.NotNil(t, tcNoID.ResponseCustomToolCall)
	require.Equal(t, "call_patch_no_item_id", tcNoID.ResponseCustomToolCall.CallID)

	require.Equal(t, "tool", llmReq.Messages[4].Role)
	require.Empty(t, llmReq.Messages[4].ID, "source custom_tool_call_output had no id")
	require.NotNil(t, llmReq.Messages[4].ToolCallID)
	require.Equal(t, "call_patch_no_item_id", *llmReq.Messages[4].ToolCallID)

	require.Equal(t, "assistant", llmReq.Messages[5].Role)
	require.Empty(t, llmReq.Messages[5].ID)

	inputRaw, ok := payload["input"]
	require.True(t, ok, "outbound must emit input array")
	var outbound []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(inputRaw, &outbound))
	require.Len(t, outbound, 6, "item order and cardinality must be preserved")

	assertOutboundItemType(t, outbound[0], "message")
	require.JSONEq(t, `"msg_user_custom"`, string(outbound[0]["id"]))
	require.JSONEq(t, `"user"`, string(outbound[0]["role"]))

	assertOutboundItemType(t, outbound[1], "custom_tool_call")
	require.JSONEq(t, `"ctc_item_legacy"`, string(outbound[1]["id"]),
		"custom_tool_call item id must be preserved and not confused with call_id")
	require.JSONEq(t, `"call_patch_legacy"`, string(outbound[1]["call_id"]))
	require.JSONEq(t, `"apply_patch"`, string(outbound[1]["name"]))

	assertOutboundItemType(t, outbound[2], "custom_tool_call_output")
	require.JSONEq(t, `"ctc_out_legacy"`, string(outbound[2]["id"]),
		"custom_tool_call_output item id must be preserved and not confused with call_id")
	require.JSONEq(t, `"call_patch_legacy"`, string(outbound[2]["call_id"]))

	assertOutboundItemType(t, outbound[3], "custom_tool_call")
	_, hasCustomCallID := outbound[3]["id"]
	require.False(t, hasCustomCallID, "source custom_tool_call had no id; outbound must omit id")
	require.JSONEq(t, `"call_patch_no_item_id"`, string(outbound[3]["call_id"]))

	assertOutboundItemType(t, outbound[4], "custom_tool_call_output")
	_, hasCustomOutID := outbound[4]["id"]
	require.False(t, hasCustomOutID, "source custom_tool_call_output had no id; outbound must omit id")
	require.JSONEq(t, `"call_patch_no_item_id"`, string(outbound[4]["call_id"]))

	assertOutboundItemType(t, outbound[5], "message")
	require.JSONEq(t, `"assistant"`, string(outbound[5]["role"]))
	_, hasAssistantID := outbound[5]["id"]
	require.False(t, hasAssistantID)
}

// G15b residual coverage: when a reasoning item is immediately followed by a
// function_call or custom_tool_call, convertReasoningWithFollowing merges the
// tool into the same assistant message. Following tool item.id and call_id must
// still round-trip independently at the public request-body seam.
func TestG15b_ReasoningFollowingToolIdentity_PreservesToolIDs(t *testing.T) {
	body, err := os.ReadFile("testdata/g15b-reasoning-following-tool-identity.request.json")
	require.NoError(t, err)

	var source map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	var sourceInput []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["input"], &sourceInput))
	require.Len(t, sourceInput, 7)

	payload, llmReq := roundTripResponsesRawPayload(t, string(body), nil)

	// reasoning + following tool merge into one assistant message per pair.
	// Expected messages: user, assistant(function), tool(function output),
	// assistant(custom), tool(custom output) => 5 canonical messages.
	require.Len(t, llmReq.Messages, 5)

	require.Equal(t, "user", llmReq.Messages[0].Role)

	require.Equal(t, "assistant", llmReq.Messages[1].Role)
	require.NotNil(t, llmReq.Messages[1].ReasoningSignature)
	require.Equal(t, "gAAAA_reason_sig", *llmReq.Messages[1].ReasoningSignature)
	require.Len(t, llmReq.Messages[1].ToolCalls, 1)
	fc := llmReq.Messages[1].ToolCalls[0]
	require.Equal(t, "function", fc.Type)
	require.Equal(t, "call_weather_after_reason", fc.ID)
	require.Equal(t, "fc_after_reason", fc.ResponseItemID)
	require.Equal(t, "get_weather", fc.Function.Name)

	require.Equal(t, "tool", llmReq.Messages[2].Role)
	require.Equal(t, "fc_out_after_reason", llmReq.Messages[2].ID)
	require.NotNil(t, llmReq.Messages[2].ToolCallID)
	require.Equal(t, "call_weather_after_reason", *llmReq.Messages[2].ToolCallID)

	require.Equal(t, "assistant", llmReq.Messages[3].Role)
	require.NotNil(t, llmReq.Messages[3].ReasoningSignature)
	require.Equal(t, "gAAAA_reason_sig_2", *llmReq.Messages[3].ReasoningSignature)
	require.Len(t, llmReq.Messages[3].ToolCalls, 1)
	ctc := llmReq.Messages[3].ToolCalls[0]
	require.Equal(t, llm.ToolTypeResponsesCustomTool, ctc.Type)
	require.Equal(t, "call_patch_after_reason", ctc.ID)
	require.Equal(t, "ctc_after_reason", ctc.ResponseItemID)
	require.NotNil(t, ctc.ResponseCustomToolCall)
	require.Equal(t, "call_patch_after_reason", ctc.ResponseCustomToolCall.CallID)

	require.Equal(t, "tool", llmReq.Messages[4].Role)
	require.Equal(t, "ctc_out_after_reason", llmReq.Messages[4].ID)
	require.NotNil(t, llmReq.Messages[4].ToolCallID)
	require.Equal(t, "call_patch_after_reason", *llmReq.Messages[4].ToolCallID)

	inputRaw, ok := payload["input"]
	require.True(t, ok)
	var outbound []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(inputRaw, &outbound))
	require.Len(t, outbound, 7, "reasoning + following tools + outputs order must be preserved")

	assertOutboundItemType(t, outbound[0], "message")
	require.JSONEq(t, `"user"`, string(outbound[0]["role"]))

	assertOutboundItemType(t, outbound[1], "reasoning")
	// Following function_call identity is the G15b claim under test.
	assertOutboundItemType(t, outbound[2], "function_call")
	require.JSONEq(t, `"fc_after_reason"`, string(outbound[2]["id"]),
		"function_call after reasoning must preserve item id independently of call_id")
	require.JSONEq(t, `"call_weather_after_reason"`, string(outbound[2]["call_id"]))
	require.JSONEq(t, `"get_weather"`, string(outbound[2]["name"]))

	assertOutboundItemType(t, outbound[3], "function_call_output")
	require.JSONEq(t, `"fc_out_after_reason"`, string(outbound[3]["id"]))
	require.JSONEq(t, `"call_weather_after_reason"`, string(outbound[3]["call_id"]))

	assertOutboundItemType(t, outbound[4], "reasoning")
	assertOutboundItemType(t, outbound[5], "custom_tool_call")
	require.JSONEq(t, `"ctc_after_reason"`, string(outbound[5]["id"]),
		"custom_tool_call after reasoning must preserve item id independently of call_id")
	require.JSONEq(t, `"call_patch_after_reason"`, string(outbound[5]["call_id"]))
	require.JSONEq(t, `"apply_patch"`, string(outbound[5]["name"]))

	assertOutboundItemType(t, outbound[6], "custom_tool_call_output")
	require.JSONEq(t, `"ctc_out_after_reason"`, string(outbound[6]["id"]))
	require.JSONEq(t, `"call_patch_after_reason"`, string(outbound[6]["call_id"]))
}
