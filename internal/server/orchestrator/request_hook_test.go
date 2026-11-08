package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestPluginOverloadedStopsRetries(t *testing.T) {
	state := &PersistenceState{
		CurrentCandidate:        &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}}},
		CurrentCandidateIndex:   0,
		ChannelModelsCandidates: []*ChannelModelsCandidate{{}, {}},
	}
	outbound := &PersistentOutboundTransformer{state: state}
	require.True(t, outbound.HasMoreChannels())
	state.PluginOverloaded = true
	require.False(t, outbound.HasMoreChannels())

	err := NewPluginOverloadedError(&llm.ResponseError{
		StatusCode: http.StatusTooManyRequests,
		Detail:     llm.ErrorDetail{Code: "plugin_overloaded", Type: "rate_limit_error", Message: "plugin demo is at capacity; please retry shortly"},
	})
	require.False(t, outbound.CanRetry(err))
	var response *llm.ResponseError
	require.ErrorAs(t, err, &response)
	require.Equal(t, http.StatusTooManyRequests, response.StatusCode)
}

func TestRequestHookModelChangeReachesAPIKeyCheck(t *testing.T) {
	state := &PersistenceState{APIKey: &ent.APIKey{
		Name: "limited",
		Profiles: &objects.APIKeyProfiles{
			ActiveProfile: "default",
			Profiles: []objects.APIKeyProfile{{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			}},
		},
	}}
	inbound := &PersistentInboundTransformer{state: state}
	request := &llm.Request{Model: "gpt-4"}
	rewritten, err := (&modelRewritingHook{}).OnInboundLlmRequest(context.Background(), request)
	require.NoError(t, err)
	_, err = checkApiKeyModelAccess(inbound).OnInboundLlmRequest(context.Background(), rewritten)
	require.ErrorIs(t, err, biz.ErrInvalidModel)

	openAI := openai.NewInboundTransformer().TransformError(context.Background(), err)
	require.Equal(t, http.StatusUnprocessableEntity, openAI.StatusCode)
	anthropicError := anthropic.NewInboundTransformer().TransformError(context.Background(), err)
	require.Equal(t, http.StatusUnprocessableEntity, anthropicError.StatusCode)
}

func TestPassThroughKeepsClientBodyAndNotifiesHooks(t *testing.T) {
	var got []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-1","object":"chat.completion","model":"mapped","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(server.Close)
	provider, err := openai.NewOutboundTransformer(server.URL, "test-key")
	require.NoError(t, err)
	enabled := true
	state := &PersistenceState{
		ChannelModelsCandidates: []*ChannelModelsCandidate{{
			Channel: &biz.Channel{Channel: &ent.Channel{
				ID:       4,
				Name:     "pass",
				Settings: &objects.ChannelSettings{PassThroughBody: &enabled},
			}, Outbound: provider},
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
			Models:    []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "upstream-model"}},
		}},
	}
	hooks := &recordingHooks{}
	registered, err := hooks.Middleware(context.Background())
	require.NoError(t, err)
	hookMiddleware := registered.Request
	inbound, outbound := NewPersistentTransformers(state, openai.NewInboundTransformer(), hookMiddleware)
	pipe := pipeline.NewFactory(httpclient.NewHttpClient()).Pipeline(
		inbound,
		outbound,
		pipeline.WithMiddlewares(
			hookMiddleware,
			applyPassThroughRequestBody(outbound, hooks),
		),
	)
	clientBody := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"temperature":0.7}`)
	_, err = pipe.Process(context.Background(), &httpclient.Request{
		Method:  http.MethodPost,
		Path:    "/v1/chat/completions",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    clientBody,
	})
	require.NoError(t, err)
	require.Equal(t, "upstream-model", gjson.GetBytes(got, "model").String())
	require.Equal(t, 0.7, gjson.GetBytes(got, "temperature").Float())
	require.Equal(t, "hi", gjson.GetBytes(got, "messages.0.content").String())
	require.Equal(t, []int{4}, hooks.channels)
}

func TestChannelOverrideWinsOverRequestHook(t *testing.T) {
	var got []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(server.Close)
	provider, err := openai.NewOutboundTransformer(server.URL, "test-key")
	require.NoError(t, err)
	state := &PersistenceState{
		ChannelModelsCandidates: []*ChannelModelsCandidate{{
			Channel: &biz.Channel{Channel: &ent.Channel{
				ID:   5,
				Name: "override",
				Settings: &objects.ChannelSettings{PassThroughBody: lo.ToPtr(true), BodyOverrideOperations: []objects.OverrideOperation{{
					Op: objects.OverrideOpSet, Path: "user", Value: "channel",
				}}},
			}, Outbound: provider},
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
			Models:    []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "provider-model"}},
		}},
	}
	hooks := &recordingHooks{}
	registered, err := hooks.Middleware(context.Background())
	require.NoError(t, err)
	hookMiddleware := registered.Request
	inbound, outbound := NewPersistentTransformers(state, openai.NewInboundTransformer(), hookMiddleware)
	pipe := pipeline.NewFactory(httpclient.NewHttpClient()).Pipeline(
		inbound,
		outbound,
		pipeline.WithMiddlewares(hookMiddleware, applyPassThroughRequestBody(outbound, hooks), ApplyRequestHookAttempt(outbound, &wireHookForTest{t: t}), applyOverrideRequestBody(outbound)),
	)
	_, err = pipe.Process(context.Background(), &httpclient.Request{
		Method:  http.MethodPost,
		Path:    "/v1/chat/completions",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"vendor":"preserved"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "channel", gjson.GetBytes(got, "user").String())
	require.True(t, gjson.GetBytes(got, "plugin_value").Bool())
	require.Equal(t, "provider-model", gjson.GetBytes(got, "model").String())
}

type hookProvider struct {
	request    pipeline.Middleware
	middleware pipeline.Middleware
}

func (p hookProvider) Middleware(context.Context) (RequestHooks, error) {
	return RequestHooks{Request: p.request, Attempt: p.middleware}, nil
}
func (hookProvider) PassThroughApplied(context.Context, int) {}

type modelRewritingHook struct{ pipeline.DummyMiddleware }

func (modelRewritingHook) Name() string { return "plugin-request-hooks" }

func (modelRewritingHook) OnInboundLlmRequest(_ context.Context, request *llm.Request) (*llm.Request, error) {
	request.Model = "secret-model"
	return request, nil
}

type recordingOutbound struct{}

type recordingHooks struct {
	channels []int
}

func (h *recordingHooks) Middleware(context.Context) (RequestHooks, error) {
	return RequestHooks{Request: &recordingHookMiddleware{}}, nil
}

func (h *recordingHooks) PassThroughApplied(_ context.Context, channelID int) {
	h.channels = append(h.channels, channelID)
}

type recordingHookMiddleware struct{ pipeline.DummyMiddleware }

func (recordingHookMiddleware) Name() string { return "plugin-request-hooks" }
func (recordingHookMiddleware) OnOutboundRawRequest(_ context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	return request, nil
}

func (recordingHookMiddleware) OnInboundLlmRequest(_ context.Context, request *llm.Request) (*llm.Request, error) {
	request.Model = "mapped"
	request.Temperature = lo.ToPtr(0.2)
	request.User = lo.ToPtr("plugin")
	return request, nil
}

type wireHookForTest struct {
	pipeline.DummyMiddleware

	t *testing.T
}

func (h *wireHookForTest) OnOutboundRawRequest(ctx context.Context, req *httpclient.Request) (*httpclient.Request, error) {
	require.Equal(h.t, "preserved", gjson.GetBytes(req.Body, "vendor").String())
	require.Equal(h.t, "provider-model", gjson.GetBytes(req.Body, "model").String())
	var obj map[string]any
	require.NoError(h.t, json.Unmarshal(req.Body, &obj))
	obj["user"] = "wire-plugin"
	obj["plugin_value"] = true
	copy := *req
	body, err := json.Marshal(obj)
	require.NoError(h.t, err)
	copy.Body = body
	return &copy, nil
}

func TestOutboundResolvesAPIFormatBeforeRawMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		wire, candidate, fallback, want llm.APIFormat
	}{
		{"wire wins", llm.APIFormatAnthropicMessage, llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIChatCompletion, llm.APIFormatAnthropicMessage},
		{"candidate fallback", "", llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIChatCompletion, llm.APIFormatOpenAIResponse},
		{"transformer fallback", "", "", llm.APIFormatOpenAIChatCompletion, llm.APIFormatOpenAIChatCompletion},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockTransformer{apiFormat: tc.fallback, requestAPIFormat: tc.wire}
			state := &PersistenceState{ChannelModelsCandidates: []*ChannelModelsCandidate{{
				Channel:   &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}, Outbound: provider},
				APIFormat: string(tc.candidate), modelAPIFormats: []string{string(tc.candidate)},
				Models: []biz.ChannelModelEntry{{RequestModel: "model", ActualModel: "model"}},
			}}}
			outbound := &PersistentOutboundTransformer{state: state}
			request, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "model"})
			require.NoError(t, err)
			require.Equal(t, string(tc.want), request.APIFormat)
		})
	}
}

// Exercise the real Process ordering so moving the hook past access checks fails.
func TestProcessRequestHookModelAccessAndCleanup(t *testing.T) {
	ctx, client, _ := newUpstreamModelPersistenceTest(t)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	ctx = contexts.WithAPIKey(ctx, &ent.APIKey{Profiles: &objects.APIKeyProfiles{
		ActiveProfile: "limited", Profiles: []objects.APIKeyProfile{{Name: "limited", ModelIDs: []string{"gpt-4"}}},
	}})
	executor := &sequenceExecutor{}
	requestHook := &processCleanupHook{Middleware: &modelRewritingHook{}}
	attemptHook := &processCleanupHook{Middleware: &pipeline.DummyMiddleware{}}
	processor := &ChatCompletionOrchestrator{
		Inbound: openai.NewInboundTransformer(), RequestService: requestService, ChannelService: channelService,
		SystemService: systemService, UsageLogService: usageLogService,
		PipelineFactory: pipeline.NewFactory(executor),
		Plugins:         PluginIntegration{Hooks: hookProvider{request: requestHook, middleware: attemptHook}},
	}
	_, err := processor.Process(ctx, buildTestRequest("gpt-4", "Hello", false))
	require.ErrorIs(t, err, biz.ErrInvalidModel)
	require.Empty(t, executor.requests)
	require.Equal(t, 1, requestHook.ends)
	require.Equal(t, 1, attemptHook.ends, "attempt middleware must be cleaned up even before its request callback")
}

type processCleanupHook struct {
	pipeline.Middleware

	ends int
}

func (h *processCleanupHook) OnProcessEnd() { h.ends++ }
