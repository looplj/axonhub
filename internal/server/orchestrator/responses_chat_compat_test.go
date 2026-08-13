package orchestrator

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func compatTestCandidate(enableCompat bool) *ChannelModelsCandidate {
	return &ChannelModelsCandidate{
		Channel: &biz.Channel{Channel: &ent.Channel{
			ID:   1,
			Name: "chat-channel",
			Settings: &objects.ChannelSettings{
				TransformOptions: objects.TransformOptions{EnableResponsesChatCompat: enableCompat},
			},
		}},
		Models:    []biz.ChannelModelEntry{{RequestModel: "gpt-5", ActualModel: "gpt-5"}},
		APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
	}
}

func responsesRequestWithCustomToolCall() *llm.Request {
	return &llm.Request{
		Model:     "gpt-5",
		APIFormat: llm.APIFormatOpenAIResponse,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_custom_1",
						Type: llm.ToolTypeResponsesCustomTool,
						ResponseCustomToolCall: &llm.ResponseCustomToolCall{
							CallID: "call_custom_1",
							Name:   "apply_patch",
							Input:  "patch",
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_custom_1"),
				Content:    llm.MessageContent{Content: lo.ToPtr("custom tool output")},
			},
		},
	}
}

func TestChannelEnablesResponsesChatCompat(t *testing.T) {
	require.False(t, channelEnablesResponsesChatCompat(nil))
	require.False(t, channelEnablesResponsesChatCompat(&ChannelModelsCandidate{}))
	require.False(t, channelEnablesResponsesChatCompat(&ChannelModelsCandidate{
		Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}},
	}))
	require.False(t, channelEnablesResponsesChatCompat(compatTestCandidate(false)))
	require.True(t, channelEnablesResponsesChatCompat(compatTestCandidate(true)))
}

func TestFilterResponseCustomToolMessagesForNonResponsesOutbound(t *testing.T) {
	req := responsesRequestWithCustomToolCall()

	// Responses request served by a Chat endpoint drops custom tool messages.
	filtered := filterResponseCustomToolMessagesForNonResponsesOutbound(req, llm.APIFormatOpenAIChatCompletion)
	require.NotSame(t, req, filtered)
	require.Empty(t, filtered.Messages)

	// Responses-native endpoints keep the messages.
	same := filterResponseCustomToolMessagesForNonResponsesOutbound(req, llm.APIFormatOpenAIResponse)
	require.Same(t, req, same)

	// Non-Responses requests are untouched.
	chatReq := &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion, Messages: req.Messages}
	same = filterResponseCustomToolMessagesForNonResponsesOutbound(chatReq, llm.APIFormatOpenAIChatCompletion)
	require.Same(t, chatReq, same)
}

func TestPersistentOutboundTransformer_ResponsesChatCompatSwitch(t *testing.T) {
	tests := []struct {
		name         string
		enableCompat bool
		wantDisabled bool
		wantMsgCount int
	}{
		{name: "disabled keeps legacy conversion", enableCompat: false, wantDisabled: true, wantMsgCount: 0},
		{name: "enabled uses beta conversion", enableCompat: true, wantDisabled: false, wantMsgCount: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbound := &mockTransformer{
				apiFormat:          llm.APIFormatOpenAIChatCompletion,
				requestAPIFormat:   llm.APIFormatOpenAIChatCompletion,
				responsesChatTools: tt.enableCompat,
			}
			candidate := compatTestCandidate(tt.enableCompat)
			candidate.Channel.Outbound = outbound

			processor := &PersistentOutboundTransformer{
				wrapped: outbound,
				state: &PersistenceState{
					ChannelModelsCandidates: []*ChannelModelsCandidate{candidate},
				},
			}

			_, err := processor.TransformRequest(context.Background(), responsesRequestWithCustomToolCall())
			require.NoError(t, err)
			require.NotNil(t, outbound.capturedRequest)
			require.Equal(t, tt.wantDisabled, outbound.capturedRequest.TransformOptions.DisableResponsesChatCompat)
			require.Len(t, outbound.capturedRequest.Messages, tt.wantMsgCount)
		})
	}
}
