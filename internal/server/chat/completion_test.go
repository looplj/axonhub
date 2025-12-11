package chat

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
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/pipeline/stream"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

// mockExecutor implements pipeline.Executor for testing.
type mockExecutor struct {
	response      *httpclient.Response
	streamEvents  []*httpclient.StreamEvent
	err           error
	requestCalled bool
	lastRequest   *httpclient.Request
}

func (m *mockExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	m.requestCalled = true

	m.lastRequest = request
	if m.err != nil {
		return nil, m.err
	}

	return m.response, nil
}

func (m *mockExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	m.requestCalled = true

	m.lastRequest = request
	if m.err != nil {
		return nil, m.err
	}

	return streams.SliceStream(m.streamEvents), nil
}

// setupTestServices creates all necessary services for integration testing.
func setupTestServices(t *testing.T, client *ent.Client) (*biz.ChannelService, *biz.RequestService, *biz.SystemService, *biz.UsageLogService) {
	t.Helper()

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: cacheConfig,
		Ent:         client,
	})

	// Create data storage service
	dataStorageService := &biz.DataStorageService{
		AbstractService: &biz.AbstractService{},
		SystemService:   systemService,
		Cache:           xcache.NewFromConfig[ent.DataStorage](cacheConfig),
	}

	usageLogService := biz.NewUsageLogService(client, systemService)
	requestService := biz.NewRequestService(client, systemService, usageLogService, dataStorageService)

	// Create a minimal channel service for testing
	channelService := biz.NewChannelServiceForTest(client)

	return channelService, requestService, systemService, usageLogService
}

// createTestChannel creates a test channel in the database.
func createTestChannel(t *testing.T, ctx context.Context, client *ent.Client) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test OpenAI Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-api-key"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-3.5-turbo").
		Save(ctx)
	require.NoError(t, err)

	return ch
}

// createTestProject creates a test project in the database.
func createTestProject(t *testing.T, ctx context.Context, client *ent.Client) *ent.Project {
	t.Helper()

	project, err := client.Project.Create().
		SetName("Test Project").
		Save(ctx)
	require.NoError(t, err)

	return project
}

// buildMockOpenAIResponse creates a mock OpenAI chat completion response.
func buildMockOpenAIResponse(id, model, content string, promptTokens, completionTokens int) []byte {
	resp := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": 1234567890,
		"model":   model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}

	body, _ := json.Marshal(resp)

	return body
}

// buildTestRequest creates a test HTTP request for chat completion.
func buildTestRequest(model, content string, stream bool) *httpclient.Request {
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
		"stream": stream,
	}

	body, _ := json.Marshal(reqBody)

	return &httpclient.Request{
		Method: "POST",
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: body,
	}
}

// TestChatCompletionProcessor_Process_NonStreaming tests the complete non-streaming flow.
func TestChatCompletionProcessor_Process_NonStreaming(t *testing.T) {
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

	channelSelector := &staticChannelSelector{channels: []*biz.Channel{bizChannel}}

	// Create processor
	processor := &ChatCompletionProcessor{
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
	result, err := processor.Process(ctx, httpRequest)

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

// TestChatCompletionProcessor_Process_Streaming tests the complete streaming flow.
func TestChatCompletionProcessor_Process_Streaming(t *testing.T) {
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

	channelSelector := &staticChannelSelector{channels: []*biz.Channel{bizChannel}}

	// Create processor
	processor := &ChatCompletionProcessor{
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
	result, err := processor.Process(ctx, httpRequest)

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

// TestChatCompletionProcessor_Process_WithModelMapping tests model mapping from API key.
func TestChatCompletionProcessor_Process_WithModelMapping(t *testing.T) {
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

	channelSelector := &staticChannelSelector{channels: []*biz.Channel{bizChannel}}

	// Create processor
	processor := &ChatCompletionProcessor{
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
	result, err := processor.Process(ctx, httpRequest)

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

// TestChatCompletionProcessor_Process_ErrorHandling tests error handling.
func TestChatCompletionProcessor_Process_ErrorHandling(t *testing.T) {
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

	channelSelector := &staticChannelSelector{channels: []*biz.Channel{bizChannel}}

	// Create processor
	processor := &ChatCompletionProcessor{
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
	httpRequest := buildTestRequest("gpt-4", "This will fail", false)

	// Set project ID in context
	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute
	result, err := processor.Process(ctx, httpRequest)

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

// TestChatCompletionProcessor_Process_WithOverrideParameters tests channel override parameters.
func TestChatCompletionProcessor_Process_WithOverrideParameters(t *testing.T) {
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

	channelSelector := &staticChannelSelector{channels: []*biz.Channel{bizChannel}}

	// Create processor
	processor := &ChatCompletionProcessor{
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
	result, err := processor.Process(ctx, httpRequest)

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

// TestChatCompletionProcessor_Process_ConnectionTracking tests connection tracking.
func TestChatCompletionProcessor_Process_ConnectionTracking(t *testing.T) {
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

	channelSelector := &staticChannelSelector{channels: []*biz.Channel{bizChannel}}

	// Create connection tracker
	connectionTracker := NewDefaultConnectionTracker(1024)

	// Create processor
	processor := &ChatCompletionProcessor{
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
	result, err := processor.Process(ctx, httpRequest)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result.ChatCompletion)

	// After completion, connection count should be back to 0
	assert.Equal(t, 0, connectionTracker.GetActiveConnections(ch.ID))
}

// staticChannelSelector is a simple channel selector for testing.
type staticChannelSelector struct {
	channels []*biz.Channel
}

func (s *staticChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	return s.channels, nil
}

// TestChatCompletionProcessor_Process_NoChannelsAvailable tests error when no channels are available.
func TestChatCompletionProcessor_Process_NoChannelsAvailable(t *testing.T) {
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
	channelSelector := &staticChannelSelector{channels: []*biz.Channel{}}

	// Create processor
	processor := &ChatCompletionProcessor{
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
	httpRequest := buildTestRequest("gpt-4", "No channels", false)
	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute
	_, err := processor.Process(ctx, httpRequest)

	// Assert - should return error about no channels
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid channels")
}

// TestChatCompletionProcessor_Process_InvalidRequest tests invalid request handling.
func TestChatCompletionProcessor_Process_InvalidRequest(t *testing.T) {
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

	channelSelector := &staticChannelSelector{channels: []*biz.Channel{bizChannel}}

	processor := &ChatCompletionProcessor{
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
	_, err = processor.Process(ctx, invalidReq)

	// Assert - should return error about missing model
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}

// TestChatCompletionProcessor_Process_MultipleRequests tests multiple sequential requests.
func TestChatCompletionProcessor_Process_MultipleRequests(t *testing.T) {
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

	channelSelector := &staticChannelSelector{channels: []*biz.Channel{bizChannel}}

	processor := &ChatCompletionProcessor{
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
		result, err := processor.Process(ctx, httpRequest)

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
