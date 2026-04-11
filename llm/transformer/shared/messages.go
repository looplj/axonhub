package shared

import "github.com/looplj/axonhub/llm"

// FilterOutResponseCustomToolMessages removes Responses-only custom tool calls
// from assistant messages and drops tool result messages that correspond to
// those removed custom tool calls.
//
// This is intended for compatibility when a request originates from an OpenAI
// Responses session and is then routed to a non-Responses channel. In that
// case, Responses-only custom tools must be stripped from the message history
// before the outbound transformer encodes the request for the target channel.
func FilterOutResponseCustomToolMessages(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}

	removedToolCallIDs := make(map[string]struct{})
	filtered := make([]llm.Message, 0, len(messages))

	for _, msg := range messages {
		if msg.Role == "tool" && msg.ToolCallID != nil {
			if _, removed := removedToolCallIDs[*msg.ToolCallID]; removed {
				continue
			}
		}

		if len(msg.ToolCalls) == 0 {
			filtered = append(filtered, msg)
			continue
		}

		cloned := msg
		cloned.ToolCalls = make([]llm.ToolCall, 0, len(msg.ToolCalls))

		for _, toolCall := range msg.ToolCalls {
			if toolCall.Type == llm.ToolTypeResponsesCustomTool || toolCall.ResponseCustomToolCall != nil {
				if toolCall.ID != "" {
					removedToolCallIDs[toolCall.ID] = struct{}{}
				}
				if toolCall.ResponseCustomToolCall != nil && toolCall.ResponseCustomToolCall.CallID != "" {
					removedToolCallIDs[toolCall.ResponseCustomToolCall.CallID] = struct{}{}
				}

				continue
			}

			cloned.ToolCalls = append(cloned.ToolCalls, toolCall)
		}

		if shouldDropMessageAfterToolFiltering(cloned) {
			continue
		}

		filtered = append(filtered, cloned)
	}

	return filtered
}

func shouldDropMessageAfterToolFiltering(msg llm.Message) bool {
	if len(msg.ToolCalls) > 0 {
		return false
	}

	if msg.Content.Content != nil || len(msg.Content.MultipleContent) > 0 {
		return false
	}

	if msg.Refusal != "" || msg.ToolCallID != nil || msg.ReasoningContent != nil || msg.Reasoning != nil || msg.Audio != nil {
		return false
	}

	return true
}
