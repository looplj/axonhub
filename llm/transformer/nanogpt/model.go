package nanogpt

import (
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Response represents a NanoGPT chat completion response.
// It extends the OpenAI response format to handle NanoGPT-specific fields like reasoning.
type Response struct {
	openai.Response

	Choices []Choice `json:"choices"`
}

// ToOpenAIResponse converts the NanoGPT Response to an OpenAI Response.
func (r *Response) ToOpenAIResponse() *openai.Response {
	r.Response.Choices = make([]openai.Choice, 0, len(r.Choices))
	for _, choice := range r.Choices {
		r.Response.Choices = append(r.Response.Choices, choice.ToOpenAIChoice())
	}

	return &r.Response
}

// Choice represents a choice in the response.
// It extends the OpenAI Choice to handle NanoGPT-specific message fields.
type Choice struct {
	openai.Choice

	Message *Message `json:"message,omitempty"`
	Delta   *Message `json:"delta,omitempty"`
}

// ToOpenAIChoice converts the NanoGPT Choice to an OpenAI Choice.
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

// Message represents a message in the response.
// It extends the OpenAI Message to handle NanoGPT-specific fields like reasoning.
type Message struct {
	openai.Message

	// Reasoning is the reasoning content from NanoGPT models (e.g., zai-org/glm-4.7:thinking).
	Reasoning *string `json:"reasoning,omitempty"`
}

// ToOpenAIMessage converts the NanoGPT Message to an OpenAI Message.
// It maps the Reasoning field to ReasoningContent for compatibility.
func (m *Message) ToOpenAIMessage() openai.Message {
	// Map reasoning to reasoning_content if present
	if m.Reasoning != nil {
		m.ReasoningContent = m.Reasoning
	}

	return m.Message
}
