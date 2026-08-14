package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestRequestFromLLM(t *testing.T) {
	tests := []struct {
		name     string
		llmReq   *llm.Request
		validate func(*testing.T, *Request)
	}{
		{
			name:   "nil request",
			llmReq: nil,
			validate: func(t *testing.T, req *Request) {
				require.Nil(t, req)
			},
		},
		{
			name: "basic request",
			llmReq: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello there!"),
						},
					},
				},
				Stream: lo.ToPtr(true),
			},
			validate: func(t *testing.T, req *Request) {
				require.NotNil(t, req)
				require.Equal(t, "gpt-4", req.Model)
				require.Len(t, req.Messages, 1)
				require.Equal(t, "assistant", req.Messages[0].Role)
				require.True(t, *req.Stream)
			},
		},
		{
			name: "request with helper fields stripped",
			llmReq: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role:         "tool",
						ToolCallID:   lo.ToPtr("call_123"),
						MessageIndex: lo.ToPtr(1), // Helper field - should not be in OpenAI model
						Content:      llm.MessageContent{Content: lo.ToPtr("result")},
					},
				},
				APIFormat: llm.APIFormatOpenAIChatCompletion, // Helper field
			},
			validate: func(t *testing.T, req *Request) {
				require.NotNil(t, req)
				require.Equal(t, "call_123", *req.Messages[0].ToolCallID)
				// OpenAI Request doesn't have MessageIndex or RawAPIFormat fields
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RequestFromLLM(tt.llmReq, ReasoningFieldNone)
			tt.validate(t, result)
		})
	}
}

func TestRequestFromLLM_FiltersResponsesOnlyToolsWithoutLifecycleMetadata(t *testing.T) {
	req := RequestFromLLM(&llm.Request{
		Model:    "gpt-4o",
		Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}}},
		Tools: []llm.Tool{
			{
				Type: llm.ToolTypeResponsesCustomTool,
				ResponseCustomTool: &llm.ResponseCustomTool{
					Name: "apply_patch",
				},
			},
			{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name:       "get_weather",
					Parameters: []byte(`{"type":"object"}`),
				},
			},
			{
				Type: llm.ToolTypeResponsesToolSearch,
				ResponseToolSearch: &llm.ResponseToolSearch{
					Execution: "client", Description: "Find deferred tools",
					Parameters: []byte(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
				},
			},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Len(t, req.Tools, 1)
	for _, tool := range req.Tools {
		require.Equal(t, llm.ToolTypeFunction, tool.Type)
	}
	require.Equal(t, "get_weather", req.Tools[0].Function.Name)
}

func TestRequestFromLLM_ReindexesAssistantToolCallsByArrayPosition(t *testing.T) {
	req := RequestFromLLM(&llm.Request{Messages: []llm.Message{{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{
			{Index: 0, ID: "call_a", Type: "function", Function: llm.FunctionCall{Name: "first", Arguments: `{}`}},
			{Index: 0, ID: "call_b", Type: "function", Function: llm.FunctionCall{Name: "second", Arguments: `{}`}},
		},
	}}}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Len(t, req.Messages, 1)
	require.Len(t, req.Messages[0].ToolCalls, 2)
	require.Equal(t, []int{0, 1}, []int{
		req.Messages[0].ToolCalls[0].Index,
		req.Messages[0].ToolCalls[1].Index,
	})
}

func TestResponsesChatToolAdapter_ConvertsHistoryAndRestoresCalls(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{
			{Type: llm.ToolTypeResponsesCustomTool, ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"}},
			{Type: llm.ToolTypeResponsesToolSearch, ResponseToolSearch: &llm.ResponseToolSearch{Execution: "client"}},
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "collaboration__spawn_agent", Namespace: "collaboration"}},
		},
		Messages: []llm.Message{{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{ID: "call_custom", ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_custom", Name: "apply_patch", Input: "*** Begin Patch"}},
				{ID: "call_search", ResponseToolSearchCall: &llm.ResponseToolSearchCall{CallID: "call_search", Execution: "client", Arguments: `{"query":"agents"}`}},
				{ID: "call_ns", Function: llm.FunctionCall{Name: "spawn_agent", Namespace: "collaboration", Arguments: `{}`}},
			},
		}},
	}

	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Len(t, chatRequest.Messages[0].ToolCalls, 3)
	require.Equal(t, llm.ToolTypeFunction, chatRequest.Messages[0].ToolCalls[0].Type)
	require.JSONEq(t, `{"input":"*** Begin Patch"}`, chatRequest.Messages[0].ToolCalls[0].Function.Arguments)
	require.Equal(t, "tool_search", chatRequest.Messages[0].ToolCalls[1].Function.Name)
	require.Equal(t, "collaboration__spawn_agent", chatRequest.Messages[0].ToolCalls[2].Function.Name)
	require.Equal(t, []int{0, 1, 2}, []int{
		chatRequest.Messages[0].ToolCalls[0].Index,
		chatRequest.Messages[0].ToolCalls[1].Index,
		chatRequest.Messages[0].ToolCalls[2].Index,
	})

	response := &llm.Response{Choices: []llm.Choice{{Message: &llm.Message{ToolCalls: []llm.ToolCall{
		{ID: "call_custom", Function: llm.FunctionCall{Name: "apply_patch", Arguments: `{"input":"*** Begin Patch"}`}},
		{ID: "call_search", Function: llm.FunctionCall{Name: "tool_search", Arguments: `{"query":"agents"}`}},
		{ID: "call_ns", Function: llm.FunctionCall{Name: "collaboration__spawn_agent", Arguments: `{}`}},
	}}}}}
	restoreResponsesChatToolCalls(response, adapter.mappings())
	calls := response.Choices[0].Message.ToolCalls
	require.Equal(t, "*** Begin Patch", calls[0].ResponseCustomToolCall.Input)
	require.Equal(t, `{"query":"agents"}`, calls[1].ResponseToolSearchCall.Arguments)
	require.Equal(t, "collaboration", calls[2].Function.Namespace)
	require.Equal(t, "spawn_agent", calls[2].Function.Name)
}

func TestResponsesChatToolAdapter_DropsEmptyAssistantHistoryMessages(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("before")}},
		{
			Role:               "assistant",
			ReasoningItems:     []llm.ReasoningItem{{ID: "rs_empty", Signature: "opaque"}},
			ReasoningSignature: lo.ToPtr("opaque"),
		},
		{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr(" \n ")}},
		{
			Role: "assistant",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{{
				Type: "compaction", Compact: &llm.CompactContent{ID: "cmp_1", EncryptedContent: "opaque"},
			}}},
		},
		{Role: "assistant", ReasoningContent: lo.ToPtr("kept reasoning")},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call_1", Type: "function", Function: llm.FunctionCall{Name: "lookup", Arguments: `{}`}}}},
		{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("after")}},
	}}

	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldContent)
	require.NoError(t, err)
	require.Len(t, chatRequest.Messages, 4)
	require.Equal(t, []string{"user", "assistant", "assistant", "user"}, []string{
		chatRequest.Messages[0].Role,
		chatRequest.Messages[1].Role,
		chatRequest.Messages[2].Role,
		chatRequest.Messages[3].Role,
	})
	require.Equal(t, "kept reasoning", lo.FromPtr(chatRequest.Messages[1].ReasoningContent))
	require.Len(t, chatRequest.Messages[2].ToolCalls, 1)
	require.Contains(t, adapter.warnings, "empty_assistant_message: dropped 3 history message(s) with no Chat-compatible payload")
}

func TestResponsesChatToolAdapter_RevalidatesEveryAssistantPayloadForm(t *testing.T) {
	assistantWithPart := func(part llm.MessageContentPart) llm.Message {
		return llm.Message{
			Role: "assistant",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{part},
			},
		}
	}
	tests := []struct {
		name     string
		message  llm.Message
		wantKept bool
	}{
		{name: "nil content", message: llm.Message{Role: "assistant"}},
		{name: "whitespace scalar content", message: llm.Message{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr(" \n\t")}}},
		{name: "empty reasoning content", message: llm.Message{Role: "assistant", ReasoningContent: lo.ToPtr("")}},
		{name: "whitespace reasoning", message: llm.Message{Role: "assistant", Reasoning: lo.ToPtr(" \n")}},
		{name: "whitespace refusal", message: llm.Message{Role: "assistant", Refusal: " \t"}},
		{name: "empty text part", message: assistantWithPart(llm.MessageContentPart{Type: "text", Text: lo.ToPtr("")})},
		{name: "empty image part", message: assistantWithPart(llm.MessageContentPart{Type: "image_url", ImageURL: &llm.ImageURL{}})},
		{name: "empty video part", message: assistantWithPart(llm.MessageContentPart{Type: "video_url", VideoURL: &llm.VideoURL{}})},
		{name: "empty input audio part", message: assistantWithPart(llm.MessageContentPart{Type: "input_audio", InputAudio: &llm.InputAudio{Format: "wav"}})},
		{name: "empty output audio", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{}}},
		{name: "expiry only output audio", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{ExpiresAt: 123}}},
		{name: "unsupported document part", message: assistantWithPart(llm.MessageContentPart{Type: "document", Document: &llm.DocumentURL{URL: "https://example.com/file.pdf"}})},
		{
			name: "multipart overrides visible scalar content",
			message: llm.Message{Role: "assistant", Content: llm.MessageContent{
				Content: lo.ToPtr("ignored"), MultipleContent: []llm.MessageContentPart{{Type: "text", Text: lo.ToPtr(" ")}},
			}},
		},
		{name: "visible scalar content", message: llm.Message{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("visible")}}, wantKept: true},
		{name: "visible reasoning content", message: llm.Message{Role: "assistant", ReasoningContent: lo.ToPtr("thinking")}, wantKept: true},
		{name: "visible reasoning", message: llm.Message{Role: "assistant", Reasoning: lo.ToPtr("thinking")}, wantKept: true},
		{name: "visible refusal", message: llm.Message{Role: "assistant", Refusal: "declined"}, wantKept: true},
		{name: "output audio id", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{ID: "audio_1"}}, wantKept: true},
		{name: "output audio data", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{Data: "YXVkaW8="}}, wantKept: true},
		{name: "output audio transcript", message: llm.Message{Role: "assistant", Audio: &llm.OutputAudio{Transcript: "spoken"}}, wantKept: true},
		{name: "visible text part", message: assistantWithPart(llm.MessageContentPart{Type: "text", Text: lo.ToPtr("visible")}), wantKept: true},
		{name: "visible image part", message: assistantWithPart(llm.MessageContentPart{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"}}), wantKept: true},
		{name: "visible video part", message: assistantWithPart(llm.MessageContentPart{Type: "video_url", VideoURL: &llm.VideoURL{URL: "https://example.com/video.mp4"}}), wantKept: true},
		{name: "visible input audio part", message: assistantWithPart(llm.MessageContentPart{Type: "input_audio", InputAudio: &llm.InputAudio{Format: "wav", Data: "YXVkaW8="}}), wantKept: true},
		{name: "plain function call", message: llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call_1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "lookup"}}}}, wantKept: true},
		{name: "non assistant", message: llm.Message{Role: "user"}, wantKept: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, adapter, err := requestFromLLMWithResponsesToolAdapter(&llm.Request{Messages: []llm.Message{tt.message}}, ReasoningFieldAll)
			require.NoError(t, err)
			if !tt.wantKept {
				require.Empty(t, request.Messages)
				require.Contains(t, adapter.warnings, "empty_assistant_message: dropped 1 history message(s) with no Chat-compatible payload")
				return
			}
			require.Len(t, request.Messages, 1)
			require.NotContains(t, adapter.warnings, "empty_assistant_message: dropped 1 history message(s) with no Chat-compatible payload")
		})
	}
}

func TestResponsesChatToolAdapter_WarnsWhenDeferredFunctionBecomesImmediate(t *testing.T) {
	request := &llm.Request{Tools: []llm.Tool{{
		Type: llm.ToolTypeFunction,
		Function: llm.Function{
			Name: "deferred_lookup", Parameters: json.RawMessage(`{"type":"object"}`), DeferLoading: true,
		},
	}}}

	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Len(t, chatRequest.Tools, 1)
	require.Equal(t, "deferred_lookup", chatRequest.Tools[0].Function.Name)
	require.Contains(t, adapter.warnings, `defer_loading_degraded: function tool "deferred_lookup" is immediately visible after conversion to Chat Completions`)
}

func TestResponsesChatToolAdapter_WarnsForEveryDeferredFunctionOrigin(t *testing.T) {
	tests := []struct {
		name string
		tool llm.Tool
	}{
		{
			name: "namespace function",
			tool: llm.Tool{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name: "workspace__later", Namespace: "workspace",
					Parameters: json.RawMessage(`{"type":"object"}`), DeferLoading: true,
				},
			},
		},
		{
			name: "future client function",
			tool: llm.Tool{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name: "later", Parameters: json.RawMessage(`{"type":"object"}`), DeferLoading: true,
				},
				ResponsesOrigin: "raw_tool", ResponsesSourceType: "future_client_tool",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(&llm.Request{
				Tools: []llm.Tool{tt.tool},
			}, ReasoningFieldNone)
			require.NoError(t, err)
			require.Len(t, chatRequest.Tools, 1)
			require.Contains(t, strings.Join(adapter.warnings, "\n"), "defer_loading_degraded:")
		})
	}
}

func TestResponsesChatToolStreamRestorer_UsesToolCallIndexForLaterChunks(t *testing.T) {
	mappings := map[string]responsesChatToolMapping{
		"apply_patch": {Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch"},
	}
	restorer := newResponsesChatToolStreamRestorer(mappings)
	first := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		ID: "call_1", Index: 0, Function: llm.FunctionCall{Name: "apply_patch", Arguments: `{"input":"*** Begin`},
	}}}}}}
	second := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		Index: 0, Function: llm.FunctionCall{Arguments: ` Patch"}`},
	}}}}}}

	restorer.restore(first)
	restorer.restore(second)
	require.NotNil(t, first.Choices[0].Delta.ToolCalls[0].ResponseCustomToolCall)
	require.NotNil(t, second.Choices[0].Delta.ToolCalls[0].ResponseCustomToolCall)
	require.Equal(t, ` Patch"}`, second.Choices[0].Delta.ToolCalls[0].ResponseCustomToolCall.Input)
}

func TestResponsesChatToolStreamRestorer_DoesNotRestoreHistoryOnlyMapping(t *testing.T) {
	mappings := map[string]responsesChatToolMapping{
		"apply_patch": {
			Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch", HistoryOnly: true,
		},
	}
	restorer := newResponsesChatToolStreamRestorer(mappings)
	first := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		ID: "call_1", Index: 0, Type: llm.ToolTypeFunction,
		Function: llm.FunctionCall{Name: "apply_patch", Arguments: `{"input":"*** Begin`},
	}}}}}}
	second := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		Index: 0, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Arguments: ` Patch"}`},
	}}}}}}

	restorer.restore(first)
	restorer.restore(second)
	firstCall := first.Choices[0].Delta.ToolCalls[0]
	secondCall := second.Choices[0].Delta.ToolCalls[0]
	require.Nil(t, firstCall.ResponseCustomToolCall)
	require.Equal(t, "call_1", firstCall.ID)
	require.Equal(t, 0, firstCall.Index)
	require.Equal(t, llm.ToolTypeFunction, firstCall.Type)
	require.Equal(t, "apply_patch", firstCall.Function.Name)
	require.Equal(t, `{"input":"*** Begin`, firstCall.Function.Arguments)
	require.Nil(t, secondCall.ResponseCustomToolCall)
	require.Empty(t, secondCall.ID)
	require.Equal(t, 0, secondCall.Index)
	require.Equal(t, llm.ToolTypeFunction, secondCall.Type)
	require.Empty(t, secondCall.Function.Name)
	require.Equal(t, ` Patch"}`, secondCall.Function.Arguments)
}

func TestResponsesChatToolStreamRestorer_BlocksResolvedHigherIndexUntilLowerNameResolves(t *testing.T) {
	mappings := map[string]responsesChatToolMapping{
		"apply_patch": {Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch"},
	}
	restorer := newResponsesChatToolStreamRestorer(mappings, []string{"apply_patch", "lookup"})
	first := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{
		{
			ID: "call_patch", Index: 0, Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "apply_", Arguments: `{"input":"pat`},
		},
		{
			ID: "call_lookup", Index: 1, Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "lookup", Arguments: `{"query":"hi"}`},
		},
	}}}}}
	second := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		Index: 0, Function: llm.FunctionCall{Name: "patch", Arguments: `ch"}`},
	}}}}}}

	restorer.restore(first)
	require.Empty(t, first.Choices[0].Delta.ToolCalls)

	restorer.restore(second)
	calls := second.Choices[0].Delta.ToolCalls
	require.Len(t, calls, 2)
	require.Equal(t, []int{0, 1}, []int{calls[0].Index, calls[1].Index})
	require.NotNil(t, calls[0].ResponseCustomToolCall)
	require.Equal(t, `{"input":"patch"}`, calls[0].ResponseCustomToolCall.Input)
	require.Equal(t, "lookup", calls[1].Function.Name)
	require.JSONEq(t, `{"query":"hi"}`, calls[1].Function.Arguments)
}

func TestResponsesChatToolStreamRestorer_AcceptsRepeatedCumulativeNameWhileHigherIndexBlocked(t *testing.T) {
	mappings := map[string]responsesChatToolMapping{
		"apply_patch": {Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch"},
	}
	restorer := newResponsesChatToolStreamRestorer(mappings, []string{"apply_patch", "lookup"})
	first := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{
		{
			ID: "call_patch", Index: 0, Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "apply_", Arguments: `{"input":"pa`},
		},
		{
			ID: "call_lookup", Index: 1, Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "lookup", Arguments: `{}`},
		},
	}}}}}
	repeatedPartial := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		Index: 0, Function: llm.FunctionCall{Name: "apply_", Arguments: `t`},
	}}}}}}
	complete := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		Index: 0, Function: llm.FunctionCall{Name: "apply_patch", Arguments: `ch"}`},
	}}}}}}

	restorer.restore(first)
	restorer.restore(repeatedPartial)
	require.Empty(t, first.Choices[0].Delta.ToolCalls)
	require.Empty(t, repeatedPartial.Choices[0].Delta.ToolCalls)

	restorer.restore(complete)
	calls := complete.Choices[0].Delta.ToolCalls
	require.Len(t, calls, 2)
	require.Equal(t, []int{0, 1}, []int{calls[0].Index, calls[1].Index})
	require.NotNil(t, calls[0].ResponseCustomToolCall)
	require.Equal(t, `{"input":"patch"}`, calls[0].ResponseCustomToolCall.Input)
	require.Equal(t, "lookup", calls[1].Function.Name)
}

func TestResponsesChatToolStreamRestorer_SortsResolvedCallsFromNonZeroIndex(t *testing.T) {
	mappings := map[string]responsesChatToolMapping{
		"apply_patch": {Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch"},
	}
	restorer := newResponsesChatToolStreamRestorer(mappings, []string{"apply_patch", "lookup"})
	response := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{
		{ID: "call_lookup", Index: 4, Function: llm.FunctionCall{Name: "lookup", Arguments: `{}`}},
		{ID: "call_patch", Index: 3, Function: llm.FunctionCall{Name: "apply_patch", Arguments: `{"input":"patch"}`}},
	}}}}}

	restorer.restore(response)
	calls := response.Choices[0].Delta.ToolCalls
	require.Len(t, calls, 2)
	require.Equal(t, []int{3, 4}, []int{calls[0].Index, calls[1].Index})
	require.NotNil(t, calls[0].ResponseCustomToolCall)
	require.Equal(t, "lookup", calls[1].Function.Name)
}

func TestResponsesChatToolStreamRestorer_FlushesIncompleteLowerIndexAtFinish(t *testing.T) {
	mappings := map[string]responsesChatToolMapping{
		"apply_patch": {Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch"},
	}
	restorer := newResponsesChatToolStreamRestorer(mappings, []string{"apply_patch", "lookup"})
	first := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{
		{Index: 2, Function: llm.FunctionCall{Name: "apply_patch", Arguments: `{"input":"patch"}`}},
		{ID: "call_lookup", Index: 3, Function: llm.FunctionCall{Name: "lookup", Arguments: `{`}},
	}}}}}
	highFragment := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		Index: 3, Function: llm.FunctionCall{Arguments: `}`},
	}}}}}}
	usage := &llm.Response{Usage: &llm.Usage{}}
	errorChunk := &llm.Response{Error: &llm.ResponseError{}}
	finish := &llm.Response{Choices: []llm.Choice{{Index: 0, FinishReason: lo.ToPtr("tool_calls")}}}

	restorer.restore(first)
	restorer.restore(highFragment)
	restorer.restore(usage)
	restorer.restore(errorChunk)
	require.Empty(t, first.Choices[0].Delta.ToolCalls)
	require.Empty(t, highFragment.Choices[0].Delta.ToolCalls)
	require.NotNil(t, usage.Usage)
	require.NotNil(t, errorChunk.Error)

	restorer.restore(finish)
	require.NotNil(t, finish.Choices[0].Delta)
	calls := finish.Choices[0].Delta.ToolCalls
	require.Len(t, calls, 2)
	require.Equal(t, []int{2, 3}, []int{calls[0].Index, calls[1].Index})
	require.NotNil(t, calls[0].ResponseCustomToolCall)
	require.Equal(t, "lookup", calls[1].Function.Name)
	require.JSONEq(t, `{}`, calls[1].Function.Arguments)
}

func TestResponsesChatToolStreamRestorer_IsolatesChoices(t *testing.T) {
	mappings := map[string]responsesChatToolMapping{
		"apply_patch": {Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch"},
	}
	restorer := newResponsesChatToolStreamRestorer(mappings, []string{"apply_patch", "lookup"})
	first := &llm.Response{Choices: []llm.Choice{
		{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{
			{ID: "call_patch", Index: 0, Function: llm.FunctionCall{Name: "apply_"}},
			{ID: "call_lookup_0", Index: 1, Function: llm.FunctionCall{Name: "lookup"}},
		}}},
		{Index: 1, Delta: &llm.Message{ToolCalls: []llm.ToolCall{
			{ID: "call_lookup_1", Index: 5, Function: llm.FunctionCall{Name: "lookup"}},
		}}},
	}}
	finishChoiceOne := &llm.Response{Choices: []llm.Choice{{
		Index: 1, Delta: &llm.Message{}, FinishReason: lo.ToPtr("tool_calls"),
	}}}
	completeChoiceZero := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		Index: 0, Function: llm.FunctionCall{Name: "patch"},
	}}}}}}

	restorer.restore(first)
	require.Empty(t, first.Choices[0].Delta.ToolCalls)
	require.Len(t, first.Choices[1].Delta.ToolCalls, 1)
	choiceOneCall := first.Choices[1].Delta.ToolCalls[0]
	require.Equal(t, "call_lookup_1", choiceOneCall.ID)
	require.Equal(t, 5, choiceOneCall.Index)
	require.Equal(t, "lookup", choiceOneCall.Function.Name)
	require.Empty(t, choiceOneCall.Function.Arguments)
	require.Nil(t, choiceOneCall.ResponseCustomToolCall)

	restorer.restore(finishChoiceOne)
	require.Empty(t, finishChoiceOne.Choices[0].Delta.ToolCalls)

	restorer.restore(completeChoiceZero)
	calls := completeChoiceZero.Choices[0].Delta.ToolCalls
	require.Len(t, calls, 2)
	require.Equal(t, []int{0, 1}, []int{calls[0].Index, calls[1].Index})
	require.Equal(t, "call_patch", calls[0].ID)
	require.NotNil(t, calls[0].ResponseCustomToolCall)
	require.Equal(t, "apply_patch", calls[0].ResponseCustomToolCall.Name)
	require.Equal(t, "call_lookup_0", calls[1].ID)
	require.Equal(t, "lookup", calls[1].Function.Name)
	require.Nil(t, calls[1].ResponseCustomToolCall)
}

func TestResponsesChatToolStreamRestorer_DuplicateIndexSeeds(t *testing.T) {
	tests := []struct {
		name       string
		calls      []llm.ToolCall
		wantID     string
		wantName   string
		wantArgs   string
		wantCustom bool
	}{
		{
			name: "incremental name and arguments in one chunk",
			calls: []llm.ToolCall{
				{ID: "call_patch", Index: 7, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "apply_", Arguments: `{"input":"pa`}},
				{Index: 7, Function: llm.FunctionCall{Name: "patch", Arguments: `tch"}`}},
			},
			wantID: "call_patch", wantName: "apply_patch", wantArgs: `{"input":"patch"}`, wantCustom: true,
		},
		{
			name: "cumulative name and arguments in one chunk",
			calls: []llm.ToolCall{
				{ID: "call_patch", Index: 7, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "apply_", Arguments: `{"input":"pa`}},
				{Index: 7, Function: llm.FunctionCall{Name: "apply_patch", Arguments: `tch"}`}},
			},
			wantID: "call_patch", wantName: "apply_patch", wantArgs: `{"input":"patch"}`, wantCustom: true,
		},
		{
			name: "conflicting duplicate ids use latest nonempty id",
			calls: []llm.ToolCall{
				{ID: "call_old", Index: 7, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "lookup", Arguments: `{`}},
				{ID: "call_new", Index: 7, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "lookup", Arguments: `}`}},
			},
			wantID: "call_new", wantName: "lookup", wantArgs: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restorer := newResponsesChatToolStreamRestorer(
				map[string]responsesChatToolMapping{
					"apply_patch": {Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch"},
				},
				[]string{"apply_patch", "lookup"},
			)
			response := &llm.Response{Choices: []llm.Choice{{
				Index: 0, Delta: &llm.Message{ToolCalls: tt.calls}, FinishReason: lo.ToPtr("tool_calls"),
			}}}

			require.NotPanics(t, func() { restorer.restore(response) })
			require.Len(t, response.Choices[0].Delta.ToolCalls, 1)
			call := response.Choices[0].Delta.ToolCalls[0]
			require.Equal(t, 7, call.Index)
			require.Equal(t, tt.wantID, call.ID)
			require.Equal(t, tt.wantName, call.Function.Name)
			require.JSONEq(t, tt.wantArgs, call.Function.Arguments)
			if tt.wantCustom {
				require.NotNil(t, call.ResponseCustomToolCall)
				require.Equal(t, tt.wantID, call.ResponseCustomToolCall.CallID)
			} else {
				require.Nil(t, call.ResponseCustomToolCall)
			}
		})
	}
}

func TestResponsesChatToolStreamRestorer_LateLowerIndexRemainsDeterministic(t *testing.T) {
	restorer := newResponsesChatToolStreamRestorer(
		map[string]responsesChatToolMapping{
			"apply_patch": {Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch"},
		},
		[]string{"apply_patch", "lookup"},
	)
	higher := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		ID: "call_high", Index: 9, Type: llm.ToolTypeFunction,
		Function: llm.FunctionCall{Name: "lookup", Arguments: `{}`},
	}}}}}}
	lower := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		ID: "call_low", Index: 3, Type: llm.ToolTypeFunction,
		Function: llm.FunctionCall{Name: "apply_patch", Arguments: `{"input":"patch"}`},
	}}}}}}
	finish := &llm.Response{Choices: []llm.Choice{{Index: 0, FinishReason: lo.ToPtr("tool_calls")}}}

	restorer.restore(higher)
	restorer.restore(lower)
	restorer.restore(finish)

	require.Len(t, higher.Choices[0].Delta.ToolCalls, 1)
	require.Equal(t, 9, higher.Choices[0].Delta.ToolCalls[0].Index)
	require.Len(t, lower.Choices[0].Delta.ToolCalls, 1)
	require.Equal(t, 3, lower.Choices[0].Delta.ToolCalls[0].Index)
	require.NotNil(t, lower.Choices[0].Delta.ToolCalls[0].ResponseCustomToolCall)
	require.Empty(t, finish.Choices[0].Delta.ToolCalls)
}

func FuzzResponsesChatToolStreamRestorer_DuplicateIndexFragments(f *testing.F) {
	f.Add("apply_", "patch", `{"input":"pa`, `tch"}`, "")
	f.Add("apply_", "apply_patch", `{"input":"pa`, `tch"}`, "call_late")
	f.Add("lookup", "lookup", `{`, `}`, "call_conflict")

	f.Fuzz(func(t *testing.T, firstName, secondName, firstArgs, secondArgs, secondID string) {
		if len(firstName)+len(secondName)+len(firstArgs)+len(secondArgs)+len(secondID) > 4096 {
			t.Skip()
		}
		restorer := newResponsesChatToolStreamRestorer(
			map[string]responsesChatToolMapping{
				"apply_patch": {Kind: responsesChatToolCustom, ChatName: "apply_patch", Name: "apply_patch"},
			},
			[]string{"apply_patch", "lookup"},
		)
		response := &llm.Response{Choices: []llm.Choice{{
			Index: 2,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{
				{ID: "call_first", Index: 5, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: firstName, Arguments: firstArgs}},
				{ID: secondID, Index: 5, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: secondName, Arguments: secondArgs}},
			}},
			FinishReason: lo.ToPtr("tool_calls"),
		}}}

		require.NotPanics(t, func() { restorer.restore(response) })
		require.Len(t, response.Choices[0].Delta.ToolCalls, 1)
		call := response.Choices[0].Delta.ToolCalls[0]
		require.Equal(t, 5, call.Index)
		require.Equal(t, firstArgs+secondArgs, call.Function.Arguments)
		if secondID == "" {
			require.Equal(t, "call_first", call.ID)
		} else {
			require.Equal(t, secondID, call.ID)
		}
	})
}

func TestResponsesChatToolAdapter_AvoidsFunctionNameCollisions(t *testing.T) {
	request := &llm.Request{Tools: []llm.Tool{
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "apply_patch"}},
		{Type: llm.ToolTypeResponsesCustomTool, ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"}},
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "collaboration__spawn_agent"}},
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "collaboration__spawn_agent", Namespace: "collaboration"}},
	}}
	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Len(t, chatRequest.Tools, 4)
	require.Equal(t, "apply_patch", chatRequest.Tools[0].Function.Name)
	require.Equal(t, "axonhub_custom_tool_1", chatRequest.Tools[1].Function.Name)
	require.Equal(t, "collaboration__spawn_agent", chatRequest.Tools[2].Function.Name)
	require.Equal(t, "axonhub_namespace_tool_1", chatRequest.Tools[3].Function.Name)

	response := &llm.Response{Choices: []llm.Choice{{Message: &llm.Message{ToolCalls: []llm.ToolCall{
		{ID: "custom", Function: llm.FunctionCall{Name: "axonhub_custom_tool_1", Arguments: `{"input":"patch"}`}},
		{ID: "namespace", Function: llm.FunctionCall{Name: "axonhub_namespace_tool_1", Arguments: `{}`}},
	}}}}}
	restoreResponsesChatToolCalls(response, adapter.mappings())
	require.Equal(t, "patch", response.Choices[0].Message.ToolCalls[0].ResponseCustomToolCall.Input)
	require.Equal(t, "collaboration", response.Choices[0].Message.ToolCalls[1].Function.Namespace)
	require.Equal(t, "spawn_agent", response.Choices[0].Message.ToolCalls[1].Function.Name)
}

func TestResponsesChatToolAdapter_DeduplicatesEquivalentFunctions(t *testing.T) {
	request := &llm.Request{Tools: []llm.Tool{
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Description: "Lookup", Parameters: []byte(`{"type":"object","properties":{"query":{"type":"string"}}}`)}},
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Description: "Lookup", Parameters: []byte(`{"properties":{"query":{"type":"string"}},"type":"object"}`)}, ResponsesOrigin: "additional_tools"},
	}}
	chatRequest, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Len(t, chatRequest.Tools, 1)
	require.Equal(t, "lookup", chatRequest.Tools[0].Function.Name)
}

func TestResponsesChatToolAdapter_KeepsFirstConflictingFunctionSchema(t *testing.T) {
	request := &llm.Request{Tools: []llm.Tool{
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Parameters: []byte(`{"type":"object","properties":{"query":{"type":"string"}}}`)}},
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Parameters: []byte(`{"type":"object","properties":{"id":{"type":"integer"}}}`)}, ResponsesOrigin: "additional_tools"},
	}}
	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Len(t, chatRequest.Tools, 1)
	require.JSONEq(t, `{"type":"object","properties":{"query":{"type":"string"}}}`, string(chatRequest.Tools[0].Function.Parameters))
	require.Contains(t, adapter.warnings, `tool_name_conflict: kept first definition of function "lookup"`)
}

func TestResponsesChatToolAdapter_DropsInvalidFunctionSchemaWithWarning(t *testing.T) {
	request := &llm.Request{Tools: []llm.Tool{
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "valid", Parameters: []byte(`{"type":"object"}`)}},
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "invalid", Parameters: []byte(`{"type":`)}},
	}}
	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Len(t, chatRequest.Tools, 1)
	require.Equal(t, "valid", chatRequest.Tools[0].Function.Name)
	require.Contains(t, adapter.warnings[0], `invalid function tool "invalid" was dropped`)
}

func TestResponsesChatToolAdapter_NormalizesFunctionParameterRoots(t *testing.T) {
	request := &llm.Request{Tools: []llm.Tool{
		{Type: llm.ToolTypeFunction, Function: llm.Function{
			Name: "plain", Parameters: []byte(`{"properties":{"query":{"type":"string"}}}`),
		}},
		{Type: llm.ToolTypeFunction, Function: llm.Function{
			Name: "agents__spawn", Namespace: "agents",
		}},
		{Type: llm.ToolTypeResponsesToolSearch, ResponseToolSearch: &llm.ResponseToolSearch{
			Execution: "client", Parameters: []byte(`null`),
		}},
		{Type: llm.ToolTypeFunction, Function: llm.Function{
			Name: "invalid_array", Parameters: []byte(`{"type":"array","items":{"type":"string"}}`),
		}},
	}}

	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Len(t, chatRequest.Tools, 3)
	for _, tool := range chatRequest.Tools {
		var schema map[string]any
		require.NoError(t, json.Unmarshal(tool.Function.Parameters, &schema))
		require.Equal(t, "object", schema["type"])
	}
	require.JSONEq(t, `{"type":"object","properties":{"query":{"type":"string"}}}`, string(chatRequest.Tools[0].Function.Parameters))
	require.JSONEq(t, `{"type":"object","properties":{}}`, string(chatRequest.Tools[1].Function.Parameters))
	require.JSONEq(t, `{"type":"object","properties":{}}`, string(chatRequest.Tools[2].Function.Parameters))
	require.Contains(t, adapter.warnings, `invalid function tool "invalid_array" was dropped: parameters schema type is required and must be "object"`)
}

func TestResponsesChatToolAdapter_RejectsNamedInvalidToolSearchSchema(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{{
			Type: llm.ToolTypeResponsesToolSearch,
			ResponseToolSearch: &llm.ResponseToolSearch{
				Execution: "client", Parameters: []byte(`{"type":"array"}`),
			},
		}},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: llm.ToolTypeResponsesToolSearch, Function: llm.ToolFunction{Name: "tool_search"},
		}},
	}

	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.Nil(t, chatRequest)
	require.ErrorContains(t, err, `named tool "tool_search" is unavailable`)
	require.Contains(t, adapter.warnings, `invalid tool_search definition was dropped: parameters schema type is required and must be "object"`)
}

func TestResponsesChatToolAdapter_RejectsUnsupportedNamedToolChoice(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{{
			Type:               llm.ToolTypeResponsesToolSearch,
			ResponseToolSearch: &llm.ResponseToolSearch{Execution: "server"},
		}},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: "tool_search", Function: llm.ToolFunction{Name: "tool_search"},
		}},
	}
	_, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.ErrorContains(t, err, "unsupported_tool_choice")
}

func TestUnsupportedRawChatToolSelector_DoesNotDegradeRepresentedSelectors(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		unsupported bool
	}{
		{name: "string mode", raw: `"auto"`},
		{name: "named function", raw: `{"type":"function","name":"lookup"}`},
		{name: "clean allowed tools", raw: `{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"lookup"}]}`},
		{name: "type only hosted tool", raw: `{"type":"web_search"}`, unsupported: true},
		{name: "future selector field", raw: `{"type":"future_selector","policy":"strict"}`, unsupported: true},
		{name: "mcp selector identity", raw: `{"type":"mcp","server_label":"docs","name":"search"}`, unsupported: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, unsupported := unsupportedRawChatToolSelector(json.RawMessage(tc.raw))
			require.Equal(t, tc.unsupported, unsupported)
		})
	}
}

func TestRawChatToolChoiceRequiresCurrentSemanticMatch(t *testing.T) {
	raw := json.RawMessage(`{"type":"web_search","future_option":"keep"}`)

	require.False(t, rawChatToolChoiceMatchesCurrent(raw, nil))
	require.True(t, rawChatToolChoiceMatchesCurrent(raw, &llm.ToolChoice{
		NamedToolChoice: &llm.NamedToolChoice{Type: "web_search"},
	}))
	require.False(t, rawChatToolChoiceMatchesCurrent(raw, &llm.ToolChoice{}))

	none := "none"
	require.False(t, rawChatToolChoiceMatchesCurrent(raw, &llm.ToolChoice{ToolChoice: &none}))
}

func TestResponsesChatToolAdapter_DoesNotRedirectDroppedNamedToolToPlainFunction(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Parameters: []byte(`{"type":"object"}`)}},
			{
				Type: llm.ToolTypeResponsesOpaqueTool,
				ResponseOpaqueTool: &llm.ResponseOpaqueTool{
					SourceType: "future_server_tool", Name: "lookup", Execution: "server",
				},
			},
		},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: "future_server_tool", Function: llm.ToolFunction{Name: "lookup"},
		}},
	}
	_, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.ErrorContains(t, err, "unsupported_tool_choice")
}

func TestResponsesChatToolAdapter_DoesNotRedirectInvalidNamedFunctionToCustom(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Parameters: []byte(`{"type":"array"}`)}},
			{Type: llm.ToolTypeResponsesCustomTool, ResponseCustomTool: &llm.ResponseCustomTool{Name: "lookup"}},
		},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: llm.ToolTypeFunction, Function: llm.ToolFunction{Name: "lookup"},
		}},
	}
	_, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.ErrorContains(t, err, `named tool "lookup" is unavailable`)
	require.Contains(t, adapter.warnings, `invalid function tool "lookup" was dropped: parameters schema type is required and must be "object"`)
}

func TestResponsesChatToolAdapter_FiltersTypeAwareAllowedTools(t *testing.T) {
	auto := "auto"
	request := &llm.Request{
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "allowed_plain", Parameters: []byte(`{"type":"object"}`)}},
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "hidden_plain", Parameters: []byte(`{"type":"object"}`)}},
			{Type: llm.ToolTypeResponsesCustomTool, ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"}},
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "collaboration__spawn_agent", Namespace: "collaboration", Parameters: []byte(`{"type":"object"}`)}},
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "collaboration__send_message", Namespace: "collaboration", Parameters: []byte(`{"type":"object"}`)}},
			{Type: llm.ToolTypeResponsesToolSearch, ResponseToolSearch: &llm.ResponseToolSearch{Execution: "client", Parameters: []byte(`{"type":"object"}`)}},
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "future_lookup", Parameters: []byte(`{"type":"object"}`)}, ResponsesSourceType: "future_client_tool"},
		},
		ToolChoice: &llm.ToolChoice{
			ToolChoice: &auto, AllowedToolsSet: true,
			AllowedTools: []llm.ToolOption{
				{Type: "function", Name: "allowed_plain"},
				{Type: "custom", Name: "apply_patch"},
				{Type: "namespace", Name: "collaboration"},
				{Type: "tool_search", Name: "tool_search"},
				{Type: "future_client_tool", Name: "future_lookup"},
			},
		},
	}

	chatRequest, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Equal(t, "auto", lo.FromPtr(chatRequest.ToolChoice.ToolChoice))
	require.Equal(t, []string{
		"allowed_plain", "apply_patch", "collaboration__spawn_agent", "collaboration__send_message", "tool_search", "future_lookup",
	}, lo.Map(chatRequest.Tools, func(tool Tool, _ int) string { return tool.Function.Name }))
}

func TestResponsesChatToolAdapter_AllowedToolsDoesNotBreakHistoryMappings(t *testing.T) {
	auto := "auto"
	request := &llm.Request{
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Parameters: []byte(`{"type":"object"}`)}},
			{Type: llm.ToolTypeResponsesCustomTool, ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"}},
		},
		Messages: []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{{
			ID: "call_patch", Type: llm.ToolTypeResponsesCustomTool,
			ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_patch", Name: "apply_patch", Input: "patch"},
		}}}},
		ToolChoice: &llm.ToolChoice{
			ToolChoice: &auto, AllowedToolsSet: true,
			AllowedTools: []llm.ToolOption{{Type: "function", Name: "lookup"}},
		},
	}

	chatRequest, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Len(t, chatRequest.Tools, 1)
	require.Equal(t, "lookup", chatRequest.Tools[0].Function.Name)
	require.Len(t, chatRequest.Messages, 1)
	require.Len(t, chatRequest.Messages[0].ToolCalls, 1)
	require.Equal(t, "apply_patch", chatRequest.Messages[0].ToolCalls[0].Function.Name)
	require.JSONEq(t, `{"input":"patch"}`, chatRequest.Messages[0].ToolCalls[0].Function.Arguments)
}

func TestResponsesChatToolAdapter_EmptyAllowedTools(t *testing.T) {
	for _, mode := range []string{"auto", "required"} {
		t.Run(mode, func(t *testing.T) {
			request := &llm.Request{
				Tools:      []llm.Tool{{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Parameters: []byte(`{"type":"object"}`)}}},
				ToolChoice: &llm.ToolChoice{ToolChoice: &mode, AllowedToolsSet: true, AllowedTools: []llm.ToolOption{}},
			}
			chatRequest, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
			if mode == "required" {
				require.ErrorContains(t, err, "required tool choice has no callable tools")
				return
			}
			require.NoError(t, err)
			require.Empty(t, chatRequest.Tools)
			require.Nil(t, chatRequest.ToolChoice)
		})
	}
}

func TestResponsesChatToolAdapter_MapsNamedFutureClientTool(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "future_lookup", Parameters: []byte(`{"type":"object"}`)}},
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "future_lookup", Parameters: []byte(`{"type":"object"}`)}, ResponsesSourceType: "future_client_tool"},
		},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: "future_client_tool", Function: llm.ToolFunction{Name: "future_lookup"},
		}},
	}

	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Len(t, chatRequest.Tools, 2)
	require.Equal(t, "axonhub_client_tool_1", chatRequest.Tools[1].Function.Name)
	require.Equal(t, "axonhub_client_tool_1", chatRequest.ToolChoice.NamedToolChoice.Function.Name)
	require.Contains(t, adapter.warnings, `client_tool_output_degraded: future_client_tool tool "future_lookup" returns as a function_call after Chat conversion`)
}

func TestResponsesChatToolAdapter_RejectsRequiredChoiceWithoutCallableTools(t *testing.T) {
	required := "required"
	request := &llm.Request{
		Tools: []llm.Tool{{
			Type: llm.ToolTypeResponsesOpaqueTool,
			ResponseOpaqueTool: &llm.ResponseOpaqueTool{
				SourceType: "future_server_tool", Name: "hosted", Execution: "server",
			},
		}},
		ToolChoice: &llm.ToolChoice{ToolChoice: &required},
	}
	_, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.ErrorContains(t, err, "required tool choice has no callable tools")

	request.Tools = append(request.Tools, llm.Tool{
		Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Parameters: []byte(`{"type":"object"}`)},
	})
	chatRequest, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Equal(t, "required", lo.FromPtr(chatRequest.ToolChoice.ToolChoice))
}

func TestResponsesChatToolAdapter_MapsNamedCustomToolChoiceAfterCollision(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "apply_patch", Parameters: []byte(`{"type":"object"}`)}},
			{Type: llm.ToolTypeResponsesCustomTool, ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"}},
		},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: "custom", Function: llm.ToolFunction{Name: "apply_patch"},
		}},
	}
	chatRequest, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.NotNil(t, chatRequest.ToolChoice)
	require.Equal(t, "axonhub_custom_tool_1", chatRequest.ToolChoice.NamedToolChoice.Function.Name)
}

func TestResponsesChatToolAdapter_MapsFunctionChoiceToNamespaceTool(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{{
			Type: llm.ToolTypeFunction,
			Function: llm.Function{
				Name: "collaboration__spawn_agent", Namespace: "collaboration",
				Parameters: []byte(`{"type":"object"}`),
			},
		}},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: llm.ToolTypeFunction, Function: llm.ToolFunction{Name: "spawn_agent"},
		}},
	}

	chatRequest, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.NotNil(t, chatRequest.ToolChoice)
	require.Equal(t, "collaboration__spawn_agent", chatRequest.ToolChoice.NamedToolChoice.Function.Name)
}

func TestResponsesChatToolAdapter_RejectsAmbiguousPlainAndNamespaceFunctionChoice(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "spawn_agent", Parameters: []byte(`{"type":"object"}`)}},
			{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name: "collaboration__spawn_agent", Namespace: "collaboration",
					Parameters: []byte(`{"type":"object"}`),
				},
			},
		},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: llm.ToolTypeFunction, Function: llm.ToolFunction{Name: "spawn_agent"},
		}},
	}

	_, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.ErrorContains(t, err, "ambiguous between plain and namespace tools")
}

func TestResponsesChatToolAdapter_SilentlyFiltersEstablishedNonChatTools(t *testing.T) {
	request := &llm.Request{Tools: []llm.Tool{
		{Type: llm.ToolTypeImageGeneration},
		{Type: llm.ToolTypeWebSearch},
		{Type: llm.ToolTypeGoogleSearch},
		{Type: llm.ToolTypeGoogleCodeExecution},
		{Type: llm.ToolTypeGoogleUrlContext},
	}}

	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Empty(t, chatRequest.Tools)
	require.Empty(t, adapter.warnings)
}

func TestResponsesChatToolAdapter_ValidatesSpecialCallIDs(t *testing.T) {
	base := []llm.Tool{{
		Type:               llm.ToolTypeResponsesCustomTool,
		ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"},
	}}

	request := &llm.Request{Tools: base, Messages: []llm.Message{{
		Role: "assistant", ToolCalls: []llm.ToolCall{{
			ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_inner", Name: "apply_patch", Input: "patch"},
		}},
	}}}
	chatRequest, _, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Equal(t, "call_inner", chatRequest.Messages[0].ToolCalls[0].ID)

	request.Messages[0].ToolCalls[0].ID = "call_outer"
	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Equal(t, "call_inner", chatRequest.Messages[0].ToolCalls[0].ID)
	require.Contains(t, adapter.warnings, `tool_call_id_conflict: used specialized call ID "call_inner" instead of outer call ID "call_outer"`)
}

func TestResponsesChatToolAdapter_PreservesUndeclaredSpecialHistory(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{
			{ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_custom", Name: "apply_patch", Input: "patch"}},
			{ResponseToolSearchCall: &llm.ResponseToolSearchCall{CallID: "call_search", Execution: "client", Arguments: `{}`}},
		},
	}}}
	chatRequest, adapter, err := requestFromLLMWithResponsesToolAdapter(request, ReasoningFieldNone)
	require.NoError(t, err)
	require.Empty(t, chatRequest.Tools)
	require.Len(t, chatRequest.Messages[0].ToolCalls, 2)
	require.Equal(t, "apply_patch", chatRequest.Messages[0].ToolCalls[0].Function.Name)
	require.Equal(t, "tool_search", chatRequest.Messages[0].ToolCalls[1].Function.Name)
	require.Len(t, adapter.mappings(), 2)

	response := &llm.Response{Choices: []llm.Choice{{Message: &llm.Message{ToolCalls: []llm.ToolCall{{
		ID: "call_new", Type: llm.ToolTypeFunction,
		Function: llm.FunctionCall{Name: "apply_patch", Arguments: `{"input":"new patch"}`},
	}}}}}}
	restoreResponsesChatToolCalls(response, adapter.mappings())
	require.Nil(t, response.Choices[0].Message.ToolCalls[0].ResponseCustomToolCall)
	require.Equal(t, llm.ToolTypeFunction, response.Choices[0].Message.ToolCalls[0].Type)
}

func TestMessageContentPartAudioRoundTrip(t *testing.T) {
	part := llm.MessageContentPart{
		Type: "input_audio",
		InputAudio: &llm.InputAudio{
			Format: "mp3",
			Data:   "audio-base64",
		},
	}

	oaiPart := MessageContentPartFromLLM(part)
	require.Equal(t, "input_audio", oaiPart.Type)
	require.NotNil(t, oaiPart.InputAudio)
	require.Equal(t, "mp3", oaiPart.InputAudio.Format)
	require.Equal(t, "audio-base64", oaiPart.InputAudio.Data)

	roundTrip := oaiPart.ToLLMPart()
	require.Equal(t, "input_audio", roundTrip.Type)
	require.NotNil(t, roundTrip.InputAudio)
	require.Equal(t, "mp3", roundTrip.InputAudio.Format)
	require.Equal(t, "audio-base64", roundTrip.InputAudio.Data)
}

func TestMessageContentFromLLM_IgnoresCompactionParts(t *testing.T) {
	content := MessageContentFromLLM(llm.MessageContent{
		MultipleContent: []llm.MessageContentPart{
			{
				Type: "compaction",
				Compact: &llm.CompactContent{
					ID:               "cmp_123",
					EncryptedContent: "secret",
				},
			},
			{
				Type: "compaction_summary",
				Compact: &llm.CompactContent{
					ID:               "cmp_456",
					EncryptedContent: "summary",
				},
			},
			{
				Type: "text",
				Text: lo.ToPtr("visible"),
			},
		},
	})

	require.Len(t, content.MultipleContent, 1)
	require.Equal(t, "text", content.MultipleContent[0].Type)
	require.NotNil(t, content.MultipleContent[0].Text)
	require.Equal(t, "visible", *content.MultipleContent[0].Text)
}

func TestRequestFromLLM_IgnoresCompactionPartsInMessages(t *testing.T) {
	req := RequestFromLLM(&llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "compaction",
							Compact: &llm.CompactContent{
								ID:               "cmp_123",
								EncryptedContent: "secret",
							},
						},
						{
							Type: "compaction_summary",
							Compact: &llm.CompactContent{
								ID:               "cmp_456",
								EncryptedContent: "summary",
							},
						},
						{
							Type: "text",
							Text: lo.ToPtr("hello"),
						},
					},
				},
			},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Len(t, req.Messages, 1)
	require.Len(t, req.Messages[0].Content.MultipleContent, 1)
	require.Equal(t, "text", req.Messages[0].Content.MultipleContent[0].Type)
	require.NotNil(t, req.Messages[0].Content.MultipleContent[0].Text)
	require.Equal(t, "hello", *req.Messages[0].Content.MultipleContent[0].Text)
}

func TestMessageAudioRoundTrip(t *testing.T) {
	msg := llm.Message{
		Role: "assistant",
		Content: llm.MessageContent{
			Content: lo.ToPtr("Audio reply"),
		},
		Audio: &llm.OutputAudio{
			ID:         "audio_123",
			Data:       "base64-audio",
			ExpiresAt:  1234567890,
			Transcript: "hello world",
		},
	}

	oaiMsg := MessageFromLLM(msg)
	require.NotNil(t, oaiMsg.Audio)
	require.Equal(t, "audio_123", oaiMsg.Audio.ID)
	require.Equal(t, "base64-audio", oaiMsg.Audio.Data)
	require.Equal(t, int64(1234567890), oaiMsg.Audio.ExpiresAt)
	require.Equal(t, "hello world", oaiMsg.Audio.Transcript)

	roundTrip := oaiMsg.ToLLMMessage()
	require.NotNil(t, roundTrip.Audio)
	require.Equal(t, "audio_123", roundTrip.Audio.ID)
	require.Equal(t, "base64-audio", roundTrip.Audio.Data)
	require.Equal(t, int64(1234567890), roundTrip.Audio.ExpiresAt)
	require.Equal(t, "hello world", roundTrip.Audio.Transcript)
}

func TestResponse_ToLLMResponse(t *testing.T) {
	tests := []struct {
		name     string
		oaiResp  *Response
		validate func(*testing.T, *llm.Response)
	}{
		{
			name:    "nil response",
			oaiResp: nil,
			validate: func(t *testing.T, resp *llm.Response) {
				require.Nil(t, resp)
			},
		},
		{
			name: "basic response",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Message: &Message{
							Role:    "assistant",
							Content: MessageContent{Content: lo.ToPtr("Hello!")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Equal(t, "chatcmpl-123", resp.ID)
				require.Equal(t, "chat.completion", resp.Object)
				require.Len(t, resp.Choices, 1)
				require.Equal(t, "Hello!", *resp.Choices[0].Message.Content.Content)
				require.Equal(t, "stop", *resp.Choices[0].FinishReason)
			},
		},
		{
			name: "streaming response with delta",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Delta: &Message{
							Content: MessageContent{Content: lo.ToPtr("chunk")},
						},
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Equal(t, "chat.completion.chunk", resp.Object)
				require.NotNil(t, resp.Choices[0].Delta)
				require.Equal(t, "chunk", *resp.Choices[0].Delta.Content.Content)
			},
		},
		{
			name: "response with usage",
			oaiResp: &Response{
				ID:     "chatcmpl-123",
				Object: "chat.completion",
				Model:  "gpt-4",
				Choices: []Choice{
					{Index: 0, Message: &Message{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("Hi")}}},
				},
				Usage: &Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.NotNil(t, resp.Usage)
				require.Equal(t, int64(10), resp.Usage.PromptTokens)
				require.Equal(t, int64(5), resp.Usage.CompletionTokens)
				require.Equal(t, int64(15), resp.Usage.TotalTokens)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.oaiResp.ToLLMResponse()
			tt.validate(t, result)
		})
	}
}

func TestMessage_ToLLMMessage_WithAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		oaiMsg   Message
		validate func(*testing.T, llm.Message)
	}{
		{
			name: "message with annotations",
			oaiMsg: Message{
				Role:    "assistant",
				Content: MessageContent{Content: lo.ToPtr("The meaning of life...")},
				Annotations: []Annotation{
					{
						Type:       "url_citation",
						StartIndex: lo.ToPtr(int64(0)),
						EndIndex:   lo.ToPtr(int64(11)),
						URLCitation: &URLCitation{
							URL:   "https://en.wikipedia.org/wiki/Meaning_of_life",
							Title: "Meaning of life - Wikipedia",
						},
					},
					{
						Type:       "url_citation",
						StartIndex: lo.ToPtr(int64(20)),
						EndIndex:   lo.ToPtr(int64(27)),
						URLCitation: &URLCitation{
							URL:   "https://plato.stanford.edu/entries/life-meaning/",
							Title: "The Meaning of Life - Stanford Encyclopedia",
						},
					},
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Len(t, msg.Annotations, 2)
				require.Equal(t, "url_citation", msg.Annotations[0].Type)
				require.NotNil(t, msg.Annotations[0].StartIndex)
				require.Equal(t, int64(0), *msg.Annotations[0].StartIndex)
				require.NotNil(t, msg.Annotations[0].EndIndex)
				require.Equal(t, int64(11), *msg.Annotations[0].EndIndex)
				require.NotNil(t, msg.Annotations[0].URLCitation)
				require.Equal(t, "https://en.wikipedia.org/wiki/Meaning_of_life", msg.Annotations[0].URLCitation.URL)
				require.Equal(t, "Meaning of life - Wikipedia", msg.Annotations[0].URLCitation.Title)
				require.NotNil(t, msg.Annotations[1].StartIndex)
				require.Equal(t, int64(20), *msg.Annotations[1].StartIndex)
				require.NotNil(t, msg.Annotations[1].EndIndex)
				require.Equal(t, int64(27), *msg.Annotations[1].EndIndex)
			},
		},
		{
			name: "message without annotations",
			oaiMsg: Message{
				Role:    "assistant",
				Content: MessageContent{Content: lo.ToPtr("Hello!")},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Nil(t, msg.Annotations)
			},
		},
		{
			name: "message with empty annotations",
			oaiMsg: Message{
				Role:        "assistant",
				Content:     MessageContent{Content: lo.ToPtr("Hello!")},
				Annotations: []Annotation{},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Nil(t, msg.Annotations)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.oaiMsg.ToLLMMessage()
			tt.validate(t, result)
		})
	}
}

func TestResponse_ToLLMResponse_WithCitations(t *testing.T) {
	tests := []struct {
		name     string
		oaiResp  *Response
		validate func(*testing.T, *llm.Response)
	}{
		{
			name: "response with citations",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "llama-3.1-sonar-small-128k-online",
				Choices: []Choice{
					{
						Index: 0,
						Message: &Message{
							Role:    "assistant",
							Content: MessageContent{Content: lo.ToPtr("The meaning of life is...")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Citations: []string{
					"https://www.theatlantic.com/family/archive/2021/10/meaning-life-macronutrients-purpose-search/620440/",
					"https://en.wikipedia.org/wiki/Meaning_of_life",
					"https://greatergood.berkeley.edu/article/item/three_ways_to_see_meaning_in_your_life",
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.NotNil(t, resp.TransformerMetadata)
				citations, ok := resp.TransformerMetadata[TransformerMetadataKeyCitations].([]string)
				require.True(t, ok)
				require.Len(t, citations, 3)
				require.Contains(t, citations, "https://www.theatlantic.com/family/archive/2021/10/meaning-life-macronutrients-purpose-search/620440/")
				require.Contains(t, citations, "https://en.wikipedia.org/wiki/Meaning_of_life")
				require.Contains(t, citations, "https://greatergood.berkeley.edu/article/item/three_ways_to_see_meaning_in_your_life")
			},
		},
		{
			name: "response without citations",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Message: &Message{
							Role:    "assistant",
							Content: MessageContent{Content: lo.ToPtr("Hello!")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				// TransformerMetadata should be nil when no citations
				require.Nil(t, resp.TransformerMetadata)
			},
		},
		{
			name: "response with empty citations",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Message: &Message{
							Role:    "assistant",
							Content: MessageContent{Content: lo.ToPtr("Hello!")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Citations: []string{},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				// TransformerMetadata should be nil when citations are empty
				require.Nil(t, resp.TransformerMetadata)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.oaiResp.ToLLMResponse()
			tt.validate(t, result)
		})
	}
}

func TestRequestFromLLM_KeepsGoogleThoughtSignatureInRequestModel(t *testing.T) {
	req := RequestFromLLM(&llm.Request{
		Model: "gemini-3-pro",
		Messages: []llm.Message{
			{
				Role:               "assistant",
				ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("sig_from_reasoning")),
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"city":"Shanghai"}`,
						},
						Index: 0,
						TransformerMetadata: map[string]any{
							TransformerMetadataKeyGoogleThoughtSignature: "sig_from_metadata",
						},
					},
				},
			},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Len(t, req.Messages, 1)
	require.Len(t, req.Messages[0].ToolCalls, 1)
	require.NotNil(t, req.Messages[0].ToolCalls[0].ExtraContent)
	require.NotNil(t, req.Messages[0].ToolCalls[0].ExtraContent.Google)
	require.Equal(t, "sig_from_metadata", req.Messages[0].ToolCalls[0].ExtraContent.Google.ThoughtSignature)
}

func TestMessageFromLLM_DoesNotOverrideFirstToolCallWhenMetadataExists(t *testing.T) {
	msg := MessageFromLLM(llm.Message{
		Role:               "assistant",
		ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("sig_from_second_tool_call")),
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "tool_a",
					Arguments: "{}",
				},
				Index: 0,
			},
			{
				ID:   "call_2",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "tool_b",
					Arguments: "{}",
				},
				Index: 1,
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyGoogleThoughtSignature: "sig_from_second_tool_call",
				},
			},
		},
	})

	require.Len(t, msg.ToolCalls, 2)
	require.Nil(t, msg.ToolCalls[0].ExtraContent)
	require.NotNil(t, msg.ToolCalls[1].ExtraContent)
	require.NotNil(t, msg.ToolCalls[1].ExtraContent.Google)
	require.Equal(t, "sig_from_second_tool_call", msg.ToolCalls[1].ExtraContent.Google.ThoughtSignature)
}

func TestMessageFromLLM_GeminiReasoningSignatureDoesNotInjectThoughtSignature(t *testing.T) {
	msg := MessageFromLLM(llm.Message{
		Role:               "assistant",
		ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("gemini_signature")),
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "tool_a",
					Arguments: "{}",
				},
				Index: 0,
			},
		},
	})

	require.Len(t, msg.ToolCalls, 1)
	require.Nil(t, msg.ToolCalls[0].ExtraContent)
}

func TestApplyReasoningEffortMapping(t *testing.T) {
	tests := []struct {
		name    string
		effort  string
		mapping []llm.ReasoningEffortMapping
		want    string
	}{
		{name: "xhigh mapped to max", effort: "xhigh", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}}, want: "max"},
		{name: "high mapped to medium", effort: "high", mapping: []llm.ReasoningEffortMapping{{From: "high", To: "medium"}}, want: "medium"},
		{name: "max passes through (not in list)", effort: "max", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}}, want: "max"},
		{name: "low passes through (not in list)", effort: "low", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}}, want: "low"},
		{name: "empty effort passes through", effort: "", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}}, want: ""},
		{name: "nil mapping passes through", effort: "xhigh", mapping: nil, want: "xhigh"},
		{name: "empty mapping passes through", effort: "xhigh", mapping: []llm.ReasoningEffortMapping{}, want: "xhigh"},
		{name: "multiple entries hit", effort: "high", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}, {From: "high", To: "medium"}}, want: "medium"},
		{name: "multiple entries miss", effort: "low", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}, {From: "high", To: "medium"}}, want: "low"},
		{name: "first matching entry wins", effort: "xhigh", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}, {From: "xhigh", To: "high"}}, want: "max"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, applyReasoningEffortMapping(tt.effort, tt.mapping))
		})
	}
}

// TestRequestFromLLM_PreservesReasoningEffort ensures RequestFromLLM does NOT map
// reasoning_effort: mapping is the OutboundTransformer's responsibility (driven by
// Config.ReasoningEffortMapping), not the package-level converter's.
func TestRequestFromLLM_PreservesReasoningEffort(t *testing.T) {
	req := RequestFromLLM(&llm.Request{
		Model:           "gpt-4",
		ReasoningEffort: "xhigh",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Equal(t, "xhigh", req.ReasoningEffort, "RequestFromLLM must keep reasoning_effort unchanged; mapping happens in TransformRequest")
}

// Assistant messages that carry tool calls but no content must still serialize a
// content field. Omitting it (nil content) or emitting null (all parts filtered
// out) is accepted by OpenAI but rejected by stricter OpenAI-compatible upstreams
// whose schema only allows a string or an array.
func TestMessageFromLLM_ToolCallOnlyMessageKeepsContentField(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: llm.FunctionCall{
				Name:      "shell_command",
				Arguments: `{"command":"ls"}`,
			},
		},
	}

	tests := []struct {
		name    string
		message llm.Message
	}{
		{
			name: "no content at all",
			message: llm.Message{
				Role:      "assistant",
				ToolCalls: toolCalls,
			},
		},
		{
			name: "every content part filtered out",
			message: llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type:    "compaction",
							Compact: &llm.CompactContent{ID: "cmp_1", EncryptedContent: "secret"},
						},
					},
				},
				ToolCalls: toolCalls,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := MessageFromLLM(tt.message)

			data, err := json.Marshal(msg)
			require.NoError(t, err)

			var decoded map[string]any
			require.NoError(t, json.Unmarshal(data, &decoded))

			content, ok := decoded["content"]
			require.True(t, ok, "content field must be present, got %s", data)
			require.NotNil(t, content, "content must not be null, got %s", data)
			require.Equal(t, "", content)
		})
	}
}

// Content that survives conversion must be preserved as-is.
func TestMessageFromLLM_ToolCallMessageKeepsExistingContent(t *testing.T) {
	msg := MessageFromLLM(llm.Message{
		Role:    "assistant",
		Content: llm.MessageContent{Content: lo.ToPtr("calling a tool")},
		ToolCalls: []llm.ToolCall{
			{
				ID:       "call_1",
				Type:     "function",
				Function: llm.FunctionCall{Name: "shell_command", Arguments: "{}"},
			},
		},
	})

	require.NotNil(t, msg.Content.Content)
	require.Equal(t, "calling a tool", *msg.Content.Content)
}

// Messages without tool calls keep their existing serialization, so the
// normalization above cannot change unrelated payloads.
func TestMessageFromLLM_WithoutToolCallsContentUnchanged(t *testing.T) {
	msg := MessageFromLLM(llm.Message{Role: "user"})

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))

	_, ok := decoded["content"]
	require.False(t, ok, "content must stay omitted for messages without tool calls, got %s", data)
}

// Responses splits text parts into input_text/output_text, but Chat Completions
// only knows "text". Types it does understand must not be rewritten.
func TestMessageContentPartFromLLM_NormalizesTextPartTypes(t *testing.T) {
	tests := []struct {
		name     string
		partType string
		expected string
	}{
		{name: "input_text becomes text", partType: "input_text", expected: "text"},
		{name: "output_text becomes text", partType: "output_text", expected: "text"},
		{name: "text is unchanged", partType: "text", expected: "text"},
		{name: "image_url is unchanged", partType: "image_url", expected: "image_url"},
		{name: "video_url is unchanged", partType: "video_url", expected: "video_url"},
		{name: "input_audio is unchanged", partType: "input_audio", expected: "input_audio"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			part := MessageContentPartFromLLM(llm.MessageContentPart{
				Type: tt.partType,
				Text: lo.ToPtr("hello"),
			})

			require.Equal(t, tt.expected, part.Type)
		})
	}
}

// Multi-part content is the path where Responses text types actually reach an
// upstream: a lone text part is collapsed into a plain string before this point.
func TestRequestFromLLM_NormalizesTextPartTypesInMessages(t *testing.T) {
	req := RequestFromLLM(&llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "input_text", Text: lo.ToPtr("describe this")},
						{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/a.png"}},
					},
				},
			},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Len(t, req.Messages, 1)
	require.Len(t, req.Messages[0].Content.MultipleContent, 2)
	require.Equal(t, "text", req.Messages[0].Content.MultipleContent[0].Type)
	require.Equal(t, "image_url", req.Messages[0].Content.MultipleContent[1].Type)
}

func TestRequestFromLLM_DowngradesResponsesToolLifecycle(t *testing.T) {
	req := &llm.Request{
		Model:     "kimi-k3",
		APIFormat: llm.APIFormatOpenAIResponse,
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID: "call_custom", Type: llm.ToolTypeResponsesCustomTool,
						ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_custom", Name: "grep", Input: "pattern"},
					},
					{
						ID: "call_fn", Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"a.txt"}`},
					},
				},
			},
			{Role: "tool", ToolCallID: lo.ToPtr("call_custom"), Content: llm.MessageContent{Content: lo.ToPtr("match")}},
			{Role: "tool", ToolCallID: lo.ToPtr("call_fn"), Content: llm.MessageContent{Content: lo.ToPtr("content")}},
		},
		Tools: []llm.Tool{
			{Type: llm.ToolTypeResponsesCustomTool, ResponseCustomTool: &llm.ResponseCustomTool{Name: "grep"}},
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "read_file", Parameters: json.RawMessage(`{"type":"object"}`)}},
		},
	}

	got := RequestFromLLM(req, ReasoningFieldContent)
	require.NotNil(t, got)

	require.Len(t, got.Tools, 1)
	require.Equal(t, "read_file", got.Tools[0].Function.Name)

	var assistant *Message
	toolMessages := make([]Message, 0)
	for i := range got.Messages {
		msg := &got.Messages[i]
		for _, call := range msg.ToolCalls {
			require.NotEmpty(t, call.Function.Name, "tool call name must survive Responses downgrade")
			require.NotEmpty(t, call.Function.Arguments, "tool call arguments must survive Responses downgrade")
			require.NotEqual(t, llm.ToolTypeResponsesCustomTool, call.Type)
		}
		switch msg.Role {
		case "assistant":
			assistant = msg
		case "tool":
			toolMessages = append(toolMessages, *msg)
		}
	}

	require.NotNil(t, assistant)
	require.Len(t, assistant.ToolCalls, 1)
	require.Equal(t, "read_file", assistant.ToolCalls[0].Function.Name)

	require.Len(t, toolMessages, 1)
	require.Equal(t, "call_fn", lo.FromPtr(toolMessages[0].ToolCallID))
}

func TestResponsesChatToolStreamRestorer_FlushBufferedAtStreamEnd(t *testing.T) {
	restorer := newResponsesChatToolStreamRestorer(nil, []string{"Task", "TaskOutput"})

	// The name "Task" is a prefix of "TaskOutput", so the restorer keeps the
	// call buffered until a finish chunk resolves it.
	chunk := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
		ID: "call_task", Index: 0, Type: llm.ToolTypeFunction,
		Function: llm.FunctionCall{Name: "Task", Arguments: `{"prompt":"fix the bug"}`},
	}}}}}}
	restorer.restore(chunk)
	require.Empty(t, chunk.Choices[0].Delta.ToolCalls)

	flushed := restorer.flushBuffered()
	require.Len(t, flushed, 1)
	require.Len(t, flushed[0].Choices, 1)
	calls := flushed[0].Choices[0].Delta.ToolCalls
	require.Len(t, calls, 1)
	require.Equal(t, "Task", calls[0].Function.Name)
	require.Equal(t, "call_task", calls[0].ID)
	require.Equal(t, `{"prompt":"fix the bug"}`, calls[0].Function.Arguments)

	require.Empty(t, restorer.flushBuffered(), "second flush must be empty")
}

// An abnormal finish truncates only the in-flight call. Buffered calls whose
// identity and arguments already arrived must be released like the
// non-streaming conversion keeps complete calls; only pending fragments are
// dropped.
func TestResponsesChatToolStreamRestorer_AbnormalFinishKeepsReadyCallsAndDropsPending(t *testing.T) {
	restorer := newResponsesChatToolStreamRestorer(nil, []string{"thread_create"})

	first := &llm.Response{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{
		{Index: 0, Function: llm.FunctionCall{Name: "thr"}},
		{ID: "call_full", Index: 1, Function: llm.FunctionCall{Name: "thread_create", Arguments: `{"a":1`}},
	}}}}}
	restorer.restore(first)
	// Call 0 stays pending on a partial name; call 1 is ready but held back by
	// the lower pending index.
	require.Empty(t, first.Choices[0].Delta.ToolCalls)

	finish := &llm.Response{Choices: []llm.Choice{{Index: 0, FinishReason: lo.ToPtr("length"), Delta: &llm.Message{ToolCalls: []llm.ToolCall{
		{Index: 0, Function: llm.FunctionCall{Arguments: `ead`}},
		{Index: 1, Function: llm.FunctionCall{Arguments: `,"b":2}`}},
	}}}}}
	restorer.restore(finish)

	calls := finish.Choices[0].Delta.ToolCalls
	require.Len(t, calls, 1)
	require.Equal(t, "call_full", calls[0].ID)
	require.Equal(t, "thread_create", calls[0].Function.Name)
	require.Equal(t, `{"a":1,"b":2}`, calls[0].Function.Arguments)
	require.Nil(t, restorer.flushBuffered())
}

// mergeResponsesChatToolCallFragments must carry namespace and
// TransformerMetadata from both fragments instead of dropping them when the
// second chunk carries the fields.
func TestMergeResponsesChatToolCallFragments_NamespaceAndMetadata(t *testing.T) {
	pending := llm.ToolCall{
		ID: "call_ns", Index: 0,
		Function: llm.FunctionCall{Name: "spawn", Arguments: `{"ta`},
	}
	current := llm.ToolCall{
		Index: 0,
		Function: llm.FunctionCall{
			Name: "_agent", Arguments: `sk":"x"}`, Namespace: "collaboration",
		},
		TransformerMetadata: map[string]any{"origin": "namespace"},
	}

	merged := mergeResponsesChatToolCallFragments(pending, current)
	require.Equal(t, "call_ns", merged.ID)
	require.Equal(t, 0, merged.Index)
	require.Equal(t, "spawn_agent", merged.Function.Name)
	require.Equal(t, "collaboration", merged.Function.Namespace)
	require.Equal(t, `{"task":"x"}`, merged.Function.Arguments)
	require.Equal(t, map[string]any{"origin": "namespace"}, merged.TransformerMetadata)

	// A later fragment without namespace or metadata keeps earlier values.
	next := llm.ToolCall{Index: 0, Function: llm.FunctionCall{Arguments: `{}`}}
	mergedAgain := mergeResponsesChatToolCallFragments(merged, next)
	require.Equal(t, "collaboration", mergedAgain.Function.Namespace)
	require.Equal(t, map[string]any{"origin": "namespace"}, mergedAgain.TransformerMetadata)
}
