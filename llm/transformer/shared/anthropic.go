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
// Returns the raw value only if the blob is recognized as Anthropic.
// Returns nil for signatures from other providers (OpenAI/Gemini) or unknown formats.
func DecodeAnthropicSignature(signature *string) *string {
	if signature == nil {
		return nil
	}

	result := GuessSignatureProvider(*signature)
	if result.Provider != ProviderAnthropic {
		return nil
	}

	return signature
}
