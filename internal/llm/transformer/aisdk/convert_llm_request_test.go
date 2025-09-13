package aisdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertToLLMRequest_SystemMessage(t *testing.T) {
	// system with parts text (concatenate)
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

	res, err := convertToLLMRequest(req)
	require.NoError(t, err)
	require.Equal(t, "gpt-4", res.Model)
	require.Len(t, res.Messages, 1)
	require.Equal(t, "system", res.Messages[0].Role)
	require.NotNil(t, res.Messages[0].Content.Content)
	require.Equal(t, "Part 1 Part 2", *res.Messages[0].Content.Content)

	// system with content string fallback
	req2 := &Request{
		Model: "gpt-4",
		Messages: []UIMessage{
			{Role: "system", Content: "System message"},
		},
	}
	res2, err := convertToLLMRequest(req2)
	require.NoError(t, err)
	require.Len(t, res2.Messages, 1)
	require.Equal(t, "System message", *res2.Messages[0].Content.Content)
}

func TestConvertToLLMRequest_UserMessage_TextAndFile(t *testing.T) {
	req := &Request{
		Model: "gpt-4",
		Messages: []UIMessage{
			{
				Role: "user",
				Parts: []UIMessagePart{
					{Type: "file", MediaType: "image/jpeg", URL: "https://example.com/image.jpg"},
					{Type: "text", Text: "Check this image"},
				},
			},
		},
	}

	res, err := convertToLLMRequest(req)
	require.NoError(t, err)
	require.Len(t, res.Messages, 1)
	m := res.Messages[0]
	require.Equal(t, "user", m.Role)
	require.Len(t, m.Content.MultipleContent, 2)

	p0 := m.Content.MultipleContent[0]
	require.Equal(t, "image_url", p0.Type)
	require.NotNil(t, p0.ImageURL)
	require.Equal(t, "https://example.com/image.jpg", p0.ImageURL.URL)

	p1 := m.Content.MultipleContent[1]
	require.Equal(t, "text", p1.Type)
	require.NotNil(t, p1.Text)
	require.Equal(t, "Check this image", *p1.Text)

	// user content string fallback
	req2 := &Request{Model: "m", Messages: []UIMessage{{Role: "user", Content: "Hello"}}}
	res2, err := convertToLLMRequest(req2)
	require.NoError(t, err)
	require.Len(t, res2.Messages, 1)
	require.Equal(t, "Hello", *res2.Messages[0].Content.Content)
}

func TestConvertToLLMRequest_Assistant_SimpleText(t *testing.T) {
	req := &Request{
		Model: "gpt-4",
		Messages: []UIMessage{
			{Role: "assistant", Content: "Hello, human!"},
		},
	}
	res, err := convertToLLMRequest(req)
	require.NoError(t, err)
	require.Len(t, res.Messages, 1)
	m := res.Messages[0]
	require.Equal(t, "assistant", m.Role)
	require.NotNil(t, m.Content.Content)
	require.Equal(t, "Hello, human!", *m.Content.Content)
}

func TestConvertToLLMRequest_Assistant_ToolCall_InputAvailable_ResultAvailable(t *testing.T) {
	req := &Request{
		Model: "gpt-4",
		Messages: []UIMessage{
			{
				Role: "assistant",
				Parts: []UIMessagePart{
					{Type: "text", Text: "Let me calculate that", State: "done"},
					{
						Type:       "tool-screenshot",
						State:      "output-available",
						ToolCallID: "call-1",
						Input: map[string]any{
							"value": "value-1",
						},
						Output: "result-1",
					},
				},
			},
		},
	}

	res, err := convertToLLMRequest(req)
	require.NoError(t, err)

	// Expect two messages: assistant (with toolCalls) then tool result message
	require.Len(t, res.Messages, 2)

	assistant := res.Messages[0]
	require.Equal(t, "assistant", assistant.Role)
	// content has text
	require.Len(t, assistant.Content.MultipleContent, 1)
	require.Equal(t, "text", assistant.Content.MultipleContent[0].Type)
	require.Equal(t, "Let me calculate that", *assistant.Content.MultipleContent[0].Text)
	// toolCalls has one function call
	require.Len(t, assistant.ToolCalls, 1)
	call := assistant.ToolCalls[0]
	require.Equal(t, "function", call.Type)
	require.Equal(t, "screenshot", call.Function.Name)
	require.JSONEq(t, `{"value":"value-1"}`, call.Function.Arguments)

	toolMsg := res.Messages[1]
	require.Equal(t, "tool", toolMsg.Role)
	require.NotNil(t, toolMsg.ToolCallID)
	require.Equal(t, "call-1", *toolMsg.ToolCallID)
	require.NotNil(t, toolMsg.Content.Content)
	require.Equal(t, "result-1", *toolMsg.Content.Content)
}

func TestConvertToLLMRequest_Assistant_ToolCall_OutputError_UsesRawInput(t *testing.T) {
	req := &Request{
		Model: "gpt-4",
		Messages: []UIMessage{
			{
				Role: "assistant",
				Parts: []UIMessagePart{
					{Type: "text", Text: "Let me calculate that", State: "done"},
					{
						Type:       "tool-calculator",
						State:      "output-error",
						ToolCallID: "call-err",
						ErrorText:  "Error: Invalid input",
						RawInput:   mustJSON(map[string]any{"operation": "add", "numbers": []int{1, 2}}),
					},
				},
			},
		},
	}

	res, err := convertToLLMRequest(req)
	require.NoError(t, err)
	require.Len(t, res.Messages, 2)

	assistant := res.Messages[0]
	require.Len(t, assistant.ToolCalls, 1)
	call := assistant.ToolCalls[0]
	require.Equal(t, "calculator", call.Function.Name)
	require.JSONEq(t, `{"operation":"add","numbers":[1,2]}`, call.Function.Arguments)

	toolMsg := res.Messages[1]
	require.Equal(t, "tool", toolMsg.Role)
	require.Equal(t, "Error: Invalid input", *toolMsg.Content.Content)
}

func TestConvertToLLMRequest_Assistant_StepBlocks_Multiple(t *testing.T) {
	req := &Request{
		Model: "gpt-4",
		Messages: []UIMessage{
			{
				Role: "assistant",
				Parts: []UIMessagePart{
					{Type: "step-start"},
					{Type: "text", Text: "response", State: "done"},
					{Type: "tool-screenshot", State: "output-available", ToolCallID: "call-1", Input: map[string]any{"value": "value-1"}, Output: "result-1"},
					{Type: "step-start"},
					{Type: "tool-screenshot", State: "output-available", ToolCallID: "call-2", Input: map[string]any{"value": "value-2"}, Output: "result-2"},
				},
			},
		},
	}

	res, err := convertToLLMRequest(req)
	require.NoError(t, err)
	// Expect: block1 => assistant + tool, block2 => assistant (just tool-call) + tool
	require.Len(t, res.Messages, 4)

	// block1 assistant
	m0 := res.Messages[0]
	require.Equal(t, "assistant", m0.Role)
	require.Len(t, m0.Content.MultipleContent, 1)
	require.Equal(t, "response", *m0.Content.MultipleContent[0].Text)
	require.Len(t, m0.ToolCalls, 1)

	// block1 tool
	m1 := res.Messages[1]
	require.Equal(t, "tool", m1.Role)
	// block2 assistant
	m2 := res.Messages[2]
	require.Equal(t, "assistant", m2.Role)
	require.Len(t, m2.ToolCalls, 1)
	// block2 tool
	m3 := res.Messages[3]
	require.Equal(t, "tool", m3.Role)
}

func TestConvertToLLMRequest_ToolsMapping(t *testing.T) {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{"type": "string"},
		},
		"required": []string{"location"},
	}
	req := &Request{
		Model:    "gpt-4",
		Messages: []UIMessage{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Type: "function", Function: Function{Name: "get_weather", Description: "Get current weather", Parameters: params}},
		},
	}

	res, err := convertToLLMRequest(req)
	require.NoError(t, err)
	require.Len(t, res.Tools, 1)
	tool := res.Tools[0]
	require.Equal(t, "function", tool.Type)
	require.Equal(t, "get_weather", tool.Function.Name)
	// parameters should be marshaled to json.RawMessage
	require.True(t, len(tool.Function.Parameters) > 0)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(tool.Function.Parameters, &decoded))
	require.Equal(t, "object", decoded["type"])
}

func TestConvertToLLMRequest_UnsupportedRole_Error(t *testing.T) {
	req := &Request{
		Model:    "gpt-4",
		Messages: []UIMessage{{Role: "unknown", Parts: []UIMessagePart{{Type: "text", Text: "msg"}}}},
	}
	res, err := convertToLLMRequest(req)
	require.Error(t, err)
	require.Nil(t, res)
	require.Contains(t, err.Error(), "unsupported role: unknown")
}

func TestConvertToLLMRequest_StreamFlag(t *testing.T) {
	stream := true
	req := &Request{
		Model:    "gpt-4",
		Stream:   &stream,
		Messages: []UIMessage{{Role: "user", Content: "hello"}},
	}
	res, err := convertToLLMRequest(req)
	require.NoError(t, err)
	require.NotNil(t, res.Stream)
	require.Equal(t, true, *res.Stream)
}

// helper to build json.RawMessage from a Go value.
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return json.RawMessage(b)
}
