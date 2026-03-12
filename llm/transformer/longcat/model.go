package longcat

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/samber/lo"
)

// Request embeds openai.Request but overrides Messages with longcat-specific Message type.
type Request struct {
	openai.Request

	Messages []Message `json:"messages"`
}

// Message embeds openai.Message but overrides Content with longcat-specific MessageContent.
type Message struct {
	openai.Message

	Content MessageContent `json:"content,omitzero"`
}

// MessageContent always marshals as an array of content parts.
// Longcat models (e.g. LongCat-Flash-Omni) reject plain string content.
type MessageContent struct {
	openai.MessageContent
}

func (c MessageContent) MarshalJSON() ([]byte, error) {
	if len(c.MultipleContent) > 0 {
		return json.Marshal(c.MultipleContent)
	}

	if c.Content != nil {
		return json.Marshal([]openai.MessageContentPart{
			{Type: "text", Text: c.Content},
		})
	}

	return json.Marshal([]openai.MessageContentPart{
		{Type: "text", Text: lo.ToPtr("")},
	})
}
