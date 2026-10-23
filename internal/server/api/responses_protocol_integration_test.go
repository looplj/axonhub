package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResponsesProtocolIntegrationHTTPUsesProductionHandlerAndUpstream(t *testing.T) {
	upstreamRequests := make(chan []byte, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := readRequestBody(r)
		require.NoError(t, err)
		upstreamRequests <- body
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"id":"provider-response-1","object":"response","status":"completed","output":[{"type":"message","id":"provider-item-1","content":[{"type":"output_text","text":"synthetic-answer"}]}]}`))
		require.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)

	client, handlers, project := newResponsesIntegrationFixture(t, upstream.URL)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", stringsReader(`{"model":"synthetic-model","input":"synthetic-input"}`))
	requestContext := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	requestContext = contexts.WithAPIKey(requestContext, &ent.APIKey{ProjectID: project.ID})
	request = request.WithContext(contexts.WithProjectID(requestContext, project.ID))
	recorder := httptest.NewRecorder()
	router := gin.New()
	router.POST("/v1/responses", handlers.CreateResponse)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "provider-response-1", response["id"])
	require.Equal(t, "provider-item-1", response["output"].([]any)[0].(map[string]any)["id"])
	var upstreamBody map[string]any
	require.NoError(t, json.Unmarshal(<-upstreamRequests, &upstreamBody))
	require.Equal(t, "synthetic-model", upstreamBody["model"])
	require.Equal(t, "synthetic-input", upstreamBody["input"])
}

func TestResponsesProtocolIntegrationIsolation(t *testing.T) {
	upstreamRequests := make(chan []byte, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := readRequestBody(r)
		require.NoError(t, err)
		upstreamRequests <- body
		w.Header().Set("Content-Type", "text/event-stream")
		_, err = io.WriteString(w, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"provider-ws-1\"}}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"provider-ws-1\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"provider-item-ws-1\"}]}}\n\n")
		require.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)
	client, handlers, project := newResponsesIntegrationFixture(t, upstream.URL)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		requestContext := authz.WithTestBypass(ent.NewContext(c.Request.Context(), client))
		requestContext = contexts.WithAPIKey(requestContext, &ent.APIKey{ProjectID: project.ID})
		c.Request = c.Request.WithContext(contexts.WithProjectID(requestContext, project.ID))
		c.Next()
	})
	router.GET("/v1/responses", handlers.CreateResponseWebSocket(5*time.Second))
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/responses"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"tenant-a","model":"synthetic-model","input":"synthetic-input"}`)))
	_, created, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "provider-ws-1", gjson.GetBytes(created, "response.id").String())
	_, completed, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "provider-item-ws-1", gjson.GetBytes(completed, "response.output.0.id").String())
	require.Equal(t, "tenant-a", gjson.GetBytes(completed, "stream_id").String())
	require.NotEmpty(t, <-upstreamRequests)
	require.NoError(t, conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done")))
	_, _, err = conn.ReadMessage()
	require.Error(t, err)
}

func newResponsesIntegrationFixture(t *testing.T, upstreamURL string, configure ...func(*ent.ChannelCreate)) (*ent.Client, *OpenAIHandlers, *ent.Project) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=1")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	project, err := client.Project.Create().SetName("responses-integration-project").SetDescription("synthetic").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)
	create := client.Channel.Create().SetType(channel.TypeOpenaiResponses).SetName("responses-integration-channel").SetStatus(channel.StatusEnabled).SetTags([]string{"synthetic"}).SetBaseURL(upstreamURL).SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).SetSupportedModels([]string{"synthetic-model"}).SetDefaultTestModel("synthetic-model")
	for _, apply := range configure {
		apply(create)
	}
	_, err = create.Save(ctx)
	require.NoError(t, err)

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	systemService := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cacheConfig, Ent: client})
	require.NoError(t, systemService.SetModelSettings(ctx, biz.SystemModelSettings{FallbackToChannelsOnModelNotFound: true, QueryAllChannelModels: true}))
	channelService := biz.NewChannelService(biz.ChannelServiceParams{CacheConfig: cacheConfig, Ent: client, SystemService: systemService, HttpClient: httpclient.NewHttpClient()})
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)
	storage := &biz.DataStorageService{AbstractService: &biz.AbstractService{}, SystemService: systemService, Cache: xcache.NewFromConfig[ent.DataStorage](cacheConfig)}
	requestService := biz.NewRequestService(client, cacheConfig, systemService, usageLogService, storage, biz.NewLiveStreamRegistry())
	modelService := biz.NewModelService(biz.ModelServiceParams{ChannelService: channelService, SystemService: systemService, Ent: client})
	selector := orchestrator.NewDefaultSelector(channelService, modelService, systemService)
	candidates, err := selector.Select(ctx, &llm.Request{Model: "synthetic-model", RequestType: llm.RequestTypeChat})
	require.NoError(t, err)
	require.NotEmpty(t, candidates)
	protection := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{CacheConfig: cacheConfig, Ent: client})
	params := OpenAIHandlersParams{
		ChannelService: channelService, ModelService: modelService, DefaultSelector: selector, RequestService: requestService,
		SystemService: systemService, UsageLogService: usageLogService, PromptService: biz.NewPromptService(biz.PromptServiceParams{Ent: client}),
		PromptProtectionRuleService: protection, QuotaService: biz.NewQuotaService(client, systemService), HttpClient: httpclient.NewHttpClient(),
		LiveStreamRegistry: biz.NewLiveStreamRegistry(), ChannelLimiterManager: orchestrator.NewChannelLimiterManager(), Client: client,
	}
	return client, NewOpenAIHandlers(params), project
}

func readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

func stringsReader(value string) *strings.Reader { return strings.NewReader(value) }
