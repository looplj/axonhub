package shared

func IsAnthropicRedactedContent(content *string) bool {
	if content == nil {
		return false
	}

	return !IsGeminiThoughtSignature(content)
}
