package responses

import (
	"github.com/looplj/axonhub/internal/llm"
)

type Usage struct {
	InputTokens       int64 `json:"input_tokens"`
	InputTokenDetails struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokens       int64 `json:"output_tokens"`
	OutputTokenDetails struct {
		ReasoningTokens int64 `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	TotalTokens int64 `json:"total_tokens"`
}

func (u *Usage) ToUsage() *llm.Usage {
	return &llm.Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.TotalTokens,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens: u.InputTokenDetails.CachedTokens,
		},
		CompletionTokensDetails: &llm.CompletionTokensDetails{
			ReasoningTokens: u.OutputTokenDetails.ReasoningTokens,
		},
	}
}
