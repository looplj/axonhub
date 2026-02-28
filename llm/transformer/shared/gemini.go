package shared

import (
	"strings"
)

// TransformerMetadataKeyGoogleThoughtSignature 用于在 ToolCall TransformerMetadata 中保存 Gemini thought signature。
const TransformerMetadataKeyGoogleThoughtSignature = "google_thought_signature"

func geminiThoughtSignaturePrefixLength(signature string) int {
	if strings.HasPrefix(signature, GeminiThoughtSignaturePrefix) {
		return len(GeminiThoughtSignaturePrefix)
	}

	return 0
}

func IsGeminiThoughtSignature(signature *string) bool {
	if signature == nil {
		return false
	}

	return geminiThoughtSignaturePrefixLength(*signature) > 0
}

func DecodeGeminiThoughtSignature(signature *string) *string {
	if signature == nil {
		return nil
	}

	prefixLength := geminiThoughtSignaturePrefixLength(*signature)
	if prefixLength == 0 {
		return nil
	}

	decoded := (*signature)[prefixLength:]

	return &decoded
}

func EncodeGeminiThoughtSignature(signature *string) *string {
	if signature == nil {
		return nil
	}

	encoded := GeminiThoughtSignaturePrefix + *signature

	return &encoded
}

// NormalizeGeminiThoughtSignature returns the internal-prefixed representation of a Gemini
// thought signature, preserving already-prefixed values and converting legacy prefixes.
// Empty strings return nil.
func NormalizeGeminiThoughtSignature(signature string) *string {
	if signature == "" {
		return nil
	}

	if decoded := DecodeGeminiThoughtSignature(&signature); decoded != nil {
		// Convert legacy prefix to the current internal prefix.
		return EncodeGeminiThoughtSignature(decoded)
	}

	// No known prefix; wrap as internal Gemini signature.
	return EncodeGeminiThoughtSignature(&signature)
}

// StripGeminiThoughtSignaturePrefix removes internal prefix from Gemini thought signatures.
func StripGeminiThoughtSignaturePrefix(signature string) string {
	prefixLength := geminiThoughtSignaturePrefixLength(signature)
	if prefixLength == 0 {
		return signature
	}

	return signature[prefixLength:]
}
