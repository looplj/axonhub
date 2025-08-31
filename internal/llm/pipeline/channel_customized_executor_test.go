package pipeline_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// mockChannelCustomizedExecutor implements both transformer.Outbound and ChannelCustomizedExecutor.
type mockChannelCustomizedExecutor struct {
	*openai.OutboundTransformer

	customizeExecutorCalled bool
	customExecutor          pipeline.Executor
}

func (m *mockChannelCustomizedExecutor) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	m.customizeExecutorCalled = true
	if m.customExecutor != nil {
		return m.customExecutor
	}

	return executor
}

// mockCustomExecutor is a custom executor that tracks if it was called.
type mockCustomExecutor struct {
	doCalled       bool
	doStreamCalled bool
	doFunc         func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error)
	doStreamFunc   func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error)
}

func (m *mockCustomExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	m.doCalled = true
	if m.doFunc != nil {
		return m.doFunc(ctx, request)
	}

	return &httpclient.Response{StatusCode: 200}, nil
}

func (m *mockCustomExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	m.doStreamCalled = true
	if m.doStreamFunc != nil {
		return m.doStreamFunc(ctx, request)
	}

	return streams.SliceStream([]*httpclient.StreamEvent{}), nil
}

func TestChannelCustomizedExecutor_StreamingPath(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Create custom executor
	customExecutor := &mockCustomExecutor{}

	// Create mock channel customized executor
	mockCustomized := &mockChannelCustomizedExecutor{
		OutboundTransformer: outbound.(*openai.OutboundTransformer),
		customExecutor:      customExecutor,
	}

	// Create original executor that returns a proper response
	originalExecutor := &mockExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			return &httpclient.Response{
				StatusCode: 200,
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
				Body:       []byte(`{"choices":[{"message":{"role":"assistant","content":"Hello!"}}]}`),
			}, nil
		},
	}

	// Create factory and pipeline
	factory := pipeline.NewFactory(originalExecutor)
	pipeline := factory.Pipeline(inbound, mockCustomized)

	// Create test request
	request := &httpclient.Request{
		Method: "POST",
		URL:    "http://test.com",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}],"stream":true}`),
	}

	// Process streaming request
	result, err := pipeline.Process(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)

	// Verify that CustomizeExecutor was called
	require.True(t, mockCustomized.customizeExecutorCalled, "CustomizeExecutor should have been called")

	// Verify that the custom executor was used for streaming
	require.True(t, customExecutor.doStreamCalled, "Custom executor's DoStream should have been called")
	require.False(t, customExecutor.doCalled, "Custom executor's Do should not have been called for streaming")
}

func TestChannelCustomizedExecutor_NonStreamingPath(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Create custom executor
	customExecutor := &mockCustomExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			return &httpclient.Response{
				StatusCode: 200,
				Headers:    map[string][]string{"Content-Type": {"application/json"}},
				Body:       []byte(`{"choices":[{"message":{"role":"assistant","content":"Hello!"}}]}`),
			}, nil
		},
	}

	// Create mock channel customized executor
	mockCustomized := &mockChannelCustomizedExecutor{
		OutboundTransformer: outbound.(*openai.OutboundTransformer),
		customExecutor:      customExecutor,
	}

	// Create original executor that returns a proper response for non-streaming
	originalExecutor := &mockExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			return &httpclient.Response{
				StatusCode: 200,
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
				Body:       []byte(`{"choices":[{"message":{"role":"assistant","content":"Hello from original executor!"}}]}`),
			}, nil
		},
	}

	// Create factory and pipeline
	factory := pipeline.NewFactory(originalExecutor)
	pipeline := factory.Pipeline(inbound, mockCustomized)

	// Create test request (non-streaming)
	request := &httpclient.Request{
		Method: "POST",
		URL:    "http://test.com",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`),
	}

	// Process non-streaming request
	result, err := pipeline.Process(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)

	// Verify that CustomizeExecutor was called for non-streaming path too
	require.True(t, mockCustomized.customizeExecutorCalled, "CustomizeExecutor should have been called")

	// Verify that the custom executor was used for non-streaming
	require.True(t, customExecutor.doCalled, "Custom executor's Do should have been called")
	require.False(t, customExecutor.doStreamCalled, "Custom executor's DoStream should not have been called for non-streaming")
}

func TestChannelCustomizedExecutor_CustomExecutorError(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Create custom executor that returns an error
	customExecutor := &mockCustomExecutor{
		doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			return nil, errors.New("custom executor error")
		},
	}

	// Create mock channel customized executor
	mockCustomized := &mockChannelCustomizedExecutor{
		OutboundTransformer: outbound.(*openai.OutboundTransformer),
		customExecutor:      customExecutor,
	}

	// Create original executor
	originalExecutor := &mockExecutor{}

	// Create factory and pipeline
	factory := pipeline.NewFactory(originalExecutor)
	pipeline := factory.Pipeline(inbound, mockCustomized)

	// Create test request
	request := &httpclient.Request{
		Method: "POST",
		URL:    "http://test.com",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}],"stream":true}`),
	}

	// Process streaming request - should return error from custom executor
	result, err := pipeline.Process(ctx, request)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "custom executor error")

	// Verify that CustomizeExecutor was called
	require.True(t, mockCustomized.customizeExecutorCalled, "CustomizeExecutor should have been called")
	require.True(t, customExecutor.doStreamCalled, "Custom executor's DoStream should have been called")
}

func TestChannelCustomizedExecutor_NoCustomization(t *testing.T) {
	ctx := context.Background()

	// Create transformers - using regular outbound transformer (not ChannelCustomizedExecutor)
	inbound := openai.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Create original executor
	originalExecutor := &mockExecutor{
		doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			return streams.SliceStream([]*httpclient.StreamEvent{}), nil
		},
	}

	// Create factory and pipeline
	factory := pipeline.NewFactory(originalExecutor)
	pipeline := factory.Pipeline(inbound, outbound)

	// Create test request
	request := &httpclient.Request{
		Method: "POST",
		URL:    "http://test.com",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}],"stream":true}`),
	}

	// Process streaming request
	result, err := pipeline.Process(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)

	// Since outbound transformer doesn't implement ChannelCustomizedExecutor,
	// the original executor should be used directly
	// This is verified by the fact that the request succeeds and we get a result
}

func TestChannelCustomizedExecutor_NonStreamingCustomExecutorError(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Create custom executor that returns an error
	customExecutor := &mockCustomExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			return nil, errors.New("custom executor non-streaming error")
		},
	}

	// Create mock channel customized executor
	mockCustomized := &mockChannelCustomizedExecutor{
		OutboundTransformer: outbound.(*openai.OutboundTransformer),
		customExecutor:      customExecutor,
	}

	// Create original executor
	originalExecutor := &mockExecutor{}

	// Create factory and pipeline
	factory := pipeline.NewFactory(originalExecutor)
	pipeline := factory.Pipeline(inbound, mockCustomized)

	// Create test request (non-streaming)
	request := &httpclient.Request{
		Method: "POST",
		URL:    "http://test.com",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`),
	}

	// Process non-streaming request - should return error from custom executor
	result, err := pipeline.Process(ctx, request)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "custom executor non-streaming error")

	// Verify that CustomizeExecutor was called
	require.True(t, mockCustomized.customizeExecutorCalled, "CustomizeExecutor should have been called")
	require.True(t, customExecutor.doCalled, "Custom executor's Do should have been called")
}

func TestChannelCustomizedExecutor_ReturnsSameExecutor(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Create mock channel customized executor that returns the same executor
	mockCustomized := &mockChannelCustomizedExecutor{
		OutboundTransformer: outbound.(*openai.OutboundTransformer),
		customExecutor:      nil, // This will cause it to return the original executor
	}

	// Create original executor
	originalExecutor := &mockExecutor{
		doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			return streams.SliceStream([]*httpclient.StreamEvent{}), nil
		},
	}

	// Create factory and pipeline
	factory := pipeline.NewFactory(originalExecutor)
	pipeline := factory.Pipeline(inbound, mockCustomized)

	// Create test request
	request := &httpclient.Request{
		Method: "POST",
		URL:    "http://test.com",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}],"stream":true}`),
	}

	// Process streaming request
	result, err := pipeline.Process(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)

	// Verify that CustomizeExecutor was called
	require.True(t, mockCustomized.customizeExecutorCalled, "CustomizeExecutor should have been called")

	// The original executor should have been used since custom executor is nil
	// This is verified by the successful execution
}
