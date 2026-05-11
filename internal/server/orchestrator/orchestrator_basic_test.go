package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/streams"
	anthropictransformer "github.com/looplj/axonhub/llm/transformer/anthropic"
	geminitransformer "github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestChatCompletionOrchestrator_Process_NonStreaming_PreservesGeminiGroundingAnnotations(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	project := createTestProject(t, ctx, client)
	channelRow, err := client.Channel.Create().
		SetType(channel.TypeGemini).
		SetName("Test Gemini Channel").
		SetBaseURL("https://generativelanguage.googleapis.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-api-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	geminiResp := geminitransformer.GenerateContentResponse{
		ResponseID:   "resp_gemini_orchestrator_grounding",
		ModelVersion: "gemini-2.5-flash",
		Candidates: []*geminitransformer.Candidate{{
			Index: 0,
			Content: &geminitransformer.Content{
				Role:  "model",
				Parts: []*geminitransformer.Part{{Text: "Grounded answer"}},
			},
			FinishReason: "STOP",
			GroundingMetadata: &geminitransformer.GroundingMetadata{
				WebSearchQueries: []string{"grounded query"},
				GroundingChunks: []*geminitransformer.GroundingChunk{{
					Web: &geminitransformer.GroundingChunkWeb{
						URI:   "https://example.com/gemini",
						Title: "Gemini Source",
					},
				}},
				GroundingSupports: []*geminitransformer.GroundingSupport{{
					Segment: &geminitransformer.Segment{
						StartIndex: 0,
						EndIndex:   8,
						Text:       "Grounded",
					},
					GroundingChunkIndices: []int32{0},
				}},
			},
		}},
		UsageMetadata: &geminitransformer.UsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
			TotalTokenCount:      15,
		},
	}
	geminiRespBody, err := json.Marshal(geminiResp)
	require.NoError(t, err)

	executor := &mockExecutor{
		response: &httpclient.Response{
			StatusCode: 200,
			Body:       geminiRespBody,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	outbound, err := geminitransformer.NewOutboundTransformer(channelRow.BaseURL, channelRow.Credentials.APIKey)
	require.NoError(t, err)

	bizChannel := &biz.Channel{Channel: channelRow, Outbound: outbound}
	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "gpt-4")}

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
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	httpRequest := buildTestRequest("gpt-4", "Summarize with citations", false)
	ctx = contexts.WithProjectID(ctx, project.ID)

	result, err := orchestrator.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Nil(t, result.ChatCompletionStream)

	var clientResp openai.Response
	err = json.Unmarshal(result.ChatCompletion.Body, &clientResp)
	require.NoError(t, err)
	require.Len(t, clientResp.Choices, 1)
	require.NotNil(t, clientResp.Choices[0].Message)
	require.Equal(t, "assistant", clientResp.Choices[0].Message.Role)
	require.Equal(t, "Grounded answer", lo.FromPtr(clientResp.Choices[0].Message.Content.Content))
	require.Len(t, clientResp.Choices[0].Message.Annotations, 1)
	require.Equal(t, "url_citation", clientResp.Choices[0].Message.Annotations[0].Type)
	require.NotNil(t, clientResp.Choices[0].Message.Annotations[0].URLCitation)
	require.Equal(t, "https://example.com/gemini", clientResp.Choices[0].Message.Annotations[0].URLCitation.URL)
	require.Equal(t, "Gemini Source", clientResp.Choices[0].Message.Annotations[0].URLCitation.Title)
	require.NotNil(t, clientResp.Choices[0].Message.Annotations[0].StartIndex)
	require.EqualValues(t, 0, *clientResp.Choices[0].Message.Annotations[0].StartIndex)
	require.NotNil(t, clientResp.Choices[0].Message.Annotations[0].EndIndex)
	require.EqualValues(t, 8, *clientResp.Choices[0].Message.Annotations[0].EndIndex)

	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	var persistedResp openai.Response
	err = json.Unmarshal(requests[0].ResponseBody, &persistedResp)
	require.NoError(t, err)
	require.Len(t, persistedResp.Choices, 1)
	require.NotNil(t, persistedResp.Choices[0].Message)
	require.Len(t, persistedResp.Choices[0].Message.Annotations, 1)
	require.Equal(t, "https://example.com/gemini", persistedResp.Choices[0].Message.Annotations[0].URLCitation.URL)
	require.Equal(t, "Gemini Source", persistedResp.Choices[0].Message.Annotations[0].URLCitation.Title)
	require.NotNil(t, persistedResp.Choices[0].Message.Annotations[0].StartIndex)
	require.EqualValues(t, 0, *persistedResp.Choices[0].Message.Annotations[0].StartIndex)
	require.NotNil(t, persistedResp.Choices[0].Message.Annotations[0].EndIndex)
	require.EqualValues(t, 8, *persistedResp.Choices[0].Message.Annotations[0].EndIndex)
}

func TestChatCompletionOrchestrator_Process_NonStreaming_PreservesAnthropicCitations(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	project := createTestProject(t, ctx, client)
	channelRow, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Test Anthropic Channel").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-api-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	anthropicResp := anthropictransformer.Message{
		ID:   "msg_citation_orchestrator",
		Type: "message",
		Role: "assistant",
		Content: []anthropictransformer.MessageContentBlock{{
			Type: "text",
			Text: lo.ToPtr("Answer with source"),
			Citations: []anthropictransformer.TextCitation{{
				Type:           "url_citation",
				URL:            "https://example.com/anthropic",
				Title:          "Anthropic Source",
				EncryptedIndex: lo.ToPtr("secret-index"),
				CitedText:      lo.ToPtr("quoted text"),
			}},
		}},
		Model:      "claude-3-7-sonnet-latest",
		StopReason: lo.ToPtr("end_turn"),
		Usage: &anthropictransformer.Usage{
			InputTokens:  15,
			OutputTokens: 25,
		},
	}
	anthropicRespBody, err := json.Marshal(anthropicResp)
	require.NoError(t, err)

	executor := &mockExecutor{
		response: &httpclient.Response{
			StatusCode: 200,
			Body:       anthropicRespBody,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	outbound, err := anthropictransformer.NewOutboundTransformer(channelRow.BaseURL, channelRow.Credentials.APIKey)
	require.NoError(t, err)

	bizChannel := &biz.Channel{Channel: channelRow, Outbound: outbound}
	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "gpt-4")}

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
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	httpRequest := buildTestRequest("gpt-4", "Summarize with citations", false)
	ctx = contexts.WithProjectID(ctx, project.ID)

	result, err := orchestrator.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Nil(t, result.ChatCompletionStream)

	var clientResp openai.Response
	err = json.Unmarshal(result.ChatCompletion.Body, &clientResp)
	require.NoError(t, err)
	require.Len(t, clientResp.Choices, 1)
	require.NotNil(t, clientResp.Choices[0].Message)
	require.Equal(t, "assistant", clientResp.Choices[0].Message.Role)
	require.Equal(t, "Answer with source", lo.FromPtr(clientResp.Choices[0].Message.Content.Content))
	require.Len(t, clientResp.Choices[0].Message.Annotations, 1)
	require.Equal(t, "url_citation", clientResp.Choices[0].Message.Annotations[0].Type)
	require.NotNil(t, clientResp.Choices[0].Message.Annotations[0].URLCitation)
	require.Equal(t, "https://example.com/anthropic", clientResp.Choices[0].Message.Annotations[0].URLCitation.URL)
	require.Equal(t, "Anthropic Source", clientResp.Choices[0].Message.Annotations[0].URLCitation.Title)

	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	var persistedResp openai.Response
	err = json.Unmarshal(requests[0].ResponseBody, &persistedResp)
	require.NoError(t, err)
	require.Len(t, persistedResp.Choices, 1)
	require.NotNil(t, persistedResp.Choices[0].Message)
	require.Len(t, persistedResp.Choices[0].Message.Annotations, 1)
	require.Equal(t, "https://example.com/anthropic", persistedResp.Choices[0].Message.Annotations[0].URLCitation.URL)
	require.Equal(t, "Anthropic Source", persistedResp.Choices[0].Message.Annotations[0].URLCitation.Title)
}

// TestChatCompletionOrchestrator_Process_NonStreaming tests the complete non-streaming flow.
func TestChatCompletionOrchestrator_Process_NonStreaming(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

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
		channelSelector:       channelSelector,
		Inbound:               openai.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
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
	ctx = authz.WithTestBypass(ctx)

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
		channelSelector:       channelSelector,
		Inbound:               openai.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		modelCircuitBreaker:   biz.NewModelCircuitBreaker(),
		channelLimiterManager: NewChannelLimiterManager(),
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
	ctx = authz.WithTestBypass(ctx)

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
		SetCredentials(objects.ChannelCredentials{APIKey: "test-api-key"}).
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
		channelSelector:       channelSelector,
		Inbound:               openai.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
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
	ctx = authz.WithTestBypass(ctx)

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
		channelSelector:       channelSelector,
		Inbound:               openai.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	ctx = contexts.WithProjectID(ctx, project.ID)

	// Execute multiple requests
	for i := range 3 {
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

type executorStep struct {
	resp *httpclient.Response
	err  error
}

type sequenceExecutor struct {
	steps    []executorStep
	stepIdx  int
	requests []*httpclient.Request
}

func (e *sequenceExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	e.requests = append(e.requests, request)

	if e.stepIdx >= len(e.steps) {
		return nil, errors.New("no more steps available")
	}

	step := e.steps[e.stepIdx]
	e.stepIdx++

	if step.err != nil {
		return nil, step.err
	}

	return step.resp, nil
}

func (e *sequenceExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, errors.New("streaming not supported by this executor")
}

func TestChatCompletionOrchestrator_Process_SameChannelRetryNextModel(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	err := systemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled:                 true,
		MaxChannelRetries:       1,
		MaxSingleChannelRetries: 1,
		RetryDelayMs:            0,
		LoadBalancerStrategy:    "adaptive",
	})
	require.NoError(t, err)

	mockResp := buildMockOpenAIResponse("chatcmpl-retry-1", "gpt-3.5-turbo", "Recovered", 10, 20)
	executor := &sequenceExecutor{
		steps: []executorStep{
			{
				err: &httpclient.Error{
					StatusCode: 500,
					Body:       []byte(`{"error":{"message":"upstream error","type":"api_error"}}`),
				},
			},
			{
				resp: &httpclient.Response{
					StatusCode: 200,
					Body:       mockResp,
					Headers:    http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	outbound, err := openai.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
	require.NoError(t, err)

	bizChannel := &biz.Channel{
		Channel:  ch,
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{
		candidates: []*ChannelModelsCandidate{
			{
				Channel:  bizChannel,
				Priority: 0,
				Models: []biz.ChannelModelEntry{
					{RequestModel: "gpt-4", ActualModel: "gpt-4"},
					{RequestModel: "gpt-4", ActualModel: "gpt-3.5-turbo"},
				},
			},
		},
	}

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
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	httpRequest := buildTestRequest("gpt-4", "Hello!", false)
	ctx = contexts.WithProjectID(ctx, project.ID)

	result, err := orchestrator.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Len(t, executor.requests, 2)

	var firstBody map[string]any

	err = json.Unmarshal(executor.requests[0].Body, &firstBody)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4", firstBody["model"])

	var secondBody map[string]any

	err = json.Unmarshal(executor.requests[1].Body, &secondBody)
	require.NoError(t, err)
	assert.Equal(t, "gpt-3.5-turbo", secondBody["model"])

	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, executions, 2)
}
