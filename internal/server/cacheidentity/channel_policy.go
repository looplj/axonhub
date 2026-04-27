package cacheidentity

import (
	"net/url"
	"strings"

	"github.com/looplj/axonhub/internal/ent/channel"
)

// ChannelAllowsPromptCacheKey determines whether a channel should receive
// the prompt_cache_key field in outbound requests.
//
// Policy: allow official OpenAI-backed channels with api.openai.com or any
// explicitly configured trusted proxy host. Third-party OpenAI-compatible
// proxies are denied by default to prevent 502 regressions (PR #1426).
func ChannelAllowsPromptCacheKey(channelType channel.Type, baseURL string, trustedHosts []string) bool {
	switch channelType {
	case channel.TypeOpenai, channel.TypeOpenaiResponses:
		return isTrustedOpenAIHost(baseURL, trustedHosts)
	default:
		// All other channel types (codex, gemini_openai, deepseek, openrouter,
		// nanogpt, etc.) do not receive prompt_cache_key on the wire.
		return false
	}
}

// isTrustedOpenAIHost checks if the base URL points to api.openai.com or
// any configured trusted proxy host. Matching is case-insensitive and
// ignores scheme, port, and path.
func isTrustedOpenAIHost(baseURL string, trustedHosts []string) bool {
	if baseURL == "" {
		// Default-deny: only explicit hosts are allowlisted.
		return false
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsed.Hostname())

	// Official OpenAI is always trusted.
	if host == "api.openai.com" {
		return true
	}

	// Check configured trusted proxy hosts.
	for _, trusted := range trustedHosts {
		if strings.EqualFold(host, trusted) {
			return true
		}
	}

	return false
}
