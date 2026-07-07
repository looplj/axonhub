package cline

import (
	"strings"

	"github.com/looplj/axonhub/llm/transformer/openai"
)

type Response struct {
	openai.Response

	Choices []Choice `json:"choices"`
}

func (r *Response) ToOpenAIResponse() *openai.Response {
	if r == nil {
		return nil
	}

	resp := r.Response
	resp.Choices = make([]openai.Choice, 0, len(r.Choices))
	for _, choice := range r.Choices {
		resp.Choices = append(resp.Choices, choice.ToOpenAIChoice())
	}

	return &resp
}

type Choice struct {
	openai.Choice

	Message *Message `json:"message,omitempty"`
	Delta   *Message `json:"delta,omitempty"`
}

func (c Choice) ToOpenAIChoice() openai.Choice {
	choice := c.Choice
	if c.Message != nil {
		msg := c.Message.ToOpenAIMessage()
		choice.Message = &msg
	}
	if c.Delta != nil {
		delta := c.Delta.ToOpenAIMessage()
		choice.Delta = &delta
	}

	return choice
}

type Message struct {
	openai.Message

	Reasoning        *string           `json:"reasoning,omitempty"`
	ReasoningDetails []ReasoningDetail `json:"reasoning_details,omitempty"`
	ProviderMetadata map[string]any    `json:"provider_metadata,omitempty"`
}

type ReasoningDetail struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Format string `json:"format"`
	Index  int    `json:"index"`
}

func (m *Message) ToOpenAIMessage() openai.Message {
	if m == nil {
		return openai.Message{}
	}

	if len(m.ReasoningDetails) > 0 {
		var reasoningText strings.Builder
		for _, detail := range m.ReasoningDetails {
			reasoningText.WriteString(detail.Text)
		}
		reasoning := reasoningText.String()
		m.Message.ReasoningContent = &reasoning
	} else if m.Reasoning != nil {
		m.Message.ReasoningContent = m.Reasoning
	}

	if len(m.Message.ToolCalls) == 0 {
		m.Message.ToolCalls = nil
	}
	if len(m.Message.Annotations) == 0 {
		m.Message.Annotations = nil
	}
	if len(m.Message.Content.MultipleContent) == 0 {
		m.Message.Content.MultipleContent = nil
	}

	return m.Message
}
