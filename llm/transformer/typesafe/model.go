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
// Success stays raw so its presence can be told apart from its value: a plain
// SystemOne response omits the key, while an envelope always carries one — even
// when the value is an explicit null.
type systemOneWireEnvelope struct {
	Success  json.RawMessage `json:"success"`
	Result   json.RawMessage `json:"result"`
	Error    json.RawMessage `json:"error"`
	Errors   json.RawMessage `json:"errors"`
	Messages json.RawMessage `json:"messages"`
}
