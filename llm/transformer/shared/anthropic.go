package shared

import (
	"encoding/base64"
	"strings"
)

func parseAnthropicSignaturePrefix(signature string) (prefixLength int, footprint string, ok bool) {
	if len(signature) >= len(AnthropicSignaturePrefix)+8 && strings.HasPrefix(signature, AnthropicSignaturePrefix) {
		fpB64 := signature[len(AnthropicSignaturePrefix) : len(AnthropicSignaturePrefix)+8]
		if decoded, err := base64.StdEncoding.DecodeString(fpB64); err == nil && len(decoded) == 6 {
			fp := string(decoded)
			if isFootprintHex6(fp) {
				return len(AnthropicSignaturePrefix) + 8, fp, true
			}
		}
	}

	return 0, "", false
}

// DecodeAnthropicSignature strips the full Anthropic prefix (type marker + footprint)
// from an encoded signature. Returns nil if the prefix does not match.
func DecodeAnthropicSignature(signature *string, footprint string) *string {
	if signature == nil {
		return nil
	}

	prefixLength, embeddedFootprint, ok := parseAnthropicSignaturePrefix(*signature)
	if !ok {
		return nil
	}

	if embeddedFootprint != "" && embeddedFootprint != footprint {
		return nil
	}

	decoded := (*signature)[prefixLength:]

	return &decoded
}

func DecodeAnthropicSignatureInScope(signature *string, scope TransportScope) *string {
	return DecodeAnthropicSignature(signature, scope.Footprint())
}

// EncodeAnthropicSignature encodes a raw signature with the Anthropic type marker and footprint.
// Format:
//   - without footprint: EnsureBase64Encoding(rawSignature)
//   - with footprint: AnthropicSignaturePrefix + base64(footprint) + EnsureBase64Encoding(rawSignature)
func EncodeAnthropicSignature(signature *string, footprint string) *string {
	if signature == nil {
		return nil
	}

	encoded := EnsureBase64Encoding(*signature)
	if footprint == "" || !isFootprintHex6(footprint) {
		return &encoded
	}

	prefix := AnthropicSignaturePrefix + base64.StdEncoding.EncodeToString([]byte(footprint))
	encoded = prefix + encoded

	return &encoded
}

func EncodeAnthropicSignatureInScope(signature *string, scope TransportScope) *string {
	return EncodeAnthropicSignature(signature, scope.Footprint())
}
