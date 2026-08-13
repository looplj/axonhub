package orchestrator

import (
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// channelEnablesResponsesChatCompat reports whether the candidate's channel
// opted into the beta Responses-to-Chat compatibility in its transform
// options. When disabled, Responses requests served by Chat endpoints fall
// back to the legacy generic conversion instead of being blocked.
func channelEnablesResponsesChatCompat(candidate *ChannelModelsCandidate) bool {
	if candidate == nil || candidate.Channel == nil || candidate.Channel.Settings == nil {
		return false
	}

	return candidate.Channel.Settings.TransformOptions.EnableResponsesChatCompat
}

// filterResponseCustomToolMessagesForNonResponsesOutbound is the legacy
// (pre-beta) guard for Responses-origin requests served by non-Responses
// endpoints: it drops messages carrying Responses-only custom tool calls,
// which Chat Completions endpoints cannot represent.
func filterResponseCustomToolMessagesForNonResponsesOutbound(
	llmRequest *llm.Request,
	outboundFormat llm.APIFormat,
) *llm.Request {
	if llmRequest == nil {
		return nil
	}

	if !isResponsesFormat(llmRequest.APIFormat) || isResponsesFormat(outboundFormat) || !containsResponseCustomToolMessages(llmRequest.Messages) {
		return llmRequest
	}

	cloned := *llmRequest
	cloned.Messages = shared.FilterOutResponseCustomToolMessages(llmRequest.Messages)

	return &cloned
}

func containsResponseCustomToolMessages(messages []llm.Message) bool {
	for _, msg := range messages {
		for _, toolCall := range msg.ToolCalls {
			if toolCall.Type == llm.ToolTypeResponsesCustomTool || toolCall.ResponseCustomToolCall != nil {
				return true
			}
		}
	}

	return false
}
