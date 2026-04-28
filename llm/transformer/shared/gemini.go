package shared

import (
	"encoding/base64"
	"strings"
)

// TransformerMetadataKeyGoogleThoughtSignature 用于在 ToolCall TransformerMetadata 中保存 Gemini thought signature。
const TransformerMetadataKeyGoogleThoughtSignature = "google_thought_signature"

func parseGeminiThoughtSignaturePrefix(signature string) (prefixLength int, footprint string, ok bool) {
	if len(signature) >= len(GeminiThoughtSignaturePrefix)+8 && strings.HasPrefix(signature, GeminiThoughtSignaturePrefix) {
		fpB64 := signature[len(GeminiThoughtSignaturePrefix) : len(GeminiThoughtSignaturePrefix)+8]
		if decoded, err := base64.StdEncoding.DecodeString(fpB64); err == nil && len(decoded) == 6 {
			fp := string(decoded)
			if isFootprintHex6(fp) {
				return len(GeminiThoughtSignaturePrefix) + 8, fp, true
			}
		}
	}

	return 0, "", false
}

func DecodeGeminiThoughtSignature(signature *string, footprint string) *string {
	if signature == nil {
		return nil
	}

	prefixLength, embeddedFootprint, ok := parseGeminiThoughtSignaturePrefix(*signature)
	if !ok {
		return nil
	}

	if embeddedFootprint != "" && embeddedFootprint != footprint {
		return nil
	}

	decoded := (*signature)[prefixLength:]

	return &decoded
}

func DecodeGeminiThoughtSignatureInScope(signature *string, scope TransportScope) *string {
	return DecodeGeminiThoughtSignature(signature, scope.Footprint())
}

func EncodeGeminiThoughtSignature(signature *string, footprint string) *string {
	if signature == nil {
		return nil
	}

	if footprint == "" || !isFootprintHex6(footprint) {
		encoded := *signature
		return &encoded
	}

	prefix := GeminiThoughtSignaturePrefix + base64.StdEncoding.EncodeToString([]byte(footprint))
	encoded := prefix + *signature

	return &encoded
}

func EncodeGeminiThoughtSignatureInScope(signature *string, scope TransportScope) *string {
	return EncodeGeminiThoughtSignature(signature, scope.Footprint())
}
