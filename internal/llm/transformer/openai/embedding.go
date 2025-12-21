package openai

import "github.com/looplj/axonhub/internal/llm"

type EmbeddingRequest struct {
	Input          llm.EmbeddingInput `json:"input"`
	Model          string             `json:"model"`
	EncodingFormat string             `json:"encoding_format,omitempty"`
	Dimensions     *int               `json:"dimensions,omitempty"`
	User           string             `json:"user,omitempty"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

type EmbeddingData struct {
	Object    string        `json:"object"`
	Embedding llm.Embedding `json:"embedding"`
	Index     int           `json:"index"`
}

type EmbeddingUsage struct {
	PromptTokens int64 `json:"prompt_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}
