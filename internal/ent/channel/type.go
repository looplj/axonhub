package channel

import (
	"strings"
)

func (t Type) IsAnthropic() bool {
	return t == TypeAnthropic
}

func (t Type) IsAnthropicLike() bool {
	return strings.HasSuffix(string(t), "_anthropic")
}

func (t Type) IsGemini() bool {
	return t == TypeGemini
}

func (t Type) IsOpenAI() bool {
	return !t.IsAnthropicLike() && !t.IsAnthropic() && !t.IsGemini()
}

// UsesAnthropicModelAPI returns true if the channel type should use Anthropic-style
// /v1/models endpoint with X-Api-Key authentication when fetching models.
func (t Type) UsesAnthropicModelAPI() bool {
	return t.IsAnthropic() || t.IsAnthropicLike() || t == TypeClaudecode
}

// SupportsGoogleNativeTools returns true if the channel type supports Google native tools.
// Google native tools (google_search, google_url_context, google_code_execution) are only
// supported by native Gemini API format channels (gemini, gemini_vertex).
// OpenAI-compatible endpoints (gemini_openai) do NOT support these tools.
func (t Type) SupportsGoogleNativeTools() bool {
	return t == TypeGemini || t == TypeGeminiVertex
}

// SupportsAnthropicNativeTools returns true if the channel type supports Anthropic native tools.
// Anthropic native tools (web_search_20250305) are supported by Anthropic-backed
// channels, Claude Code, and DeepSeek's verified Anthropic-compatible endpoint.
// Other Anthropic-compatible channels remain excluded until support is verified.
func (t Type) SupportsAnthropicNativeTools() bool {
	return t == TypeAnthropic || t == TypeAnthropicAWS || t == TypeAnthropicGcp ||
		t == TypeClaudecode || t == TypeDeepseekAnthropic
}
