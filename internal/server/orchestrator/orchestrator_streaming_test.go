package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// TestChatCompletionOrchestrator_Process_Streaming tests the complete streaming flow.
func TestChatCompletionOrchestrator_Process_Streaming(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	// Create mock stream events
	streamEvents := []*httpclient.StreamEvent{
		{
			Data: []byte(
				`{"id":"chatcmpl-123","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			),
		},
		{Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)},
		{Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}`)},
		{
			Data: []byte(
				`{"id":"chatcmpl-123","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
			),
		},
	}

	executor := &mockExecutor{
		streamEvents: streamEvents,
	}

	// Create outbound transformer
	outbound, err := openai.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
	require.NoError(t, err)

	// Create channel selector
	bizChannel := &biz.Channel{
		Channel:  ch,
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "gpt-4")}

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:   channelSelector,
		Inbound:           openai.NewInboundTransformer(),
		RequestService:    requestService,
		ChannelService:    channelService,
		PromptProvider:    &stubPromptProvider{},
		SystemService:     systemService,
		UsageLogService:   usageLogService,
		PipelineFactory:   pipeline.NewFactory(executor),
		ModelMapper:       NewModelMapper(),
		connectionTracker: NewDefaultConnectionTracker(1024),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	// Build streaming request
	httpRequest := buildTestRequest("gpt-4", "Hi!", true)

	// Set project ID in context
	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute
	result, err := orchestrator.Process(ctx, httpRequest)

	// Assert - no error
	require.NoError(t, err)
	assert.Nil(t, result.ChatCompletion)
	assert.NotNil(t, result.ChatCompletionStream)

	// Consume the stream
	var chunks []*httpclient.StreamEvent
	for result.ChatCompletionStream.Next() {
		chunks = append(chunks, result.ChatCompletionStream.Current())
	}

	err = result.ChatCompletionStream.Close()
	require.NoError(t, err)

	// Verify chunks were received
	assert.Len(t, chunks, 4)

	// Verify request was created in database
	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	dbRequest := requests[0]
	assert.Equal(t, "gpt-4", dbRequest.ModelID)
	assert.Equal(t, project.ID, dbRequest.ProjectID)

	// Verify request execution was created
	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)

	dbExec := executions[0]
	assert.Equal(t, ch.ID, dbExec.ChannelID)
	assert.Equal(t, dbRequest.ID, dbExec.RequestID)
}

// TestChatCompletionOrchestrator_Process_StreamingError tests that mid-stream errors
// properly mark both request and request execution as failed.
func TestChatCompletionOrchestrator_Process_StreamingError(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	// Create a stream that emits some events then errors
	midStreamErr := errors.New("upstream connection reset")
	executor := &mockExecutorWithErrorStream{
		events: []*httpclient.StreamEvent{
			{
				Data: []byte(
					`{"id":"chatcmpl-err","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
				),
			},
			{Data: []byte(`{"id":"chatcmpl-err","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)},
		},
		streamErr: midStreamErr,
	}

	// Create outbound transformer
	outbound, err := openai.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
	require.NoError(t, err)

	bizChannel := &biz.Channel{
		Channel:  ch,
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "gpt-4")}

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:   channelSelector,
		Inbound:           openai.NewInboundTransformer(),
		RequestService:    requestService,
		ChannelService:    channelService,
		PromptProvider:    &stubPromptProvider{},
		SystemService:     systemService,
		UsageLogService:   usageLogService,
		PipelineFactory:   pipeline.NewFactory(executor),
		ModelMapper:       NewModelMapper(),
		connectionTracker: NewDefaultConnectionTracker(1024),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	// Build streaming request
	httpRequest := buildTestRequest("gpt-4", "Hi!", true)
	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute - the stream should be established successfully
	result, err := orchestrator.Process(ctx, httpRequest)
	require.NoError(t, err)
	assert.Nil(t, result.ChatCompletion)
	assert.NotNil(t, result.ChatCompletionStream)

	// Consume the stream - it should error mid-way
	var chunks []*httpclient.StreamEvent
	for result.ChatCompletionStream.Next() {
		chunks = append(chunks, result.ChatCompletionStream.Current())
	}

	// Verify stream error
	assert.Error(t, result.ChatCompletionStream.Err())

	// Close the stream (triggers persistence)
	err = result.ChatCompletionStream.Close()
	require.NoError(t, err)

	// Verify request was created and marked as failed
	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	dbRequest := requests[0]
	assert.Equal(t, request.StatusFailed, dbRequest.Status, "request should be marked as failed on stream error")

	// Verify request execution was created and marked as failed
	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)

	dbExec := executions[0]
	assert.Equal(t, requestexecution.StatusFailed, dbExec.Status, "request execution should be marked as failed on stream error")
}

// TestChatCompletionOrchestrator_Process_StreamingSuccess_NotMarkedAsError verifies that
// a successfully completed stream does NOT mark request/execution as failed.
func TestChatCompletionOrchestrator_Process_StreamingSuccess_NotMarkedAsError(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	streamEvents := []*httpclient.StreamEvent{
		{
			Data: []byte(
				`{"id":"chatcmpl-ok","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			),
		},
		{Data: []byte(`{"id":"chatcmpl-ok","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)},
		{
			Data: []byte(
				`{"id":"chatcmpl-ok","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
			),
		},
	}

	executor := &mockExecutor{streamEvents: streamEvents}

	outbound, err := openai.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
	require.NoError(t, err)

	bizChannel := &biz.Channel{
		Channel:  ch,
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "gpt-4")}

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:   channelSelector,
		Inbound:           openai.NewInboundTransformer(),
		RequestService:    requestService,
		ChannelService:    channelService,
		PromptProvider:    &stubPromptProvider{},
		SystemService:     systemService,
		UsageLogService:   usageLogService,
		PipelineFactory:   pipeline.NewFactory(executor),
		ModelMapper:       NewModelMapper(),
		connectionTracker: NewDefaultConnectionTracker(1024),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	httpRequest := buildTestRequest("gpt-4", "Hi!", true)
	ctx = contexts.WithProjectID(ctx, project.ID)

	result, err := orchestrator.Process(ctx, httpRequest)
	require.NoError(t, err)
	assert.NotNil(t, result.ChatCompletionStream)

	// Consume stream fully
	for result.ChatCompletionStream.Next() {
		_ = result.ChatCompletionStream.Current()
	}

	require.NoError(t, result.ChatCompletionStream.Err())

	err = result.ChatCompletionStream.Close()
	require.NoError(t, err)

	// Verify request is completed, NOT failed
	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	dbRequest := requests[0]
	assert.Equal(t, request.StatusCompleted, dbRequest.Status, "successful stream should be marked as completed")

	// Verify request execution is completed
	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)

	dbExec := executions[0]
	assert.Equal(t, requestexecution.StatusCompleted, dbExec.Status, "successful stream execution should be marked as completed")
}

// mockExecutorWithErrorStream returns a stream that emits events then errors.
type mockExecutorWithErrorStream struct {
	events    []*httpclient.StreamEvent
	streamErr error
}

func (m *mockExecutorWithErrorStream) Do(_ context.Context, _ *httpclient.Request) (*httpclient.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *mockExecutorWithErrorStream) DoStream(_ context.Context, _ *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return &errorAfterEventsStream{
		items: m.events,
		err:   m.streamErr,
	}, nil
}

// errorAfterEventsStream emits all items then returns an error.
type errorAfterEventsStream struct {
	items []*httpclient.StreamEvent
	idx   int
	err   error
}

func (s *errorAfterEventsStream) Next() bool {
	return s.idx < len(s.items)
}

func (s *errorAfterEventsStream) Current() *httpclient.StreamEvent {
	item := s.items[s.idx]
	s.idx++

	return item
}

func (s *errorAfterEventsStream) Err() error {
	if s.idx >= len(s.items) {
		return s.err
	}

	return nil
}

func (s *errorAfterEventsStream) Close() error { return nil }

// TestChatCompletionOrchestrator_Process_ConnectionTracking tests connection tracking.
func TestChatCompletionOrchestrator_Process_ConnectionTracking(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	// Create mock executor
	mockResp := buildMockOpenAIResponse("chatcmpl-conn", "gpt-4", "Connection test", 5, 10)
	executor := &mockExecutor{
		response: &httpclient.Response{
			StatusCode: 200,
			Body:       mockResp,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	// Create outbound transformer
	outbound, err := openai.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
	require.NoError(t, err)

	bizChannel := &biz.Channel{
		Channel:  ch,
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "gpt-4")}

	// Create connection tracker
	connectionTracker := NewDefaultConnectionTracker(1024)

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:   channelSelector,
		Inbound:           openai.NewInboundTransformer(),
		RequestService:    requestService,
		ChannelService:    channelService,
		PromptProvider:    &stubPromptProvider{},
		SystemService:     systemService,
		UsageLogService:   usageLogService,
		PipelineFactory:   pipeline.NewFactory(executor),
		ModelMapper:       NewModelMapper(),
		connectionTracker: connectionTracker,
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	// Verify initial connection count is 0
	assert.Equal(t, 0, connectionTracker.GetActiveConnections(ch.ID))

	// Build request
	httpRequest := buildTestRequest("gpt-4", "Connection test", false)
	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute
	result, err := orchestrator.Process(ctx, httpRequest)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result.ChatCompletion)

	// After completion, connection count should be back to 0
	assert.Equal(t, 0, connectionTracker.GetActiveConnections(ch.ID))
}
