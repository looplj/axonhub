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

func TestFilterOutResponsesChatToolLifecycleMessages_RemovesSpecialCallsAndPairedOutputs(t *testing.T) {
	tests := []struct {
		name string
		call llm.ToolCall
	}{
		{
			name: "custom",
			call: llm.ToolCall{
				ID: "call_special", Type: llm.ToolTypeResponsesCustomTool,
				ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_special", Name: "apply_patch"},
			},
		},
		{
			name: "custom nested call id only",
			call: llm.ToolCall{
				Type:                   llm.ToolTypeResponsesCustomTool,
				ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_special", Name: "apply_patch"},
			},
		},
		{
			name: "tool search",
			call: llm.ToolCall{
				ID: "call_special", Type: llm.ToolTypeResponsesToolSearch,
				ResponseToolSearchCall: &llm.ResponseToolSearchCall{CallID: "call_special", Execution: "client"},
			},
		},
		{
			name: "tool search nested call id only",
			call: llm.ToolCall{
				Type:                   llm.ToolTypeResponsesToolSearch,
				ResponseToolSearchCall: &llm.ResponseToolSearchCall{CallID: "call_special", Execution: "client"},
			},
		},
		{
			name: "namespace",
			call: llm.ToolCall{
				ID: "call_special", Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{Name: "spawn_agent", Namespace: "collaboration", Arguments: `{}`},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []llm.Message{
				{Role: "tool", ToolCallID: lo.ToPtr("call_special"), Content: llm.MessageContent{Content: lo.ToPtr("special output before call")}},
				{Role: "assistant", ToolCalls: []llm.ToolCall{
					tt.call,
					{ID: "call_plain", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "lookup", Arguments: `{}`}},
				}},
				{Role: "tool", ToolCallID: lo.ToPtr("call_plain"), Content: llm.MessageContent{Content: lo.ToPtr("plain output")}},
			}

			got := FilterOutResponsesChatToolLifecycleMessages(input)
			require.Len(t, got, 2)
			require.Equal(t, "assistant", got[0].Role)
			require.Len(t, got[0].ToolCalls, 1)
			require.Equal(t, "call_plain", got[0].ToolCalls[0].ID)
			require.Equal(t, "call_plain", lo.FromPtr(got[1].ToolCallID))

			emptyLifecycle := []llm.Message{
				{Role: "assistant", ToolCalls: []llm.ToolCall{tt.call}},
				{Role: "tool", ToolCallID: lo.ToPtr("call_special"), Content: llm.MessageContent{Content: lo.ToPtr("special output")}},
			}
			require.Empty(t, FilterOutResponsesChatToolLifecycleMessages(emptyLifecycle))
		})
	}
}

func TestFilterOutResponsesChatToolLifecycleMessages_RevalidatesAssistantPayload(t *testing.T) {
	specialCall := llm.ToolCall{
		ID: "call_special", Type: llm.ToolTypeResponsesCustomTool,
		ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_special", Name: "apply_patch"},
	}
	plainCall := llm.ToolCall{
		ID: "call_plain", Type: llm.ToolTypeFunction,
		Function: llm.FunctionCall{Name: "lookup", Arguments: `{}`},
	}
	assistantWithPart := func(part llm.MessageContentPart) llm.Message {
		return llm.Message{
			Role: "assistant",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{part},
			},
		}
	}
	tests := []struct {
		name         string
		message      llm.Message
		wantKept     bool
		wantToolCall string
	}{
		{name: "nil content", message: llm.Message{Role: "assistant"}},
		{name: "empty content", message: llm.Message{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("")}}},
		{name: "whitespace content", message: llm.Message{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr(" \n\t ")}}},
		{name: "empty reasoning content", message: llm.Message{Role: "assistant", ReasoningContent: lo.ToPtr("")}},
		{name: "whitespace reasoning", message: llm.Message{Role: "assistant", Reasoning: lo.ToPtr("  \n")}},
		{name: "whitespace refusal", message: llm.Message{Role: "assistant", Refusal: " \n\t"}},
		{name: "nil text part", message: assistantWithPart(llm.MessageContentPart{Type: "text"})},
		{name: "whitespace text part", message: assistantWithPart(llm.MessageContentPart{Type: "text", Text: lo.ToPtr(" \n")})},
		{name: "empty image part", message: assistantWithPart(llm.MessageContentPart{Type: "image_url", ImageURL: &llm.ImageURL{}})},
		{name: "empty input audio part", message: assistantWithPart(llm.MessageContentPart{Type: "input_audio", InputAudio: &llm.InputAudio{Format: "wav"}})},
		{name: "empty output audio", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{}}},
		{name: "whitespace output audio", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{ID: " ", Data: "\n", Transcript: "\t"}}},
		{name: "expiry only output audio", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{ExpiresAt: 123}}},
		{name: "unsupported document part", message: assistantWithPart(llm.MessageContentPart{Type: "document", Document: &llm.DocumentURL{URL: "https://example.com/file.pdf"}})},
		{
			name: "multiple content overrides visible scalar content",
			message: llm.Message{Role: "assistant", Content: llm.MessageContent{
				Content: lo.ToPtr("ignored"), MultipleContent: []llm.MessageContentPart{{Type: "text", Text: lo.ToPtr(" ")}},
			}},
		},
		{name: "content", message: llm.Message{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("visible")}}, wantKept: true},
		{name: "reasoning content", message: llm.Message{Role: "assistant", ReasoningContent: lo.ToPtr("thinking")}, wantKept: true},
		{name: "reasoning", message: llm.Message{Role: "assistant", Reasoning: lo.ToPtr("thinking")}, wantKept: true},
		{name: "refusal", message: llm.Message{Role: "assistant", Refusal: "declined"}, wantKept: true},
		{name: "output audio", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{ID: "audio_1"}}, wantKept: true},
		{name: "output audio data", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{Data: "YXVkaW8="}}, wantKept: true},
		{name: "output audio transcript", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{Transcript: "spoken"}}, wantKept: true},
		{name: "text part", message: assistantWithPart(llm.MessageContentPart{Type: "text", Text: lo.ToPtr("visible")}), wantKept: true},
		{name: "image part", message: assistantWithPart(llm.MessageContentPart{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"}}), wantKept: true},
		{name: "video part", message: assistantWithPart(llm.MessageContentPart{Type: "video_url", VideoURL: &llm.VideoURL{URL: "https://example.com/video.mp4"}}), wantKept: true},
		{name: "input audio part", message: assistantWithPart(llm.MessageContentPart{Type: "input_audio", InputAudio: &llm.InputAudio{Format: "wav", Data: "YXVkaW8="}}), wantKept: true},
		{
			name: "later valid multipart payload",
			message: llm.Message{Role: "assistant", Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "text", Text: lo.ToPtr(" ")},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"}},
			}}},
			wantKept: true,
		},
		{
			name: "reasoning survives unsupported multipart payload",
			message: llm.Message{Role: "assistant", ReasoningContent: lo.ToPtr("thinking"), Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{{Type: "document", Document: &llm.DocumentURL{URL: "https://example.com/file.pdf"}}},
			}},
			wantKept: true,
		},
		{name: "plain function", message: llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{plainCall}}, wantKept: true, wantToolCall: "call_plain"},
		{name: "non assistant", message: llm.Message{Role: "user"}, wantKept: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := tt.message
			message.ToolCalls = append(message.ToolCalls, specialCall)
			got := FilterOutResponsesChatToolLifecycleMessages([]llm.Message{message})
			if !tt.wantKept {
				require.Empty(t, got)
				return
			}
			require.Len(t, got, 1)
			if tt.wantToolCall == "" {
				require.Empty(t, got[0].ToolCalls)
				return
			}
			require.Len(t, got[0].ToolCalls, 1)
			require.Equal(t, tt.wantToolCall, got[0].ToolCalls[0].ID)
		})
	}
}

func TestFilterOutResponsesChatToolLifecycleMessages_PairsReusedCallIDsByTurn(t *testing.T) {
	callID := "call_reused"
	messages := []llm.Message{
		{Role: "assistant", ToolCalls: []llm.ToolCall{{
			ID: callID, Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "lookup", Arguments: `{}`},
		}}},
		{Role: "tool", ToolCallID: &callID, Content: llm.MessageContent{Content: lo.ToPtr("plain output")}},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{
			ID: callID, Type: llm.ToolTypeResponsesCustomTool,
			ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: callID, Name: "apply_patch", Input: "patch"},
		}}},
		{Role: "tool", ToolCallID: &callID, Content: llm.MessageContent{Content: lo.ToPtr("custom output")}},
	}

	filtered := FilterOutResponsesChatToolLifecycleMessages(messages)
	require.Len(t, filtered, 2)
	require.Equal(t, "assistant", filtered[0].Role)
	require.Len(t, filtered[0].ToolCalls, 1)
	require.Equal(t, callID, filtered[0].ToolCalls[0].ID)
	require.Equal(t, "lookup", filtered[0].ToolCalls[0].Function.Name)
	require.Equal(t, "tool", filtered[1].Role)
	require.Equal(t, callID, lo.FromPtr(filtered[1].ToolCallID))
	require.Equal(t, "plain output", lo.FromPtr(filtered[1].Content.Content))
}

func TestFilterOutResponsesChatToolLifecycleMessages_PairsOutputBeforeCallWithoutCrossTurnLeak(t *testing.T) {
	callID := "call_reused"
	messages := []llm.Message{
		{Role: "tool", ToolCallID: &callID, Content: llm.MessageContent{Content: lo.ToPtr("custom output before call")}},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{
			ID: callID, Type: llm.ToolTypeResponsesCustomTool,
			ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: callID, Name: "apply_patch", Input: "patch"},
		}}},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{
			ID: callID, Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "lookup", Arguments: `{}`},
		}}},
		{Role: "tool", ToolCallID: &callID, Content: llm.MessageContent{Content: lo.ToPtr("plain output")}},
	}

	filtered := FilterOutResponsesChatToolLifecycleMessages(messages)
	require.Len(t, filtered, 2)
	require.Equal(t, "assistant", filtered[0].Role)
	require.Equal(t, "lookup", filtered[0].ToolCalls[0].Function.Name)
	require.Equal(t, "tool", filtered[1].Role)
	require.Equal(t, "plain output", lo.FromPtr(filtered[1].Content.Content))
}
