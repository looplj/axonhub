package pipeline

import (
	"net/http"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func forwardStreamResponseHeaders(stream streams.Stream[*httpclient.StreamEvent]) http.Header {
	return httpclient.ForwardResponseHeaders(httpclient.GetUpstreamResponseHeaders(stream))
}
