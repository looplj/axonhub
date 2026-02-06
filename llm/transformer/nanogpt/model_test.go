package nanogpt

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestResponse_ToOpenAIResponse(t *testing.T) {
	tests := []struct {
		name     string
		response Response
		wantLen  int
		validate func(*testing.T, *openai.Response)
	}{
		{
			name: "empty choices",
			response: Response{
				Response: openai.Response{
					ID:      "test-1",
					Model:   "test-model",
					Choices: []openai.Choice{},
				},
				Choices: []Choice{},
			},
			wantLen: 0,
		},
		{
			name: "single choice with reasoning",
			response: Response{
				Response: openai.Response{
					ID:      "test-2",
					Model:   "test-model",
					Choices: []openai.Choice{},
				},
				Choices: []Choice{
					{
						Message: &Message{
							Reasoning: stringPtr("thinking..."),
						},
					},
				},
			},
			wantLen: 1,
			validate: func(t *testing.T, resp *openai.Response) {
				assert.Equal(t, "thinking...", *resp.Choices[0].Message.ReasoningContent)
			},
		},
		{
			name: "multiple choices",
			response: Response{
				Response: openai.Response{
					ID:      "test-3",
					Model:   "test-model",
					Choices: []openai.Choice{},
				},
				Choices: []Choice{
					{Message: &Message{Reasoning: stringPtr("reason1")}},
					{Message: &Message{Reasoning: stringPtr("reason2")}},
					{Message: &Message{Reasoning: stringPtr("reason3")}},
				},
			},
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.response.ToOpenAIResponse()
			assert.Len(t, resp.Choices, tt.wantLen)
			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

func TestChoice_ToOpenAIChoice(t *testing.T) {
	tests := []struct {
		name     string
		choice   Choice
		validate func(*testing.T, openai.Choice)
	}{
		{
			name: "choice with message containing reasoning",
			choice: Choice{
				Message: &Message{
					Reasoning: stringPtr("reasoning content"),
				},
			},
			validate: func(t *testing.T, c openai.Choice) {
				assert.NotNil(t, c.Message)
				assert.Equal(t, "reasoning content", *c.Message.ReasoningContent)
			},
		},
		{
			name: "choice with delta containing reasoning",
			choice: Choice{
				Delta: &Message{
					Reasoning: stringPtr("streaming reasoning"),
				},
			},
			validate: func(t *testing.T, c openai.Choice) {
				assert.NotNil(t, c.Delta)
				assert.Equal(t, "streaming reasoning", *c.Delta.ReasoningContent)
			},
		},
		{
			name: "choice with both message and delta",
			choice: Choice{
				Message: &Message{
					Reasoning: stringPtr("final reasoning"),
				},
				Delta: &Message{
					Reasoning: stringPtr("partial reasoning"),
				},
			},
			validate: func(t *testing.T, c openai.Choice) {
				assert.NotNil(t, c.Message)
				assert.NotNil(t, c.Delta)
				assert.Equal(t, "final reasoning", *c.Message.ReasoningContent)
				assert.Equal(t, "partial reasoning", *c.Delta.ReasoningContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			choice := tt.choice.ToOpenAIChoice()
			if tt.validate != nil {
				tt.validate(t, choice)
			}
		})
	}
}

func TestMessage_ToOpenAIMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  Message
		validate func(*testing.T, openai.Message)
	}{
		{
			name: "message with reasoning maps to reasoning_content",
			message: Message{
				Reasoning: stringPtr("thinking..."),
			},
			validate: func(t *testing.T, msg openai.Message) {
				assert.NotNil(t, msg.ReasoningContent)
				assert.Equal(t, "thinking...", *msg.ReasoningContent)
			},
		},
		{
			name:    "message without reasoning",
			message: Message{},
			validate: func(t *testing.T, msg openai.Message) {
				assert.Nil(t, msg.ReasoningContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.message.ToOpenAIMessage()
			if tt.validate != nil {
				tt.validate(t, msg)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
