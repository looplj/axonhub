package httpclient

import (
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm/streams"
)

// ReasoningIncludedHeader tells Codex that upstream usage already includes
// previously generated reasoning tokens.
const ReasoningIncludedHeader = "X-Reasoning-Included"

type responseHeadersProvider interface {
	responseHeaders() http.Header
}

type responseHeadersStream struct {
	streams.Stream[*StreamEvent]
	headers http.Header
}

func (s *responseHeadersStream) responseHeaders() http.Header {
	return s.headers
}

// WithResponseHeaders attaches HTTP response metadata to a stream without
// widening the generic streams.Stream interface.
func WithResponseHeaders(stream streams.Stream[*StreamEvent], headers http.Header) streams.Stream[*StreamEvent] {
	if stream == nil || len(headers) == 0 {
		return stream
	}

	return &responseHeadersStream{
		Stream:  stream,
		headers: headers.Clone(),
	}
}

// GetResponseHeaders returns a copy of response metadata attached to a stream.
func GetResponseHeaders(stream streams.Stream[*StreamEvent]) http.Header {
	provider, ok := stream.(responseHeadersProvider)
	if !ok {
		return nil
	}

	return provider.responseHeaders().Clone()
}

// MergeForwardResponseHeaders copies the small, explicit set of upstream
// headers that are safe and meaningful at AxonHub's client boundary.
func MergeForwardResponseHeaders(dst, src http.Header) http.Header {
	forward := hasOnlyTrueHeaderValues(src, ReasoningIncludedHeader)
	if dst != nil {
		dst.Del(ReasoningIncludedHeader)
	}
	if !forward {
		return dst
	}
	if dst == nil {
		dst = make(http.Header)
	}

	dst[ReasoningIncludedHeader] = []string{"true"}

	return dst
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
