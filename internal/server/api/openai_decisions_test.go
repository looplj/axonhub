package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai/decisions"
)

func TestOpenAIHandlers_CreateDecisions_rejectsMalformedClientRequest(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := ent.NewContext(authz.WithTestBypass(context.Background()), client)
	channelService, requestService, systemService, usageLogService := setupSpeechAPITestServices(t, client)
	limiter := orchestrator.NewChannelLimiterManager()

	handlers := NewOpenAIHandlers(OpenAIHandlersParams{
		ChannelService:        channelService,
		RequestService:        requestService,
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		HttpClient:            httpclient.NewHttpClient(),
		LiveStreamRegistry:    biz.NewLiveStreamRegistry(),
		ChannelLimiterManager: limiter,
		SSEKeepAliveConfig:    SSEKeepAliveConfig{},
	})

	router := gin.New()
	router.POST("/v1/decisions", handlers.CreateDecisions)

	request := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(`{"model":`))
	request = request.WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "invalid_request_error")
}

type decisionsAPITestSelector struct {
	candidates []*orchestrator.ChannelModelsCandidate
}

func (s *decisionsAPITestSelector) Select(context.Context, *llm.Request) ([]*orchestrator.ChannelModelsCandidate, error) {
	return s.candidates, nil
}

type decisionsAPITestExecutor struct {
	request *httpclient.Request
}

func (e *decisionsAPITestExecutor) Do(_ context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	e.request = request
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"model":"gpt-6-luna","answers":[{"value":"yes"}],"usage":{"input_tokens":1}}`),
		Request:    request,
	}, nil
}

func (e *decisionsAPITestExecutor) DoStream(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, nil
}

func TestOpenAIHandlers_CreateDecisions_authenticatedRoutePersistsMetadata(t *testing.T) {
	setupCtx := authz.WithTestBypass(context.Background())
	client := enttest.NewEntClient(t, "sqlite3", "file:decisions-route?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	setupCtx = ent.NewContext(setupCtx, client)

	owner, err := client.User.Create().
		SetEmail("decisions-route@example.com").
		SetPassword("test-password").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)
	proj, err := client.Project.Create().
		SetName("decisions-route-project").
		SetDescription("route test").
		SetStatus(project.StatusActive).
		Save(setupCtx)
	require.NoError(t, err)
	apiKeyValue, err := biz.GenerateAPIKey("ah")
	require.NoError(t, err)
	_, err = client.APIKey.Create().
		SetName("decisions-route-key").
		SetKey(apiKeyValue).
		SetUserID(owner.ID).
		SetProjectID(proj.ID).
		SetType(apikey.TypeUser).
		SetStatus(apikey.StatusEnabled).
		Save(setupCtx)
	require.NoError(t, err)

	channelRow, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("decisions-route-channel").
		SetBaseURL("https://provider.invalid/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "provider-key"}).
		SetSupportedModels([]string{"gpt-6-luna"}).
		SetDefaultTestModel("gpt-6-luna").
		SetStatus(channel.StatusEnabled).
		Save(setupCtx)
	require.NoError(t, err)
	outbound, err := decisions.NewOutboundTransformer(channelRow.BaseURL, channelRow.Credentials.APIKey)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupSpeechAPITestServices(t, client)
	promptService := biz.NewPromptService(biz.PromptServiceParams{Ent: client})
	promptProtectionService := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	t.Cleanup(promptProtectionService.Stop)
	limiter := orchestrator.NewChannelLimiterManager()
	executor := &decisionsAPITestExecutor{}
	selector := &decisionsAPITestSelector{candidates: []*orchestrator.ChannelModelsCandidate{{
		Channel: &biz.Channel{Channel: channelRow, Outbound: outbound},
		Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-6-luna", ActualModel: "gpt-6-luna"}},
	}}}
	orchestratorInstance := orchestrator.NewChatCompletionOrchestrator(
		channelService,
		nil,
		requestService,
		httpclient.NewHttpClient(),
		decisions.NewInboundTransformer(),
		systemService,
		usageLogService,
		promptService,
		nil,
		promptProtectionService,
		biz.NewLiveStreamRegistry(),
		limiter,
		nil,
	).WithChannelSelector(selector)
	orchestratorInstance.PipelineFactory = pipeline.NewFactory(executor)
	handlers := &OpenAIHandlers{DecisionsHandlers: NewChatCompletionHandlers(orchestratorInstance)}

	projectService := biz.NewProjectService(biz.ProjectServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client})
	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig:    xcache.Config{Mode: xcache.ModeMemory},
		Ent:            client,
		ProjectService: projectService,
		KeyPrefix:      "ah",
	})
	t.Cleanup(apiKeyService.Stop)
	authService := biz.NewAuthService(biz.AuthServiceParams{APIKeyService: apiKeyService, Ent: client})
	router := gin.New()
	router.Use(middleware.WithEntClient(client), middleware.WithAPIKeyConfig(authService, nil))
	router.POST("/v1/decisions", handlers.CreateDecisions)

	body := `{"model":"gpt-6-luna","input":"text","questions":[{"id":"q1","choices":[{"value":"yes"}]}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+apiKeyValue)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.NotNil(t, executor.request)
	require.Equal(t, string(llm.RequestTypeDecisions), executor.request.RequestType)
	require.Equal(t, string(llm.APIFormatOpenAIDecisions), executor.request.APIFormat)
	requests, err := client.Request.Query().All(authz.WithTestBypass(ent.NewContext(context.Background(), client)))
	require.NoError(t, err)
	require.Len(t, requests, 1)
	require.Equal(t, string(llm.APIFormatOpenAIDecisions), requests[0].Format)
	require.Equal(t, "gpt-6-luna", requests[0].ModelID)
}

func TestOpenAIHandlers_CreateDecisions_unauthenticatedRouteIsRejected(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:decisions-auth?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig:    xcache.Config{Mode: xcache.ModeMemory},
		Ent:            client,
		ProjectService: biz.NewProjectService(biz.ProjectServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client}),
		KeyPrefix:      "ah",
	})
	t.Cleanup(apiKeyService.Stop)
	authService := biz.NewAuthService(biz.AuthServiceParams{APIKeyService: apiKeyService, Ent: client})
	reached := false
	router := gin.New()
	router.Use(middleware.WithEntClient(client), middleware.WithAPIKeyConfig(authService, nil))
	router.POST("/v1/decisions", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(`{}`))

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.False(t, reached)
}
