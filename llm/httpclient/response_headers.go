package httpclient

import (
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm/streams"
)

// ReasoningIncludedHeader tells a compatible client that usage already
// includes previously generated reasoning tokens.
const ReasoningIncludedHeader = "X-Reasoning-Included"

type upstreamResponseHeadersProvider interface {
	UpstreamResponseHeaders() http.Header
}

// GetUpstreamResponseHeaders returns a copy of optional HTTP handshake headers
// carried by a streaming transport. A plain streams.Stream has no metadata,
// so callers must handle nil.
func GetUpstreamResponseHeaders(stream streams.Stream[*StreamEvent]) http.Header {
	if stream == nil {
		return nil
	}

	provider, ok := stream.(upstreamResponseHeadersProvider)
	if !ok {
		return nil
	}

	return provider.UpstreamResponseHeaders().Clone()
}

// ForwardResponseHeaders returns the small, explicit set of upstream response
// headers that are safe and meaningful at AxonHub's client boundary.
//
// Do not replace this with a full header copy: upstream responses can contain
// hop-by-hop, credential-related, or provider-private headers.
func ForwardResponseHeaders(headers http.Header) http.Header {
	if !hasOnlyTrueHeaderValues(headers, ReasoningIncludedHeader) {
		return nil
	}

	return http.Header{ReasoningIncludedHeader: []string{"true"}}
}

func hasOnlyTrueHeaderValues(headers http.Header, name string) bool {
	found := false
	for key, values := range headers {
		if !strings.EqualFold(key, name) {
			continue
		}

		for _, value := range values {
			found = true
			if !strings.EqualFold(strings.TrimSpace(value), "true") {
				return false
			}
		}
	}

	return found
}
