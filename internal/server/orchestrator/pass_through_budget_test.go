package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestPassThroughAnthropicStream_RawBudget(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
		data  string
	}{
		{name: "filtered event count", count: 1025, data: `{"type":"ping"}`},
		{name: "filtered event bytes", count: 4, data: `{"type":"ping","padding":"` + strings.Repeat("x", 17*1024*1024) + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "filtered event bytes" {
				events := make([]*httpclient.StreamEvent, tc.count)
				for i := range events {
					events[i] = &httpclient.StreamEvent{Type: "ping", Data: []byte(tc.data)}
				}
				recorder := &rawEventRecorder{source: streams.SliceStream(events)}
				for range tc.count - 1 {
					require.True(t, recorder.Next())
				}
				require.False(t, recorder.Next())
				require.ErrorIs(t, recorder.Err(), pipeline.ErrPreCommitBufferExceeded)
				return
			}

			// Given: filtered raw pings exceed a pre-attachment budget.
			events := make([]*httpclient.StreamEvent, 0, tc.count+9)
			for range tc.count {
				events = append(events, &httpclient.StreamEvent{Type: "ping", Data: []byte(tc.data)})
			}
			events = append(events, anthropicEmptyThinkingEvents(0)...)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, event := range events {
					if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data); err != nil {
						return
					}
				}
			}))
			defer server.Close()
			process, request := rawBudgetPipeline(t, httpclient.NewHttpClientWithClient(server.Client()), server.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// When: retry pre-read consumes the provider prefix.
			result, err := process(ctx, request)
			if result != nil && result.EventStream != nil {
				defer result.EventStream.Close()
			}

			// Then: overflow fails explicitly rather than attaching or stalling.
			require.ErrorContains(t, err, "pass-through pre-attachment raw event budget exceeded")
			require.ErrorIs(t, err, pipeline.ErrPreCommitBufferExceeded)
		})
	}
}

func recordedEvents(count int, event *httpclient.StreamEvent) *rawEventRecorder {
	events := make([]*httpclient.StreamEvent, count)
	for i := range events {
		events[i] = event
	}

	return &rawEventRecorder{source: streams.SliceStream(events)}
}

func TestRawEventRecorder_ExactBudgets(t *testing.T) {
	t.Run("count before attach", func(t *testing.T) {
		recorder := recordedEvents(1025, &httpclient.StreamEvent{})
		for range 1024 {
			require.True(t, recorder.Next())
		}
		require.False(t, recorder.Next())
		require.ErrorIs(t, recorder.Err(), pipeline.ErrPreCommitBufferExceeded)
	})
	t.Run("count is not bounded after attach", func(t *testing.T) {
		recorder := recordedEvents(1025, &httpclient.StreamEvent{})
		for range 1024 {
			require.True(t, recorder.Next())
		}
		require.Len(t, recorder.take(), 1024)
		require.True(t, recorder.Next())
		require.NoError(t, recorder.Err())
	})
	t.Run("bytes include type and ID", func(t *testing.T) {
		recorder := &rawEventRecorder{source: streams.SliceStream([]*httpclient.StreamEvent{
			{Type: "ping", LastEventID: "1", Data: make([]byte, 64*1024*1024-5)},
			{Data: []byte("x")},
		})}
		require.True(t, recorder.Next())
		require.False(t, recorder.Next())
		require.ErrorIs(t, recorder.Err(), pipeline.ErrPreCommitBufferExceeded)
		require.Len(t, recorder.take(), 1)
	})
}

func TestRawEventRecorder_AttachedBudgets(t *testing.T) {
	t.Run("single event size", func(t *testing.T) {
		recorder := recordedEvents(1, &httpclient.StreamEvent{Data: make([]byte, 32*1024*1024+1)})
		recorder.take()
		require.False(t, recorder.Next())
		require.ErrorIs(t, recorder.Err(), pipeline.ErrPreCommitBufferExceeded)
	})
	t.Run("retained bytes until taken", func(t *testing.T) {
		recorder := recordedEvents(5, &httpclient.StreamEvent{Data: make([]byte, 32*1024*1024)})
		recorder.take()
		for range 4 {
			require.True(t, recorder.Next())
		}
		require.False(t, recorder.Next())
		require.ErrorIs(t, recorder.Err(), pipeline.ErrPreCommitBufferExceeded)
	})
	t.Run("taking releases bytes", func(t *testing.T) {
		recorder := recordedEvents(5, &httpclient.StreamEvent{Data: make([]byte, 32*1024*1024)})
		recorder.take()
		for range 4 {
			require.True(t, recorder.Next())
		}
		require.Len(t, recorder.take(), 4)
		require.True(t, recorder.Next())
		require.NoError(t, recorder.Err())
	})
}

func TestPassThroughAnthropicStream_HTTPRetryIsolation(t *testing.T) {
	var attempts atomic.Int32
	success := anthropicEmptyThinkingEvents(200)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if attempts.Add(1) == 1 {
			for range 80 {
				fmt.Fprint(w, "event: ping\ndata: {\"type\":\"ping\",\"attempt\":\"abandoned\"}\n\n")
			}
			fmt.Fprint(w, "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"retry me\"}}\n\n")
			return
		}
		for _, event := range success {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
		}
	}))
	defer server.Close()
	process, request := rawBudgetPipeline(t, httpclient.NewHttpClientWithClient(server.Client()), server.URL)
	events := processPassThroughStream(t, process, request)
	require.EqualValues(t, 2, attempts.Load())
	require.Len(t, events, len(success))
	for i, event := range events {
		require.Equal(t, success[i].Type, event.Type)
		require.Equal(t, success[i].Data, event.Data)
	}
}

func TestPassThroughAnthropicStream_HTTPCancel(t *testing.T) {
	readStarted := make(chan struct{})
	upstreamStopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for range 80 {
			fmt.Fprint(w, "event: ping\ndata: {\"type\":\"ping\"}\n\n")
		}
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(upstreamStopped)
	}))
	defer server.Close()
	executor := &rawCancelObserver{Executor: httpclient.NewHttpClientWithClient(server.Client()), observed: readStarted}
	process, request := rawBudgetPipeline(t, executor, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := process(ctx, request)
		finished <- err
	}()
	select {
	case <-readStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream did not start")
	}
	cancel()
	select {
	case err := <-finished:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline did not stop")
	}
	select {
	case <-upstreamStopped:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream connection did not close")
	}
}

type rawCancelObserver struct {
	pipeline.Executor

	observed chan struct{}
}

func (e *rawCancelObserver) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	stream, err := e.Executor.DoStream(ctx, request)
	if err != nil {
		return nil, err
	}
	count := 0
	return streams.Map(stream, func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
		count++
		if count == 80 {
			close(e.observed)
		}
		return event
	}), nil
}

func rawBudgetPipeline(t *testing.T, executor pipeline.Executor, endpoint string) (func(context.Context, *httpclient.Request) (*pipeline.Result, error), *httpclient.Request) {
	t.Helper()
	provider, err := anthropic.NewOutboundTransformer(endpoint, "test-key")
	require.NoError(t, err)
	channel := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "anthropic", Settings: &objects.ChannelSettings{
			PassThroughBody:        lo.ToPtr(true),
			RetryableErrorPatterns: []objects.RetryableErrorPattern{{Pattern: "overloaded_error"}},
		},
	}, Outbound: provider}
	state := &PersistenceState{
		OriginalModel: "claude-opus-4-7",
		ChannelModelsCandidates: []*ChannelModelsCandidate{{
			Channel:   channel,
			Models:    []biz.ChannelModelEntry{{RequestModel: "claude-opus-4-7", ActualModel: "claude-opus-4-7", Source: "direct"}},
			APIFormat: string(llm.APIFormatAnthropicMessage),
		}},
	}
	inbound, outbound := NewPersistentTransformers(state, anthropic.NewInboundTransformer())
	pipe := pipeline.NewFactory(executor).Pipeline(inbound, outbound,
		pipeline.WithRetry(0, 1, 0),
		pipeline.WithMiddlewares(applyPassThroughStream(outbound), applyPassThroughRequestBody(outbound, nil), captureRawProviderStream(outbound)),
	)
	return pipe.Process, &httpclient.Request{
		Method: http.MethodPost, URL: "/v1/messages", ContentType: "application/json",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(`{"model":"claude-opus-4-7","stream":true,"max_tokens":1024,"messages":[{"role":"user","content":"Reply with OK"}]}`),
	}
}
