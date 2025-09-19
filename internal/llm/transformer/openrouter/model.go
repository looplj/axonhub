package openrouter

import (
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
)

type Response struct {
	openai.Response

	Choices []Choice `json:"choices"`
}

func (r *Response) ToOpenAIResponse() *openai.Response {
	for _, choice := range r.Choices {
		r.Response.Choices = append(r.Response.Choices, choice.ToLLMChoice())
	}

	return &r.Response
}

type Choice struct {
	llm.Choice

	Message *Message `json:"message,omitempty"`
	Delta   *Message `json:"delta,omitempty"`
}

func (c *Choice) ToLLMChoice() llm.Choice {
	if c.Message != nil {
		c.Choice.Message = lo.ToPtr(c.Message.ToLLMMessage())
	}

	if c.Delta != nil {
		c.Choice.Delta = lo.ToPtr(c.Delta.ToLLMMessage())
	}

	return c.Choice
}

// Message is the message content from the OpenRouter response.
// The difference from llm.Message is that it has a Reasoning field.
type Message struct {
	llm.Message

	Reasoning *string `json:"reasoning,omitempty"`
}

func (m *Message) ToLLMMessage() llm.Message {
	m.ReasoningContent = m.Reasoning
	return m.Message
}
