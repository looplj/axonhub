package pipeline

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type mockInbound struct {
	transformer.Inbound

	transformRequest      func(context.Context, *httpclient.Request) (*llm.Request, error)
	transformStream       func(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error)
	aggregateStreamChunks func(context.Context, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error)
}

func (m *mockInbound) TransformRequest(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
	if m.transformRequest != nil {
		return m.transformRequest(ctx, req)
	}

	return &llm.Request{}, nil
}

func (m *mockInbound) TransformResponse(ctx context.Context, resp *llm.Response) (*httpclient.Response, error) {
	return &httpclient.Response{}, nil
}

func (m *mockInbound) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	if m.transformStream != nil {
		return m.transformStream(ctx, stream)
	}

	// Pass-through: convert each llm.Response to an empty StreamEvent
	return streams.Map(stream, func(resp *llm.Response) *httpclient.StreamEvent {
		return &httpclient.StreamEvent{}
	}), nil
}

func (m *mockInbound) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	if m.aggregateStreamChunks != nil {
		return m.aggregateStreamChunks(ctx, chunks)
	}

	return []byte(`{}`), llm.ResponseMeta{}, nil
}

type mockOutbound struct {
	transformer.Outbound

	apiFormat             llm.APIFormat
	hasMoreChannels       func() bool
	nextChannel           func(context.Context) error
	canRetry              func(error) bool
	prepareForRetry       func(context.Context) error
	transformRequest      func(context.Context, *llm.Request) (*httpclient.Request, error)
	transformResponse     func(context.Context, *httpclient.Response) (*llm.Response, error)
	transformStream       func(context.Context, *httpclient.Request, streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error)
	transformError        func(context.Context, *httpclient.Error) *llm.ResponseError
	aggregateStreamChunks func(context.Context, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error)
}

func (m *mockOutbound) APIFormat() llm.APIFormat { return m.apiFormat }
func (m *mockOutbound) TransformRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	if m.transformRequest != nil {
		return m.transformRequest(ctx, req)
	}

	return &httpclient.Request{}, nil
}

func (m *mockOutbound) TransformResponse(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
	if m.transformResponse != nil {
		return m.transformResponse(ctx, resp)
	}

	return &llm.Response{}, nil
}

func (m *mockOutbound) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	if m.transformStream != nil {
		return m.transformStream(ctx, req, stream)
	}

	return nil, nil
}

func (m *mockOutbound) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	if m.transformError != nil {
		return m.transformError(ctx, err)
	}

	return &llm.ResponseError{}
}

func (m *mockOutbound) HasMoreChannels() bool {
	if m.hasMoreChannels != nil {
		return m.hasMoreChannels()
	}

	return false
}

func (m *mockOutbound) NextChannel(ctx context.Context) error {
	if m.nextChannel != nil {
		return m.nextChannel(ctx)
	}

	return nil
}

func (m *mockOutbound) CanRetry(err error) bool {
	if m.canRetry != nil {
		return m.canRetry(err)
	}

	return false
}

func (m *mockOutbound) PrepareForRetry(ctx context.Context) error {
	if m.prepareForRetry != nil {
		return m.prepareForRetry(ctx)
	}

	return nil
}

type mockExecutor struct {
	do       func(context.Context, *httpclient.Request) (*httpclient.Response, error)
	doStream func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error)
}

func (m *mockExecutor) Do(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
	return m.do(ctx, req)
}

func (m *mockExecutor) DoStream(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	if m.doStream != nil {
		return m.doStream(ctx, req)
	}

	return nil, nil
}

type llmErrorAfterStream struct {
	items   []*llm.Response
	index   int
	current *llm.Response
	err     error
}

func (s *llmErrorAfterStream) Next() bool {
	if s.index >= len(s.items) {
		return false
	}

	s.current = s.items[s.index]
	s.index++

	return true
}

func (s *llmErrorAfterStream) Current() *llm.Response {
	return s.current
}

func (s *llmErrorAfterStream) Err() error {
	if s.index >= len(s.items) {
		return s.err
	}

	return nil
}

func (s *llmErrorAfterStream) Close() error {
	return nil
}

type mockMiddleware struct {
	Middleware

	errorCalls int
}

func (m *mockMiddleware) OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	return request, nil
}

func (m *mockMiddleware) OnInboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	return response, nil
}

func (m *mockMiddleware) OnInboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	return stream, nil
}

func (m *mockMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	return request, nil
}

func (m *mockMiddleware) OnOutboundRawError(ctx context.Context, err error) {
	m.errorCalls++
}

func (m *mockMiddleware) OnOutboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	return response, nil
}

func (m *mockMiddleware) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	return response, nil
}

func (m *mockMiddleware) OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	return stream, nil
}

func (m *mockMiddleware) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return stream, nil
}

func TestPipeline_Process_RetryLogic(t *testing.T) {
	ctx := context.Background()
	inbound := &mockInbound{}

	t.Run("SameChannelRetrySuccess", func(t *testing.T) {
		execCalls := 0
		executor := &mockExecutor{
			do: func(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				execCalls++
				if execCalls == 1 {
					return nil, errors.New("temporary error")
				}

				return &httpclient.Response{}, nil
			},
		}

		prepareCalls := 0
		outbound := &mockOutbound{
			canRetry: func(err error) bool { return true },
			prepareForRetry: func(ctx context.Context) error {
				prepareCalls++
				return nil
			},
		}

		mw := &mockMiddleware{}
		p := &pipeline{
			Executor:              executor,
			Inbound:               inbound,
			Outbound:              outbound,
			maxSameChannelRetries: 2,
			middlewares:           []Middleware{mw},
		}

		res, err := p.Process(ctx, &httpclient.Request{})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, 2, execCalls)
		require.Equal(t, 1, prepareCalls)
		require.Equal(t, 1, mw.errorCalls)
	})

	t.Run("CrossChannelRetrySuccess", func(t *testing.T) {
		execCalls := 0
		executor := &mockExecutor{
			do: func(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				execCalls++
				if execCalls == 1 {
					return nil, errors.New("channel error")
				}

				return &httpclient.Response{}, nil
			},
		}

		switchCalls := 0
		outbound := &mockOutbound{
			canRetry:        func(err error) bool { return false }, // No same channel retry
			hasMoreChannels: func() bool { return true },
			nextChannel: func(ctx context.Context) error {
				switchCalls++
				return nil
			},
		}

		p := &pipeline{
			Executor:          executor,
			Inbound:           inbound,
			Outbound:          outbound,
			maxChannelRetries: 1,
		}

		res, err := p.Process(ctx, &httpclient.Request{})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, 2, execCalls)
		require.Equal(t, 1, switchCalls)
	})

	t.Run("MixedRetrySuccess", func(t *testing.T) {
		execCalls := 0
		executor := &mockExecutor{
			do: func(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				execCalls++
				if execCalls < 4 { // Fail 3 times
					return nil, errors.New("fail")
				}

				return &httpclient.Response{}, nil
			},
		}

		prepareCalls := 0
		switchCalls := 0
		outbound := &mockOutbound{
			canRetry: func(err error) bool { return true },
			prepareForRetry: func(ctx context.Context) error {
				prepareCalls++
				return nil
			},
			hasMoreChannels: func() bool { return true },
			nextChannel: func(ctx context.Context) error {
				switchCalls++
				return nil
			},
		}

		p := &pipeline{
			Executor:              executor,
			Inbound:               inbound,
			Outbound:              outbound,
			maxSameChannelRetries: 2,
			maxChannelRetries:     1,
		}

		// Sequence:
		// 1. exec 1 -> fail
		// 2. same-channel retry 1 (prepare 1) -> exec 2 -> fail
		// 3. same-channel retry 2 (prepare 2) -> exec 3 -> fail
		// 4. same-channel exhausted -> switch 1 -> same-channel reset -> exec 4 -> success
		res, err := p.Process(ctx, &httpclient.Request{})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, 4, execCalls)
		require.Equal(t, 2, prepareCalls)
		require.Equal(t, 1, switchCalls)
	})

	t.Run("AllExhausted", func(t *testing.T) {
		execCalls := 0
		executor := &mockExecutor{
			do: func(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				execCalls++
				return nil, errors.New("permanent fail")
			},
		}

		outbound := &mockOutbound{
			canRetry:        func(err error) bool { return true },
			hasMoreChannels: func() bool { return true },
		}

		p := &pipeline{
			Executor:              executor,
			Inbound:               inbound,
			Outbound:              outbound,
			maxSameChannelRetries: 1,
			maxChannelRetries:     1,
		}

		// Sequence:
		// 1. exec 1 -> fail
		// 2. same-channel retry 1 -> exec 2 -> fail
		// 3. same-channel exhausted -> switch 1 -> same-channel reset
		// 4. exec 3 -> fail
		// 5. same-channel retry 1 -> exec 4 -> fail
		// 6. same-channel exhausted -> switch 2 (but maxChannelRetries=1) -> stop
		res, err := p.Process(ctx, &httpclient.Request{})
		require.Error(t, err)
		require.Nil(t, res)
		require.Equal(t, 4, execCalls)
	})
}

func TestPipeline_Process_RetryPreservesOriginalStreamIntent(t *testing.T) {
	ctx := context.Background()

	inbound := &mockInbound{
		aggregateStreamChunks: func(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
			return []byte(`{"ok":true}`), llm.ResponseMeta{}, nil
		},
	}

	attempts := 0
	executor := &mockExecutor{
		doStream: func(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("temporary stream failure")
			}

			return streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte("chunk")}}), nil
		},
	}

	transformAttempts := 0
	outbound := &mockOutbound{
		canRetry: func(err error) bool { return true },
		prepareForRetry: func(ctx context.Context) error {
			return nil
		},
		transformRequest: func(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
			transformAttempts++
			require.False(t, req.Stream != nil && *req.Stream, "attempt %d should begin as non-stream", transformAttempts)
			req.Stream = lo.ToPtr(true)

			return &httpclient.Request{}, nil
		},
		transformStream: func(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
			return streams.SliceStream([]*llm.Response{{}}), nil
		},
	}

	p := &pipeline{
		Executor:              executor,
		Inbound:               inbound,
		Outbound:              outbound,
		maxSameChannelRetries: 1,
	}

	res, err := p.Process(ctx, &httpclient.Request{})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.False(t, res.Stream)
	require.NotNil(t, res.Response)
	require.Equal(t, `{"ok":true}`, string(res.Response.Body))
	require.Equal(t, 2, attempts)
	require.Equal(t, 2, transformAttempts)
}

func TestPipeline_Process_StreamRetriesPreCommitError(t *testing.T) {
	ctx := context.Background()
	streamFlag := true
	upstreamErr := &llm.ResponseError{
		StatusCode: http.StatusInternalServerError,
		Detail: llm.ErrorDetail{
			Message: "upstream stream failed before content",
			Type:    "server_error",
		},
	}

	inbound := &mockInbound{
		transformRequest: func(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
			return &llm.Request{Stream: &streamFlag}, nil
		},
		transformStream: transformLlmContentToEvents,
	}

	attempts := 0
	executor := &mockExecutor{
		doStream: func(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			attempts++
			return streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte("raw")}}), nil
		},
	}

	prepareCalls := 0
	outbound := &mockOutbound{
		canRetry: func(err error) bool {
			return errors.Is(err, upstreamErr)
		},
		prepareForRetry: func(ctx context.Context) error {
			prepareCalls++
			return nil
		},
		transformStream: func(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
			if attempts == 1 {
				return &llmErrorAfterStream{
					items: []*llm.Response{
						{
							Object: "chat.completion.chunk",
							Choices: []llm.Choice{{
								Delta: &llm.Message{},
							}},
						},
					},
					err: upstreamErr,
				}, nil
			}

			return streams.SliceStream([]*llm.Response{
				llmContentChunk("ok"),
				llm.DoneResponse,
			}), nil
		},
	}

	p := NewFactory(executor).Pipeline(inbound, outbound, WithRetry(0, 1, 0))

	res, err := p.Process(ctx, &httpclient.Request{})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.Stream)
	require.Equal(t, 2, attempts)
	require.Equal(t, 1, prepareCalls)

	events, err := streams.All(res.EventStream)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, "ok", string(events[0].Data))
	require.Equal(t, "[DONE]", string(events[1].Data))
}

func TestPipeline_Process_StreamDoesNotRetryAfterContent(t *testing.T) {
	ctx := context.Background()
	streamFlag := true
	upstreamErr := &llm.ResponseError{
		StatusCode: http.StatusInternalServerError,
		Detail: llm.ErrorDetail{
			Message: "upstream stream failed after content",
			Type:    "server_error",
		},
	}

	inbound := &mockInbound{
		transformRequest: func(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
			return &llm.Request{Stream: &streamFlag}, nil
		},
		transformStream: transformLlmContentToEvents,
	}

	attempts := 0
	executor := &mockExecutor{
		doStream: func(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			attempts++
			return streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte("raw")}}), nil
		},
	}

	prepareCalls := 0
	outbound := &mockOutbound{
		canRetry: func(err error) bool {
			return true
		},
		prepareForRetry: func(ctx context.Context) error {
			prepareCalls++
			return nil
		},
		transformStream: func(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
			return &llmErrorAfterStream{
				items: []*llm.Response{llmContentChunk("partial")},
				err:   upstreamErr,
			}, nil
		},
	}

	p := NewFactory(executor).Pipeline(inbound, outbound, WithRetry(0, 1, 0))

	res, err := p.Process(ctx, &httpclient.Request{})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.Stream)
	require.Equal(t, 1, attempts)
	require.Equal(t, 0, prepareCalls)

	require.True(t, res.EventStream.Next())
	require.Equal(t, "partial", string(res.EventStream.Current().Data))
	require.False(t, res.EventStream.Next())
	require.ErrorIs(t, res.EventStream.Err(), upstreamErr)
}

func TestPipeline_Process_StreamStopsPreCommitProbeAfterBound(t *testing.T) {
	ctx := context.Background()
	streamFlag := true
	const nonContentEventsBeforeContent = 64

	inbound := &mockInbound{
		transformRequest: func(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
			return &llm.Request{Stream: &streamFlag}, nil
		},
		transformStream: transformLlmContentToEvents,
	}

	attempts := 0
	executor := &mockExecutor{
		doStream: func(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			attempts++
			return streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte("raw")}}), nil
		},
	}

	var sourceStream *llmErrorAfterStream
	outbound := &mockOutbound{
		canRetry: func(err error) bool {
			return true
		},
		transformStream: func(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
			items := make([]*llm.Response, 0, nonContentEventsBeforeContent+2)
			for i := 0; i < nonContentEventsBeforeContent; i++ {
				items = append(items, llmEmptyChunk())
			}
			items = append(items, llmContentChunk("ok"), llm.DoneResponse)

			sourceStream = &llmErrorAfterStream{items: items}

			return sourceStream, nil
		},
	}

	p := NewFactory(executor).Pipeline(inbound, outbound, WithRetry(0, 1, 0))

	res, err := p.Process(ctx, &httpclient.Request{})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.Stream)
	require.Equal(t, 1, attempts)
	require.NotNil(t, sourceStream)
	require.Equal(t, maxPreCommitRetryProbeEvents, sourceStream.index)

	events, err := streams.All(res.EventStream)
	require.NoError(t, err)
	require.Len(t, events, nonContentEventsBeforeContent+2)
	require.Equal(t, "metadata", string(events[0].Data))
	require.Equal(t, "ok", string(events[nonContentEventsBeforeContent].Data))
	require.Equal(t, "[DONE]", string(events[nonContentEventsBeforeContent+1].Data))
}

func llmEmptyChunk() *llm.Response {
	return &llm.Response{
		Object: "chat.completion.chunk",
		Choices: []llm.Choice{{
			Delta: &llm.Message{},
		}},
	}
}

func llmContentChunk(content string) *llm.Response {
	return &llm.Response{
		Object: "chat.completion.chunk",
		Choices: []llm.Choice{{
			Delta: &llm.Message{
				Content: llm.MessageContent{Content: lo.ToPtr(content)},
			},
		}},
	}
}

func transformLlmContentToEvents(_ context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.Map(stream, func(resp *llm.Response) *httpclient.StreamEvent {
		if resp == llm.DoneResponse || (resp != nil && resp.Object == "[DONE]") {
			return &httpclient.StreamEvent{Data: []byte("[DONE]")}
		}

		for _, choice := range resp.Choices {
			if choice.Delta != nil && choice.Delta.Content.Content != nil {
				return &httpclient.StreamEvent{Data: []byte(*choice.Delta.Content.Content)}
			}
		}

		return &httpclient.StreamEvent{Data: []byte("metadata")}
	}), nil
}
