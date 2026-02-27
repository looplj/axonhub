package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// mockInboundTransformer is a mock transformer for testing.
type mockInboundTransformer struct {
	aggregateResponseBody []byte
	aggregateMeta         llm.ResponseMeta
	aggregateErr          error
}

func (m *mockInboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIChatCompletion
}

func (m *mockInboundTransformer) TransformRequest(ctx context.Context, request *httpclient.Request) (*llm.Request, error) {
	return &llm.Request{}, nil
}

func (m *mockInboundTransformer) TransformResponse(ctx context.Context, response *llm.Response) (*httpclient.Response, error) {
	return &httpclient.Response{}, nil
}

func (m *mockInboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, nil
}

func (m *mockInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	return nil
}

func (m *mockInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return m.aggregateResponseBody, m.aggregateMeta, m.aggregateErr
}

// mockStream is a simple mock stream for testing.
type mockStream struct {
	events     []*httpclient.StreamEvent
	currentIdx int
	closed     bool
	err        error
}

func (m *mockStream) Next() bool {
	if m.currentIdx >= len(m.events) {
		return false
	}
	m.currentIdx++
	return true
}

func (m *mockStream) Current() *httpclient.StreamEvent {
	if m.currentIdx > len(m.events) {
		return nil
	}
	return m.events[m.currentIdx-1]
}

func (m *mockStream) Err() error {
	return m.err
}

func (m *mockStream) Close() error {
	m.closed = true
	return nil
}

// createTestRequestService creates a minimal request service for testing.
func createTestRequestService(t *testing.T, client *ent.Client) *biz.RequestService {
	t.Helper()

	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})

	dataStorageService := &biz.DataStorageService{
		AbstractService: &biz.AbstractService{},
		SystemService:   systemService,
		Cache:           xcache.NewFromConfig[ent.DataStorage](xcache.Config{Mode: xcache.ModeMemory}),
	}

	channelService := biz.NewChannelServiceForTest(client)
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	return biz.NewRequestService(client, systemService, usageLogService, dataStorageService)
}

// TestInboundPersistentStream_Close_WithCompleteResponse tests the NEW behavior:
// complete response without terminal event (e.g., Codex executor that aggregates internally)
func TestInboundPersistentStream_Close_WithCompleteResponse(t *testing.T) {
	// Setup
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	// Create a complete response chunk with non-terminal event type
	// This simulates Codex executor behavior where the response is complete but lacks terminal event
	completeResponseChunk := &httpclient.StreamEvent{
		Type: "chunk",
		Data: []byte(`{"id":"chatcmpl-abc123","object":"chat.completion","created":1234567890,"model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`),
	}

	// Create mock stream with single complete response chunk (no [DONE] event)
	mockStream := &mockStream{
		events: []*httpclient.StreamEvent{completeResponseChunk},
	}

	// Create mock transformer that returns valid aggregated response
	mockTransformer := &mockInboundTransformer{
		aggregateResponseBody: []byte(`{"id":"chatcmpl-abc123","object":"chat.completion","created":1234567890,"model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`),
		aggregateMeta: llm.ResponseMeta{
			ID: "chatcmpl-abc123",
			Usage: &llm.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
		aggregateErr: nil,
	}

	// Create real request service
	requestService := createTestRequestService(t, client)

	// Create test request and execution
	testRequest := &ent.Request{
		ID: 1,
	}
	testRequestExec := &ent.RequestExecution{
		ID: 1,
	}

	// Create persistence state
	state := &PersistenceState{
		StreamCompleted: false,
	}

	// Create the InboundPersistentStream
	stream := NewInboundPersistentStream(
		ctx,
		mockStream,
		testRequest,
		testRequestExec,
		requestService,
		mockTransformer,
		nil,
		state,
	)

	// Execute - Call Next() to process the chunk, then Close()
	require.True(t, stream.Next(), "Expected Next() to return true")
	event := stream.Current()
	require.NotNil(t, event, "Expected current event to not be nil")

	// Verify that StreamCompleted is still false (no terminal event yet)
	assert.False(t, state.StreamCompleted, "StreamCompleted should be false before Close()")

	// Call Close()
	err := stream.Close()
	require.NoError(t, err, "Close() should not return an error")

	// Assert - Verify StreamCompleted is set to true
	assert.True(t, state.StreamCompleted, "StreamCompleted should be true after Close() with complete response")

	// Verify stream is closed
	assert.True(t, mockStream.closed, "Stream should be closed")
}

// TestInboundPersistentStream_Close_WithTerminalEvent tests the EXISTING behavior:
// terminal event (e.g., [DONE] event from OpenAI)
func TestInboundPersistentStream_Close_WithTerminalEvent(t *testing.T) {
	// Setup
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	// Create a regular response chunk
	regularResponseChunk := &httpclient.StreamEvent{
		Type: "chunk",
		Data: []byte(`{"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`),
	}

	// Create the [DONE] terminal event
	doneEvent := &httpclient.StreamEvent{
		Data: []byte("[DONE]"),
	}

	// Create mock stream with response chunk followed by [DONE] event
	mockStream := &mockStream{
		events: []*httpclient.StreamEvent{regularResponseChunk, doneEvent},
	}

	// Create mock transformer
	mockTransformer := &mockInboundTransformer{
		aggregateResponseBody: []byte(`{"id":"chatcmpl-abc123","object":"chat.completion","created":1234567890,"model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`),
		aggregateMeta: llm.ResponseMeta{
			ID: "chatcmpl-abc123",
			Usage: &llm.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
		aggregateErr: nil,
	}

	// Create real request service
	requestService := createTestRequestService(t, client)

	// Create test request and execution
	testRequest := &ent.Request{
		ID: 1,
	}
	testRequestExec := &ent.RequestExecution{
		ID: 1,
	}

	// Create persistence state
	state := &PersistenceState{
		StreamCompleted: false,
	}

	// Create the InboundPersistentStream
	stream := NewInboundPersistentStream(
		ctx,
		mockStream,
		testRequest,
		testRequestExec,
		requestService,
		mockTransformer,
		nil,
		state,
	)

	// Execute - Process all events
	require.True(t, stream.Next(), "Expected Next() to return true for first chunk")
	_ = stream.Current()

	require.True(t, stream.Next(), "Expected Next() to return true for [DONE] event")
	event := stream.Current()
	require.NotNil(t, event, "Expected current event to not be nil")

	// Verify that StreamCompleted is set to true by the [DONE] event
	assert.True(t, state.StreamCompleted, "StreamCompleted should be true after [DONE] event")

	// Call Close()
	err := stream.Close()
	require.NoError(t, err, "Close() should not return an error")

	// Assert - Verify StreamCompleted is still true
	assert.True(t, state.StreamCompleted, "StreamCompleted should remain true after Close()")

	// Verify stream is closed
	assert.True(t, mockStream.closed, "Stream should be closed")
}

// TestInboundPersistentStream_Close_WithAggregationError tests the error path:
// aggregation fails but fallback behavior still works (persistResponseChunks called in final block)
func TestInboundPersistentStream_Close_WithAggregationError(t *testing.T) {
	// Setup
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	// Create a regular response chunk
	regularResponseChunk := &httpclient.StreamEvent{
		Type: "chunk",
		Data: []byte(`{"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`),
	}

	// Create mock stream with response chunk (no terminal event)
	mockStream := &mockStream{
		events: []*httpclient.StreamEvent{regularResponseChunk},
	}

	// Create mock transformer with aggregation error
	mockTransformer := &mockInboundTransformer{
		aggregateResponseBody: nil,
		aggregateMeta:         llm.ResponseMeta{},
		aggregateErr:          errors.New("aggregation failed"),
	}

	// Create real request service
	requestService := createTestRequestService(t, client)

	// Create test request and execution
	testRequest := &ent.Request{
		ID: 1,
	}
	testRequestExec := &ent.RequestExecution{
		ID: 1,
	}

	// Create persistence state
	state := &PersistenceState{
		StreamCompleted: false,
	}

	// Create the InboundPersistentStream
	stream := NewInboundPersistentStream(
		ctx,
		mockStream,
		testRequest,
		testRequestExec,
		requestService,
		mockTransformer,
		nil,
		state,
	)

	// Execute - Process the chunk
	require.True(t, stream.Next(), "Expected Next() to return true for first chunk")
	_ = stream.Current()

	// Verify that StreamCompleted is still false (no terminal event yet)
	assert.False(t, state.StreamCompleted, "StreamCompleted should be false before Close()")

	// Call Close()
	err := stream.Close()
	require.NoError(t, err, "Close() should not return an error")

	// Assert - Verify StreamCompleted is still false (no terminal event received)
	assert.False(t, state.StreamCompleted, "StreamCompleted should remain false after Close() with aggregation error")

	// Verify stream is closed
	assert.True(t, mockStream.closed, "Stream should be closed")
}
