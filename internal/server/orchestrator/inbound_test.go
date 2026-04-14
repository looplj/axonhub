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
	"github.com/looplj/axonhub/llm/transformer"
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
	liveStreamRegistry := biz.NewLiveStreamRegistry()

	channelService := biz.NewChannelServiceForTest(client)
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	return biz.NewRequestService(client, systemService, usageLogService, dataStorageService, liveStreamRegistry)
}

// newInboundPersistentStreamHelper creates a configured InboundPersistentStream for testing.
// It encapsulates common setup: ent.Client, context, RequestService, test request/execution, and persistence state.
// The caller provides the mock stream and transformer to test specific behaviors.
func newInboundPersistentStreamHelper(
	t *testing.T,
	mockStream streams.Stream[*httpclient.StreamEvent],
	mockTransformer transformer.Inbound,
) (*InboundPersistentStream, *ent.Client, context.Context, *PersistenceState) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	requestService := createTestRequestService(t, client)

	testRequest := &ent.Request{ID: 1}
	testRequestExec := &ent.RequestExecution{ID: 1}

	state := &PersistenceState{StreamCompleted: false}

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

	return stream, client, ctx, state
}

// TestInboundPersistentStream_Close_WithCompleteResponse tests the NEW behavior:
// complete response without terminal event (e.g., Codex executor that aggregates internally)
func TestInboundPersistentStream_Close_WithCompleteResponse(t *testing.T) {
	completeResponseChunk := &httpclient.StreamEvent{
		Type: "chunk",
		Data: []byte(`{"id":"chatcmpl-abc123","object":"chat.completion","created":1234567890,"model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`),
	}

	mockStream := &mockStream{
		events: []*httpclient.StreamEvent{completeResponseChunk},
	}

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

	stream, client, _, state := newInboundPersistentStreamHelper(t, mockStream, mockTransformer)
	defer client.Close()

	require.True(t, stream.Next(), "Expected Next() to return true")
	event := stream.Current()
	require.NotNil(t, event, "Expected current event to not be nil")

	assert.False(t, state.StreamCompleted, "StreamCompleted should be false before Close()")

	err := stream.Close()
	require.NoError(t, err, "Close() should not return an error")

	assert.True(t, state.StreamCompleted, "StreamCompleted should be true after Close() with complete response")
	assert.True(t, mockStream.closed, "Stream should be closed")
}

// TestInboundPersistentStream_Close_WithTerminalEvent tests the EXISTING behavior:
// terminal event (e.g., [DONE] event from OpenAI)
func TestInboundPersistentStream_Close_WithTerminalEvent(t *testing.T) {
	regularResponseChunk := &httpclient.StreamEvent{
		Type: "chunk",
		Data: []byte(`{"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`),
	}

	doneEvent := &httpclient.StreamEvent{
		Data: []byte("[DONE]"),
	}

	mockStream := &mockStream{
		events: []*httpclient.StreamEvent{regularResponseChunk, doneEvent},
	}

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

	stream, client, _, state := newInboundPersistentStreamHelper(t, mockStream, mockTransformer)
	defer client.Close()

	require.True(t, stream.Next(), "Expected Next() to return true for first chunk")
	_ = stream.Current()

	require.True(t, stream.Next(), "Expected Next() to return true for [DONE] event")
	event := stream.Current()
	require.NotNil(t, event, "Expected current event to not be nil")

	assert.True(t, state.StreamCompleted, "StreamCompleted should be true after [DONE] event")

	err := stream.Close()
	require.NoError(t, err, "Close() should not return an error")

	assert.True(t, state.StreamCompleted, "StreamCompleted should remain true after Close()")
	assert.True(t, mockStream.closed, "Stream should be closed")
}

// TestInboundPersistentStream_Close_WithAggregationError tests the error path:
// aggregation fails but fallback behavior still works (persistResponseChunks called in final block).
func TestInboundPersistentStream_Close_WithAggregationError(t *testing.T) {
	regularResponseChunk := &httpclient.StreamEvent{
		Type: "chunk",
		Data: []byte(`{"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`),
	}

	mockStream := &mockStream{
		events: []*httpclient.StreamEvent{regularResponseChunk},
	}

	mockTransformer := &mockInboundTransformer{
		aggregateResponseBody: nil,
		aggregateMeta:         llm.ResponseMeta{},
		aggregateErr:          errors.New("aggregation failed"),
	}

	stream, client, _, state := newInboundPersistentStreamHelper(t, mockStream, mockTransformer)
	defer client.Close()

	require.True(t, stream.Next(), "Expected Next() to return true for first chunk")
	_ = stream.Current()

	assert.False(t, state.StreamCompleted, "StreamCompleted should be false before Close()")

	err := stream.Close()
	require.NoError(t, err, "Close() should not return an error")

	assert.False(t, state.StreamCompleted, "StreamCompleted should remain false after Close() with aggregation error")
	assert.True(t, mockStream.closed, "Stream should be closed")
}
