package llm

// RerankRequest represents a rerank request.
type RerankRequest struct {
	// Query is the search query to compare documents against.
	Query string `json:"query" binding:"required"`

	// Documents is the list of documents to rerank.
	Documents []string `json:"documents" binding:"required,min=1"`

	// TopN is the number of most relevant documents to return. Optional.
	TopN *int `json:"top_n,omitempty"`

	// ReturnDocuments is a flag to indicate whether to return the original documents in the response. Optional.
	ReturnDocuments *bool `json:"return_documents,omitempty"`
}

// RerankResponse represents the response from a rerank request.
type RerankResponse struct {
	// Object is the object type, typically "list".
	Object string `json:"object"`

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
	Document *RerankDocument `json:"document,omitempty"`
}

type RerankDocument struct {
	Text string `json:"text"`
}

// RerankUsage represents token usage for rerank requests.
type RerankUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
