package anthropic

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/axon/agent"
)

func TestConvertMessages(t *testing.T) {
	t.Run("empty messages", func(t *testing.T) {
		system, params := convertMessages(nil)

		assert.Nil(t, system)
		assert.Nil(t, params)

		system, params = convertMessages([]agent.Message{})

		assert.Nil(t, system)
		assert.Nil(t, params)
	})

	t.Run("system message with text", func(t *testing.T) {
		text := "You are a helpful assistant."
		messages := []agent.Message{
			{Role: agent.RoleSystem, Content: &agent.Content{Text: &text}},
		}

		system, params := convertMessages(messages)

		require.Len(t, system, 1)
		assert.Equal(t, text, system[0].Text)
		assert.Nil(t, params)
	})

	t.Run("system message with parts", func(t *testing.T) {
		messages := []agent.Message{
			{
				Role: agent.RoleSystem,
				Content: &agent.Content{Parts: []agent.ContentPart{
					{Type: agent.ContentPartText, Text: "Part 1 "},
					{Type: agent.ContentPartText, Text: "Part 2"},
				}},
			},
		}

		system, params := convertMessages(messages)

		require.Len(t, system, 2)
		assert.Equal(t, "Part 1 ", system[0].Text)
		assert.Equal(t, "Part 2", system[1].Text)
		assert.Nil(t, params)
	})

	t.Run("system message with nil content", func(t *testing.T) {
		messages := []agent.Message{
			{Role: agent.RoleSystem, Content: nil},
		}

		system, params := convertMessages(messages)

		assert.Nil(t, system)
		assert.Nil(t, params)
	})

	t.Run("system message with mixed parts", func(t *testing.T) {
		messages := []agent.Message{
			{
				Role: agent.RoleSystem,
				Content: &agent.Content{Parts: []agent.ContentPart{
					{Type: agent.ContentPartText, Text: "Text part"},
					{Type: agent.ContentPartImage, Text: "Should be ignored"},
				}},
			},
		}

		system, params := convertMessages(messages)

		require.Len(t, system, 1)
		assert.Equal(t, "Text part", system[0].Text)
		assert.Nil(t, params)
	})

	t.Run("user message with text", func(t *testing.T) {
		text := "Hello"
		messages := []agent.Message{
			{Role: agent.RoleUser, Content: &agent.Content{Text: &text}},
		}

		system, params := convertMessages(messages)

		assert.Nil(t, system)
		require.Len(t, params, 1)
	})

	t.Run("user message with parts", func(t *testing.T) {
		messages := []agent.Message{
			{
				Role: agent.RoleUser,
				Content: &agent.Content{Parts: []agent.ContentPart{
					{Type: agent.ContentPartText, Text: "Hello"},
				}},
			},
		}

		system, params := convertMessages(messages)

		assert.Nil(t, system)
		require.Len(t, params, 1)
	})

	t.Run("assistant message with text", func(t *testing.T) {
		text := "Hi there"
		messages := []agent.Message{
			{Role: agent.RoleAssistant, Content: &agent.Content{Text: &text}},
		}

		system, params := convertMessages(messages)

		assert.Nil(t, system)
		require.Len(t, params, 1)
	})

	t.Run("assistant message with tool use", func(t *testing.T) {
		messages := []agent.Message{
			{
				Role: agent.RoleAssistant,
				ToolUse: &agent.ToolUse{
					ID:    "tool_123",
					Name:  "get_weather",
					Input: `{"location": "Tokyo"}`,
				},
			},
		}

		system, params := convertMessages(messages)

		assert.Nil(t, system)
		require.Len(t, params, 1)
	})

	t.Run("assistant messages with same request index are aggregated", func(t *testing.T) {
		t1 := "A1"
		t2 := "A2"
		messages := []agent.Message{
			{Role: agent.RoleAssistant, Content: &agent.Content{Text: &t1}, RequestIndex: 7},
			{Role: agent.RoleAssistant, Content: &agent.Content{Text: &t2}, RequestIndex: 7},
		}

		system, params := convertMessages(messages)

		assert.Nil(t, system)
		require.Len(t, params, 1)
	})

	t.Run("tool result message", func(t *testing.T) {
		content := "Tool result content"
		toolUseID := "tool_123"
		messages := []agent.Message{
			{
				Role:      agent.RoleTool,
				Content:   &agent.Content{Text: &content},
				ToolUseID: &toolUseID,
			},
		}

		system, params := convertMessages(messages)

		assert.Nil(t, system)
		require.Len(t, params, 1)
	})

	t.Run("tool result message with error", func(t *testing.T) {
		content := "Error occurred"
		toolUseID := "tool_123"
		isError := true
		messages := []agent.Message{
			{
				Role:      agent.RoleTool,
				Content:   &agent.Content{Text: &content},
				ToolUseID: &toolUseID,
				IsError:   &isError,
			},
		}

		system, params := convertMessages(messages)

		assert.Nil(t, system)
		require.Len(t, params, 1)
	})

	t.Run("tool result message with nil content", func(t *testing.T) {
		toolUseID := "tool_123"
		messages := []agent.Message{
			{
				Role:      agent.RoleTool,
				Content:   nil,
				ToolUseID: &toolUseID,
			},
		}

		system, params := convertMessages(messages)

		assert.Nil(t, system)
		require.Len(t, params, 1)
	})

	t.Run("mixed messages", func(t *testing.T) {
		systemText := "System prompt"
		userText := "Hello"
		assistantText := "Hi"
		toolUseID := "tool_123"
		toolContent := "Result"

		messages := []agent.Message{
			{Role: agent.RoleSystem, Content: &agent.Content{Text: &systemText}},
			{Role: agent.RoleUser, Content: &agent.Content{Text: &userText}},
			{Role: agent.RoleAssistant, Content: &agent.Content{Text: &assistantText}},
			{Role: agent.RoleTool, Content: &agent.Content{Text: &toolContent}, ToolUseID: &toolUseID},
		}

		system, params := convertMessages(messages)

		require.Len(t, system, 1)
		assert.Equal(t, systemText, system[0].Text)
		require.Len(t, params, 3)
	})
}

func TestContentToBlocks(t *testing.T) {
	t.Run("nil content", func(t *testing.T) {
		msg := agent.Message{Content: nil}
		blocks := contentToBlocks(msg)

		assert.Nil(t, blocks)
	})

	t.Run("text content", func(t *testing.T) {
		text := "Hello world"
		msg := agent.Message{Content: &agent.Content{Text: &text}}
		blocks := contentToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("parts with text", func(t *testing.T) {
		msg := agent.Message{
			Content: &agent.Content{Parts: []agent.ContentPart{
				{Type: agent.ContentPartText, Text: "Hello"},
			}},
		}
		blocks := contentToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("parts with image URL", func(t *testing.T) {
		msg := agent.Message{
			Content: &agent.Content{Parts: []agent.ContentPart{
				{Type: agent.ContentPartImage, URL: "https://example.com/image.png"},
			}},
		}
		blocks := contentToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("parts with image base64", func(t *testing.T) {
		msg := agent.Message{
			Content: &agent.Content{Parts: []agent.ContentPart{
				{
					Type:     agent.ContentPartImage,
					Data:     "base64data",
					MimeType: "image/png",
				},
			}},
		}
		blocks := contentToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("mixed parts", func(t *testing.T) {
		msg := agent.Message{
			Content: &agent.Content{Parts: []agent.ContentPart{
				{Type: agent.ContentPartText, Text: "Text"},
				{Type: agent.ContentPartImage, URL: "https://example.com/image.png"},
				{Type: "unknown", Text: "Fallback"},
			}},
		}
		blocks := contentToBlocks(msg)

		require.Len(t, blocks, 3)
	})
}

func TestAssistantToBlocks(t *testing.T) {
	t.Run("empty message", func(t *testing.T) {
		msg := agent.Message{}
		blocks := assistantToBlocks(msg)

		assert.Nil(t, blocks)
	})

	t.Run("tool use only", func(t *testing.T) {
		msg := agent.Message{
			ToolUse: &agent.ToolUse{
				ID:    "tool_123",
				Name:  "get_weather",
				Input: `{"location": "Tokyo"}`,
			},
		}
		blocks := assistantToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("tool use with empty input", func(t *testing.T) {
		msg := agent.Message{
			ToolUse: &agent.ToolUse{
				ID:    "tool_123",
				Name:  "empty_tool",
				Input: "",
			},
		}
		blocks := assistantToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("tool use with invalid JSON input", func(t *testing.T) {
		msg := agent.Message{
			ToolUse: &agent.ToolUse{
				ID:    "tool_123",
				Name:  "invalid_tool",
				Input: `not valid json`,
			},
		}
		blocks := assistantToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("text content", func(t *testing.T) {
		text := "Hello"
		msg := agent.Message{
			Content: &agent.Content{Text: &text},
		}
		blocks := assistantToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("parts with text", func(t *testing.T) {
		msg := agent.Message{
			Content: &agent.Content{Parts: []agent.ContentPart{
				{Type: agent.ContentPartText, Text: "Text part"},
			}},
		}
		blocks := assistantToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("parts with thinking", func(t *testing.T) {
		msg := agent.Message{
			Content: &agent.Content{Parts: []agent.ContentPart{
				{
					Type:              agent.ContentPartThinking,
					Thinking:          "Thinking content",
					ThinkingSignature: "sig_123",
				},
			}},
		}
		blocks := assistantToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("parts with redacted thinking", func(t *testing.T) {
		msg := agent.Message{
			Content: &agent.Content{Parts: []agent.ContentPart{
				{
					Type: agent.ContentPartRedactedThinking,
					Data: "redacted_data",
				},
			}},
		}
		blocks := assistantToBlocks(msg)

		require.Len(t, blocks, 1)
	})

	t.Run("combined tool use and content", func(t *testing.T) {
		text := "Let me help you."
		msg := agent.Message{
			ToolUse: &agent.ToolUse{
				ID:    "tool_123",
				Name:  "get_weather",
				Input: `{"location": "Tokyo"}`,
			},
			Content: &agent.Content{Text: &text},
		}
		blocks := assistantToBlocks(msg)

		require.Len(t, blocks, 2)
	})

	t.Run("combined tool use and parts", func(t *testing.T) {
		msg := agent.Message{
			ToolUse: &agent.ToolUse{
				ID:    "tool_123",
				Name:  "get_weather",
				Input: `{"location": "Tokyo"}`,
			},
			Content: &agent.Content{Parts: []agent.ContentPart{
				{Type: agent.ContentPartText, Text: "Text"},
				{Type: agent.ContentPartThinking, Thinking: "Thinking"},
			}},
		}
		blocks := assistantToBlocks(msg)

		require.Len(t, blocks, 3)
	})
}

func TestConvertTools(t *testing.T) {
	t.Run("empty tools", func(t *testing.T) {
		params := convertTools(nil)
		assert.Len(t, params, 0)

		params = convertTools([]agent.ToolDefinition{})
		assert.Len(t, params, 0)
	})

	t.Run("single tool", func(t *testing.T) {
		tools := []agent.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get weather info",
				Parameters: jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"location": {
							Type: "string",
						},
					},
					Required: []string{"location"},
				},
			},
		}

		params := convertTools(tools)

		require.Len(t, params, 1)
	})

	t.Run("multiple tools", func(t *testing.T) {
		tools := []agent.ToolDefinition{
			{
				Name:        "tool1",
				Description: "First tool",
				Parameters: jsonschema.Schema{
					Type:       "object",
					Properties: map[string]*jsonschema.Schema{},
				},
			},
			{
				Name:        "tool2",
				Description: "Second tool",
				Parameters: jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"param": {
							Type: "string",
						},
					},
				},
			},
		}

		params := convertTools(tools)

		require.Len(t, params, 2)
	})

	t.Run("tool with empty parameters", func(t *testing.T) {
		tools := []agent.ToolDefinition{
			{
				Name:        "empty_tool",
				Description: "Tool with no params",
				Parameters:  jsonschema.Schema{},
			},
		}

		params := convertTools(tools)

		require.Len(t, params, 1)
	})

	t.Run("tool with complex parameters", func(t *testing.T) {
		tools := []agent.ToolDefinition{
			{
				Name:        "complex_tool",
				Description: "Complex tool",
				Parameters: jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"nested": {
							Type: "object",
							Properties: map[string]*jsonschema.Schema{
								"inner": {
									Type: "string",
								},
							},
						},
						"array": {
							Type:  "array",
							Items: &jsonschema.Schema{Type: "integer"},
						},
					},
					Required: []string{"nested"},
				},
			},
		}

		params := convertTools(tools)

		require.Len(t, params, 1)
	})
}

func TestConvertStopReason(t *testing.T) {
	tests := []struct {
		name     string
		input    anthropic.StopReason
		expected agent.StopReason
	}{
		{"end_turn", anthropic.StopReasonEndTurn, agent.StopReasonEndTurn},
		{"tool_use", anthropic.StopReasonToolUse, agent.StopReasonToolUse},
		{"max_tokens", anthropic.StopReasonMaxTokens, agent.StopReasonMaxTokens},
		{"unknown", anthropic.StopReason("unknown"), agent.StopReasonEndTurn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertStopReason(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
