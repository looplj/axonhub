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
	"github.com/looplj/axonhub/internal/objects"
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
		channelLimiterManager:      NewChannelLimiterManager(),
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
		channelLimiterManager:      NewChannelLimiterManager(),
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
		channelLimiterManager:      NewChannelLimiterManager(),
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

// TestChatCompletionOrchestrator_Process_QueueRejectionDoesNotConsumeRPM is a
// regression test for the middleware ordering invariant: channel admission must
// run BEFORE rate-limit tracking so a locally rejected request does not bump the
// per-channel RPM counter for a request that never reached upstream. Reversing
// the order would let a flood of queue rejections push the channel into a false
// "RPM exhausted" state and force avoidable failover.
func TestChatCompletionOrchestrator_Process_QueueRejectionDoesNotConsumeRPM(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	mockResp := buildMockOpenAIResponse("chatcmpl-rpm", "gpt-4", "rpm test", 5, 10)
	executor := &mockExecutor{
		response: &httpclient.Response{
			StatusCode: 200,
			Body:       mockResp,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	outbound, err := openai.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
	require.NoError(t, err)

	maxConcurrent := int64(1)
	queueSize := int64(2)
	queueTimeoutMs := int64(30)
	bizChannel := &biz.Channel{
		Channel: &ent.Channel{
			ID:               ch.ID,
			Name:             ch.Name,
			BaseURL:          ch.BaseURL,
			Credentials:      ch.Credentials,
			SupportedModels:  ch.SupportedModels,
			DefaultTestModel: ch.DefaultTestModel,
			Status:           ch.Status,
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{
					MaxConcurrent:  &maxConcurrent,
					QueueSize:      &queueSize,
					QueueTimeoutMs: &queueTimeoutMs,
				},
			},
		},
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "gpt-4")}

	mgr := NewChannelLimiterManager()
	rateLimitTracker := NewChannelRequestTracker()

	// Saturate capacity externally so the orchestrator's Acquire must enter the
	// queue and eventually hit the per-channel timeout.
	lim := mgr.GetOrCreate(bizChannel)
	require.NotNil(t, lim)
	require.NoError(t, lim.Acquire(ctx))
	defer lim.Release()

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:       channelSelector,
		Inbound:               openai.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: mgr,
		rateLimitTracker:      rateLimitTracker,
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	httpRequest := buildTestRequest("gpt-4", "rpm test", false)
	ctx = contexts.WithProjectID(ctx, project.ID)

	_, err = orchestrator.Process(ctx, httpRequest)
	require.Error(t, err)

	var queueErr *ChannelQueueError
	require.ErrorAs(t, err, &queueErr, "expected channel queue rejection")
	assert.Equal(t, channelQueueReasonTimeout, queueErr.Reason)

	assert.Zero(t, rateLimitTracker.GetRequestCount(ch.ID),
		"queue rejection must not consume RPM budget — middleware order regression")
}

// TestChatCompletionOrchestrator_Process_ChannelLimiter exercises the channel admission
// middleware end-to-end: configure a channel with MaxConcurrent + QueueSize, run a
// request through the orchestrator, and confirm the limiter slot is released after
// completion.
func TestChatCompletionOrchestrator_Process_ChannelLimiter(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	mockResp := buildMockOpenAIResponse("chatcmpl-conn", "gpt-4", "Connection test", 5, 10)
	executor := &mockExecutor{
		response: &httpclient.Response{
			StatusCode: 200,
			Body:       mockResp,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	outbound, err := openai.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
	require.NoError(t, err)

	maxConcurrent := int64(2)
	queueSize := int64(5)
	bizChannel := &biz.Channel{
		Channel: &ent.Channel{
			ID:               ch.ID,
			Name:             ch.Name,
			BaseURL:          ch.BaseURL,
			Credentials:      ch.Credentials,
			SupportedModels:  ch.SupportedModels,
			DefaultTestModel: ch.DefaultTestModel,
			Status:           ch.Status,
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{
					MaxConcurrent: &maxConcurrent,
					QueueSize:     &queueSize,
				},
			},
		},
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "gpt-4")}

	mgr := NewChannelLimiterManager()

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:       channelSelector,
		Inbound:               openai.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: mgr,
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	httpRequest := buildTestRequest("gpt-4", "Connection test", false)
	ctx = contexts.WithProjectID(ctx, project.ID)

	result, err := orchestrator.Process(ctx, httpRequest)
	require.NoError(t, err)
	assert.NotNil(t, result.ChatCompletion)

	inFlight, waiting, ok := mgr.Stats(ch.ID)
	require.True(t, ok, "limiter should have been created for the configured channel")
	assert.Equal(t, 0, inFlight, "slot must be released after request completion")
	assert.Equal(t, 0, waiting)
}
