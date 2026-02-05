package shared

import (
	"encoding/base64"
	"strings"
)

// OpenAIEncryptedContentPrefix is the prefix used for OpenAI encrypted content.
// This is used to preserve OpenAI-specific encrypted blocks when converting between different providers.
// By marking these blocks with a unique signature, AxonHub can ensure that encrypted data
// is not lost or corrupted during the transformation pipeline, allowing it to be restored
// when the conversation returns to an OpenAI-compatible model.
var OpenAIEncryptedContentPrefix = base64.StdEncoding.EncodeToString([]byte("<OPENAI_ENCRYPTED_CONTENT>"))

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
