package openrouter

import (
	"strings"

	"github.com/looplj/axonhub/llm/transformer/openai"
)

// ImageGenerationRequest is the request body for OpenRouter's image router.
// InputReferences intentionally use OpenAI's image_url content-part shape,
// which is the shape required by OpenRouter's API.
//
// OpenRouter's image router does not accept OpenAI's moderation, response_format
// or user fields, so those are intentionally dropped instead of forwarded.
type ImageGenerationRequest struct {
	Model             string                      `json:"model"`
	Prompt            string                      `json:"prompt"`
	N                 *int64                      `json:"n,omitempty"`
	Size              string                      `json:"size,omitempty"`
	Quality           string                      `json:"quality,omitempty"`
	Background        string                      `json:"background,omitempty"`
	OutputFormat      string                      `json:"output_format,omitempty"`
	OutputCompression *int64                      `json:"output_compression,omitempty"`
	Seed              *int64                      `json:"seed,omitempty"`
	InputReferences   []openai.MessageContentPart `json:"input_references,omitempty"`
}

// ImageGenerationResponse is the non-streaming response from OpenRouter's
// image router.
type ImageGenerationResponse struct {
	Created int64                 `json:"created"`
	Data    []ImageGenerationData `json:"data"`
	Usage   *ImageGenerationUsage `json:"usage,omitempty"`
}

type ImageGenerationData struct {
	B64JSON   string `json:"b64_json"`
	MediaType string `json:"media_type,omitempty"`
}

type ImageGenerationUsage struct {
	PromptTokens            int64                         `json:"prompt_tokens"`
	CompletionTokens        int64                         `json:"completion_tokens"`
	TotalTokens             int64                         `json:"total_tokens"`
	PromptTokensDetails     *ImagePromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *ImageCompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

type ImagePromptTokensDetails struct {
	ImageTokens  int64 `json:"image_tokens,omitempty"`
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

type ImageCompletionTokensDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
}

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

	return m.Message
}
