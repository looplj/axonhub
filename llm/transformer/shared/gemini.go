package shared

import (
	"encoding/base64"
	"strings"
)

// TransformerMetadataKeyGoogleThoughtSignature 用于在 ToolCall TransformerMetadata 中保存 Gemini thought signature。
const TransformerMetadataKeyGoogleThoughtSignature = "google_thought_signature"

// GeminiThoughtSignaturePrefix is the prefix used for Gemini thought/reasoning signatures.
// In models like Gemini 2.0, reasoning process is a first-class citizen.
// This signature allows AxonHub to "wrap" and preserve these reasoning blocks in the internal
// message structure. This ensures that when switching between different providers (e.g., Gemini -> OpenAI -> Gemini),
// the original reasoning context is maintained and can be restored, preventing model performance degradation.
var GeminiThoughtSignaturePrefix = base64.StdEncoding.EncodeToString([]byte("<GEMINI_THOUGHT_SIGNATURE>"))

func IsGeminiThoughtSignature(signature *string) bool {
	if signature == nil {
		return false
	}

	return strings.HasPrefix(*signature, GeminiThoughtSignaturePrefix)
}

func DecodeGeminiThoughtSignature(signature *string) *string {
	if !IsGeminiThoughtSignature(signature) {
		return nil
	}

	decoded := (*signature)[len(GeminiThoughtSignaturePrefix):]

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

	if IsGeminiThoughtSignature(&signature) {
		return &signature
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
