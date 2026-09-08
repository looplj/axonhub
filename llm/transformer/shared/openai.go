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
// Returns the raw value only if the blob is recognized as OpenAI.
// Returns nil for signatures from other providers (Anthropic/Gemini) or unknown formats.
func DecodeOpenAIEncryptedContent(content *string) *string {
	if content == nil {
		return nil
	}

	result := GuessSignatureProvider(*content)
	if result.Provider != ProviderOpenAI {
		return nil
	}

	return content
}
