package cacheidentity

import (
	"net/url"
	"strings"

	"github.com/looplj/axonhub/internal/ent/channel"
)

// ChannelAllowsPromptCacheKey determines whether a channel should receive
// the prompt_cache_key field in outbound requests.
//
// V1 policy: only allow official OpenAI-backed channels with the canonical
// api.openai.com host. Third-party OpenAI-compatible proxies are denied
// to prevent 502 regressions (PR #1426).
func ChannelAllowsPromptCacheKey(channelType channel.Type, baseURL string) bool {
	switch channelType {
	case channel.TypeOpenai, channel.TypeOpenaiResponses:
		return isOfficialOpenAIHost(baseURL)
	default:
		// All other channel types (codex, gemini_openai, deepseek, openrouter,
		// nanogpt, etc.) do not receive prompt_cache_key on the wire.
		return false
	}
}

// isOfficialOpenAIHost checks if the base URL points to api.openai.com.
func isOfficialOpenAIHost(baseURL string) bool {
	if baseURL == "" {
		// Empty base URL typically means "use default", which is api.openai.com.
		return true
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsed.Hostname())

	return host == "api.openai.com"
}
