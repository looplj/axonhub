package shared

import (
	"strings"
)

func IsOpenAIEncryptedContent(content *string) bool {
	if content == nil {
		return false
	}

	return strings.HasPrefix(*content, OpenAIEncryptedContentPrefix)
}

func DecodeOpenAIEncryptedContent(content *string) *string {
	if !IsOpenAIEncryptedContent(content) {
		return nil
	}

	decoded := (*content)[len(OpenAIEncryptedContentPrefix):]

	return &decoded
}

func EncodeOpenAIEncryptedContent(content *string) *string {
	if content == nil {
		return nil
	}

	encoded := OpenAIEncryptedContentPrefix + *content

	return &encoded
}
