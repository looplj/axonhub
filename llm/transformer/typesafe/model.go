package typesafe

import "github.com/looplj/axonhub/llm"

type systemOneWireRequest struct {
	Model     string                          `json:"model"`
	State     any                             `json:"state"`
	Questions map[string]llm.SystemOneQuestion `json:"questions"`
	Stream    *bool                           `json:"stream,omitempty"`
}

type systemOneWireUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type systemOneWireResponse struct {
	Model   string                         `json:"model"`
	Answers map[string]llm.SystemOneAnswer `json:"answers"`
	Usage   *systemOneWireUsage            `json:"usage,omitempty"`
}
