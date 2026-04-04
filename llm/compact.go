package llm

// CompactRequest represents the unified compact request model.
// Used by the Responses API /responses/compact endpoint.
type CompactRequest struct {
	// Input is the list of messages to compact.
	Input []Message `json:"input,omitempty"`

	// Instructions is a system (or developer) message inserted into the model's context.
	Instructions string `json:"instructions,omitempty"`

	// PreviousResponseID is not supported for compact requests yet.
	// PreviousResponseID string `json:"previous_response_id,omitempty"`

	// PromptCacheKey is a key to use when reading from or writing to the prompt cache.
	PromptCacheKey string `json:"prompt_cache_key,omitempty"`
}

// CompactResponse represents the unified compact response model.
type CompactResponse struct {
	// ID is the unique identifier for the compacted response.
	ID string `json:"id"`

	// CreatedAt is the Unix timestamp (in seconds) when the compacted conversation was created.
	CreatedAt int64 `json:"created_at"`

	// Object is the object type. Always "response.compaction".
	Object string `json:"object"`

	// Instructions is the system (or developer) message used for the compaction pass.
	Instructions string `json:"instructions,omitempty"`

	// Output is the ordered compacted output messages.
	Output []Message `json:"output"`

	// Usage is the token accounting for the compaction pass.
	Usage *Usage `json:"usage,omitempty"`
}
