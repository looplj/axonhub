package jina

import "github.com/looplj/axonhub/internal/llm"

type RerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            *int     `json:"top_n,omitempty"`
	ReturnDocuments *bool    `json:"return_documents,omitempty"`
}

type RerankResponse struct {
	Model   string         `json:"model"`
	Object  string         `json:"object"`
	Results []RerankResult `json:"results"`
	Usage   *RerankUsage   `json:"usage,omitempty"`
}

type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`

	// Document is the original document text (optional, can be omitted to save bandwidth).
	Document *RerankDocument `json:"document,omitempty"`
}

type RerankDocument struct {
	Text string `json:"text"`
}

type RerankUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type EmbeddingRequest struct {
	Input          llm.EmbeddingInput `json:"input"`
	Model          string             `json:"model"`
	Task           string             `json:"task,omitempty"`
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
