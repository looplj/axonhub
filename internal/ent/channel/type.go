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

// SupportsGoogleNativeTools returns true if the channel type supports Google native tools.
// Google native tools (google_search, google_url_context, google_code_execution) are only
// supported by native Gemini API format channels (gemini, gemini_vertex).
// OpenAI-compatible endpoints (gemini_openai) do NOT support these tools.
func (t Type) SupportsGoogleNativeTools() bool {
	return t == TypeGemini || t == TypeGeminiVertex
}
