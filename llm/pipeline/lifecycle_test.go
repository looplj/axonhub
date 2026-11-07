package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type lifecycleMiddleware struct {
	DummyMiddleware

	end func()
}

func (m *lifecycleMiddleware) OnProcessEnd() { m.end() }

type lifecycleInbound struct {
	testInbound

	fail   bool
	stream bool
}

func (m *lifecycleInbound) TransformRequest(context.Context, *httpclient.Request) (*llm.Request, error) {
	if m.fail {
		return nil, errors.New("invalid inbound request")
	}
	return &llm.Request{Stream: &m.stream}, nil
}

func TestOnProcessEndLifecycle(t *testing.T) {
	for _, mode := range []string{"success", "inbound failure", "request rejection", "retry success", "retry exhausted", "stream", "panic"} {
		t.Run(mode, func(t *testing.T) {
			var ended []string
			first := &lifecycleMiddleware{end: func() { ended = append(ended, "first") }}
			last := &lifecycleMiddleware{end: func() { ended = append(ended, "last") }}
			attempts := 0
			before := OnLlmRequest("reject", func(_ context.Context, req *llm.Request) (*llm.Request, error) {
				if mode == "panic" {
					panic("hook panic")
				}
				if mode == "request rejection" {
					return nil, errors.New("rejected")
				}
				return req, nil
			})
			attempt := OnRawRequest("attempt", func(_ context.Context, req *httpclient.Request) (*httpclient.Request, error) {
				require.Empty(t, ended, "cleanup must not run between retries")
				attempts++
				if mode == "retry exhausted" || (mode == "retry success" && attempts == 1) {
					return nil, errors.New("retry")
				}
				return req, nil
			})
			executor := &testExecutor{}
			pipe := NewFactory(executor).Pipeline(
				&lifecycleInbound{fail: mode == "inbound failure", stream: mode == "stream"},
				&lifecycleOutbound{testChannelRetryableOutbound{maxRetries: 1}},
				WithRetry(0, 1, 0), WithMiddlewares(first, before, attempt, last),
			)
			if mode == "panic" {
				require.Panics(t, func() { _, _ = pipe.Process(t.Context(), &httpclient.Request{}) })
			} else {
				result, err := pipe.Process(t.Context(), &httpclient.Request{})
				if mode == "inbound failure" || mode == "request rejection" || mode == "retry exhausted" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, []string{"last", "first"}, ended)
				if mode == "stream" {
					require.True(t, result.Stream)
					require.True(t, result.EventStream.Next())
					require.Equal(t, "done", string(result.EventStream.Current().Data))
					require.False(t, result.EventStream.Next())
					require.NoError(t, result.EventStream.Err())
					require.NoError(t, result.EventStream.Close())
				}
			}
			require.Equal(t, []string{"last", "first"}, ended)
			if mode == "retry success" || mode == "retry exhausted" {
				require.Equal(t, 2, attempts)
			}
		})
	}
}

func TestOnProcessEndPanicStillCleansOtherMiddlewares(t *testing.T) {
	cleaned := false
	pipe := NewFactory(&testExecutor{}).Pipeline(&lifecycleInbound{fail: true}, &testOutbound{}, WithMiddlewares(
		&lifecycleMiddleware{end: func() { cleaned = true }},
		&lifecycleMiddleware{end: func() { panic("cleanup panic") }},
	))
	require.Panics(t, func() { _, _ = pipe.Process(t.Context(), &httpclient.Request{}) })
	require.True(t, cleaned)
}

type lifecycleOutbound struct{ testChannelRetryableOutbound }

func (o *lifecycleOutbound) TransformStream(context.Context, *httpclient.Request, streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	reason := "stop"
	return streams.SliceStream([]*llm.Response{{Choices: []llm.Choice{{Index: 0, FinishReason: &reason}}}}), nil
}

func (m *lifecycleInbound) TransformStream(_ context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.Map(stream, func(*llm.Response) *httpclient.StreamEvent { return &httpclient.StreamEvent{Data: []byte("done")} }), nil
}
