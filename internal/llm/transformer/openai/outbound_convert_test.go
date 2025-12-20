package openai

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

func TestRequestFromLLM(t *testing.T) {
	tests := []struct {
		name     string
		llmReq   *llm.Request
		validate func(*testing.T, *Request)
	}{
		{
			name:   "nil request",
			llmReq: nil,
			validate: func(t *testing.T, req *Request) {
				require.Nil(t, req)
			},
		},
		{
			name: "basic request",
			llmReq: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello there!"),
						},
					},
				},
				Stream: lo.ToPtr(true),
			},
			validate: func(t *testing.T, req *Request) {
				require.NotNil(t, req)
				require.Equal(t, "gpt-4", req.Model)
				require.Len(t, req.Messages, 1)
				require.Equal(t, "assistant", req.Messages[0].Role)
				require.True(t, *req.Stream)
			},
		},
		{
			name: "request with helper fields stripped",
			llmReq: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role:         "tool",
						ToolCallID:   lo.ToPtr("call_123"),
						MessageIndex: lo.ToPtr(1), // Helper field - should not be in OpenAI model
						Content:      llm.MessageContent{Content: lo.ToPtr("result")},
					},
				},
				RawAPIFormat: llm.APIFormatOpenAIChatCompletion, // Helper field
			},
			validate: func(t *testing.T, req *Request) {
				require.NotNil(t, req)
				require.Equal(t, "call_123", *req.Messages[0].ToolCallID)
				// OpenAI Request doesn't have MessageIndex or RawAPIFormat fields
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RequestFromLLM(tt.llmReq)
			tt.validate(t, result)
		})
	}
}

func TestResponse_ToLLMResponse(t *testing.T) {
	tests := []struct {
		name     string
		oaiResp  *Response
		validate func(*testing.T, *llm.Response)
	}{
		{
			name:    "nil response",
			oaiResp: nil,
			validate: func(t *testing.T, resp *llm.Response) {
				require.Nil(t, resp)
			},
		},
		{
			name: "basic response",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Message: &Message{
							Role:    "assistant",
							Content: MessageContent{Content: lo.ToPtr("Hello!")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Equal(t, "chatcmpl-123", resp.ID)
				require.Equal(t, "chat.completion", resp.Object)
				require.Len(t, resp.Choices, 1)
				require.Equal(t, "Hello!", *resp.Choices[0].Message.Content.Content)
				require.Equal(t, "stop", *resp.Choices[0].FinishReason)
			},
		},
		{
			name: "streaming response with delta",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Delta: &Message{
							Content: MessageContent{Content: lo.ToPtr("chunk")},
						},
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Equal(t, "chat.completion.chunk", resp.Object)
				require.NotNil(t, resp.Choices[0].Delta)
				require.Equal(t, "chunk", *resp.Choices[0].Delta.Content.Content)
			},
		},
		{
			name: "response with usage",
			oaiResp: &Response{
				ID:     "chatcmpl-123",
				Object: "chat.completion",
				Model:  "gpt-4",
				Choices: []Choice{
					{Index: 0, Message: &Message{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("Hi")}}},
				},
				Usage: &Usage{
					Usage: llm.Usage{
						PromptTokens:     10,
						CompletionTokens: 5,
						TotalTokens:      15,
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.NotNil(t, resp.Usage)
				require.Equal(t, int64(10), resp.Usage.PromptTokens)
				require.Equal(t, int64(5), resp.Usage.CompletionTokens)
				require.Equal(t, int64(15), resp.Usage.TotalTokens)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.oaiResp.ToLLMResponse()
			tt.validate(t, result)
		})
	}
}
