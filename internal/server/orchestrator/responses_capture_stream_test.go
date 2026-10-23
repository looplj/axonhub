package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// Responses transformers consume their queued event in Current, not Next.
type consumingResponsesStream struct {
	events []*httpclient.StreamEvent
	reads  int
}

func (s *consumingResponsesStream) Next() bool { return len(s.events) > 0 }
func (s *consumingResponsesStream) Current() *httpclient.StreamEvent {
	s.reads++
	if len(s.events) == 0 {
		return nil
	}
	event := s.events[0]
	s.events = s.events[1:]
	return event
}
func (s *consumingResponsesStream) Err() error   { return nil }
func (s *consumingResponsesStream) Close() error { return nil }

func TestResponsesCapturePreservesConsumedEvents(t *testing.T) {
	for _, kind := range []string{"provider", "accepted"} {
		t.Run(kind, func(t *testing.T) {
			events := []*httpclient.StreamEvent{
				{Data: []byte(`{"type":"response.created"}`)},
				{Data: []byte(`{"type":"response.output_text.delta","delta":"canary"}`)},
				{Data: []byte(`{"type":"response.completed","response":{"id":"synthetic","status":"completed","output":[]}}`)},
			}
			source := &consumingResponsesStream{events: events}
			continuation := newResponsesContinuation(nil, []byte(`{"input":"synthetic"}`))
			var stream streams.Stream[*httpclient.StreamEvent]
			if kind == "provider" {
				stream = &responsesProviderStream{Stream: source, completion: continuation.capture}
			} else {
				stream = &responsesAcceptedStream{Stream: source, ctx: t.Context(), continuation: continuation}
			}
			defer stream.Close()
			for _, event := range events {
				require.True(t, stream.Next())
				require.Same(t, event, stream.Current())
				require.Same(t, event, stream.Current())
			}
			require.False(t, stream.Next())
			require.Nil(t, stream.Current())
			require.Equal(t, len(events), source.reads)
		})
	}
}
