package shared

// IsAnthropicRedactedContent checks if the content should be treated as Anthropic redacted content.
// It explicitly excludes signatures from other providers (like Gemini or OpenAI) to prevent
// conflicts during the transformation process. This isolation ensures that model-specific
// private protocols do not interfere with each other when converting to the Anthropic format.
func IsAnthropicRedactedContent(content *string) bool {
	if content == nil {
		return false
	}

	return !IsGeminiThoughtSignature(content) && !IsOpenAIEncryptedContent(content)
}
