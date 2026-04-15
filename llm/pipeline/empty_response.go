package pipeline

import (
	"errors"

	"github.com/looplj/axonhub/llm"
)

// ErrEmptyResponse indicates the response contains no meaningful content.
// This error triggers channel retry when empty response detection is enabled.
var ErrEmptyResponse = errors.New("empty response detected")

func hasMessageContent(msg *llm.Message) bool {
	if msg == nil {
		return false
	}

	if msg.Content.Content != nil && *msg.Content.Content != "" {
		return true
	}

	if len(msg.Content.MultipleContent) > 0 {
		return true
	}

	if len(msg.ToolCalls) > 0 {
		return true
	}

	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		return true
	}

	if msg.Reasoning != nil && *msg.Reasoning != "" {
		return true
	}

	if msg.Refusal != "" {
		return true
	}

	if msg.Audio != nil {
		return true
	}

	return false
}

// hasResponseContent checks if an llm.Response contains meaningful content.
func hasResponseContent(resp *llm.Response) bool {
	if resp == nil || resp == llm.DoneResponse {
		return false
	}

	for _, choice := range resp.Choices {
		if hasMessageContent(choice.Delta) || hasMessageContent(choice.Message) {
			return true
		}
	}

	return false
}
