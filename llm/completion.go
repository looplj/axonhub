package llm

// CompletionRequest represents the completion request model.
// Note: Common fields like Stream and StreamOptions are in the parent Request struct, not here.
type CompletionRequest struct {
	Prompt           string           `json:"prompt"`
	Suffix           string           `json:"suffix,omitempty"`
	MaxTokens        *int64           `json:"max_tokens,omitempty"`
	Temperature      *float64         `json:"temperature,omitempty"`
	TopP             *float64         `json:"top_p,omitempty"`
	N                *int64           `json:"n,omitempty"`
	Logprobs         *int64           `json:"logprobs,omitempty"`
	Echo             *bool            `json:"echo,omitempty"`
	Stop             *Stop            `json:"stop,omitempty"`
	PresencePenalty  *float64         `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64         `json:"frequency_penalty,omitempty"`
	BestOf           *int64           `json:"best_of,omitempty"`
	LogitBias        map[string]int64 `json:"logit_bias,omitempty"`
	Seed             *int64           `json:"seed,omitempty"`
	User             string           `json:"user,omitempty"`
}

// CompletionResponse represents the completion response model.
// Note: Common fields like Usage are in the parent Response struct, not here.
type CompletionResponse struct {
	Choices []CompletionChoice `json:"choices"`
}

type CompletionChoice struct {
	Text         string           `json:"text"`
	Index        int              `json:"index"`
	Logprobs     *LogprobsContent `json:"logprobs,omitempty"`
	FinishReason *string          `json:"finish_reason,omitempty"`
}
