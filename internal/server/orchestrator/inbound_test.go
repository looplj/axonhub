package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
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

	return biz.NewRequestService(client, systemService.CacheConfig, systemService, usageLogService, dataStorageService, liveStreamRegistry)
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

// TestInboundPersistentStream_Close_WithCompleteResponse verifies that a complete
// response carrying finish_reason is recognized before Close persists it.
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

	assert.True(t, state.StreamCompleted, "StreamCompleted should be true after consuming finish_reason")

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

func TestIsTerminalStreamEvent_AudioDoneEvents(t *testing.T) {
	// OpenAI audio SSE streams have no [DONE] sentinel; terminal completion is
	// signaled by typed *.done events surfaced via StreamEvent.Type.
	require.True(t, IsTerminalStreamEvent(&httpclient.StreamEvent{Type: "speech.audio.done"}))
	require.True(t, IsTerminalStreamEvent(&httpclient.StreamEvent{Type: "transcript.text.done"}))
	require.True(t, IsTerminalStreamEvent(&httpclient.StreamEvent{Type: httpclient.BinaryStreamDoneEventType}))

	// Other events must not be treated as terminal.
	require.False(t, IsTerminalStreamEvent(&httpclient.StreamEvent{Type: "speech.audio.delta"}))
	require.False(t, IsTerminalStreamEvent(&httpclient.StreamEvent{Type: "transcript.text.delta"}))
	require.False(t, IsTerminalStreamEvent(&httpclient.StreamEvent{Type: "audio/mpeg"}))
}

// TestInboundPersistentStream_Close_IncompleteStillPersistsChunks ensures a clean
// upstream EOF without terminal/completion still saves buffered chunks when
// store_chunks is on — without marking the request completed.
func TestInboundPersistentStream_Close_IncompleteStillPersistsChunks(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	_, requestService, systemService, _ := setupTestServices(t, client)
	require.NoError(t, systemService.SetStoragePolicy(ctx, &biz.StoragePolicy{
		StoreChunks:       true,
		StoreRequestBody:  true,
		StoreResponseBody: true,
	}))

	req, err := client.Request.Create().
		SetProjectID(project.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetStatus(request.StatusProcessing).
		SetRequestBody([]byte(`{"stream":true}`)).
		SetStream(true).
		Save(ctx)
	require.NoError(t, err)

	partial := &httpclient.StreamEvent{
		Type: "chunk",
		Data: []byte(`{"id":"chatcmpl-partial","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`),
	}
	mockStream := &mockStream{events: []*httpclient.StreamEvent{partial}}
	mockTransformer := &mockInboundTransformer{
		aggregateErr: errors.New("incomplete aggregation"),
	}
	state := &PersistenceState{}

	stream := NewInboundPersistentStream(
		ctx,
		mockStream,
		req,
		&ent.RequestExecution{ID: 1},
		requestService,
		mockTransformer,
		nil,
		state,
	)
	require.True(t, stream.Next())
	_ = stream.Current()
	require.NoError(t, stream.Close())

	require.False(t, state.StreamCompleted)

	dbReq, err := client.Request.Get(ctx, req.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusFailed, dbReq.Status)
	require.NotEmpty(t, dbReq.ResponseChunks, "failed request should keep response_chunks in DB")

	chunks, err := requestService.LoadResponseChunks(ctx, dbReq)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
}

func TestIsTerminalStreamEvent_SemanticCompletionInData(t *testing.T) {
	tests := []struct {
		name  string
		event *httpclient.StreamEvent
		want  bool
	}{
		{
			name:  "responses completion without SSE event field",
			event: &httpclient.StreamEvent{Data: []byte(`{"type":"response.completed","response":{"status":"completed"}}`)},
			want:  true,
		},
		{
			name:  "anthropic message stop without SSE event field",
			event: &httpclient.StreamEvent{Data: []byte(`{"type":"message_stop"}`)},
			want:  true,
		},
		{
			name:  "chat completion finish reason",
			event: &httpclient.StreamEvent{Data: []byte(`{"choices":[{"index":0,"finish_reason":"stop"}]}`)},
			want:  true,
		},
		{
			name:  "chat completion without finish reason",
			event: &httpclient.StreamEvent{Data: []byte(`{"choices":[{"index":0,"finish_reason":null}]}`)},
			want:  false,
		},
		{
			name:  "gemini finish reason stop",
			event: &httpclient.StreamEvent{Data: []byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"world!"}]},"finishReason":"STOP"}]}`)},
			want:  true,
		},
		{
			name:  "gemini finish reason max tokens",
			event: &httpclient.StreamEvent{Data: []byte(`{"candidates":[{"index":0,"finishReason":"MAX_TOKENS"}]}`)},
			want:  true,
		},
		{
			name:  "gemini chunk without finish reason",
			event: &httpclient.StreamEvent{Data: []byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"hello"}]}}]}`)},
			want:  false,
		},
		{
			name:  "gemini empty finish reason",
			event: &httpclient.StreamEvent{Data: []byte(`{"candidates":[{"index":0,"finishReason":""}]}`)},
			want:  false,
		},
		{
			name:  "non-terminal responses event",
			event: &httpclient.StreamEvent{Data: []byte(`{"type":"response.output_text.delta","delta":"done"}`)},
			want:  false,
		},
		{
			name: "nil event",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsTerminalStreamEvent(tt.event))
		})
	}
}
