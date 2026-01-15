package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type mockInbound struct{ transformer.Inbound }

func (m *mockInbound) TransformRequest(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
	return &llm.Request{}, nil
}

func (m *mockInbound) TransformResponse(ctx context.Context, resp *llm.Response) (*httpclient.Response, error) {
	return &httpclient.Response{}, nil
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
	do func(context.Context, *httpclient.Request) (*httpclient.Response, error)
}

func (m *mockExecutor) Do(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
	return m.do(ctx, req)
}

func (m *mockExecutor) DoStream(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, nil
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
