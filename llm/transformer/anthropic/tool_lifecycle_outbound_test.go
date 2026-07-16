package anthropic

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

// B1: public outbound seam must NOT globally scan/hoist a late tool result
// across an intervening user turn. Anthropic requires tool_result to
// immediately follow tool_use; invalid source order is rejected.
func TestOutboundTransformer_ToolLifecycle_B1_RejectsNonAdjacentLateToolResult(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_A",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "lookup",
						Arguments: `{"q":"x"}`,
					},
				}},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("intervening user turn"),
				},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_A"),
				Content: llm.MessageContent{
					Content: lo.ToPtr("late result"),
				},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	require.Error(t, err, "late tool_result across intervening user turn must not be silently reordered")
	require.True(t, errors.Is(err, transformer.ErrInvalidRequest), "got %v", err)
	require.Nil(t, result)
}

// B2: parallel tool_use batch requires exact adjacent result coverage. Missing
// results are rejected; results are never synthesized.
func TestOutboundTransformer_ToolLifecycle_B2_RejectsIncompleteParallelResultBatch(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_A",
						Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{
							Name:      "alpha",
							Arguments: `{}`,
						},
					},
					{
						ID:   "call_B",
						Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{
							Name:      "beta",
							Arguments: `{}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_A"),
				Content: llm.MessageContent{
					Content: lo.ToPtr("only A"),
				},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	require.Error(t, err, "incomplete parallel tool_result batch must be rejected")
	require.True(t, errors.Is(err, transformer.ErrInvalidRequest), "got %v", err)
	require.Nil(t, result)
}

// B2: expected tool_use with zero results (no adjacent batch at all) must reject.
// Anthropic requires every tool_use to be followed by a user tool_result turn.
func TestOutboundTransformer_ToolLifecycle_B2_RejectsMissingAllParallelResults(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_A",
						Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{Name: "alpha", Arguments: `{}`},
					},
					{
						ID:   "call_B",
						Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{Name: "beta", Arguments: `{}`},
					},
				},
			},
			// No tool results at all — only a later user turn.
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("continue without results"),
				},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	require.Error(t, err, "tool_use without any tool_result must be rejected")
	require.True(t, errors.Is(err, transformer.ErrInvalidRequest), "got %v", err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "missing")
}

// B2 single-call variant: trailing assistant tool_use with no subsequent result.
func TestOutboundTransformer_ToolLifecycle_B2_RejectsTrailingToolUseWithoutResults(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_only",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "lookup", Arguments: `{"q":"x"}`},
				}},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	require.Error(t, err, "trailing tool_use without tool_result must be rejected")
	require.True(t, errors.Is(err, transformer.ErrInvalidRequest), "got %v", err)
	require.Nil(t, result)
}

// B1 guard: intervening user with NO late result must also reject (missing results),
// not succeed by dropping the lifecycle pairing.
func TestOutboundTransformer_ToolLifecycle_B1_RejectsInterveningUserWithNoResults(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_A",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "lookup", Arguments: `{"q":"x"}`},
				}},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("intervening user turn, no tool result ever"),
				},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	require.Error(t, err)
	require.True(t, errors.Is(err, transformer.ErrInvalidRequest), "got %v", err)
	require.Nil(t, result)
}

// B2: adjacent batch with an unknown tool_call_id is rejected.
func TestOutboundTransformer_ToolLifecycle_B2_RejectsUnknownAdjacentToolResultID(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_A",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "alpha", Arguments: `{}`},
				}},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_unknown"),
				Content:    llm.MessageContent{Content: lo.ToPtr("not for this turn")},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	require.Error(t, err, "adjacent tool_result with unknown tool_call_id must be rejected")
	require.True(t, errors.Is(err, transformer.ErrInvalidRequest), "got %v", err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "call_unknown")
}

// B2: adjacent batch with duplicate results for the same call_id is rejected.
func TestOutboundTransformer_ToolLifecycle_B2_RejectsDuplicateAdjacentToolResultID(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_A",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "alpha", Arguments: `{}`},
				}},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_A"),
				Content:    llm.MessageContent{Content: lo.ToPtr("first")},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_A"),
				Content:    llm.MessageContent{Content: lo.ToPtr("duplicate")},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	require.Error(t, err, "duplicate adjacent tool_result for same call_id must be rejected")
	require.True(t, errors.Is(err, transformer.ErrInvalidRequest), "got %v", err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "duplicate")
}

// Adjacent complete parallel batch remains valid (control fixture for B2).
func TestOutboundTransformer_ToolLifecycle_B2_AcceptsCompleteAdjacentParallelBatch(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_A",
						Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{Name: "alpha", Arguments: `{}`},
					},
					{
						ID:   "call_B",
						Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{Name: "beta", Arguments: `{}`},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_A"),
				Content:    llm.MessageContent{Content: lo.ToPtr("A ok")},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_B"),
				Content:    llm.MessageContent{Content: lo.ToPtr("B ok")},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	require.NoError(t, err)

	var anthropicReq MessageRequest
	require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
	require.GreaterOrEqual(t, len(anthropicReq.Messages), 2)

	assistant := anthropicReq.Messages[0]
	require.Equal(t, "assistant", assistant.Role)
	useIDs := []string{}
	for _, block := range assistant.Content.MultipleContent {
		if block.Type == "tool_use" {
			useIDs = append(useIDs, block.ID)
		}
	}
	require.Equal(t, []string{"call_A", "call_B"}, useIDs)

	user := anthropicReq.Messages[1]
	require.Equal(t, "user", user.Role)
	resultIDs := []string{}
	for _, block := range user.Content.MultipleContent {
		if block.Type == "tool_result" && block.ToolUseID != nil {
			resultIDs = append(resultIDs, *block.ToolUseID)
		}
	}
	require.Equal(t, []string{"call_A", "call_B"}, resultIDs)
}

// B3: OpenAI Chat/Responses custom tools must not become empty-name Anthropic
// tools or empty-name tool_use blocks. Freeform custom is lossy, not coerced.
func TestOutboundTransformer_ToolLifecycle_B3_ChatCustomDoesNotEmitEmptyAnthropicTool(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Tools: []llm.Tool{{
			Type: "custom",
			OpenAIChatCustomTool: &llm.OpenAIChatCustomTool{
				Name: "run_sql",
			},
		}},
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_c1",
					Type: "custom",
					OpenAIChatCustomToolCall: &llm.OpenAIChatCustomToolCall{
						Name:  "run_sql",
						Input: "SELECT 1",
					},
				}},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_c1"),
				Content:    llm.MessageContent{Content: lo.ToPtr("ok")},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	// Either reject the unsupported custom lifecycle, or convert without empty
	// Anthropic tool shapes and with an explicit LossyDowngrade.
	if err != nil {
		require.True(t, errors.Is(err, transformer.ErrInvalidRequest), "got %v", err)
		require.NotEmpty(t, llm.LossyDowngrades(req), "custom lifecycle loss must be diagnosed before reject")
		return
	}

	var anthropicReq MessageRequest
	require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))

	for _, tool := range anthropicReq.Tools {
		require.NotEmpty(t, tool.Name, "must not emit Anthropic tool with empty name")
		require.NotEqual(t, "custom", tool.Type, "must not coerce freeform custom into Anthropic tool type")
		// Freeform custom has no JSON input_schema equivalent; never invent {}.
		if tool.Name == "run_sql" {
			require.Fail(t, "must not auto-bridge freeform custom tool declaration to Anthropic tools")
		}
	}

	for _, msg := range anthropicReq.Messages {
		for _, block := range msg.Content.MultipleContent {
			if block.Type == "tool_use" {
				require.NotNil(t, block.Name)
				require.NotEmpty(t, *block.Name, "must not emit empty-name tool_use for custom call")
				require.NotEqual(t, "call_c1", block.ID, "custom call must not be coerced into empty function tool_use")
			}
		}
	}

	downgrades := llm.LossyDowngrades(req)
	require.NotEmpty(t, downgrades, "custom→Anthropic loss must emit LossyDowngrade")
	found := false
	for _, d := range downgrades {
		if d.TargetProtocol == llm.APIFormatAnthropicMessage &&
			(d.SourceField == "tools[].type=custom" ||
				d.SourceField == "messages[].tool_calls[].type=custom" ||
				d.SourceField == "input[].type=custom_tool_call") {
			found = true
			require.Equal(t, llm.LossyDowngradeReasonNoEquivalentSemantics, d.Reason)
		}
	}
	require.True(t, found, "expected custom-tool LossyDowngrade, got %#v", downgrades)
}

func TestOutboundTransformer_ToolLifecycle_B3_ResponsesCustomDoesNotEmitEmptyAnthropicTool(t *testing.T) {
	transformerOut, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		APIFormat: llm.APIFormatOpenAIResponse,
		Tools: []llm.Tool{{
			Type: llm.ToolTypeResponsesCustomTool,
			ResponseCustomTool: &llm.ResponseCustomTool{
				Name: "apply_patch",
			},
		}},
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_patch_1",
					Type: llm.ToolTypeResponsesCustomTool,
					ResponseCustomToolCall: &llm.ResponseCustomToolCall{
						CallID: "call_patch_1",
						Name:   "apply_patch",
						Input:  "*** Begin Patch\n*** End Patch",
					},
				}},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_patch_1"),
				Content:    llm.MessageContent{Content: lo.ToPtr("done")},
			},
		},
	}

	result, err := transformerOut.TransformRequest(t.Context(), req)
	if err != nil {
		require.True(t, errors.Is(err, transformer.ErrInvalidRequest), "got %v", err)
		require.NotEmpty(t, llm.LossyDowngrades(req), "Responses custom loss must be diagnosed")
		return
	}

	var anthropicReq MessageRequest
	require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))

	for _, tool := range anthropicReq.Tools {
		require.NotEmpty(t, tool.Name)
		require.NotEqual(t, "apply_patch", tool.Name, "must not auto-bridge Responses custom declaration")
	}
	for _, msg := range anthropicReq.Messages {
		for _, block := range msg.Content.MultipleContent {
			if block.Type == "tool_use" {
				require.NotNil(t, block.Name)
				require.NotEmpty(t, *block.Name)
				// Empty Function carrier would previously yield empty name.
				require.NotEqual(t, "", *block.Name)
				require.NotEqual(t, "call_patch_1", block.ID)
			}
		}
	}

	downgrades := llm.LossyDowngrades(req)
	require.NotEmpty(t, downgrades)
	found := false
	for _, d := range downgrades {
		if d.TargetProtocol == llm.APIFormatAnthropicMessage &&
			(d.SourceField == "tools[].type=custom" ||
				d.SourceField == "input[].type=custom_tool_call" ||
				d.SourceField == "messages[].tool_calls[].type=custom") {
			found = true
		}
	}
	require.True(t, found, "expected Responses custom LossyDowngrade, got %#v", downgrades)
}
