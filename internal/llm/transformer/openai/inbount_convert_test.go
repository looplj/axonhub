package openai

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

func TestRequest_ToLLMRequest(t *testing.T) {
	tests := []struct {
		name     string
		oaiReq   *Request
		validate func(*testing.T, *llm.Request)
	}{
		{
			name:   "nil request",
			oaiReq: nil,
			validate: func(t *testing.T, req *llm.Request) {
				require.Nil(t, req)
			},
		},
		{
			name: "basic request",
			oaiReq: &Request{
				Model: "gpt-4",
				Messages: []Message{
					{
						Role: "user",
						Content: MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				Temperature: lo.ToPtr(0.7),
				MaxTokens:   lo.ToPtr(int64(100)),
			},
			validate: func(t *testing.T, req *llm.Request) {
				require.NotNil(t, req)
				require.Equal(t, "gpt-4", req.Model)
				require.Len(t, req.Messages, 1)
				require.Equal(t, "user", req.Messages[0].Role)
				require.Equal(t, "Hello", *req.Messages[0].Content.Content)
				require.Equal(t, 0.7, *req.Temperature)
				require.Equal(t, int64(100), *req.MaxTokens)
			},
		},
		{
			name: "request with tools",
			oaiReq: &Request{
				Model: "gpt-4",
				Messages: []Message{
					{Role: "user", Content: MessageContent{Content: lo.ToPtr("Call a function")}},
				},
				Tools: []Tool{
					{
						Type: "function",
						Function: Function{
							Name:        "get_weather",
							Description: "Get weather info",
							Parameters:  json.RawMessage(`{"type":"object"}`),
						},
					},
				},
				ToolChoice: &ToolChoice{
					ToolChoice: lo.ToPtr("auto"),
				},
			},
			validate: func(t *testing.T, req *llm.Request) {
				require.NotNil(t, req)
				require.Len(t, req.Tools, 1)
				require.Equal(t, "function", req.Tools[0].Type)
				require.Equal(t, "get_weather", req.Tools[0].Function.Name)
				require.NotNil(t, req.ToolChoice)
				require.Equal(t, "auto", *req.ToolChoice.ToolChoice)
			},
		},
		{
			name: "request with stop sequences",
			oaiReq: &Request{
				Model:    "gpt-4",
				Messages: []Message{{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hi")}}},
				Stop: &Stop{
					MultipleStop: []string{"END", "STOP"},
				},
			},
			validate: func(t *testing.T, req *llm.Request) {
				require.NotNil(t, req)
				require.NotNil(t, req.Stop)
				require.Equal(t, []string{"END", "STOP"}, req.Stop.MultipleStop)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.oaiReq.ToLLMRequest()
			tt.validate(t, result)
		})
	}
}

func TestResponseFromLLM(t *testing.T) {
	tests := []struct {
		name     string
		llmResp  *llm.Response
		validate func(*testing.T, *Response)
	}{
		{
			name:    "nil response",
			llmResp: nil,
			validate: func(t *testing.T, resp *Response) {
				require.Nil(t, resp)
			},
		},
		{
			name: "basic response",
			llmResp: &llm.Response{
				ID:      "chatcmpl-456",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role:    "assistant",
							Content: llm.MessageContent{Content: lo.ToPtr("Response text")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validate: func(t *testing.T, resp *Response) {
				require.NotNil(t, resp)
				require.Equal(t, "chatcmpl-456", resp.ID)
				require.Len(t, resp.Choices, 1)
				require.Equal(t, "Response text", *resp.Choices[0].Message.Content.Content)
			},
		},
		{
			name: "response with transformer metadata stripped",
			llmResp: &llm.Response{
				ID:     "chatcmpl-789",
				Object: "chat.completion",
				Model:  "gpt-4",
				Choices: []llm.Choice{
					{
						Index:               0,
						Message:             &llm.Message{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("test")}},
						TransformerMetadata: map[string]any{"key": "value"}, // Should not be in OpenAI model
					},
				},
				TransformerMetadata: map[string]any{"another": "data"}, // Should not be in OpenAI model
			},
			validate: func(t *testing.T, resp *Response) {
				require.NotNil(t, resp)
				require.Equal(t, "chatcmpl-789", resp.ID)
				// OpenAI Response doesn't have TransformerMetadata fields
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResponseFromLLM(tt.llmResp)
			tt.validate(t, result)
		})
	}
}

func TestMessage_ToLLMMessage(t *testing.T) {
	tests := []struct {
		name     string
		msg      Message
		validate func(*testing.T, llm.Message)
	}{
		{
			name: "simple text message",
			msg: Message{
				Role:    "user",
				Content: MessageContent{Content: lo.ToPtr("Hello")},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "user", msg.Role)
				require.Equal(t, "Hello", *msg.Content.Content)
			},
		},
		{
			name: "message with tool calls",
			msg: Message{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{
						ID:       "call_123",
						Type:     "function",
						Function: FunctionCall{Name: "get_weather", Arguments: `{"city":"NYC"}`},
						Index:    0,
					},
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Len(t, msg.ToolCalls, 1)
				require.Equal(t, "call_123", msg.ToolCalls[0].ID)
				require.Equal(t, "get_weather", msg.ToolCalls[0].Function.Name)
			},
		},
		{
			name: "tool response message",
			msg: Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_123"),
				Content:    MessageContent{Content: lo.ToPtr(`{"temp":72}`)},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "tool", msg.Role)
				require.Equal(t, "call_123", *msg.ToolCallID)
			},
		},
		{
			name: "message with reasoning content",
			msg: Message{
				Role:             "assistant",
				Content:          MessageContent{Content: lo.ToPtr("Final answer")},
				ReasoningContent: lo.ToPtr("Let me think..."),
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "Final answer", *msg.Content.Content)
				require.Equal(t, "Let me think...", *msg.ReasoningContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.msg.ToLLMMessage()
			tt.validate(t, result)
		})
	}
}
