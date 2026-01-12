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
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// TestChatCompletionOrchestrator_Process_ErrorHandling tests error handling.
func TestChatCompletionOrchestrator_Process_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	// Create mock executor that returns an error
	executor := &mockExecutor{
		err: &httpclient.Error{
			StatusCode: 401,
			Body:       []byte(`{"error":{"message":"Invalid API key","type":"invalid_request_error"}}`),
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

	// Build request
	httpRequest := buildTestRequest("gpt-4", "This will fail", false)

	// Set project ID in context
	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute
	result, err := orchestrator.Process(ctx, httpRequest)

	// Assert - should return error
	require.Error(t, err)
	assert.Empty(t, result)

	// Verify request was created but marked as failed
	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	dbRequest := requests[0]
	assert.Equal(t, request.StatusFailed, dbRequest.Status)

	// Verify request execution was created and marked as failed
	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)

	dbExec := executions[0]
	assert.NotEmpty(t, dbExec.ErrorMessage)
}

// TestChatCompletionOrchestrator_Process_NoChannelsAvailable tests error when no channels are available.
func TestChatCompletionOrchestrator_Process_NoChannelsAvailable(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	executor := &mockExecutor{}

	// Empty channel selector
	channelSelector := &staticChannelSelector{candidates: []*ChannelModelCandidate{}}

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

	// Build request
	httpRequest := buildTestRequest("gpt-4", "No channels", false)
	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute
	_, err := orchestrator.Process(ctx, httpRequest)
	require.ErrorIs(t, err, biz.ErrInvalidModel)
}

// TestChatCompletionOrchestrator_Process_InvalidRequest tests invalid request handling.
func TestChatCompletionOrchestrator_Process_InvalidRequest(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	executor := &mockExecutor{}

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

	// Build invalid request (missing model)
	invalidReq := &httpclient.Request{
		Method: "POST",
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"messages":[{"role":"user","content":"test"}]}`),
	}

	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute
	_, err = orchestrator.Process(ctx, invalidReq)

	// Assert - should return error about missing model
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}
