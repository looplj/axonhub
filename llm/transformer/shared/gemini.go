package shared

import (
	"strings"
)

// TransformerMetadataKeyGoogleThoughtSignature 用于在 ToolCall TransformerMetadata 中保存 Gemini thought signature。
const TransformerMetadataKeyGoogleThoughtSignature = "google_thought_signature"

// GeminiThoughtSignaturePrefix is the prefix used for Gemini thought/reasoning signatures.
// In models like Gemini 2.0, reasoning process is a first-class citizen.
// This signature allows AxonHub to "wrap" and preserve these reasoning blocks in the internal
// message structure. This ensures that when switching between different providers (e.g., Gemini -> OpenAI -> Gemini),
// the original reasoning context is maintained and can be restored, preventing model performance degradation.
//
// NOTE: This prefix MUST NOT end with "=".
// If the prefix contains base64 padding "=", concatenating "prefix + raw_signature" may produce an invalid whole base64 string.
// Plaintext marker before base64: "GEMINI_THOUGHT_SIGNATURE_V1".
var GeminiThoughtSignaturePrefix = "R0VNSU5JX1RIT1VHSFRfU0lHTkFUVVJFX1Yx"

var legacyGeminiThoughtSignaturePrefix = "PEdFTUlOSV9USE9VR0hUX1NJR05BVFVSRT4="

func geminiThoughtSignaturePrefixLength(signature string) int {
	if strings.HasPrefix(signature, GeminiThoughtSignaturePrefix) {
		return len(GeminiThoughtSignaturePrefix)
	}

	if strings.HasPrefix(signature, legacyGeminiThoughtSignaturePrefix) {
		return len(legacyGeminiThoughtSignaturePrefix)
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

// NormalizeGeminiThoughtSignature normalizes Gemini thought signatures into internal encoded format.
func NormalizeGeminiThoughtSignature(signature string) *string {
	if signature == "" {
		return nil
	}

	if decoded := DecodeGeminiThoughtSignature(&signature); decoded != nil {
		return EncodeGeminiThoughtSignature(decoded)
	}

	return EncodeGeminiThoughtSignature(&signature)
}

// StripGeminiThoughtSignaturePrefix removes internal prefix from Gemini thought signatures.
func StripGeminiThoughtSignaturePrefix(signature string) string {
	if !IsGeminiThoughtSignature(&signature) {
		return signature
	}

	decoded := DecodeGeminiThoughtSignature(&signature)
	if decoded == nil {
		return signature
	}

	return *decoded
}
