package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type barrierDeliverySSEStream struct {
	*deliverySSEStream

	mu sync.Mutex
}

func (s *barrierDeliverySSEStream) Deliver(ctx context.Context, write func() (bool, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	terminal, err := write()
	if err == nil && terminal {
		s.AcknowledgeDelivery()
	}
	return err
}

func TestStreamDeliverySSESemanticBoundaries(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, complete := range []bool{false, true} {
			name := map[bool]string{false: "ordinary", true: "heartbeat"}[heartbeat] + "/" + map[bool]string{false: "partial_choices", true: "all_choices"}[complete]
			t.Run(name, func(t *testing.T) {
				// Given: two expected choices, with usage unable to establish completion.
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				trace := &deliverySSETrace{}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
				c.Writer = &deliverySSEWriter{ResponseWriter: c.Writer, trace: trace}
				events := []*httpclient.StreamEvent{
					{Data: []byte(`{"choices":[{"index":0,"finish_reason":"stop"}]}`)},
					{Data: []byte(`{"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":34}}`)},
				}
				if complete {
					events = append(events, &httpclient.StreamEvent{Data: []byte(`{"choices":[{"index":1,"finish_reason":"stop"}]}`)})
				}
				s := &deliverySSEStream{Stream: streams.SliceStream(events), trace: trace, choices: 2, onAck: cancel}
				// When
				runDeliverySSE(c, s, heartbeat)
				// Then: all-choice delivery survives cancellation triggered after acknowledgement.
				require.Equal(t, complete, s.DeliveryAcknowledged())
				if complete {
					require.ErrorIs(t, ctx.Err(), context.Canceled)
					require.Equal(t, 1, countDeliveryAcks(trace.steps))
				} else {
					require.NotContains(t, trace.steps, "ack")
				}
				t.Logf("all-choice delivery order: %v", trace.steps)
			})
		}
	}
}

func TestStreamDeliverySSEAdapterFactory(t *testing.T) {
	// Given: an API factory decorates the actual stream with a capability-erasing mapper.
	trace := &deliverySSETrace{}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Writer = &deliverySSEWriter{ResponseWriter: c.Writer, trace: trace}
	s := &deliverySSEStream{trace: trace, choices: 1, Stream: streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte(`{"choices":[{"index":0,"finish_reason":"stop"}]}`)}})}
	h := &ChatCompletionHandlers{streamAdapterFactory: func(_ context.Context, stream streams.Stream[*httpclient.StreamEvent], _ *biz.SystemService) (streams.Stream[*httpclient.StreamEvent], StreamErrorEncoder) {
		return streams.MapErr(stream, func(e *httpclient.StreamEvent) (*httpclient.StreamEvent, error) { return e, nil }), encodeStreamError(FormatStreamError)
	}}
	// When
	h.writeSSEStream(c, s)
	// Then
	require.True(t, s.DeliveryAcknowledged())
	require.Equal(t, 1, countDeliveryAcks(trace.steps))
}

func TestStreamDeliverySSEFlushCancellationDoesNotEraseSuccessfulDelivery(t *testing.T) {
	// Given: cancellation is raised by the terminal flush before the transport
	// returns to the acknowledgement step.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	trace := &deliverySSETrace{}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	w := &deliverySSEWriter{ResponseWriter: c.Writer, trace: trace}
	w.afterFlush = cancel
	c.Writer = w
	s := &barrierDeliverySSEStream{deliverySSEStream: &deliverySSEStream{
		Stream: streams.SliceStream([]*httpclient.StreamEvent{
			{Data: []byte(`{"choices":[{"index":0,"finish_reason":"stop"}]}`)},
		}),
		trace:   trace,
		choices: 1,
	}}

	// When
	runDeliverySSE(c, s, false)

	// Then: a successful terminal flush remains the delivery linearization point.
	require.True(t, s.DeliveryAcknowledged())
}
