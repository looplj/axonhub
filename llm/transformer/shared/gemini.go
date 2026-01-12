package shared

import (
	"strings"
)

const (
	GEMINI_THOUGHT_SIGNATURE_PREFIX = "<GEMINI_THOUGHT_SIGNATURE>"
)

func IsGeminiThoughtSignature(signature *string) bool {
	if signature == nil {
		return false
	}

	return strings.HasPrefix(*signature, GEMINI_THOUGHT_SIGNATURE_PREFIX)
}

func DecodeGeminiThoughtSignature(signature *string) *string {
	if !IsGeminiThoughtSignature(signature) {
		return nil
	}

	decoded := (*signature)[len(GEMINI_THOUGHT_SIGNATURE_PREFIX):]

	return &decoded
}

func EncodeGeminiThoughtSignature(signature *string) *string {
	if signature == nil {
		return nil
	}

	encoded := GEMINI_THOUGHT_SIGNATURE_PREFIX + *signature

	return &encoded
}
