package orchestrator

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/pipeline/stream"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
)

// TestChatCompletionOrchestrator_Process_Streaming tests the complete streaming flow.
func TestChatCompletionOrchestrator_Process_Streaming(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

// TestChatCompletionOrchestrator_Process_ConnectionTracking tests connection tracking.
func TestChatCompletionOrchestrator_Process_ConnectionTracking(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
