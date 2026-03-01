package shared

import (
	"strings"
)

// IsAnthropicSignature checks if the signature is an Anthropic-encoded signature.
func IsAnthropicSignature(signature *string) bool {
	if signature == nil {
		return false
	}

	return strings.HasPrefix(*signature, AnthropicSignaturePrefix)
}

// DecodeAnthropicSignature strips the Anthropic prefix from an encoded signature.
// Returns nil if the signature does not have the Anthropic prefix.
func DecodeAnthropicSignature(signature *string) *string {
	if !IsAnthropicSignature(signature) {
		return nil
	}

	decoded := (*signature)[len(AnthropicSignaturePrefix):]

	return &decoded
}

// EncodeAnthropicSignature adds the Anthropic prefix to a raw signature.
func EncodeAnthropicSignature(signature *string) *string {
	if signature == nil {
		return nil
	}

	// Some provider, like Deepseek, will response uuid as signature instead of base64-encoded string.
	// We should encode it to base64-encoded string.
	encoded := EnsureBase64Encoding(*signature)
	encoded = AnthropicSignaturePrefix + encoded
	return &encoded
}

// IsAnthropicRedactedContent checks if the content should be treated as Anthropic redacted content.
// It explicitly excludes signatures from all providers (Gemini, OpenAI, and Anthropic encoding) to prevent
// conflicts during the transformation process. This isolation ensures that model-specific
// private protocols do not interfere with each other when converting to the Anthropic format.
func IsAnthropicRedactedContent(content *string) bool {
	if content == nil {
		return false
	}

	return !IsGeminiThoughtSignature(content) && !IsOpenAIEncryptedContent(content) && !IsAnthropicSignature(content)
}
