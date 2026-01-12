package openai

import "github.com/looplj/axonhub/llm"

// Usage represents the usage response from OpenAI compatible format.
// Difference provider may have different format, so we use this to convert to unified format.
type Usage struct {
	llm.Usage

	// CachedTokens is the number of tokens that were cached for Moonshot.
	CachedTokens int64 `json:"cached_tokens"`
}

func (u *Usage) ToLLMUsage() *llm.Usage {
	if u == nil {
		return nil
	}

	if (u.PromptTokensDetails == nil || u.PromptTokensDetails.CachedTokens == 0) && u.CachedTokens > 0 {
		if u.PromptTokensDetails == nil {
			u.PromptTokensDetails = &llm.PromptTokensDetails{}
		}

		u.PromptTokensDetails.CachedTokens = u.CachedTokens
	}

	return &u.Usage
}

// UsageFromLLM creates OpenAI Usage from unified llm.Usage.
func UsageFromLLM(u *llm.Usage) *Usage {
	if u == nil {
		return nil
	}

	usage := &Usage{
		Usage: *u,
	}

	// Extract CachedTokens from PromptTokensDetails if present
	if u.PromptTokensDetails != nil {
		usage.CachedTokens = u.PromptTokensDetails.CachedTokens
	}

	return usage
}
