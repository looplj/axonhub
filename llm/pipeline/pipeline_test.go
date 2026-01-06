package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// testRetryableOutbound implements both transformer.Outbound and Retryable interfaces.
type testRetryableOutbound struct {
	channels             []string
	currentChannelIndex  int
	switchChannelCalls   int
	hasMoreChannelsCalls int
}

func (t *testRetryableOutbound) APIFormat() llm.APIFormat {
	return "test/retryable"
}

func (t *testRetryableOutbound) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	return &httpclient.Request{}, nil
}

func (t *testRetryableOutbound) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (t *testRetryableOutbound) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.SliceStream([]*llm.Response{}), nil
}

func (t *testRetryableOutbound) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return &llm.ResponseError{}
}

func (t *testRetryableOutbound) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return []byte(`{}`), llm.ResponseMeta{}, nil
}

func (t *testRetryableOutbound) HasMoreChannels() bool {
	t.hasMoreChannelsCalls++
	return t.currentChannelIndex < len(t.channels)-1
}

func (t *testRetryableOutbound) NextChannel(ctx context.Context) error {
	t.switchChannelCalls++
	if t.currentChannelIndex < len(t.channels)-1 {
		t.currentChannelIndex++
		return nil
	}

	return errors.New("no more channels")
}

// testChannelRetryableOutbound implements both transformer.Outbound and ChannelRetryable interfaces.
type testChannelRetryableOutbound struct {
	maxRetries        int
	currentRetries    int
	canRetryCalls     int
	prepareRetryCalls int
}

func (t *testChannelRetryableOutbound) APIFormat() llm.APIFormat {
	return "test/channel-retryable"
}

func (t *testChannelRetryableOutbound) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	return &httpclient.Request{}, nil
}

func (t *testChannelRetryableOutbound) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (t *testChannelRetryableOutbound) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.SliceStream([]*llm.Response{}), nil
}

func (t *testChannelRetryableOutbound) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return &llm.ResponseError{}
}

func (t *testChannelRetryableOutbound) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return []byte(`{}`), llm.ResponseMeta{}, nil
}

func (t *testChannelRetryableOutbound) CanRetry(err error) bool {
	t.canRetryCalls++
	return t.currentRetries < t.maxRetries
}

func (t *testChannelRetryableOutbound) PrepareForRetry(ctx context.Context) error {
	t.prepareRetryCalls++
	t.currentRetries++

	return nil
}

// testCustomExecutorOutbound implements both transformer.Outbound and ChannelCustomizedExecutor interfaces.
type testCustomExecutorOutbound struct {
	customizeExecutorCalls int
	customExecutor         Executor
}

func (t *testCustomExecutorOutbound) APIFormat() llm.APIFormat {
	return "test/custom-executor"
}

func (t *testCustomExecutorOutbound) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	return &httpclient.Request{}, nil
}

func (t *testCustomExecutorOutbound) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (t *testCustomExecutorOutbound) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.SliceStream([]*llm.Response{}), nil
}

func (t *testCustomExecutorOutbound) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return &llm.ResponseError{}
}

func (t *testCustomExecutorOutbound) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return []byte(`{}`), llm.ResponseMeta{}, nil
}

func (t *testCustomExecutorOutbound) CustomizeExecutor(executor Executor) Executor {
	t.customizeExecutorCalls++
	if t.customExecutor != nil {
		return t.customExecutor
	}

	return executor
}

func TestRetryable_HasMoreChannels(t *testing.T) {
	outbound := &testRetryableOutbound{
		channels:            []string{"channel1", "channel2", "channel3"},
		currentChannelIndex: 0,
	}

	// Test HasMoreChannels when there are more channels
	require.True(t, outbound.HasMoreChannels())
	require.Equal(t, 1, outbound.hasMoreChannelsCalls)

	// Move to last channel
	outbound.currentChannelIndex = 2
	require.False(t, outbound.HasMoreChannels())
	require.Equal(t, 2, outbound.hasMoreChannelsCalls)
}

func TestRetryable_NextChannel(t *testing.T) {
	ctx := context.Background()
	outbound := &testRetryableOutbound{
		channels:            []string{"channel1", "channel2"},
		currentChannelIndex: 0,
	}

	// Test successful channel switch
	err := outbound.NextChannel(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, outbound.currentChannelIndex)
	require.Equal(t, 1, outbound.switchChannelCalls)

	// Test error when no more channels
	err = outbound.NextChannel(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no more channels")
	require.Equal(t, 2, outbound.switchChannelCalls)
}

func TestChannelRetryable_CanRetry(t *testing.T) {
	outbound := &testChannelRetryableOutbound{
		maxRetries:     2,
		currentRetries: 0,
	}

	// Test CanRetry when retries available
	require.True(t, outbound.CanRetry(nil))
	require.Equal(t, 1, outbound.canRetryCalls)

	// Exhaust retries
	outbound.currentRetries = 2
	require.False(t, outbound.CanRetry(nil))
	require.Equal(t, 2, outbound.canRetryCalls)
}

func TestChannelRetryable_PrepareForRetry(t *testing.T) {
	ctx := context.Background()
	outbound := &testChannelRetryableOutbound{
		maxRetries:     2,
		currentRetries: 0,
	}

	// Test PrepareForRetry
	err := outbound.PrepareForRetry(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, outbound.currentRetries)
	require.Equal(t, 1, outbound.prepareRetryCalls)
}

func TestChannelCustomizedExecutor_CustomizeExecutor(t *testing.T) {
	customExecutor := &testExecutor{}
	outbound := &testCustomExecutorOutbound{
		customExecutor: customExecutor,
	}

	originalExecutor := &testExecutor{}

	// Test CustomizeExecutor returns custom executor
	result := outbound.CustomizeExecutor(originalExecutor)
	require.Equal(t, customExecutor, result)
	require.Equal(t, 1, outbound.customizeExecutorCalls)
}

func TestPipeline_GetMaxSameChannelRetries(t *testing.T) {
	p := &pipeline{
		maxSameChannelRetries: 3,
	}

	require.Equal(t, 3, p.getMaxSameChannelRetries())
}

func TestWithRetry(t *testing.T) {
	p := &pipeline{}

	option := WithRetry(5, 3, 100*time.Millisecond)
	option(p)

	require.Equal(t, 5, p.maxRetries)
	require.Equal(t, 3, p.maxSameChannelRetries)
	require.Equal(t, 100*time.Millisecond, p.retryDelay)
}

func TestWithDecorators(t *testing.T) {
	p := &pipeline{}

	// This would require actual decorator implementations
	// For now, just test that the option function works
	option := WithMiddlewares()
	option(p)

	// The decorators slice should be initialized (even if empty)
	require.Len(t, p.middlewares, 0)
}

func TestFactory_NewFactory(t *testing.T) {
	executor := &testExecutor{}
	factory := NewFactory(executor)

	require.NotNil(t, factory)
	require.Equal(t, executor, factory.Executor)
}

// testExecutor is a simple test executor.
type testExecutor struct {
	callCount int
}

func (t *testExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	t.callCount++
	return &httpclient.Response{}, nil
}

func (t *testExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	t.callCount++
	return streams.SliceStream([]*httpclient.StreamEvent{}), nil
}
