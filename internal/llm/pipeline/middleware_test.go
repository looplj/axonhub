package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// testInbound implements transformer.Inbound interface for testing.
type testInbound struct{}

func (t *testInbound) APIFormat() llm.APIFormat {
	return "test/inbound"
}

func (t *testInbound) TransformRequest(ctx context.Context, request *httpclient.Request) (*llm.Request, error) {
	return &llm.Request{}, nil
}

func (t *testInbound) TransformResponse(ctx context.Context, response *llm.Response) (*httpclient.Response, error) {
	return &httpclient.Response{}, nil
}

func (t *testInbound) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.SliceStream([]*httpclient.StreamEvent{}), nil
}

func (t *testInbound) TransformError(ctx context.Context, err error) *httpclient.Error {
	return &httpclient.Error{}
}

func (t *testInbound) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return []byte(`{}`), llm.ResponseMeta{}, nil
}

// testOutbound implements transformer.Outbound interface for testing.
type testOutbound struct{}

func (t *testOutbound) APIFormat() llm.APIFormat {
	return "test/format"
}

func (t *testOutbound) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	return &httpclient.Request{}, nil
}

func (t *testOutbound) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (t *testOutbound) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.SliceStream([]*llm.Response{}), nil
}

func (t *testOutbound) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return &llm.ResponseError{}
}

func (t *testOutbound) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return []byte(`{}`), llm.ResponseMeta{}, nil
}

// trackingMiddleware tracks the order of middleware calls.
type trackingMiddleware struct {
	name                      string
	callOrder                 *[]string
	inboundRequestCalled      bool
	inboundRawResponseCalled  bool
	outboundRequestCalled     bool
	outboundRawResponseCalled bool
	outboundLlmResponseCalled bool
	outboundRawStreamCalled   bool
	outboundLlmStreamCalled   bool
	outboundRawErrorCalled    bool
	shouldFailOnLlmRequest    bool
	shouldFailOnRawRequest    bool
	shouldFailOnRawResponse   bool
	shouldFailOnLlmResponse   bool
	shouldFailOnRawStream     bool
	shouldFailOnLlmStream     bool
}

func newTrackingMiddleware(name string, callOrder *[]string) *trackingMiddleware {
	return &trackingMiddleware{
		name:      name,
		callOrder: callOrder,
	}
}

func (m *trackingMiddleware) Name() string {
	return m.name
}

func (m *trackingMiddleware) OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	m.inboundRequestCalled = true

	*m.callOrder = append(*m.callOrder, m.name+":OnInboundLlmRequest")
	if m.shouldFailOnLlmRequest {
		return nil, errors.New("llm request middleware error")
	}

	return request, nil
}

func (m *trackingMiddleware) OnInboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	m.inboundRawResponseCalled = true

	*m.callOrder = append(*m.callOrder, m.name+":OnInboundRawResponse")

	return response, nil
}

func (m *trackingMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	m.outboundRequestCalled = true

	*m.callOrder = append(*m.callOrder, m.name+":OnOutboundRawRequest")
	if m.shouldFailOnRawRequest {
		return nil, errors.New("raw request middleware error")
	}

	return request, nil
}

func (m *trackingMiddleware) OnOutboundRawError(ctx context.Context, err error) {
	m.outboundRawErrorCalled = true
	*m.callOrder = append(*m.callOrder, m.name+":OnOutboundRawErrorResponse")
}

func (m *trackingMiddleware) OnOutboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	m.outboundRawResponseCalled = true

	*m.callOrder = append(*m.callOrder, m.name+":OnOutboundRawResponse")
	if m.shouldFailOnRawResponse {
		return nil, errors.New("raw response middleware error")
	}

	return response, nil
}

func (m *trackingMiddleware) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	m.outboundLlmResponseCalled = true

	*m.callOrder = append(*m.callOrder, m.name+":OnOutboundLlmResponse")
	if m.shouldFailOnLlmResponse {
		return nil, errors.New("llm response middleware error")
	}

	return response, nil
}

func (m *trackingMiddleware) OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	m.outboundRawStreamCalled = true

	*m.callOrder = append(*m.callOrder, m.name+":OnOutboundRawStream")
	if m.shouldFailOnRawStream {
		return nil, errors.New("raw stream middleware error")
	}

	return stream, nil
}

func (m *trackingMiddleware) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	m.outboundLlmStreamCalled = true

	*m.callOrder = append(*m.callOrder, m.name+":OnOutboundLlmStream")
	if m.shouldFailOnLlmStream {
		return nil, errors.New("llm stream middleware error")
	}

	return stream, nil
}

// TestMiddleware_NonStreaming_CallOrder tests that middlewares are called in the correct order for non-streaming requests.
func TestMiddleware_NonStreaming_CallOrder(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	// Create three middlewares to test the order
	middleware1 := newTrackingMiddleware("M1", &callOrder)
	middleware2 := newTrackingMiddleware("M2", &callOrder)
	middleware3 := newTrackingMiddleware("M3", &callOrder)

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware1, middleware2, middleware3),
	)

	request := &httpclient.Request{}
	result, err := p.Process(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)

	// Verify all middlewares were called
	require.True(t, middleware1.inboundRequestCalled)
	require.True(t, middleware1.inboundRawResponseCalled)
	require.True(t, middleware1.outboundRequestCalled)
	require.True(t, middleware1.outboundRawResponseCalled)
	require.True(t, middleware1.outboundLlmResponseCalled)

	require.True(t, middleware2.inboundRequestCalled)
	require.True(t, middleware2.inboundRawResponseCalled)
	require.True(t, middleware2.outboundRequestCalled)
	require.True(t, middleware2.outboundRawResponseCalled)
	require.True(t, middleware2.outboundLlmResponseCalled)

	require.True(t, middleware3.inboundRequestCalled)
	require.True(t, middleware3.inboundRawResponseCalled)
	require.True(t, middleware3.outboundRequestCalled)
	require.True(t, middleware3.outboundRawResponseCalled)
	require.True(t, middleware3.outboundLlmResponseCalled)

	// Verify the call order follows the onion model:
	// Request: M1 -> M2 -> M3 (forward)
	// Response: M3 -> M2 -> M1 (reverse)
	// Final inbound raw response: M1 -> M2 -> M3 (forward, after final transformation)
	expectedOrder := []string{
		// Request phase (forward order)
		"M1:OnInboundLlmRequest",
		"M2:OnInboundLlmRequest",
		"M3:OnInboundLlmRequest",
		"M1:OnOutboundRawRequest",
		"M2:OnOutboundRawRequest",
		"M3:OnOutboundRawRequest",
		// Response phase (reverse order)
		"M3:OnOutboundRawResponse",
		"M2:OnOutboundRawResponse",
		"M1:OnOutboundRawResponse",
		"M3:OnOutboundLlmResponse",
		"M2:OnOutboundLlmResponse",
		"M1:OnOutboundLlmResponse",
		// Inbound raw response phase (forward order, after final transformation)
		"M1:OnInboundRawResponse",
		"M2:OnInboundRawResponse",
		"M3:OnInboundRawResponse",
	}

	require.Equal(t, expectedOrder, callOrder, "Middleware call order should follow onion model")
}

// TestMiddleware_Streaming_CallOrder tests that middlewares are called in the correct order for streaming requests.
func TestMiddleware_Streaming_CallOrder(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	// Create three middlewares to test the order
	middleware1 := newTrackingMiddleware("M1", &callOrder)
	middleware2 := newTrackingMiddleware("M2", &callOrder)
	middleware3 := newTrackingMiddleware("M3", &callOrder)

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware1, middleware2, middleware3),
	)

	stream := true
	request := &httpclient.Request{}
	llmRequest := &llm.Request{Stream: &stream}
	llmRequest.RawRequest = request

	result, err := p.processRequest(ctx, llmRequest)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)

	// Verify all stream middlewares were called
	require.True(t, middleware1.outboundRequestCalled)
	require.True(t, middleware1.outboundRawStreamCalled)
	require.True(t, middleware1.outboundLlmStreamCalled)

	require.True(t, middleware2.outboundRequestCalled)
	require.True(t, middleware2.outboundRawStreamCalled)
	require.True(t, middleware2.outboundLlmStreamCalled)

	require.True(t, middleware3.outboundRequestCalled)
	require.True(t, middleware3.outboundRawStreamCalled)
	require.True(t, middleware3.outboundLlmStreamCalled)

	// Verify the call order follows the onion model:
	// Request: M1 -> M2 -> M3 (forward)
	// Stream: M3 -> M2 -> M1 (reverse)
	expectedOrder := []string{
		// Request phase (forward order)
		"M1:OnOutboundRawRequest",
		"M2:OnOutboundRawRequest",
		"M3:OnOutboundRawRequest",
		// Stream phase (reverse order)
		"M3:OnOutboundRawStream",
		"M2:OnOutboundRawStream",
		"M1:OnOutboundRawStream",
		"M3:OnOutboundLlmStream",
		"M2:OnOutboundLlmStream",
		"M1:OnOutboundLlmStream",
	}

	require.Equal(t, expectedOrder, callOrder, "Middleware call order should follow onion model for streaming")
}

// TestMiddleware_ErrorResponse_CallOrder tests that error response middlewares are called in reverse order.
func TestMiddleware_ErrorResponse_CallOrder(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	// Create three middlewares to test the order
	middleware1 := newTrackingMiddleware("M1", &callOrder)
	middleware2 := newTrackingMiddleware("M2", &callOrder)
	middleware3 := newTrackingMiddleware("M3", &callOrder)

	// Create an executor that always fails
	executor := &failingExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware1, middleware2, middleware3),
	)

	request := &httpclient.Request{}
	result, err := p.Process(ctx, request)

	require.Error(t, err)
	require.Nil(t, result)

	// Verify error response middlewares were called
	require.True(t, middleware1.outboundRawErrorCalled)
	require.True(t, middleware2.outboundRawErrorCalled)
	require.True(t, middleware3.outboundRawErrorCalled)

	// Verify the call order is reverse (M3 -> M2 -> M1)
	expectedOrder := []string{
		"M1:OnInboundLlmRequest",
		"M2:OnInboundLlmRequest",
		"M3:OnInboundLlmRequest",
		"M1:OnOutboundRawRequest",
		"M2:OnOutboundRawRequest",
		"M3:OnOutboundRawRequest",
		// Error response phase (reverse order)
		"M3:OnOutboundRawErrorResponse",
		"M2:OnOutboundRawErrorResponse",
		"M1:OnOutboundRawErrorResponse",
	}

	require.Equal(t, expectedOrder, callOrder, "Error response middlewares should be called in reverse order")
}

// TestMiddleware_LlmRequest_Error tests that an error in OnInboundLlmRequest stops the pipeline.
func TestMiddleware_LlmRequest_Error(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	middleware1 := newTrackingMiddleware("M1", &callOrder)
	middleware2 := newTrackingMiddleware("M2", &callOrder)
	middleware2.shouldFailOnLlmRequest = true
	middleware3 := newTrackingMiddleware("M3", &callOrder)

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware1, middleware2, middleware3),
	)

	request := &httpclient.Request{}
	result, err := p.Process(ctx, request)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "llm request middleware error")

	// M1 and M2 should be called, but M3 should not
	require.True(t, middleware1.inboundRequestCalled)
	require.True(t, middleware2.inboundRequestCalled)
	require.False(t, middleware3.inboundRequestCalled)

	// No other middlewares should be called
	require.False(t, middleware1.outboundRequestCalled)
	require.False(t, middleware2.outboundRequestCalled)
	require.False(t, middleware3.outboundRequestCalled)
	require.False(t, middleware1.inboundRawResponseCalled)
	require.False(t, middleware2.inboundRawResponseCalled)
	require.False(t, middleware3.inboundRawResponseCalled)
}

// TestMiddleware_RawRequest_Error tests that an error in OnOutboundRawRequest stops the pipeline.
func TestMiddleware_RawRequest_Error(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	middleware1 := newTrackingMiddleware("M1", &callOrder)
	middleware2 := newTrackingMiddleware("M2", &callOrder)
	middleware2.shouldFailOnRawRequest = true
	middleware3 := newTrackingMiddleware("M3", &callOrder)

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware1, middleware2, middleware3),
	)

	request := &httpclient.Request{}
	result, err := p.Process(ctx, request)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "raw request middleware error")

	// All inbound request middlewares should be called
	require.True(t, middleware1.inboundRequestCalled)
	require.True(t, middleware2.inboundRequestCalled)
	require.True(t, middleware3.inboundRequestCalled)

	// M1 and M2 should be called for raw request, but M3 should not
	require.True(t, middleware1.outboundRequestCalled)
	require.True(t, middleware2.outboundRequestCalled)
	require.False(t, middleware3.outboundRequestCalled)

	// No response middlewares should be called
	require.False(t, middleware1.outboundRawResponseCalled)
	require.False(t, middleware2.outboundRawResponseCalled)
	require.False(t, middleware3.outboundRawResponseCalled)
	require.False(t, middleware1.outboundLlmResponseCalled)
	require.False(t, middleware2.outboundLlmResponseCalled)
	require.False(t, middleware3.outboundLlmResponseCalled)

	// No inbound response middlewares should be called since error occurred before them
	require.False(t, middleware1.inboundRawResponseCalled)
	require.False(t, middleware2.inboundRawResponseCalled)
	require.False(t, middleware3.inboundRawResponseCalled)
}

// TestMiddleware_RawResponse_Error tests that an error in OnOutboundRawResponse stops the pipeline.
func TestMiddleware_RawResponse_Error(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	middleware1 := newTrackingMiddleware("M1", &callOrder)
	middleware2 := newTrackingMiddleware("M2", &callOrder)
	middleware2.shouldFailOnRawResponse = true
	middleware3 := newTrackingMiddleware("M3", &callOrder)

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware1, middleware2, middleware3),
	)

	request := &httpclient.Request{}
	result, err := p.Process(ctx, request)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "raw response middleware error")

	// All request middlewares should be called
	require.True(t, middleware1.inboundRequestCalled)
	require.True(t, middleware2.inboundRequestCalled)
	require.True(t, middleware3.inboundRequestCalled)
	require.True(t, middleware1.outboundRequestCalled)
	require.True(t, middleware2.outboundRequestCalled)
	require.True(t, middleware3.outboundRequestCalled)

	// M3 and M2 should be called for raw response (reverse order), but M1 should not
	require.True(t, middleware3.outboundRawResponseCalled)
	require.True(t, middleware2.outboundRawResponseCalled)
	require.False(t, middleware1.outboundRawResponseCalled)

	// No LLM response middlewares should be called
	require.False(t, middleware1.outboundLlmResponseCalled)
	require.False(t, middleware2.outboundLlmResponseCalled)
	require.False(t, middleware3.outboundLlmResponseCalled)

	// No inbound response middlewares should be called since error occurred before them
	require.False(t, middleware1.inboundRawResponseCalled)
	require.False(t, middleware2.inboundRawResponseCalled)
	require.False(t, middleware3.inboundRawResponseCalled)
}

// TestMiddleware_LlmResponse_Error tests that an error in OnOutboundLlmResponse stops the pipeline.
func TestMiddleware_LlmResponse_Error(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	middleware1 := newTrackingMiddleware("M1", &callOrder)
	middleware2 := newTrackingMiddleware("M2", &callOrder)
	middleware2.shouldFailOnLlmResponse = true
	middleware3 := newTrackingMiddleware("M3", &callOrder)

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware1, middleware2, middleware3),
	)

	request := &httpclient.Request{}
	result, err := p.Process(ctx, request)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "llm response middleware error")

	// All request middlewares should be called
	require.True(t, middleware1.inboundRequestCalled)
	require.True(t, middleware2.inboundRequestCalled)
	require.True(t, middleware3.inboundRequestCalled)
	require.True(t, middleware1.outboundRequestCalled)
	require.True(t, middleware2.outboundRequestCalled)
	require.True(t, middleware3.outboundRequestCalled)

	// All raw response middlewares should be called
	require.True(t, middleware1.outboundRawResponseCalled)
	require.True(t, middleware2.outboundRawResponseCalled)
	require.True(t, middleware3.outboundRawResponseCalled)

	// M3 and M2 should be called for LLM response (reverse order), but M1 should not
	require.True(t, middleware3.outboundLlmResponseCalled)
	require.True(t, middleware2.outboundLlmResponseCalled)
	require.False(t, middleware1.outboundLlmResponseCalled)

	// No inbound response middlewares should be called since error occurred before them
	require.False(t, middleware1.inboundRawResponseCalled)
	require.False(t, middleware2.inboundRawResponseCalled)
	require.False(t, middleware3.inboundRawResponseCalled)
}

// TestMiddleware_RawStream_Error tests that an error in OnOutboundRawStream stops the pipeline.
func TestMiddleware_RawStream_Error(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	middleware1 := newTrackingMiddleware("M1", &callOrder)
	middleware2 := newTrackingMiddleware("M2", &callOrder)
	middleware2.shouldFailOnRawStream = true
	middleware3 := newTrackingMiddleware("M3", &callOrder)

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware1, middleware2, middleware3),
	)

	stream := true
	request := &httpclient.Request{}
	llmRequest := &llm.Request{Stream: &stream}
	llmRequest.RawRequest = request

	result, err := p.processRequest(ctx, llmRequest)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "raw stream middleware error")

	// All request middlewares should be called
	require.True(t, middleware1.outboundRequestCalled)
	require.True(t, middleware2.outboundRequestCalled)
	require.True(t, middleware3.outboundRequestCalled)

	// M3 and M2 should be called for raw stream (reverse order), but M1 should not
	require.True(t, middleware3.outboundRawStreamCalled)
	require.True(t, middleware2.outboundRawStreamCalled)
	require.False(t, middleware1.outboundRawStreamCalled)

	// No LLM stream middlewares should be called
	require.False(t, middleware1.outboundLlmStreamCalled)
	require.False(t, middleware2.outboundLlmStreamCalled)
	require.False(t, middleware3.outboundLlmStreamCalled)
}

// TestMiddleware_LlmStream_Error tests that an error in OnOutboundLlmStream stops the pipeline.
func TestMiddleware_LlmStream_Error(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	middleware1 := newTrackingMiddleware("M1", &callOrder)
	middleware2 := newTrackingMiddleware("M2", &callOrder)
	middleware2.shouldFailOnLlmStream = true
	middleware3 := newTrackingMiddleware("M3", &callOrder)

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware1, middleware2, middleware3),
	)

	stream := true
	request := &httpclient.Request{}
	llmRequest := &llm.Request{Stream: &stream}
	llmRequest.RawRequest = request

	result, err := p.processRequest(ctx, llmRequest)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "llm stream middleware error")

	// All request middlewares should be called
	require.True(t, middleware1.outboundRequestCalled)
	require.True(t, middleware2.outboundRequestCalled)
	require.True(t, middleware3.outboundRequestCalled)

	// All raw stream middlewares should be called
	require.True(t, middleware1.outboundRawStreamCalled)
	require.True(t, middleware2.outboundRawStreamCalled)
	require.True(t, middleware3.outboundRawStreamCalled)

	// M3 and M2 should be called for LLM stream (reverse order), but M1 should not
	require.True(t, middleware3.outboundLlmStreamCalled)
	require.True(t, middleware2.outboundLlmStreamCalled)
	require.False(t, middleware1.outboundLlmStreamCalled)
}

// TestMiddleware_NoMiddlewares tests that the pipeline works correctly without any middlewares.
func TestMiddleware_NoMiddlewares(t *testing.T) {
	ctx := context.Background()

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
	)

	request := &httpclient.Request{}
	result, err := p.Process(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)
}

// TestMiddleware_SingleMiddleware tests that a single middleware is called correctly.
func TestMiddleware_SingleMiddleware(t *testing.T) {
	ctx := context.Background()
	callOrder := []string{}

	middleware := newTrackingMiddleware("M1", &callOrder)

	executor := &testExecutor{}
	factory := NewFactory(executor)

	p := factory.Pipeline(
		&testInbound{},
		&testOutbound{},
		WithMiddlewares(middleware),
	)

	request := &httpclient.Request{}
	result, err := p.Process(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)

	require.True(t, middleware.inboundRequestCalled)
	require.True(t, middleware.inboundRawResponseCalled)
	require.True(t, middleware.outboundRequestCalled)
	require.True(t, middleware.outboundRawResponseCalled)
	require.True(t, middleware.outboundLlmResponseCalled)

	expectedOrder := []string{
		"M1:OnInboundLlmRequest",
		"M1:OnOutboundRawRequest",
		"M1:OnOutboundRawResponse",
		"M1:OnOutboundLlmResponse",
		"M1:OnInboundRawResponse",
	}

	require.Equal(t, expectedOrder, callOrder)
}

// failingExecutor is an executor that always fails.
type failingExecutor struct{}

func (f *failingExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	return nil, errors.New("executor error")
}

func (f *failingExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, errors.New("executor stream error")
}
