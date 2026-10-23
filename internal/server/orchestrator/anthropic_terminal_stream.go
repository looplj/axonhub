package orchestrator

import (
	"context"
	"sync"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

// closeCompletedAnthropicStream treats message_stop as the upstream protocol
// boundary. It runs before capture/persistence so both pass-through consumers
// and the normalized accounting pipeline see EOF without canceling delivery.
func closeCompletedAnthropicStream(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawStream("anthropic-message-stop", func(_ context.Context, source streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
		request := outbound.state.RawProviderRequest
		if request == nil || request.APIFormat != string(llm.APIFormatAnthropicMessage) {
			return source, nil
		}
		return httpclient.WithResponseHeaders(&anthropicTerminalStream{Stream: source}, httpclient.GetResponseHeaders(source)), nil
	})
}

// anthropicTerminalStream closes only the provider transport after a complete
// message. The outer stream still owns client delivery and usage finalization.
type anthropicTerminalStream struct {
	streams.Stream[*httpclient.StreamEvent]

	current   *httpclient.StreamEvent
	complete  bool
	closeOnce sync.Once
	closeErr  error
}

// Next forwards message_stop once and never waits for HTTP EOF after it.
func (s *anthropicTerminalStream) Next() bool {
	if s.complete || !s.Stream.Next() {
		return false
	}
	s.current = s.Stream.Current()
	if s.current != nil && gjson.GetBytes(s.current.Data, "type").String() == "message_stop" {
		// Keep the final event stable even if closing releases provider buffers.
		copy := *s.current
		copy.Data = append([]byte(nil), s.current.Data...)
		s.current = &copy
		s.complete = true
		_ = s.Close()
	}
	return true
}

// Current returns the event consumed by Next without advancing the provider.
func (s *anthropicTerminalStream) Current() *httpclient.StreamEvent { return s.current }

// Err preserves failures before message_stop; transport-close errors after
// semantic completion are cleanup, not an incomplete generation.
func (s *anthropicTerminalStream) Err() error {
	if s.complete {
		return nil
	}
	return s.Stream.Err()
}

// Close releases the provider connection exactly once, without canceling the
// request lifecycle that guards downstream delivery.
func (s *anthropicTerminalStream) Close() error {
	s.closeOnce.Do(func() { s.closeErr = s.Stream.Close() })
	return s.closeErr
}
