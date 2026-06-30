package shared

import (
	"maps"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// PropagateRequestMetadata copies the llm.Request's TransformerMetadata onto the
// httpclient.Request so it survives the round-trip to the upstream and back.
// Each outbound's TransformRequest should call this after building the
// httpclient.Request, mirroring the pattern established by the responses
// outbound (outbound.go:322). Without this, protocol-specific metadata
// (e.g. the namespace tool map, cache_control, reasoning flags) is lost
// before the request even reaches the upstream.
func PropagateRequestMetadata(httpReq *httpclient.Request, llmReq *llm.Request) {
	if llmReq != nil && llmReq.TransformerMetadata != nil {
		httpReq.TransformerMetadata = llmReq.TransformerMetadata
	}
}

// MergeResponseMetadata clones the request's TransformerMetadata (carried on
// httpResp.Request) into the llm.Response so the inbound transformer can
// restore protocol-specific fields on the return trip. Existing response-level
// metadata (e.g. citations set by the chat outbound) is preserved — request
// keys are only added when absent, never overwritten.
//
// Each outbound's TransformResponse should call this before returning, mirroring
// the pattern established by the responses outbound (outbound.go:414-415).
func MergeResponseMetadata(llmResp *llm.Response, httpResp *httpclient.Response) {
	if httpResp == nil || httpResp.Request == nil || httpResp.Request.TransformerMetadata == nil {
		return
	}
	if llmResp.TransformerMetadata == nil {
		llmResp.TransformerMetadata = maps.Clone(httpResp.Request.TransformerMetadata)
		return
	}
	for k, v := range httpResp.Request.TransformerMetadata {
		if _, exists := llmResp.TransformerMetadata[k]; !exists {
			llmResp.TransformerMetadata[k] = v
		}
	}
}
