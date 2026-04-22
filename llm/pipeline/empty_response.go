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

	if resp.Embedding != nil && len(resp.Embedding.Data) > 0 {
		return true
	}

	if resp.Rerank != nil && len(resp.Rerank.Results) > 0 {
		return true
	}

	if resp.Image != nil && len(resp.Image.Data) > 0 {
		return true
	}

	if resp.Video != nil &&
		(resp.Video.ID != "" || resp.Video.Status != "" || resp.Video.VideoURL != "" || resp.Video.Error != nil) {
		return true
	}

	if resp.Compact != nil && len(resp.Compact.Output) > 0 {
		return true
	}

	for _, choice := range resp.Choices {
		if hasMessageContent(choice.Delta) || hasMessageContent(choice.Message) {
			return true
		}
	}

	return false
}
