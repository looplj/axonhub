package llm

// RerankRequest represents a rerank request.
type RerankRequest struct {
	// Model is the model ID used for reranking.
	Model string `json:"model" binding:"required"`

	// Query is the search query to compare documents against.
	Query string `json:"query" binding:"required"`

	// Documents is the list of documents to rerank.
	Documents []string `json:"documents" binding:"required,min=1"`

	// TopN is the number of most relevant documents to return. Optional.
	TopN *int `json:"top_n,omitempty"`
}

// RerankResponse represents the response from a rerank request.
type RerankResponse struct {
	// Results contains the reranked documents with relevance scores.
	Results []RerankResult `json:"results"`

	// Usage contains token usage information if available.
	Usage *RerankUsage `json:"usage,omitempty"`
}

// RerankResult represents a single reranked document result.
type RerankResult struct {
	// Index is the index of the document in the original list.
	Index int `json:"index"`

	// RelevanceScore is the relevance score of the document to the query.
	RelevanceScore float64 `json:"relevance_score"`

	// Document is the original document text (optional, can be omitted to save bandwidth).
	Document string `json:"document,omitempty"`
}

// RerankUsage represents token usage for rerank requests.
type RerankUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
