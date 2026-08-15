package openai

import "github.com/looplj/axonhub/llm"

// PromptTokensDetails Breakdown of tokens used in the prompt.
type PromptTokensDetails struct {
	AudioTokens  int64 `json:"audio_tokens"`
	CachedTokens int64 `json:"cached_tokens"`
	// hidden field, used for internal calculation.
	WriteCachedTokens int64 `json:"write_cached_tokens,omitempty"`
}

// CompletionTokensDetails Breakdown of tokens used in a completion.
type CompletionTokensDetails struct {
	AudioTokens              int64 `json:"audio_tokens"`
	ReasoningTokens          int64 `json:"reasoning_tokens"`
	AcceptedPredictionTokens int64 `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int64 `json:"rejected_prediction_tokens"`
}

// Usage represents the usage response from OpenAI compatible format.
// Difference provider may have different format, so we use this to convert to unified format.
type Usage struct {
	PromptTokens            int64                   `json:"prompt_tokens"`
	CompletionTokens        int64                   `json:"completion_tokens"`
	TotalTokens             int64                   `json:"total_tokens"`
	PromptTokensDetails     PromptTokensDetails     `json:"prompt_tokens_details"`
	CompletionTokensDetails CompletionTokensDetails `json:"completion_tokens_details"`

	// ReasoningTokens is a top-level reasoning token count emitted by some
	// OpenAI-compatible providers (e.g. SGLang) that do not populate the nested
	// completion_tokens_details. Merged into llm.Usage.CompletionTokensDetails by
	// ToLLMUsage; omitempty ensures it is never emitted back to clients (UsageFromLLM
	// does not set it).
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`

	// CachedTokens is the number of tokens that were cached for Moonshot.
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

func (u *Usage) ToLLMUsage() *llm.Usage {
	if u == nil {
		return nil
	}

	usage := &llm.Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}

	if u.PromptTokensDetails != (PromptTokensDetails{}) {
		usage.PromptTokensDetails = &llm.PromptTokensDetails{
			AudioTokens:       u.PromptTokensDetails.AudioTokens,
			CachedTokens:      u.PromptTokensDetails.CachedTokens,
			WriteCachedTokens: u.PromptTokensDetails.WriteCachedTokens,
		}
	}

	// Some OpenAI-compatible providers (e.g. SGLang) report reasoning tokens as a
	// top-level `reasoning_tokens` field instead of the nested
	// completion_tokens_details. Prefer the nested value (the OpenAI standard used by
	// OpenAI/DeepSeek/Gemini), falling back to the top-level value only when the nested
	// value is absent (zero).
	reasoningTokens := u.CompletionTokensDetails.ReasoningTokens
	if reasoningTokens == 0 {
		reasoningTokens = u.ReasoningTokens
	}

	if u.CompletionTokensDetails != (CompletionTokensDetails{}) || reasoningTokens != 0 {
		usage.CompletionTokensDetails = &llm.CompletionTokensDetails{
			AudioTokens:              u.CompletionTokensDetails.AudioTokens,
			ReasoningTokens:          reasoningTokens,
			AcceptedPredictionTokens: u.CompletionTokensDetails.AcceptedPredictionTokens,
			RejectedPredictionTokens: u.CompletionTokensDetails.RejectedPredictionTokens,
		}
	}

	if (usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens == 0) && u.CachedTokens > 0 {
		if usage.PromptTokensDetails == nil {
			usage.PromptTokensDetails = &llm.PromptTokensDetails{}
		}

		usage.PromptTokensDetails.CachedTokens = u.CachedTokens
	}

	return usage
}

// UsageFromLLM creates OpenAI Usage from unified llm.Usage.
func UsageFromLLM(u *llm.Usage) *Usage {
	if u == nil {
		return nil
	}

	usage := &Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}

	if u.PromptTokensDetails != nil {
		usage.PromptTokensDetails = PromptTokensDetails{
			AudioTokens:       u.PromptTokensDetails.AudioTokens,
			CachedTokens:      u.PromptTokensDetails.CachedTokens,
			WriteCachedTokens: u.PromptTokensDetails.WriteCachedTokens,
		}
	}

	if u.CompletionTokensDetails != nil {
		usage.CompletionTokensDetails = CompletionTokensDetails{
			AudioTokens:              u.CompletionTokensDetails.AudioTokens,
			ReasoningTokens:          u.CompletionTokensDetails.ReasoningTokens,
			AcceptedPredictionTokens: u.CompletionTokensDetails.AcceptedPredictionTokens,
			RejectedPredictionTokens: u.CompletionTokensDetails.RejectedPredictionTokens,
		}
	}

	return usage
}
