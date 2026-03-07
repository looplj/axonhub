package objects

type MessageChannelSettings struct {
	Feishu *FeishuSettings `json:"feishu,omitempty"`
}

type FeishuSettings struct {
	AppID             string   `json:"appId,omitempty"`
	AppSecret         string   `json:"appSecret,omitempty"`
	EncryptKey        string   `json:"encryptKey,omitempty"`
	VerificationToken string   `json:"verificationToken,omitempty"`
	AllowFrom         []string `json:"allowFrom,omitempty"`
	ExcludeKeywords   []string `json:"excludeKeywords,omitempty"`
}

type MessageChatType string

const (
	MessageChatTypeDM    MessageChatType = "dm"
	MessageChatTypeGroup MessageChatType = "group"
)

// MessageChannelAgentInstanceBinding holds the configuration for a message channel to agent instance binding.
type MessageChannelAgentInstanceBinding struct {
	ChatType        MessageChatType `json:"chatType,omitempty"`
	ChatID          string          `json:"chatID,omitempty"`
	AllowFrom       []string        `json:"allowFrom,omitempty"`
	ExcludeKeywords []string        `json:"excludeKeywords,omitempty"`
}

func (b *MessageChannelAgentInstanceBinding) Equals(other *MessageChannelAgentInstanceBinding) bool {
	if b == nil || other == nil {
		return b == other
	}

	if b.ChatType != other.ChatType {
		return false
	}

	if b.ChatID != other.ChatID {
		return false
	}

	if len(b.AllowFrom) != len(other.AllowFrom) {
		return false
	}

	for i := range b.AllowFrom {
		if b.AllowFrom[i] != other.AllowFrom[i] {
			return false
		}
	}

	if len(b.ExcludeKeywords) != len(other.ExcludeKeywords) {
		return false
	}

	for i := range b.ExcludeKeywords {
		if b.ExcludeKeywords[i] != other.ExcludeKeywords[i] {
			return false
		}
	}

	return true
}
