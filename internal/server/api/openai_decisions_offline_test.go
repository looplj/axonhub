package api

import (
	"context"
	"encoding/json"
	"io"
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
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/promptprotectionrule"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/internal/tracing"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/decisions"
)

func TestOpenAIHandlers_CreateDecisions_usesOfflineUpstreamAndPreservesClientShape(t *testing.T) {
	const clientBody = `{"model":"gpt-6-luna","input":"synthetic prompt","questions":[{"type":"choice","name":"route","choices":[{"value":true}]}],"x_beta":{"keep":true}}`
	const upstreamBody = `{"model":"provider-model","answers":[{"type":"choice","name":"route","choice":true}],"usage":{"input_tokens":120,"output_tokens":999},"x_beta":{"preserve":true}}`
	const clientKey = "ah-test-decisions-route-key"
	const providerKey = "provider-test-decisions-key"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/decisions", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "Bearer "+providerKey, r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, clientBody, string(body))
		w.Header().Set("Content-Type", "application/json")
		_, err = io.WriteString(w, upstreamBody)
		require.NoError(t, err)
	}))
	defer upstream.Close()

	setupCtx := ent.NewContext(authz.WithTestBypass(context.Background()), enttest.NewEntClient(t, "sqlite3", "file:decisions-offline-route?mode=memory&_fk=0"))
	client := ent.FromContext(setupCtx)
	require.NotNil(t, client)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	owner, err := client.User.Create().SetEmail("decisions-offline@example.com").SetPassword("test-password").SetStatus(user.StatusActivated).Save(setupCtx)
	require.NoError(t, err)
	proj, err := client.Project.Create().SetName("decisions-offline-project").SetDescription("offline route test").SetStatus(project.StatusActive).Save(setupCtx)
	require.NoError(t, err)
	_, err = client.APIKey.Create().SetName("decisions-offline-key").SetKey(clientKey).SetUserID(owner.ID).SetProjectID(proj.ID).SetType(apikey.TypeUser).SetStatus(apikey.StatusEnabled).Save(setupCtx)
	require.NoError(t, err)
	channelRow, err := client.Channel.Create().SetType(channel.TypeOpenai).SetName("decisions-offline-channel").SetBaseURL(upstream.URL).SetCredentials(objects.ChannelCredentials{APIKey: providerKey}).SetSupportedModels([]string{"gpt-6-luna"}).SetDefaultTestModel("gpt-6-luna").SetStatus(channel.StatusEnabled).Save(setupCtx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupSpeechAPITestServices(t, client)
	projectService := biz.NewProjectService(biz.ProjectServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client})
	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client, ProjectService: projectService, KeyPrefix: "ah"})
	t.Cleanup(apiKeyService.Stop)
	authService := biz.NewAuthService(biz.AuthServiceParams{APIKeyService: apiKeyService, Ent: client})
	promptService := biz.NewPromptService(biz.PromptServiceParams{Ent: client})
	promptProtectionService := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client})
	t.Cleanup(promptProtectionService.Stop)
	outbound, err := decisions.NewOutboundTransformer(channelRow.BaseURL, providerKey)
	require.NoError(t, err)
	orchestratorInstance := orchestrator.NewChatCompletionOrchestrator(channelService, nil, requestService, httpclient.NewHttpClient(), decisions.NewInboundTransformer(), systemService, usageLogService, promptService, nil, promptProtectionService, biz.NewLiveStreamRegistry(), orchestrator.NewChannelLimiterManager(), nil).WithChannelSelector(&decisionsAPITestSelector{candidates: []*orchestrator.ChannelModelsCandidate{{Channel: &biz.Channel{Channel: channelRow, Outbound: outbound}, Models: []biz.ChannelModelEntry{{RequestModel: "gpt-6-luna", ActualModel: "gpt-6-luna"}}}}})
	handlers := &OpenAIHandlers{DecisionsHandlers: NewChatCompletionHandlers(orchestratorInstance)}
	router := gin.New()
	router.Use(middleware.WithEntClient(client), middleware.WithAPIKeyConfig(authService, nil))
	router.POST("/v1/decisions", handlers.CreateDecisions)

	req := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(clientBody))
	req.Header.Set("Authorization", "Bearer "+clientKey)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))
	require.JSONEq(t, upstreamBody, response.Body.String())
}

type decisionsOfflineHarness struct {
	client         *ent.Client
	ctx            context.Context
	apiKey         string
	project        *ent.Project
	requestService *biz.RequestService
	router         *gin.Engine
}

func newDecisionsOfflineHarness(t *testing.T, upstreamBody string, withTrace bool, rejectPrompt bool) (*decisionsOfflineHarness, *int) {
	t.Helper()
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, err := io.WriteString(w, upstreamBody)
		require.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)

	ctx := ent.NewContext(authz.WithTestBypass(context.Background()), enttest.NewEntClient(t, "sqlite3", "file:decisions-offline-route-extension?mode=memory&_fk=0"))
	client := ent.FromContext(ctx)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	owner, err := client.User.Create().SetEmail("decisions-offline-extension@example.com").SetPassword("test-password").SetStatus(user.StatusActivated).Save(ctx)
	require.NoError(t, err)
	projectRow, err := client.Project.Create().SetName("decisions-offline-extension-project").SetDescription("offline route extension test").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)
	apiKey := "ah-test-decisions-route-extension-key"
	_, err = client.APIKey.Create().SetName("decisions-offline-extension-key").SetKey(apiKey).SetUserID(owner.ID).SetProjectID(projectRow.ID).SetType(apikey.TypeUser).SetStatus(apikey.StatusEnabled).Save(ctx)
	require.NoError(t, err)
	channelRow, err := client.Channel.Create().SetType(channel.TypeOpenai).SetName("decisions-offline-extension-channel").SetBaseURL(upstream.URL).SetCredentials(objects.ChannelCredentials{APIKey: "provider-key"}).SetSupportedModels([]string{"gpt-6-luna"}).SetDefaultTestModel("gpt-6-luna").SetStatus(channel.StatusEnabled).Save(ctx)
	require.NoError(t, err)
	_, err = client.DataStorage.Create().SetName("decisions-offline-extension-storage").SetDescription("offline route extension storage").SetPrimary(true).SetType(datastorage.TypeDatabase).SetSettings(&objects.DataStorageSettings{}).SetStatus(datastorage.StatusActive).Save(ctx)
	require.NoError(t, err)
	outbound, err := decisions.NewOutboundTransformer(channelRow.BaseURL, channelRow.Credentials.APIKey)
	require.NoError(t, err)
	if rejectPrompt {
		_, err = client.PromptProtectionRule.Create().
			SetName("reject-secret").
			SetPattern("secret").
			SetStatus(promptprotectionrule.StatusEnabled).
			SetSettings(&objects.PromptProtectionSettings{Action: objects.PromptProtectionActionReject}).
			Save(ctx)
		require.NoError(t, err)
	}

	channelService, requestService, systemService, usageLogService := setupSpeechAPITestServices(t, client)
	promptService := biz.NewPromptService(biz.PromptServiceParams{Ent: client})
	promptProtectionService := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client})
	t.Cleanup(promptProtectionService.Stop)
	orchestratorInstance := orchestrator.NewChatCompletionOrchestrator(channelService, nil, requestService, httpclient.NewHttpClient(), decisions.NewInboundTransformer(), systemService, usageLogService, promptService, nil, promptProtectionService, biz.NewLiveStreamRegistry(), orchestrator.NewChannelLimiterManager(), nil).
		WithChannelSelector(&decisionsAPITestSelector{candidates: []*orchestrator.ChannelModelsCandidate{{Channel: &biz.Channel{Channel: channelRow, Outbound: outbound}, Models: []biz.ChannelModelEntry{{RequestModel: "gpt-6-luna", ActualModel: "gpt-6-luna"}}}}})
	handlers := &OpenAIHandlers{DecisionsHandlers: NewChatCompletionHandlers(orchestratorInstance)}
	projectService := biz.NewProjectService(biz.ProjectServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client})
	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client, ProjectService: projectService, KeyPrefix: "ah"})
	t.Cleanup(apiKeyService.Stop)
	authService := biz.NewAuthService(biz.AuthServiceParams{APIKeyService: apiKeyService, Ent: client})
	router := gin.New()
	router.Use(middleware.WithEntClient(client), middleware.WithAPIKeyConfig(authService, nil))
	if withTrace {
		traceService := biz.NewTraceService(biz.TraceServiceParams{RequestService: requestService, Ent: client})
		router.Use(middleware.WithTrace(tracing.Config{ResponseTraceHeaders: []string{"AH-Trace-Id"}}, traceService))
	}
	router.POST("/v1/decisions", handlers.CreateDecisions)

	return &decisionsOfflineHarness{client: client, ctx: ctx, apiKey: apiKey, project: projectRow, requestService: requestService, router: router}, &upstreamCalls
}

func TestOpenAIHandlers_CreateDecisions_rejectsConfiguredPromptProtectionBeforeUpstream(t *testing.T) {
	harness, upstreamCalls := newDecisionsOfflineHarness(t, `{"answers":[]}`, false, true)

	req := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(`{"model":"gpt-6-luna","input":"contains secret","questions":[{"type":"choice","name":"route","choices":[{"value":"yes"}]}]}`))
	req.Header.Set("Authorization", "Bearer "+harness.apiKey)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	harness.router.ServeHTTP(response, req)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "prompt protection")
	require.Equal(t, 0, *upstreamCalls)
}

func TestOpenAIHandlers_CreateDecisions_persistsFinalRequestAndExecutionUsage(t *testing.T) {
	harness, _ := newDecisionsOfflineHarness(t, `{"model":"provider-model","answers":[],"usage":{"input_tokens":120,"output_tokens":0}}`, false, false)
	req := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(`{"model":"gpt-6-luna","input":"synthetic prompt","questions":[{"type":"choice","name":"route","choices":[{"value":"yes"}]}]}`))
	req.Header.Set("Authorization", "Bearer "+harness.apiKey)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	harness.router.ServeHTTP(response, req)

	require.Equal(t, http.StatusOK, response.Code)
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), harness.client))
	requestRow, err := harness.client.Request.Query().Where(request.ProjectIDEQ(harness.project.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, request.StatusCompleted, requestRow.Status)
	require.JSONEq(t, `{"model":"provider-model","answers":[],"usage":{"input_tokens":120,"output_tokens":0}}`, string(requestRow.ResponseBody))
	execution, err := harness.client.RequestExecution.Query().Where(requestexecution.RequestIDEQ(requestRow.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusCompleted, execution.Status)
	require.JSONEq(t, `{"model":"provider-model","answers":[],"usage":{"input_tokens":120,"output_tokens":0}}`, string(execution.ResponseBody))
}

func TestOpenAIHandlers_CreateDecisions_persistsTraceHeaderAndRedactedImageSpan(t *testing.T) {
	harness, _ := newDecisionsOfflineHarness(t, `{"model":"provider-model","answers":[],"usage":{"input_tokens":12}}`, true, false)
	traceID := "trace-decisions-inline-image"
	body := `{"model":"gpt-6-luna","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"choose"},{"type":"input_image","image_url":"data:image/png;base64,secret"}]}],"questions":[{"type":"choice","name":"route","choices":[{"value":"yes"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+harness.apiKey)
	req.Header.Set("Ah-Trace-Id", traceID)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	harness.router.ServeHTTP(response, req)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, traceID, response.Header().Get("Ah-Trace-Id"))
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), harness.client))
	var traceRows []*ent.Trace
	traceRows, err := harness.client.Trace.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, traceRows, 1)
	requestRow, err := harness.client.Request.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, string(llm.APIFormatOpenAIDecisions), requestRow.Format)
	require.NotEmpty(t, requestRow.RequestBody)
	traceService := biz.NewTraceService(biz.TraceServiceParams{RequestService: harness.requestService, Ent: harness.client})
	root, err := traceService.GetRootSegment(ctx, traceRows[0].ID)
	require.NoError(t, err)
	require.NotNil(t, root)
	require.NotEmpty(t, root.RequestSpans)
	encoded, err := json.Marshal(root.RequestSpans)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "[image]")
	require.NotContains(t, string(encoded), "data:image/png;base64,secret")
}
