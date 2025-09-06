package openai

import (
	"github.com/looplj/axonhub/internal/llm"
)

type Response struct {
	llm.Response

	Usage *Usage `json:"usage"`
}

func (r *Response) ToLLMResponse() *llm.Response {
	if r.Usage != nil {
		r.Response.Usage = r.Usage.ToLLMUsage()
	}

	return &r.Response
}
