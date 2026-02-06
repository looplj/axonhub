package shared

import (
	"encoding/base64"
	"strings"
)

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
