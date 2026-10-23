package httpclient

import (
	"context"
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
	headers         http.Header
	observer        streams.DeliveryAcknowledger
	waiter          streams.FinalizationWaiter
	expectedChoices int
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

	result := &responseHeadersStream{
		Stream:  stream,
		headers: headers.Clone(),
	}
	if observer, ok := stream.(streams.DeliveryAcknowledger); ok {
		result.observer = observer
	}
	if waiter, ok := stream.(streams.FinalizationWaiter); ok {
		result.waiter = waiter
	}
	if choices, ok := stream.(interface{ ExpectedStreamChoices() int }); ok {
		result.expectedChoices = choices.ExpectedStreamChoices()
	}
	return result
}

func (s *responseHeadersStream) AcknowledgeDelivery() {
	if s.observer != nil {
		s.observer.AcknowledgeDelivery()
	}
}

func (s *responseHeadersStream) DeliveryAcknowledged() bool {
	return s.observer != nil && s.observer.DeliveryAcknowledged()
}

func (s *responseHeadersStream) WaitForFinalization() {
	if s.waiter != nil {
		s.waiter.WaitForFinalization()
	}
}

func (s *responseHeadersStream) Deliver(ctx context.Context, write func() (bool, error)) error {
	boundary, ok := s.Stream.(streams.DeliveryBoundary)
	if ok {
		return boundary.Deliver(ctx, write)
	}
	terminal, err := write()
	if err == nil && terminal {
		s.AcknowledgeDelivery()
	}
	return err
}

func (s *responseHeadersStream) ExpectedStreamChoices() int { return s.expectedChoices }

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
		for key := range dst {
			if strings.EqualFold(key, ReasoningIncludedHeader) {
				delete(dst, key)
			}
		}
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
