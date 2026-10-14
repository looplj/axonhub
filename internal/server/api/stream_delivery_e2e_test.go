package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

type streamDeliveryE2ESelector struct {
	candidates []*orchestrator.ChannelModelsCandidate
}

func (s *streamDeliveryE2ESelector) Select(context.Context, *llm.Request) ([]*orchestrator.ChannelModelsCandidate, error) {
	return s.candidates, nil
}

func TestStreamDeliveryE2E(t *testing.T) {
	tests := []struct {
		name        string
		format      llm.APIFormat
		path        string
		upstream    string
		heartbeat   bool
		passthrough bool
		wantStatus  request.Status
		wantUsage   bool
	}{
		{name: "direct_chat_ordinary", format: llm.APIFormatOpenAIChatCompletion, path: "/v1/chat/completions", upstream: streamDeliveryChat(12, 34, true), wantStatus: request.StatusCompleted, wantUsage: true},
		{name: "direct_chat_heartbeat_raw_passthrough", format: llm.APIFormatOpenAIChatCompletion, path: "/v1/chat/completions", upstream: streamDeliveryChat(12, 34, true), heartbeat: true, wantStatus: request.StatusCompleted, wantUsage: true},
		{name: "passthrough_chat", format: llm.APIFormatOpenAIChatCompletion, path: "/v1/chat/completions", upstream: streamDeliveryChat(12, 34, true), passthrough: true, wantStatus: request.StatusCompleted, wantUsage: true},
		{name: "chat_to_responses", format: llm.APIFormatOpenAIResponse, path: "/v1/chat/completions", upstream: streamDeliveryResponses(12, 34, "completed"), wantStatus: request.StatusCompleted, wantUsage: true},
		{name: "chat_to_anthropic_message_stop_without_done", format: llm.APIFormatAnthropicMessage, path: "/v1/chat/completions", upstream: streamDeliveryAnthropic(12, 34), wantStatus: request.StatusCompleted, wantUsage: true},
		{name: "2616_zero_placeholders_finish_zero_choices_less_final", format: llm.APIFormatOpenAIChatCompletion, path: "/v1/chat/completions", upstream: streamDeliveryChat2616(), wantStatus: request.StatusCompleted, wantUsage: true},
		{name: "final_zero", format: llm.APIFormatOpenAIChatCompletion, path: "/v1/chat/completions", upstream: streamDeliveryChat(0, 0, true), wantStatus: request.StatusCompleted, wantUsage: true},
		{name: "missing_stats", format: llm.APIFormatOpenAIResponse, path: "/v1/chat/completions", upstream: streamDeliveryResponsesMissing(), wantStatus: request.StatusCompleted, wantUsage: false},
		{name: "provider_failure", format: llm.APIFormatOpenAIResponse, path: "/v1/chat/completions", upstream: streamDeliveryResponses(12, 34, "failed"), wantStatus: request.StatusFailed, wantUsage: true},
		{name: "provider_incomplete", format: llm.APIFormatOpenAIResponse, path: "/v1/chat/completions", upstream: streamDeliveryResponses(12, 34, "incomplete"), wantStatus: request.StatusFailed, wantUsage: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newStreamDeliveryE2EFixture(t, tc.format, tc.upstream, tc.heartbeat)
			fixture.passthrough = tc.passthrough
			fixture.runHTTP(t, tc.path)
			fixture.assertDatabase(t, tc.wantStatus, tc.wantUsage)
		})
	}

	t.Run("pre_terminal_cancellation", func(t *testing.T) {
		started := make(chan struct{})
		canceled := make(chan struct{})
		server := newResponsesWebSocketTestServer(t, func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			return orchestrator.ChatCompletionResult{}, ctx.Err()
		}, nil)
		conn := dialResponsesWebSocket(t, server.URL, nil)
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","input":"hello"}`)))
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("synthetic turn did not start")
		}
		require.NoError(t, conn.Close())
		select {
		case <-canceled:
		case <-time.After(5 * time.Second):
			t.Fatal("pre-terminal cancellation did not reach synthetic upstream")
		}
	})

	t.Run("responses_websocket", func(t *testing.T) {
		stream := newResponsesWebSocketDeliveryStream("response.completed")
		server := newResponsesWebSocketTestServer(t, func(context.Context, *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
			return orchestrator.ChatCompletionResult{ChatCompletionStream: stream}, nil
		}, nil)
		conn := dialResponsesWebSocket(t, server.URL, nil)
		defer conn.Close()
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","input":"hello"}`)))
		_, event, err := conn.ReadMessage()
		require.NoError(t, err)
		require.JSONEq(t, `{"type":"response.completed","response":{"status":"completed"}}`, string(event))
		select {
		case <-stream.acknowledgedC:
		case <-time.After(5 * time.Second):
			t.Fatal("terminal WebSocket write was not acknowledged")
		}
		require.True(t, stream.DeliveryAcknowledged())
	})
}

type streamDeliveryE2EFixture struct {
	ctx             context.Context
	client          *ent.Client
	channel         *ent.Channel
	server          *httptest.Server
	format          llm.APIFormat
	heartbeat       bool
	passthrough     bool
	upstreamStarted chan struct{}
	upstreamRelease chan struct{}
}

func newStreamDeliveryE2EFixture(t *testing.T, format llm.APIFormat, payload string, heartbeat bool) *streamDeliveryE2EFixture {
	t.Helper()
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	client := enttest.NewEntClient(t, "sqlite3", "file:stream-delivery-e2e-"+dbName+"?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	project, err := client.Project.Create().SetName("stream-delivery-e2e").Save(ctx)
	require.NoError(t, err)
	_, err = client.DataStorage.Create().SetName("stream-delivery-e2e-storage").SetDescription("stream delivery test storage").SetPrimary(true).SetType(datastorage.TypeDatabase).SetSettings(&objects.DataStorageSettings{}).SetStatus(datastorage.StatusActive).Save(ctx)
	require.NoError(t, err)
	upstreamStarted := make(chan struct{})
	upstreamRelease := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if payload == streamDeliveryBlockingResponseMarker {
			close(upstreamStarted)
			_, err := io.WriteString(w, streamDeliveryBlockingResponsePrefix())
			require.NoError(t, err)
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-upstreamRelease:
			}
			return
		}
		_, err := io.WriteString(w, payload)
		require.NoError(t, err)
		w.(http.Flusher).Flush()
	}))
	t.Cleanup(func() {
		close(upstreamRelease)
		upstream.CloseClientConnections()
		upstream.Close()
	})
	channelType := channel.TypeOpenai
	if format == llm.APIFormatOpenAIResponse {
		channelType = channel.TypeOpenaiResponses
	}
	if format == llm.APIFormatAnthropicMessage {
		channelType = channel.TypeAnthropic
	}
	channelRow, err := client.Channel.Create().SetType(channelType).SetName("stream-delivery-e2e-channel").SetBaseURL(upstream.URL).SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).SetSupportedModels([]string{"gpt-test"}).SetDefaultTestModel("gpt-test").SetStatus(channel.StatusEnabled).SetEndpoints([]objects.ChannelEndpoint{{APIFormat: format.String(), BaseURL: upstream.URL, Transport: objects.ChannelEndpointTransportHTTP}}).Save(ctx)
	require.NoError(t, err)
	return &streamDeliveryE2EFixture{ctx: contexts.WithProjectID(ctx, project.ID), client: client, channel: channelRow, format: format, heartbeat: heartbeat, upstreamStarted: upstreamStarted, upstreamRelease: upstreamRelease}
}

func (f *streamDeliveryE2EFixture) handler(t *testing.T) *ChatCompletionHandlers {
	channelService, requestService, systemService, usageService := setupSpeechAPITestServices(t, f.client)
	promptService := biz.NewPromptService(biz.PromptServiceParams{Ent: f.client})
	storage, err := f.client.DataStorage.Query().Only(f.ctx)
	require.NoError(t, err)
	require.NoError(t, systemService.SetDefaultDataStorageID(f.ctx, storage.ID))
	promptProtection := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: f.client})
	t.Cleanup(promptProtection.Stop)
	if f.passthrough {
		require.NoError(t, systemService.SetPassThrough(f.ctx, true))
	}
	price := decimal.NewFromFloat(0.01)
	_, err = f.client.ChannelModelPrice.Create().SetChannelID(f.channel.ID).SetModelID("gpt-test").SetPrice(objects.ModelPrice{Items: []objects.ModelPriceItem{{ItemCode: objects.PriceItemCodeUsage, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &price}}, {ItemCode: objects.PriceItemCodeCompletion, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &price}}}}).SetReferenceID("stream-delivery-e2e-price").Save(f.ctx)
	require.NoError(t, err)
	built, err := channelService.GetChannel(f.ctx, f.channel.ID)
	require.NoError(t, err)
	channelService.PreloadModelPricesForTest(f.ctx, built)
	channelService.SetEnabledChannelsForTest([]*biz.Channel{built})
	selector := &streamDeliveryE2ESelector{candidates: []*orchestrator.ChannelModelsCandidate{{Channel: built, Models: []biz.ChannelModelEntry{{RequestModel: "gpt-test", ActualModel: "gpt-test", Source: "direct"}}}}}
	inbound := openai.NewInboundTransformer()
	orch := orchestrator.NewChatCompletionOrchestrator(channelService, nil, requestService, httpclient.NewHttpClient(), inbound, systemService, usageService, promptService, nil, promptProtection, biz.NewLiveStreamRegistry(), orchestrator.NewChannelLimiterManager(), nil).WithChannelSelector(selector)
	h := NewChatCompletionHandlers(orch)
	if f.format == llm.APIFormatOpenAIResponse {
		h.streamAdapterFactory = newResponsesStreamAdapter
	}
	h.sseKeepAlive = SSEKeepAliveConfig{Enabled: f.heartbeat, Interval: time.Millisecond}
	h.sseHeartbeatFormat = sseHeartbeatOpenAI
	return h
}

func (f *streamDeliveryE2EFixture) runHTTP(t *testing.T, path string) { f.runHTTPUntil(t, path, nil) }

func (f *streamDeliveryE2EFixture) runHTTPUntil(t *testing.T, path string, first <-chan struct{}) {
	t.Helper()
	h := f.handler(t)
	engine := gin.New()
	engine.Use(f.testContext)
	handlerDone := make(chan struct{})
	engine.POST(path, func(c *gin.Context) {
		defer close(handlerDone)
		h.ChatCompletion(c)
	})
	f.server = httptest.NewServer(engine)
	t.Cleanup(f.server.Close)
	requestCtx := f.ctx
	var cancel context.CancelFunc
	if first != nil {
		requestCtx, cancel = context.WithCancel(f.ctx)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, f.server.URL+path, strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"stream":true}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	if first != nil {
		<-first
		cancel()
		require.NoError(t, resp.Body.Close())
		select {
		case <-handlerDone:
		case <-time.After(5 * time.Second):
			t.Fatal("handler did not return after cancellation")
		}
		return
	}
	assertStreamDeliveryWire(t, resp, f.heartbeat)
	select {
	case <-handlerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not return after stream completion")
	}
}

func (f *streamDeliveryE2EFixture) testContext(c *gin.Context) {
	ctx := authz.WithTestBypass(c.Request.Context())
	ctx = ent.NewContext(ctx, f.client)
	projectID, ok := contexts.GetProjectID(f.ctx)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	ctx = contexts.WithProjectID(ctx, projectID)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func (f *streamDeliveryE2EFixture) assertDatabase(t *testing.T, want request.Status, wantUsage bool) {
	requests, err := f.client.Request.Query().All(f.ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	require.Equal(t, want, requests[0].Status)
	executions, err := f.client.RequestExecution.Query().All(f.ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)
	if want == request.StatusCompleted {
		require.Equal(t, requestexecution.StatusCompleted, executions[0].Status)
	} else {
		require.NotEqual(t, requestexecution.StatusCompleted, executions[0].Status)
	}
	usage, err := f.client.UsageLog.Query().All(f.ctx)
	require.NoError(t, err)
	if !wantUsage {
		require.Empty(t, usage)
		return
	}
	require.Len(t, usage, 1)
	require.Equal(t, requests[0].ID, usage[0].RequestID)
	require.Equal(t, executions[0].ChannelID, usage[0].ChannelID)
	require.Equal(t, usagelog.Source(requests[0].Source), usage[0].Source)
	require.NotNil(t, usage[0].TotalCost)
}

func streamDeliveryChat2616() string {
	return "data: {\"id\":\"chat\",\"choices\":[{\"delta\":{},\"finish_reason\":null}],\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":0,\"total_tokens\":0}}\n\ndata: {\"id\":\"chat\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":0,\"total_tokens\":0}}\n\ndata: {\"id\":\"chat\",\"choices\":[],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":34,\"total_tokens\":46}}\n\ndata: [DONE]\n\n"
}

func streamDeliveryResponses(input, output int, status string) string {
	return fmt.Sprintf("event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp\",\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.%s\ndata: {\"type\":\"response.%s\",\"response\":{\"id\":\"resp\",\"status\":\"%s\",\"output\":[],\"usage\":{\"input_tokens\":%d,\"output_tokens\":%d,\"total_tokens\":%d}}}\n\n", status, status, status, input, output, input+output)
}
