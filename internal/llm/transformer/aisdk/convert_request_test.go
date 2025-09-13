package aisdk

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

func TestConvertToLLMRequestComprehensive_SystemMessage(t *testing.T) {
	t.Run("simple system message", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "system",
					Parts: []UIMessagePart{
						{Type: "text", Text: "System message"},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		expected := llm.Message{
			Role: "system",
			Content: llm.MessageContent{
				Content: lo.ToPtr("System message"),
			},
		}
		assert.Equal(t, expected, result.Messages[0])
	})

	t.Run("system message with provider metadata", func(t *testing.T) {
		metadata := map[string]interface{}{
			"testProvider": map[string]interface{}{
				"systemSignature": "abc123",
			},
		}
		metadataBytes, _ := json.Marshal(metadata)

		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "system",
					Parts: []UIMessagePart{
						{
							Type:             "text",
							Text:             "System message with metadata",
							ProviderMetadata: json.RawMessage(metadataBytes),
						},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "system", result.Messages[0].Role)
		assert.Equal(t, "System message with metadata", *result.Messages[0].Content.Content)
		// TODO: Verify provider metadata when LLM structs support it
	})

	t.Run("system message with multiple text parts", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "system",
					Parts: []UIMessagePart{
						{Type: "text", Text: "Part 1"},
						{Type: "text", Text: " Part 2"},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "system", result.Messages[0].Role)
		assert.Equal(t, "Part 1 Part 2", *result.Messages[0].Content.Content)
	})

	t.Run("system message from content string", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role:    "system",
					Content: "System message from content",
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "system", result.Messages[0].Role)
		assert.Equal(t, "System message from content", *result.Messages[0].Content.Content)
	})
}

func TestConvertToLLMRequestComprehensive_UserMessage(t *testing.T) {
	t.Run("simple user message", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "user",
					Parts: []UIMessagePart{
						{Type: "text", Text: "Hello, AI!"},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "user", result.Messages[0].Role)
		require.Len(t, result.Messages[0].Content.MultipleContent, 1)
		assert.Equal(t, "text", result.Messages[0].Content.MultipleContent[0].Type)
		assert.Equal(t, "Hello, AI!", *result.Messages[0].Content.MultipleContent[0].Text)
	})

	t.Run("user message with file part", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "user",
					Parts: []UIMessagePart{
						{
							Type:      "file",
							MediaType: "image/jpeg",
							URL:       "https://example.com/image.jpg",
						},
						{Type: "text", Text: "Check this image"},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "user", result.Messages[0].Role)
		require.Len(t, result.Messages[0].Content.MultipleContent, 2)

		// Check image part
		imagePart := result.Messages[0].Content.MultipleContent[0]
		assert.Equal(t, "image_url", imagePart.Type)
		require.NotNil(t, imagePart.ImageURL)
		assert.Equal(t, "https://example.com/image.jpg", imagePart.ImageURL.URL)

		// Check text part
		textPart := result.Messages[0].Content.MultipleContent[1]
		assert.Equal(t, "text", textPart.Type)
		assert.Equal(t, "Check this image", *textPart.Text)
	})

	t.Run("user message with filename", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "user",
					Parts: []UIMessagePart{
						{
							Type:      "file",
							MediaType: "image/jpeg",
							URL:       "https://example.com/image.jpg",
							Filename:  "image.jpg",
						},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "user", result.Messages[0].Role)
		require.Len(t, result.Messages[0].Content.MultipleContent, 1)

		imagePart := result.Messages[0].Content.MultipleContent[0]
		assert.Equal(t, "image_url", imagePart.Type)
		require.NotNil(t, imagePart.ImageURL)
		assert.Equal(t, "https://example.com/image.jpg", imagePart.ImageURL.URL)
	})

	t.Run("user message from content string", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role:    "user",
					Content: "Hello from content",
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "user", result.Messages[0].Role)
		assert.Equal(t, "Hello from content", *result.Messages[0].Content.Content)
	})
}

func TestConvertToLLMRequestComprehensive_AssistantMessage(t *testing.T) {
	t.Run("simple assistant text message", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{Type: "text", Text: "Hello, human!", State: "done"},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "assistant", result.Messages[0].Role)
		require.Len(t, result.Messages[0].Content.MultipleContent, 1)
		assert.Equal(t, "text", result.Messages[0].Content.MultipleContent[0].Type)
		assert.Equal(t, "Hello, human!", *result.Messages[0].Content.MultipleContent[0].Text)
	})

	t.Run("assistant message with reasoning", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{Type: "reasoning", Text: "Thinking...", State: "done"},
						{Type: "text", Text: "Hello, human!", State: "done"},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "assistant", result.Messages[0].Role)
		require.Len(t, result.Messages[0].Content.MultipleContent, 2)

		// Check reasoning part (mapped to text)
		reasoningPart := result.Messages[0].Content.MultipleContent[0]
		assert.Equal(t, "text", reasoningPart.Type)
		assert.Equal(t, "Thinking...", *reasoningPart.Text)

		// Check text part
		textPart := result.Messages[0].Content.MultipleContent[1]
		assert.Equal(t, "text", textPart.Type)
		assert.Equal(t, "Hello, human!", *textPart.Text)
	})

	t.Run("assistant message with file parts", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{
							Type:      "file",
							MediaType: "image/png",
							URL:       "data:image/png;base64,dGVzdA==",
						},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "assistant", result.Messages[0].Role)
		require.Len(t, result.Messages[0].Content.MultipleContent, 1)

		imagePart := result.Messages[0].Content.MultipleContent[0]
		assert.Equal(t, "image_url", imagePart.Type)
		require.NotNil(t, imagePart.ImageURL)
		assert.Equal(t, "data:image/png;base64,dGVzdA==", imagePart.ImageURL.URL)
	})

	t.Run("assistant message from content string", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role:    "assistant",
					Content: "Hello from content",
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)

		assert.Equal(t, "assistant", result.Messages[0].Role)
		assert.Equal(t, "Hello from content", *result.Messages[0].Content.Content)
	})
}

func TestConvertToLLMRequestComprehensive_ToolCalls(t *testing.T) {
	t.Run("assistant message with tool output available", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{Type: "step-start"},
						{Type: "text", Text: "Let me calculate that for you.", State: "done"},
						{
							Type:       "tool-calculator",
							State:      "output-available",
							ToolCallID: "call1",
							Input:      map[string]interface{}{"operation": "add", "numbers": []int{1, 2}},
							Output:     "3",
						},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 2) // assistant + tool

		// Check assistant message
		assistantMsg := result.Messages[0]
		assert.Equal(t, "assistant", assistantMsg.Role)
		require.Len(t, assistantMsg.Content.MultipleContent, 1)
		assert.Equal(t, "Let me calculate that for you.", *assistantMsg.Content.MultipleContent[0].Text)

		// Check tool calls
		require.Len(t, assistantMsg.ToolCalls, 1)
		toolCall := assistantMsg.ToolCalls[0]
		assert.Equal(t, "call1", toolCall.ID)
		assert.Equal(t, "function", toolCall.Type)
		assert.Equal(t, "calculator", toolCall.Function.Name)

		// Check tool result message
		toolMsg := result.Messages[1]
		assert.Equal(t, "tool", toolMsg.Role)
		assert.Equal(t, "call1", *toolMsg.ToolCallID)
		assert.Equal(t, "3", *toolMsg.Content.Content)
	})

	t.Run("assistant message with tool output error", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{Type: "step-start"},
						{Type: "text", Text: "Let me calculate that for you.", State: "done"},
						{
							Type:       "tool-calculator",
							State:      "output-error",
							ToolCallID: "call1",
							Input:      map[string]interface{}{"operation": "add", "numbers": []int{1, 2}},
							ErrorText:  "Error: Invalid input",
						},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 2) // assistant + tool

		// Check assistant message
		assistantMsg := result.Messages[0]
		assert.Equal(t, "assistant", assistantMsg.Role)
		require.Len(t, assistantMsg.ToolCalls, 1)

		// Check tool result message with error
		toolMsg := result.Messages[1]
		assert.Equal(t, "tool", toolMsg.Role)
		assert.Equal(t, "call1", *toolMsg.ToolCallID)
		assert.Equal(t, "Error: Invalid input", *toolMsg.Content.Content)
	})

	t.Run("assistant message with tool output error using raw input", func(t *testing.T) {
		rawInput := json.RawMessage(`{"operation": "add", "numbers": [1, 2]}`)
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{Type: "step-start"},
						{Type: "text", Text: "Let me calculate that for you.", State: "done"},
						{
							Type:       "tool-calculator",
							State:      "output-error",
							ToolCallID: "call1",
							RawInput:   rawInput,
							ErrorText:  "Error: Invalid input",
						},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 2)

		// Check tool call uses raw input
		assistantMsg := result.Messages[0]
		require.Len(t, assistantMsg.ToolCalls, 1)
		toolCall := assistantMsg.ToolCalls[0]
		// JSON order may vary, so check the content rather than exact string
		var args map[string]interface{}

		err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		require.NoError(t, err)
		assert.Equal(t, "add", args["operation"])
		assert.Equal(t, []interface{}{float64(1), float64(2)}, args["numbers"])
	})

	t.Run("dynamic tool", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{Type: "step-start"},
						{
							Type:       "dynamic-tool",
							ToolName:   "custom-calculator",
							State:      "output-available",
							ToolCallID: "call1",
							Input:      map[string]interface{}{"value": 42},
							Output:     "result",
						},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 2)

		// Check tool call
		assistantMsg := result.Messages[0]
		require.Len(t, assistantMsg.ToolCalls, 1)
		toolCall := assistantMsg.ToolCalls[0]
		assert.Equal(t, "call1", toolCall.ID)
		assert.Equal(t, "custom-calculator", toolCall.Function.Name)
	})

	t.Run("multiple tool invocations with step information", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{Type: "step-start"},
						{Type: "text", Text: "response", State: "done"},
						{
							Type:       "tool-screenshot",
							State:      "output-available",
							ToolCallID: "call-1",
							Input:      map[string]interface{}{"value": "value-1"},
							Output:     "result-1",
						},
						{Type: "step-start"},
						{
							Type:       "tool-screenshot",
							State:      "output-available",
							ToolCallID: "call-2",
							Input:      map[string]interface{}{"value": "value-2"},
							Output:     "result-2",
						},
						{
							Type:       "tool-screenshot",
							State:      "output-available",
							ToolCallID: "call-3",
							Input:      map[string]interface{}{"value": "value-3"},
							Output:     "result-3",
						},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		// Expect multiple messages due to step separation
		require.True(t, len(result.Messages) >= 4)

		// First block: text + tool-1
		assert.Equal(t, "assistant", result.Messages[0].Role)
		require.Len(t, result.Messages[0].ToolCalls, 1)
		assert.Equal(t, "call-1", result.Messages[0].ToolCalls[0].ID)

		assert.Equal(t, "tool", result.Messages[1].Role)
		assert.Equal(t, "call-1", *result.Messages[1].ToolCallID)

		// Second block: tool-2 + tool-3
		assert.Equal(t, "assistant", result.Messages[2].Role)
		require.Len(t, result.Messages[2].ToolCalls, 2)
		assert.Equal(t, "call-2", result.Messages[2].ToolCalls[0].ID)
		assert.Equal(t, "call-3", result.Messages[2].ToolCalls[1].ID)

		// Should have tool messages for the results
		toolMsgCount := 0

		for i := 3; i < len(result.Messages); i++ {
			if result.Messages[i].Role == "tool" {
				toolMsgCount++
			}
		}

		assert.Equal(t, 2, toolMsgCount) // Two tool results
	})
}

func TestConvertToLLMRequestComprehensive_IgnoreIncompleteToolCalls(t *testing.T) {
	t.Run("filters incomplete tool calls", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{Type: "step-start"},
						{
							Type:       "tool-screenshot",
							State:      "output-available",
							ToolCallID: "call-1",
							Input:      map[string]interface{}{"value": "value-1"},
							Output:     "result-1",
						},
						{Type: "step-start"},
						{
							Type:       "tool-screenshot",
							State:      "input-streaming",
							ToolCallID: "call-2",
							Input:      map[string]interface{}{"value": "value-2"},
						},
						{
							Type:       "tool-screenshot",
							State:      "input-available",
							ToolCallID: "call-3",
							Input:      map[string]interface{}{"value": "value-3"},
						},
						{
							Type:       "dynamic-tool",
							ToolName:   "tool-screenshot2",
							State:      "input-available",
							ToolCallID: "call-4",
							Input:      map[string]interface{}{"value": "value-4"},
						},
						{Type: "text", Text: "response", State: "done"},
					},
				},
			},
		}

		options := &ConvertToLLMRequestOptions{
			IgnoreIncompleteToolCalls: true,
		}

		result, err := convertToLLMRequestWithOptions(req, options)
		require.NoError(t, err)

		// Should have completed tool call and text, incomplete ones filtered out
		require.True(t, len(result.Messages) >= 2)

		// Find the assistant message with the completed tool call
		var assistantMsg *llm.Message

		for i := range result.Messages {
			if result.Messages[i].Role == "assistant" && len(result.Messages[i].ToolCalls) > 0 {
				assistantMsg = &result.Messages[i]
				break
			}
		}

		require.NotNil(t, assistantMsg)
		require.Len(t, assistantMsg.ToolCalls, 1)
		assert.Equal(t, "call-1", assistantMsg.ToolCalls[0].ID)

		// Should have text content in some assistant message
		textFound := false

		for _, msg := range result.Messages {
			if msg.Role == "assistant" && len(msg.Content.MultipleContent) > 0 {
				for _, part := range msg.Content.MultipleContent {
					if part.Type == "text" && part.Text != nil && *part.Text == "response" {
						textFound = true
						break
					}
				}
			}
		}

		assert.True(t, textFound, "Should find 'response' text in assistant messages")
	})
}

func TestConvertToLLMRequestComprehensive_Tools(t *testing.T) {
	t.Run("converts tools", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "user",
					Parts: []UIMessagePart{
						{Type: "text", Text: "Hello"},
					},
				},
			},
			Tools: []Tool{
				{
					Type: "function",
					Function: Function{
						Name:        "calculator",
						Description: "Perform calculations",
						Parameters: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"operation": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)

		require.Len(t, result.Tools, 1)
		tool := result.Tools[0]
		assert.Equal(t, "function", tool.Type)
		assert.Equal(t, "calculator", tool.Function.Name)
		assert.Equal(t, "Perform calculations", tool.Function.Description)
		assert.NotNil(t, tool.Function.Parameters)
	})
}

func TestConvertToLLMRequestComprehensive_MultipleMessages(t *testing.T) {
	t.Run("handles conversation with multiple messages", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "user",
					Parts: []UIMessagePart{
						{Type: "text", Text: "What's the weather like?"},
					},
				},
				{
					Role: "assistant",
					Parts: []UIMessagePart{
						{Type: "text", Text: "I'll check that for you.", State: "done"},
					},
				},
				{
					Role: "user",
					Parts: []UIMessagePart{
						{Type: "text", Text: "Thanks!"},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 3)

		assert.Equal(t, "user", result.Messages[0].Role)
		assert.Equal(t, "assistant", result.Messages[1].Role)
		assert.Equal(t, "user", result.Messages[2].Role)
	})
}

func TestConvertToLLMRequestComprehensive_ErrorHandling(t *testing.T) {
	t.Run("throws error for unsupported role", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4",
			Messages: []UIMessage{
				{
					Role: "unknown",
					Parts: []UIMessagePart{
						{Type: "text", Text: "unknown role message"},
					},
				},
			},
		}

		_, err := convertToLLMRequest(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported role: unknown")
	})
}

func TestConvertToLLMRequestComprehensive_PreservesRequestFields(t *testing.T) {
	t.Run("preserves model and stream settings", func(t *testing.T) {
		stream := true
		req := &Request{
			Model:  "gpt-4-turbo",
			Stream: &stream,
			Messages: []UIMessage{
				{
					Role: "user",
					Parts: []UIMessagePart{
						{Type: "text", Text: "Hello"},
					},
				},
			},
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)

		assert.Equal(t, "gpt-4-turbo", result.Model)
		assert.Equal(t, &stream, result.Stream)
	})
}
