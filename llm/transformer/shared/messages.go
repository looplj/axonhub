package shared

import (
	"encoding/json"
	"strings"

	"github.com/looplj/axonhub/llm"
)

// FilterOutResponseCustomToolMessages removes Responses-only custom tool calls
// from assistant messages and drops tool result messages that correspond to
// those removed custom tool calls.
//
// This is intended for compatibility when a request originates from an OpenAI
// Responses session and is then routed to a non-Responses channel. In that
// case, Responses-only custom tools must be stripped from the message history
// before the outbound transformer encodes the request for the target channel.
func FilterOutResponseCustomToolMessages(messages []llm.Message) []llm.Message {
	return filterOutToolLifecycleMessages(messages, func(toolCall llm.ToolCall) bool {
		return toolCall.Type == llm.ToolTypeResponsesCustomTool || toolCall.ResponseCustomToolCall != nil
	})
}

// FilterOutResponsesChatToolLifecycleMessages removes calls that need the
// reversible Responses-to-Chat adapter and their paired tool outputs. Plain
// function calls remain available to provider-specific Chat transformers.
func FilterOutResponsesChatToolLifecycleMessages(messages []llm.Message) []llm.Message {
	return filterOutToolLifecycleMessages(messages, func(toolCall llm.ToolCall) bool {
		return toolCall.Type == llm.ToolTypeResponsesCustomTool ||
			toolCall.ResponseCustomToolCall != nil ||
			toolCall.Type == llm.ToolTypeResponsesToolSearch ||
			toolCall.ResponseToolSearchCall != nil ||
			toolCall.Function.Namespace != ""
	})
}

func filterOutToolLifecycleMessages(messages []llm.Message, shouldRemove func(llm.ToolCall) bool) []llm.Message {
	if len(messages) == 0 {
		return nil
	}

	type lifecycleOccurrence struct {
		messageIndex int
		remove       bool
	}
	occurrences := make(map[string][]lifecycleOccurrence)
	for messageIndex, msg := range messages {
		for _, toolCall := range msg.ToolCalls {
			remove := shouldRemove(toolCall)
			ids := []string{toolCall.ID}
			if toolCall.ResponseCustomToolCall != nil {
				ids = append(ids, toolCall.ResponseCustomToolCall.CallID)
			}
			if toolCall.ResponseToolSearchCall != nil {
				ids = append(ids, toolCall.ResponseToolSearchCall.CallID)
			}
			seen := make(map[string]struct{}, len(ids))
			for _, id := range ids {
				if id == "" {
					continue
				}
				if _, duplicate := seen[id]; duplicate {
					continue
				}
				seen[id] = struct{}{}
				occurrences[id] = append(occurrences[id], lifecycleOccurrence{messageIndex: messageIndex, remove: remove})
			}
		}
	}
	shouldRemoveOutput := func(id string, messageIndex int) bool {
		matches := occurrences[id]
		if len(matches) == 0 {
			return false
		}
		for i := len(matches) - 1; i >= 0; i-- {
			if matches[i].messageIndex < messageIndex {
				return matches[i].remove
			}
		}
		return matches[0].remove
	}

	filtered := make([]llm.Message, 0, len(messages))

	for messageIndex, msg := range messages {
		if msg.Role == "tool" && msg.ToolCallID != nil {
			if shouldRemoveOutput(*msg.ToolCallID, messageIndex) {
				continue
			}
		}

		cloned := msg
		if len(msg.ToolCalls) > 0 {
			cloned.ToolCalls = make([]llm.ToolCall, 0, len(msg.ToolCalls))
			for _, toolCall := range msg.ToolCalls {
				if shouldRemove(toolCall) {
					continue
				}
				cloned.ToolCalls = append(cloned.ToolCalls, toolCall)
			}
		}

		if !HasChatCompatibleAssistantPayload(cloned) {
			continue
		}

		filtered = append(filtered, cloned)
	}

	return filtered
}

// HasChatCompatibleAssistantPayload reports whether an assistant message still
// contains substantive data after Responses-only lifecycle fields are removed.
// Other roles have different validation rules and always pass through.
func HasChatCompatibleAssistantPayload(msg llm.Message) bool {
	if msg.Role != "assistant" {
		return true
	}
	if len(msg.ToolCalls) > 0 {
		return true
	}
	if msg.ReasoningContent != nil && strings.TrimSpace(*msg.ReasoningContent) != "" ||
		msg.Reasoning != nil && strings.TrimSpace(*msg.Reasoning) != "" ||
		strings.TrimSpace(msg.Refusal) != "" || hasOutputAudioPayload(msg.Audio) {
		return true
	}
	if len(msg.Content.MultipleContent) == 0 {
		return msg.Content.Content != nil && strings.TrimSpace(*msg.Content.Content) != ""
	}
	for _, part := range msg.Content.MultipleContent {
		switch part.Type {
		case "text":
			if part.Text != nil && strings.TrimSpace(*part.Text) != "" {
				return true
			}
		case "image_url":
			if part.ImageURL != nil && strings.TrimSpace(part.ImageURL.URL) != "" {
				return true
			}
		case "video_url":
			if part.VideoURL != nil && strings.TrimSpace(part.VideoURL.URL) != "" {
				return true
			}
		case "input_audio":
			if part.InputAudio != nil && strings.TrimSpace(part.InputAudio.Data) != "" {
				return true
			}
		}
	}
	return false
}

func hasOutputAudioPayload(audio *llm.OutputAudio) bool {
	return audio != nil && (strings.TrimSpace(audio.ID) != "" || strings.TrimSpace(audio.Data) != "" ||
		strings.TrimSpace(audio.Transcript) != "")
}

// SanitizeChatToolArguments repairs assistant tool-call arguments that are not
// valid JSON. Truncated streams can leave clients replaying partial arguments,
// which strict Chat providers reject for the whole request. Returns the
// repaired slice and whether anything changed.
func SanitizeChatToolArguments(messages []llm.Message) ([]llm.Message, bool) {
	result := messages
	changed := false

	for messageIndex, message := range messages {
		if message.Role != "assistant" || len(message.ToolCalls) == 0 {
			continue
		}

		for callIndex, call := range message.ToolCalls {
			repaired, ok := repairToolCallArguments(call)
			if !ok {
				continue
			}

			if !changed {
				result = append([]llm.Message(nil), messages...)
				changed = true
			}
			if &result[messageIndex].ToolCalls[0] == &message.ToolCalls[0] {
				result[messageIndex].ToolCalls = append([]llm.ToolCall(nil), message.ToolCalls...)
			}
			result[messageIndex].ToolCalls[callIndex] = repaired
		}
	}

	return result, changed
}

// repairToolCallArguments substitutes an empty JSON object for arguments that
// no Chat provider can accept, reporting whether the call changed.
func repairToolCallArguments(call llm.ToolCall) (llm.ToolCall, bool) {
	if call.ResponseToolSearchCall != nil {
		if isValidToolCallArguments(call.ResponseToolSearchCall.Arguments) {
			return call, false
		}
		repairedSearchCall := *call.ResponseToolSearchCall
		repairedSearchCall.Arguments = "{}"
		call.ResponseToolSearchCall = &repairedSearchCall
		call.Function.Arguments = "{}"
		return call, true
	}

	if call.ResponseCustomToolCall != nil || isValidToolCallArguments(call.Function.Arguments) {
		return call, false
	}
	call.Function.Arguments = "{}"
	return call, true
}

func isValidToolCallArguments(arguments string) bool {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" || trimmed == "null" {
		return false
	}
	return json.Valid([]byte(trimmed))
}

const emptyToolOutputPlaceholder = "(empty)"

// SanitizeChatMessageContent removes messages that carry no expressible text
// content and substitutes empty tool outputs, so strict Chat providers do not
// reject replayed history with "text content is empty". Interrupted upstream
// turns leave empty output items in client history; replaying them verbatim
// poisons every subsequent request.
func SanitizeChatMessageContent(messages []llm.Message) ([]llm.Message, bool) {
	result := make([]llm.Message, 0, len(messages))
	changed := false

	for _, message := range messages {
		sanitized, keep, modified := sanitizeChatMessageContent(message)
		if !keep {
			changed = true
			continue
		}
		if modified {
			changed = true
		}
		result = append(result, sanitized)
	}

	if !changed {
		return messages, false
	}
	return result, true
}

func sanitizeChatMessageContent(message llm.Message) (llm.Message, bool, bool) {
	// Tool-call turns are valid with null/empty content on every provider.
	if len(message.ToolCalls) > 0 {
		return message, true, false
	}

	if len(message.Content.MultipleContent) > 0 {
		filtered := make([]llm.MessageContentPart, 0, len(message.Content.MultipleContent))
		for _, part := range message.Content.MultipleContent {
			if visibleChatContentPart(part) {
				filtered = append(filtered, part)
			}
		}
		if len(filtered) == len(message.Content.MultipleContent) {
			return message, true, false
		}
		if len(filtered) > 0 {
			message.Content.MultipleContent = filtered
			return message, true, true
		}
		message.Content.MultipleContent = nil
		if keepReasoningOnlyAssistant(message) {
			return message, true, true
		}
		return substituteEmptyToolOutput(message)
	}

	if message.Content.Content != nil && strings.TrimSpace(*message.Content.Content) != "" {
		return message, true, false
	}

	if keepReasoningOnlyAssistant(message) {
		return message, true, false
	}
	return substituteEmptyToolOutput(message)
}

// keepReasoningOnlyAssistant retains assistant turns whose only substantive
// payload is reasoning, refusal, or audio. HasChatCompatibleAssistantPayload
// counts these as valid payloads; dropping the message would make replayed
// history disagree with that rule and lose echoable reasoning summaries.
func keepReasoningOnlyAssistant(message llm.Message) bool {
	return message.Role == "assistant" && HasChatCompatibleAssistantPayload(message)
}

// substituteEmptyToolOutput keeps tool messages paired with their call by
// giving them a placeholder, and drops every other contentless message.
func substituteEmptyToolOutput(message llm.Message) (llm.Message, bool, bool) {
	if message.Role == "tool" {
		placeholder := emptyToolOutputPlaceholder
		message.Content = llm.MessageContent{Content: &placeholder}
		return message, true, true
	}
	return message, false, false
}

func visibleChatContentPart(part llm.MessageContentPart) bool {
	switch part.Type {
	case "text", "input_text", "output_text":
		return part.Text != nil && strings.TrimSpace(*part.Text) != ""
	case "image_url":
		return part.ImageURL != nil && strings.TrimSpace(part.ImageURL.URL) != ""
	case "video_url":
		return part.VideoURL != nil && strings.TrimSpace(part.VideoURL.URL) != ""
	case "input_audio":
		return part.InputAudio != nil && strings.TrimSpace(part.InputAudio.Data) != ""
	case "compaction", "compaction_summary":
		// Chat conversion drops these parts, so they cannot keep a message alive.
		return false
	default:
		return true
	}
}
