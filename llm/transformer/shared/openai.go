package shared

import (
	"net/url"
	"strings"
)

// EncodeOpenAIEncryptedContent encodes raw OpenAI encrypted content for storage.
// OpenAI encrypted_content is already base64-encoded, so this is a passthrough.
func EncodeOpenAIEncryptedContent(content *string) *string {
	if content == nil {
		return nil
	}
	return content
}

// DecodeOpenAIEncryptedContent checks whether a blob is safe to use as OpenAI encrypted content.
// Returns the raw value only if the blob is recognized as OpenAI.
// Returns nil for signatures from other providers (Anthropic/Gemini) or unknown formats.
func DecodeOpenAIEncryptedContent(content *string) *string {
	if content == nil {
		return nil
	}

	result := GuessSignatureProvider(*content)
	if result.Provider != ProviderOpenAI {
		return nil
	}

	return content
}

// SupportsPromptCacheKey reports whether the OpenAI-protocol host documents
// prompt_cache_key. Official OpenAI Chat Completions and Responses do;
// NVIDIA NIM and many OpenAI-compatible gateways reject the field.
func SupportsPromptCacheKey(baseURL string) bool {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return false
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "api.openai.com" || strings.HasSuffix(host, ".api.openai.com")
}
