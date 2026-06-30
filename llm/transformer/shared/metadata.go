package shared

import (
	"maps"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
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

// PropagateStreamMetadata wraps a stream of llm.Response chunks so that the
// request's TransformerMetadata is attached to the first real chunk. This lets
// cross-protocol fields (e.g. the namespace tool map) survive the streaming
// round-trip: the outbound stream carries the metadata, and the inbound stream's
// mergeTransformerMetadata picks it up before any function_call events arrive.
//
// Each outbound's TransformStream should wrap its output stream with this when
// the request carries TransformerMetadata.
func PropagateStreamMetadata(stream streams.Stream[*llm.Response], metadata map[string]any) streams.Stream[*llm.Response] {
	if metadata == nil {
		return stream
	}
	return &metadataStream{
		source:   stream,
		metadata: metadata,
	}
}

// metadataStream injects request TransformerMetadata onto the first emitted
// chunk, then passes through unchanged.
type metadataStream struct {
	source   streams.Stream[*llm.Response]
	metadata map[string]any
	emitted  bool
}

func (s *metadataStream) Next() bool {
	return s.source.Next()
}

func (s *metadataStream) Current() *llm.Response {
	resp := s.source.Current()
	if !s.emitted && resp != nil && resp != llm.DoneResponse {
		if resp.TransformerMetadata == nil {
			resp.TransformerMetadata = maps.Clone(s.metadata)
		} else {
			for k, v := range s.metadata {
				if _, exists := resp.TransformerMetadata[k]; !exists {
					resp.TransformerMetadata[k] = v
				}
			}
		}
		s.emitted = true
	}
	return resp
}

func (s *metadataStream) Err() error {
	return s.source.Err()
}

func (s *metadataStream) Close() error {
	return s.source.Close()
}
