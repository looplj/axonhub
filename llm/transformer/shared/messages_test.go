package shared

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestFilterOutResponseCustomToolMessages(t *testing.T) {
	input := []llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_custom_1",
					Type: llm.ToolTypeResponsesCustomTool,
					ResponseCustomToolCall: &llm.ResponseCustomToolCall{
						CallID: "call_custom_1",
						Name:   "apply_patch",
						Input:  "*** Begin Patch\n*** End Patch\n",
					},
				},
				{
					ID:   "call_function_1",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "get_weather",
						Arguments: "{\"city\":\"Shanghai\"}",
					},
				},
			},
		},
		{
			Role:       "tool",
			ToolCallID: lo.ToPtr("call_custom_1"),
			Content: llm.MessageContent{
				Content: lo.ToPtr("custom tool output"),
			},
		},
		{
			Role:       "tool",
			ToolCallID: lo.ToPtr("call_function_1"),
			Content: llm.MessageContent{
				Content: lo.ToPtr("{\"temperature\":22}"),
			},
		},
	}

	got := FilterOutResponseCustomToolMessages(input)

	require.Len(t, got, 2)
	require.Len(t, got[0].ToolCalls, 1)
	require.Equal(t, llm.ToolTypeFunction, got[0].ToolCalls[0].Type)
	require.Equal(t, "call_function_1", got[0].ToolCalls[0].ID)
	require.NotNil(t, got[1].ToolCallID)
	require.Equal(t, "call_function_1", *got[1].ToolCallID)
}

func TestFilterOutResponseCustomToolMessages_KeepsVisibleAssistantMessage(t *testing.T) {
	input := []llm.Message{
		{
			Role: "assistant",
			Content: llm.MessageContent{
				Content: lo.ToPtr("I'll update that."),
			},
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_custom_1",
					Type: llm.ToolTypeResponsesCustomTool,
					ResponseCustomToolCall: &llm.ResponseCustomToolCall{
						CallID: "call_custom_1",
						Name:   "apply_patch",
						Input:  "*** Begin Patch\n*** End Patch\n",
					},
				},
			},
		},
	}

	got := FilterOutResponseCustomToolMessages(input)

	require.Len(t, got, 1)
	require.Equal(t, "assistant", got[0].Role)
	require.NotNil(t, got[0].Content.Content)
	require.Equal(t, "I'll update that.", *got[0].Content.Content)
	require.Empty(t, got[0].ToolCalls)
}

func TestFilterOutResponsesOnlyTools(t *testing.T) {
	input := []llm.Tool{
		{
			Type: llm.ToolTypeFunction,
			Function: llm.Function{
				Name: "get_weather",
			},
			ProtocolExtensions: &llm.ProtocolExtensions{OpenAIResponses: &llm.OpenAIResponsesExtensions{}},
		},
		{Type: llm.ToolTypeWebSearch},
		{Type: "web_search_preview"},
		{
			Type: "future_codex_tool",
			ProtocolExtensions: &llm.ProtocolExtensions{OpenAIResponses: &llm.OpenAIResponsesExtensions{
				Tools: []llm.OpenAIResponsesRawItem{{Type: "future_codex_tool"}},
			}},
		},
	}

	got := FilterOutResponsesOnlyTools(input)

	require.Len(t, got, 1)
	require.Equal(t, llm.ToolTypeFunction, got[0].Type)
	require.Equal(t, "get_weather", got[0].Function.Name)
	require.Nil(t, got[0].ProtocolExtensions)
	require.NotNil(t, input[0].ProtocolExtensions)
}

func TestIsResponsesOnlyTool(t *testing.T) {
	cases := []struct {
		name string
		tool llm.Tool
		want bool
	}{
		{
			name: "web_search",
			tool: llm.Tool{Type: llm.ToolTypeWebSearch},
			want: true,
		},
		{
			name: "web_search_preview",
			tool: llm.Tool{Type: "web_search_preview"},
			want: true,
		},
		{
			name: "unknown with responses extension",
			tool: llm.Tool{
				Type:               "future_codex_tool",
				ProtocolExtensions: &llm.ProtocolExtensions{OpenAIResponses: &llm.OpenAIResponsesExtensions{}},
			},
			want: true,
		},
		{
			name: "function without responses extension",
			tool: llm.Tool{Type: llm.ToolTypeFunction},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, IsResponsesOnlyTool(tc.tool))
		})
	}
}
