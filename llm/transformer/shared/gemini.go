package shared

// TransformerMetadataKeyGoogleThoughtSignature is the key used to store Gemini thought signature
// in ToolCall TransformerMetadata.
const TransformerMetadataKeyGoogleThoughtSignature = "google_thought_signature"

// EncodeGeminiThoughtSignature encodes a raw Gemini thought signature for storage.
// Gemini thought signatures are already base64-encoded, so this is a passthrough.
func EncodeGeminiThoughtSignature(signature *string) *string {
	if signature == nil {
		return nil
	}
	return signature
}

// DecodeGeminiThoughtSignature checks whether a signature blob is safe to use as a Gemini thought signature.
// Returns the raw value only if the blob is recognized as a Gemini signature.
// Returns nil for signatures from other providers (OpenAI/Anthropic) or unknown formats.
func DecodeGeminiThoughtSignature(signature *string) *string {
	if signature == nil {
		return nil
	}

	result := GuessSignatureProvider(*signature)
	if result.Provider == ProviderGemini {
		return signature
	}

	return nil
}
