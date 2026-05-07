package shared

// EncodeOpenAIEncryptedContent encodes raw OpenAI encrypted content for storage.
// OpenAI encrypted_content is already base64-encoded, so this is a passthrough.
func EncodeOpenAIEncryptedContent(content *string) *string {
	if content == nil {
		return nil
	}
	return content
}

// DecodeOpenAIEncryptedContent checks whether a blob is safe to use as OpenAI encrypted content.
// Returns the raw value if the blob is likely OpenAI or its provider is unknown.
// Returns nil if the blob is clearly from a different provider (Anthropic/Gemini).
func DecodeOpenAIEncryptedContent(content *string) *string {
	if content == nil {
		return nil
	}

	result := GuessSignatureProvider(*content)
	if result.Provider == ProviderAnthropic || result.Provider == ProviderGemini {
		return nil
	}

	return content
}
