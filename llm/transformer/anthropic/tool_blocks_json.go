package anthropic

import "encoding/json"

// UnmarshalJSON preserves server-tool result objects (including search errors).
// Ordinary message and client-tool content still uses MessageContent validation.
func (b *MessageContentBlock) UnmarshalJSON(data []byte) error {
	type blockAlias MessageContentBlock
	var wire struct {
		*blockAlias

		Content json.RawMessage `json:"content"`
	}
	wire.blockAlias = (*blockAlias)(b)
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	b.Content = nil
	if len(wire.Content) == 0 || string(wire.Content) == "null" {
		return nil
	}
	b.Content = new(MessageContent)
	if isAnthropicSpecialToolResultBlock(b.Type) {
		b.Content.SetRaw(wire.Content)
		return nil
	}
	return json.Unmarshal(wire.Content, b.Content)
}
