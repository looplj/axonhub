package openrouter

import (
	"encoding/json"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai"
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

	Reasoning        *string           `json:"reasoning,omitempty"`
	ReasoningDetails []ReasoningDetail `json:"reasoning_details,omitempty"`
	Images           []Image           `json:"images,omitempty"`
}

type ReasoningDetail struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Format string `json:"format"`
	Index  int    `json:"index"`
}

func (m *Message) ToOpenAIMessage() openai.Message {
	// Handle reasoning content - prefer reasoning_details if available, fallback to reasoning
	if len(m.ReasoningDetails) > 0 {
		var reasoningText strings.Builder
		for _, detail := range m.ReasoningDetails {
			reasoningText.WriteString(detail.Text)
		}

		reasoning := reasoningText.String()
		m.ReasoningContent = &reasoning
	} else if m.Reasoning != nil {
		m.ReasoningContent = m.Reasoning
	}

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

	// Carry structured reasoning_details and images onto the embedded openai.Message
	// so they survive into canonical (instead of only being collapsed into
	// reasoning text / content parts above).
	if len(m.ReasoningDetails) > 0 {
		details := make([]json.RawMessage, 0, len(m.ReasoningDetails))
		for _, d := range m.ReasoningDetails {
			b, err := json.Marshal(d)
			if err != nil {
				return m.Message
			}
			details = append(details, b)
		}
		m.Message.ReasoningDetails = details
	}

	if len(m.Images) > 0 {
		images := make([]llm.ChatImage, 0, len(m.Images))
		for _, img := range m.Images {
			if img.ImageURL != nil {
				images = append(images, llm.ChatImage{ImageURL: llm.ChatImageURL{URL: img.ImageURL.URL}})
			}
		}
		m.Message.Images = images
	}

	return m.Message
}
