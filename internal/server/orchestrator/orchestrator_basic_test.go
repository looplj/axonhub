package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// TestChatCompletionOrchestrator_Process_NonStreaming tests the complete non-streaming flow.
func TestChatCompletionOrchestrator_Process_NonStreaming(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	// Create mock executor with response
	mockResp := buildMockOpenAIResponse("chatcmpl-123", "gpt-4", "Hello! How can I help you?", 10, 20)
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

	// Create channel selector that returns our test channel
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

	// Build request
	httpRequest := buildTestRequest("gpt-4", "Hello!", false)

	// Set project ID in context
	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute
	result, err := orchestrator.Process(ctx, httpRequest)

	// Assert - no error
	require.NoError(t, err)
	assert.NotNil(t, result.ChatCompletion)
	assert.Nil(t, result.ChatCompletionStream)

	// Verify executor was called
	assert.True(t, executor.requestCalled)

	// Verify request was created in database
	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	dbRequest := requests[0]
	assert.Equal(t, "gpt-4", dbRequest.ModelID)
	assert.Equal(t, project.ID, dbRequest.ProjectID)
	assert.Equal(t, ch.ID, dbRequest.ChannelID)
	assert.Equal(t, request.StatusCompleted, dbRequest.Status)
	assert.Equal(t, "chatcmpl-123", dbRequest.ExternalID)

	// Verify request execution was created
	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)

	dbExec := executions[0]
	assert.Equal(t, ch.ID, dbExec.ChannelID)
	assert.Equal(t, dbRequest.ID, dbExec.RequestID)
	assert.Equal(t, "chatcmpl-123", dbExec.ExternalID)

	// Verify usage log was created
	usageLogs, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, usageLogs, 1)

	dbUsageLog := usageLogs[0]
	assert.Equal(t, dbRequest.ID, dbUsageLog.RequestID)
	assert.Equal(t, int64(10), dbUsageLog.PromptTokens)
	assert.Equal(t, int64(20), dbUsageLog.CompletionTokens)
	assert.Equal(t, int64(30), dbUsageLog.TotalTokens)
}

// TestChatCompletionOrchestrator_Process_WithModelMapping tests model mapping from API key.
func TestChatCompletionOrchestrator_Process_WithModelMapping(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	// Create a user for the API key
	user, err := client.User.Create().
		SetEmail("testuser@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	// Create API key with model mapping
	apiKey, err := client.APIKey.Create().
		SetName("Test API Key").
		SetKey("sk-test-key").
		SetProjectID(project.ID).
		SetUserID(user.ID).
		SetProfiles(&objects.APIKeyProfiles{
			ActiveProfile: "default",
			Profiles: []objects.APIKeyProfile{
				{
					Name: "default",
					ModelMappings: []objects.ModelMapping{
						{From: "my-custom-model", To: "gpt-4"},
					},
				},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Create mock executor
	mockResp := buildMockOpenAIResponse("chatcmpl-456", "gpt-4", "Mapped response", 15, 25)
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

	// Build request with custom model name
	httpRequest := buildTestRequest("my-custom-model", "Test mapping", false)

	// Set context with API key and project
	ctx = contexts.WithProjectID(ctx, project.ID)
	ctx = contexts.WithAPIKey(ctx, apiKey)

	// Execute
	result, err := orchestrator.Process(ctx, httpRequest)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result.ChatCompletion)

	// Verify the request was made with mapped model (gpt-4)
	// The original model in request should be stored, but actual request to provider uses mapped model
	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	// The stored model should be the mapped model (gpt-4) since that's what was actually used
	dbRequest := requests[0]
	assert.Equal(t, "gpt-4", dbRequest.ModelID)
}

// TestChatCompletionOrchestrator_Process_WithOverrideParameters tests channel override parameters.
func TestChatCompletionOrchestrator_Process_WithOverrideParameters(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)

	// Create channel with override parameters
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel with Overrides").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-api-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetSettings(&objects.ChannelSettings{
			OverrideParameters: `{"temperature": 0.9, "max_tokens": 2000}`,
		}).
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	// Create mock executor that captures the request
	mockResp := buildMockOpenAIResponse("chatcmpl-789", "gpt-4", "Override test", 10, 15)
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

	// Build request without temperature
	httpRequest := buildTestRequest("gpt-4", "Test override", false)

	// Set project ID in context
	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute
	result, err := orchestrator.Process(ctx, httpRequest)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result.ChatCompletion)

	// Verify the request was modified with override parameters
	assert.True(t, executor.requestCalled)
	assert.NotNil(t, executor.lastRequest)

	// Parse the request body to verify overrides were applied
	var reqBody map[string]any

	err = json.Unmarshal(executor.lastRequest.Body, &reqBody)
	require.NoError(t, err)

	// Check that temperature was overridden
	assert.Equal(t, 0.9, reqBody["temperature"])
	assert.Equal(t, float64(2000), reqBody["max_tokens"])
}

// TestChatCompletionOrchestrator_Process_MultipleRequests tests multiple sequential requests.
func TestChatCompletionOrchestrator_Process_MultipleRequests(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Setup
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	requestCount := 0
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
		SystemService:     systemService,
		UsageLogService:   usageLogService,
		PipelineFactory:   pipeline.NewFactory(executor),
		ModelMapper:       NewModelMapper(),
		connectionTracker: NewDefaultConnectionTracker(1024),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute multiple requests
	for i := 0; i < 3; i++ {
		requestCount++
		respID := lo.RandomString(10, lo.LettersCharset)
		mockResp := buildMockOpenAIResponse(
			respID,
			"gpt-4",
			"Response "+string(rune('A'+i)),
			10+i,
			20+i,
		)
		executor.response = &httpclient.Response{
			StatusCode: 200,
			Body:       mockResp,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		}

		httpRequest := buildTestRequest("gpt-4", "Request "+string(rune('A'+i)), false)
		result, err := orchestrator.Process(ctx, httpRequest)

		require.NoError(t, err)
		assert.NotNil(t, result.ChatCompletion)
	}

	// Verify all requests were created
	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	assert.Len(t, requests, 3)

	// Verify all executions were created
	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	assert.Len(t, executions, 3)

	// Verify all usage logs were created
	usageLogs, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	assert.Len(t, usageLogs, 3)
}
