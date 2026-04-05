package agent

import (
	"encoding/json"
	"fmt"
)

// ContentPartType identifies the kind of content part.
type ContentPartType string

const (
	ContentPartText             ContentPartType = "text"
	ContentPartThinking         ContentPartType = "thinking"
	ContentPartRedactedThinking ContentPartType = "redacted_thinking"
	ContentPartImage            ContentPartType = "image"
)

type Content struct {
	Text  *string
	Parts []ContentPart
}

func (c Content) MarshalJSON() ([]byte, error) {
	if c.Text != nil {
		return json.Marshal(c.Text)
	}

	return json.Marshal(c.Parts)
}

func (c *Content) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		c.Text = &str
		return nil
	}

	var parts []ContentPart
	if err := json.Unmarshal(data, &parts); err == nil {
		c.Parts = parts
		return nil
	}

	return fmt.Errorf("invalid content format: expected string or array")
}

func (c Content) String() string {
	if c.Text != nil {
		return *c.Text
	}

	var result string
	for _, part := range c.Parts {
		if part.Type == ContentPartText {
			result += part.Text
		}
	}
	return result
}

// ContentPart is a union type representing one block inside a Content array.
// Only the fields relevant to the given Type are populated.
type ContentPart struct {
	Type ContentPartType `json:"type"`

	Text          string `json:"text,omitempty"`
	TextSignature string `json:"text_signature,omitempty"`

	Thinking          string `json:"thinking,omitempty"`
	ThinkingSignature string `json:"thinking_signature,omitempty"`
	RedactedThinking  string `json:"redacted_thinking,omitempty"`

	Data     string `json:"data,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	URL      string `json:"url,omitempty"`
}
