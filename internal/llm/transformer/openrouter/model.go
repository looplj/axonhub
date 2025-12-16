package openrouter

import (
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
)

type Response struct {
	openai.Response

	Choices []Choice `json:"choices"`
}

func (r *Response) ToOpenAIResponse() *openai.Response {
	for _, choice := range r.Choices {
		r.Response.Choices = append(r.Response.Choices, choice.ToOpenAIChoice())
	}

	return &r.Response
}

type Choice struct {
	openai.Choice

	Message *Message `json:"message,omitempty"`
	Delta   *Message `json:"delta,omitempty"`
}

type Image openai.MessageContentPart

func (c *Choice) ToOpenAIChoice() openai.Choice {
	if c.Message != nil {
		msg := c.Message.ToOpenAIMessage()
		c.Choice.Message = &msg
	}

	if c.Delta != nil {
		delta := c.Delta.ToOpenAIMessage()
		c.Choice.Delta = &delta
	}

	return c.Choice
}

// Message is the message content from the OpenRouter response.
// The difference from openai.Message is that it has a Reasoning field.
type Message struct {
	openai.Message

	Reasoning *string `json:"reasoning,omitempty"`
	Images    []Image `json:"images,omitempty"`
}

func (m *Message) ToOpenAIMessage() openai.Message {
	m.ReasoningContent = m.Reasoning
	if len(m.Images) > 0 {
		var parts []openai.MessageContentPart
		if m.Content.Content != nil && *m.Content.Content != "" {
			parts = append(parts, openai.MessageContentPart{
				Type: "text",
				Text: m.Content.Content,
			})
		} else {
			parts = m.Content.MultipleContent
		}

		for _, image := range m.Images {
			parts = append(parts, openai.MessageContentPart(image))
		}

		m.Content = openai.MessageContent{MultipleContent: parts}
	} else {
		// Preserve nil for empty slices to match test expectations
		if len(m.Content.MultipleContent) == 0 {
			m.Content.MultipleContent = nil
		}

		if len(m.ToolCalls) == 0 {
			m.ToolCalls = nil
		}
	}

	return m.Message
}
