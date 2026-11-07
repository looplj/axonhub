package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestToolUseIDsAcrossAnthropicResponses(t *testing.T) {
	ids := []string{"Bash:0", "Bash.0", "Bash_0", "call-AbC_123", "工具:0", "", strings.Repeat("x:", 1000)}
	seen := make(map[string]string)
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			call := llm.ToolCall{ID: id, Type: "function", Function: llm.FunctionCall{Name: "Bash", Arguments: `{ "command": "echo hi" }`}}
			message := llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{call}}
			response := convertToAnthropicResponse(&llm.Response{Choices: []llm.Choice{{Message: &message}}})
			require.Len(t, response.Content, 1)
			mapped := response.Content[0].ID
			require.Regexp(t, `^[a-zA-Z0-9_-]+$`, mapped)
			if id == "Bash_0" || id == "call-AbC_123" {
				require.Equal(t, id, mapped)
			} else {
				require.NotEqual(t, id, mapped)
				require.Len(t, mapped, 38)
			}
			previous, exists := seen[mapped]
			require.False(t, exists, "IDs %q and %q collided", previous, id)
			seen[mapped] = id

			// Continuation chunks omit the ID. They must not start a second block or
			// change the ID already emitted in content_block_start.
			first := call
			first.Function.Arguments = `{"command":`
			continuation := llm.ToolCall{Index: 0, Function: llm.FunctionCall{Arguments: `"echo hi"}`}}
			events := collectInboundStreamEvents(t, NewInboundTransformer(), []*llm.Response{
				{ID: "msg_test", Choices: []llm.Choice{{Delta: &llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{first}}}}},
				{ID: "msg_test", Choices: []llm.Choice{{Delta: &llm.Message{ToolCalls: []llm.ToolCall{continuation}}}}},
				{ID: "msg_test", Choices: []llm.Choice{{FinishReason: lo.ToPtr("tool_calls")}}},
			})
			var streamIDs []string
			var arguments strings.Builder
			for _, event := range events {
				if event.Type == "content_block_start" && event.ContentBlock.Type == "tool_use" {
					streamIDs = append(streamIDs, event.ContentBlock.ID)
				}
				if event.Type == "content_block_delta" && event.Delta.PartialJSON != nil {
					arguments.WriteString(*event.Delta.PartialJSON)
				}
			}
			require.Equal(t, []string{mapped}, streamIDs)
			require.JSONEq(t, call.Function.Arguments, arguments.String())

			require.Equal(t, id, message.ToolCalls[0].ID)
		})
	}
}

func TestToolUseIDNormalizationPreservesSpecialToolBlocks(t *testing.T) {
	for _, blockType := range []string{"server_tool_use", "mcp_tool_use"} {
		t.Run(blockType, func(t *testing.T) {
			call := llm.ToolCall{
				ID: "special:id", Function: llm.FunctionCall{Name: "search", Arguments: `{}`},
				TransformerMetadata: map[string]any{TransformerMetadataKeyAnthropicType: blockType},
			}
			response := convertToAnthropicResponse(&llm.Response{Choices: []llm.Choice{{Message: &llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{call}}}}})
			require.Equal(t, call.ID, response.Content[0].ID)
		})
	}
}

func TestToolUseIDNormalizationAfterRestoringResponseContent(t *testing.T) {
	for _, blockType := range []string{"server_tool_use", "mcp_tool_use"} {
		for _, serialized := range []bool{false, true} {
			name := blockType + "/typed_metadata"
			if serialized {
				name = blockType + "/serialized_metadata"
			}
			t.Run(name, func(t *testing.T) {
				upstream := &Message{
					ID: "msg_mixed_tools", Role: "assistant", Model: "test",
					Content: []MessageContentBlock{
						{Type: "text", Text: lo.ToPtr("Using tools")},
						{Type: "tool_use", ID: "Bash:0", Name: lo.ToPtr("Bash"), Input: json.RawMessage(`{"command":"echo hi"}`)},
						{Type: blockType, ID: "special:id", Name: lo.ToPtr("search"), Input: json.RawMessage(`{}`)},
						{Type: "tool_use", ID: "call_valid", Name: lo.ToPtr("lookup"), Input: json.RawMessage(`{}`)},
					},
				}
				response := convertToLlmResponse(upstream, PlatformDirect)
				require.Contains(t, response.TransformerMetadata, TransformerMetadataKeyAnthropicResponseContent)
				if serialized {
					data, err := json.Marshal(response.TransformerMetadata)
					require.NoError(t, err)
					require.NoError(t, json.Unmarshal(data, &response.TransformerMetadata))
				}
				before, err := json.Marshal(response)
				require.NoError(t, err)

				result := convertToAnthropicResponse(response)
				require.Len(t, result.Content, len(upstream.Content))
				require.NotEqual(t, "Bash:0", result.Content[1].ID)
				require.Regexp(t, `^toolu_[a-f0-9]{32}$`, result.Content[1].ID)
				expected := append([]MessageContentBlock(nil), upstream.Content...)
				expected[1].ID = normalizeToolUseID("Bash:0")
				require.Equal(t, expected, result.Content)
				require.Equal(t, "Bash:0", upstream.Content[1].ID)
				after, err := json.Marshal(response)
				require.NoError(t, err)
				require.JSONEq(t, string(before), string(after))
			})
		}
	}
}
