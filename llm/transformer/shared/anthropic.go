package shared

// EncodeAnthropicSignature encodes a raw Anthropic signature for storage in ReasoningSignature.
// Anthropic signatures are typically raw bytes, so we base64-encode them if needed.
func EncodeAnthropicSignature(signature *string) *string {
	if signature == nil {
		return nil
	}

	encoded := EnsureBase64Encoding(*signature)
	return &encoded
}

// DecodeAnthropicSignature checks whether a signature blob is safe to use as an Anthropic thinking signature.
// Returns the raw value if the blob is likely Anthropic or its provider is unknown.
// Returns nil if the blob is clearly from a different provider (OpenAI/Gemini).
func DecodeAnthropicSignature(signature *string) *string {
	if signature == nil {
		return nil
	}

	result := GuessSignatureProvider(*signature)
	if result.Provider == ProviderOpenAI || result.Provider == ProviderGemini {
		return nil
	}

	return signature
}
