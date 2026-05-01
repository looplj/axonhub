package openai

import "github.com/looplj/axonhub/llm"

type CompletionRequest struct {
	Model            string             `json:"model"`
	Prompt           string             `json:"prompt"`
	Suffix           string             `json:"suffix,omitempty"`
	MaxTokens        *int64             `json:"max_tokens,omitempty"`
	Temperature      *float64           `json:"temperature,omitempty"`
	TopP             *float64           `json:"top_p,omitempty"`
	N                *int64             `json:"n,omitempty"`
	Stream           *bool              `json:"stream,omitempty"`
	StreamOptions    *llm.StreamOptions `json:"stream_options,omitempty"`
	Logprobs         *int64             `json:"logprobs,omitempty"`
	Echo             *bool              `json:"echo,omitempty"`
	Stop             *Stop              `json:"stop,omitempty"`
	PresencePenalty  *float64           `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64           `json:"frequency_penalty,omitempty"`
	BestOf           *int64             `json:"best_of,omitempty"`
	LogitBias        map[string]int64   `json:"logit_bias,omitempty"`
	Seed             *int64             `json:"seed,omitempty"`
	User             string             `json:"user,omitempty"`
}

type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   CompletionUsage    `json:"usage"`
}

type CompletionChoice struct {
	Text         string               `json:"text"`
	Index        int                  `json:"index"`
	Logprobs     *llm.LogprobsContent `json:"logprobs,omitempty"`
	FinishReason *string              `json:"finish_reason,omitempty"`
}

type CompletionUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}
