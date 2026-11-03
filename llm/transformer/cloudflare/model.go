package cloudflare

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

type systemOneWireRequest struct {
	Model     string                           `json:"model"`
	State     any                              `json:"state"`
	Questions map[string]llm.SystemOneQuestion `json:"questions"`
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

// envelope is the Cloudflare API v4 response wrapper:
// {"success":true,"result":{...},"errors":[],"messages":[]}.
type envelope struct {
	Success *bool           `json:"success,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Errors  []envelopeError `json:"errors,omitempty"`
}

type envelopeError struct {
	Code    json.RawMessage `json:"code,omitempty"`
	Message string          `json:"message"`
}
