package llm

import "encoding/json"

// DecisionsRequest preserves the original Decisions request body while exposing
// only the fields needed for routing and validation.
type DecisionsRequest struct {
	Body      []byte            `json:"-"`
	Model     string            `json:"model"`
	Input     json.RawMessage   `json:"input"`
	Questions []json.RawMessage `json:"questions"`
}

// DecisionsUsage contains the only documented usage value mapped by AxonHub.
type DecisionsUsage struct {
	InputTokens int64 `json:"input_tokens"`
}

// DecisionsResponse preserves the original Decisions response body while
// exposing answers and the documented input-token usage value.
type DecisionsResponse struct {
	Body    []byte            `json:"-"`
	Model   string            `json:"model"`
	Answers []json.RawMessage `json:"answers"`
	Usage   *DecisionsUsage   `json:"usage,omitempty"`
}
