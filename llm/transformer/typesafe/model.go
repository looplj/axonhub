package typesafe

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

type systemOneWireRequest struct {
	Model     string                           `json:"model"`
	State     any                              `json:"state"`
	Questions map[string]llm.SystemOneQuestion `json:"questions"`
	Stream    *bool                            `json:"stream,omitempty"`
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

// systemOneWireEnvelope models the universal API envelope used by gateways such
// as Cloudflare Workers AI, which nests the provider payload under "result".
// Success is a pointer so a plain SystemOne response (no envelope) can be told
// apart from an envelope that explicitly reports success:false.
type systemOneWireEnvelope struct {
	Success  *bool           `json:"success"`
	Result   json.RawMessage `json:"result"`
	Error    json.RawMessage `json:"error"`
	Errors   json.RawMessage `json:"errors"`
	Messages json.RawMessage `json:"messages"`
}
