package responses

import (
	"github.com/looplj/axonhub/internal/llm"
)

type Usage struct {
	InputTokens       int `json:"input_tokens"`
	InputTokenDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokens       int `json:"output_tokens"`
	OutputTokenDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	TotalTokens int `json:"total_tokens"`
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
