package shared

import (
	"encoding/base64"
	"strings"
)

func parseOpenAIEncryptedContentPrefix(content string) (prefixLength int, footprint string, ok bool) {
	if len(content) >= len(OpenAIEncryptedContentPrefix)+8 && strings.HasPrefix(content, OpenAIEncryptedContentPrefix) {
		fpB64 := content[len(OpenAIEncryptedContentPrefix) : len(OpenAIEncryptedContentPrefix)+8]
		if decoded, err := base64.StdEncoding.DecodeString(fpB64); err == nil && len(decoded) == 6 {
			fp := string(decoded)
			if isFootprintHex6(fp) {
				return len(OpenAIEncryptedContentPrefix) + 8, fp, true
			}
		}
	}

	return 0, "", false
}

func DecodeOpenAIEncryptedContent(content *string, footprint string) *string {
	if content == nil {
		return nil
	}

	prefixLength, embeddedFootprint, ok := parseOpenAIEncryptedContentPrefix(*content)
	if !ok {
		return nil
	}

	if embeddedFootprint != "" && embeddedFootprint != footprint {
		return nil
	}

	decoded := (*content)[prefixLength:]

	return &decoded
}

func DecodeOpenAIEncryptedContentInScope(content *string, scope TransportScope) *string {
	return DecodeOpenAIEncryptedContent(content, scope.Footprint())
}

func EncodeOpenAIEncryptedContent(content *string, footprint string) *string {
	if content == nil {
		return nil
	}

	if footprint == "" || !isFootprintHex6(footprint) {
		encoded := *content
		return &encoded
	}

	prefix := OpenAIEncryptedContentPrefix + base64.StdEncoding.EncodeToString([]byte(footprint))
	encoded := prefix + *content

	return &encoded
}

func EncodeOpenAIEncryptedContentInScope(content *string, scope TransportScope) *string {
	return EncodeOpenAIEncryptedContent(content, scope.Footprint())
}
