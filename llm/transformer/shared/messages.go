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
	retainedToolCallIDs := make(map[string]struct{})
	filtered := make([]llm.Message, 0, len(messages))

	for _, msg := range messages {
		if msg.Role == "tool" && msg.ToolCallID != nil {
			if _, removed := removedToolCallIDs[*msg.ToolCallID]; removed {
				continue
			}
			if msg.ProtocolExtensions != nil {
				if _, retained := retainedToolCallIDs[*msg.ToolCallID]; !retained {
					continue
				}
			}
		} else if msg.Role == "tool" && msg.ProtocolExtensions != nil {
			continue
		}

		if len(msg.ToolCalls) == 0 {
			filtered = append(filtered, stripResponsesProtocolExtensionsFromMessage(msg))
			continue
		}

		cloned := stripResponsesProtocolExtensionsFromMessage(msg)
		cloned.ToolCalls = make([]llm.ToolCall, 0, len(msg.ToolCalls))

		for _, toolCall := range msg.ToolCalls {
			if IsResponsesOnlyToolCall(toolCall) {
				if toolCall.ID != "" {
					removedToolCallIDs[toolCall.ID] = struct{}{}
				}

				if toolCall.ResponseCustomToolCall != nil && toolCall.ResponseCustomToolCall.CallID != "" {
					removedToolCallIDs[toolCall.ResponseCustomToolCall.CallID] = struct{}{}
				}

				continue
			}

			toolCall.ProtocolExtensions = nil
			if toolCall.ID != "" {
				retainedToolCallIDs[toolCall.ID] = struct{}{}
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

func stripResponsesProtocolExtensionsFromMessage(msg llm.Message) llm.Message {
	msg.ProtocolExtensions = nil
	for i := range msg.Content.MultipleContent {
		msg.Content.MultipleContent[i].TransformerMetadata = nil
	}
	return msg
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

// FilterOutResponsesOnlyTools removes tool definitions that are only valid in the
// OpenAI Responses API and strips lossless protocol metadata from retained tools.
func FilterOutResponsesOnlyTools(tools []llm.Tool) []llm.Tool {
	if len(tools) == 0 {
		return nil
	}

	filtered := make([]llm.Tool, 0, len(tools))
	for _, tool := range tools {
		if IsResponsesOnlyTool(tool) {
			continue
		}
		tool.ProtocolExtensions = nil
		filtered = append(filtered, tool)
	}
	return filtered
}

func IsResponsesOnlyTool(tool llm.Tool) bool {
	switch tool.Type {
	case llm.ToolTypeResponsesCustomTool, "namespace", "tool_search", "local_shell", "mcp", "shell", "apply_patch", "file_search", "code_interpreter", "computer_use_preview":
		return true
	default:
		return false
	}
}

func IsResponsesOnlyToolCall(toolCall llm.ToolCall) bool {
	switch toolCall.Type {
	case llm.ToolTypeResponsesCustomTool, "local_shell_call", "shell_call", "mcp_call", "apply_patch_call", "tool_search_call":
		return true
	default:
		return toolCall.ResponseCustomToolCall != nil
	}
}
