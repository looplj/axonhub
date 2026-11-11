package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func localAdmissionCases() map[string]error {
	ch := &biz.Channel{Channel: &ent.Channel{ID: 42, Name: "test"}}
	return map[string]error{
		"queue full":    asChannelQueueError(ch, ErrChannelQueueFull),
		"queue timeout": asChannelQueueError(ch, ErrChannelQueueTimeout),
		"rpm":           newLocalRPMExhaustedError(ch, 1),
		"plugin":        NewPluginOverloadedError(&llm.ResponseError{StatusCode: http.StatusTooManyRequests}),
	}
}

func TestLocalAdmissionErrorClassification(t *testing.T) {
	for name, err := range localAdmissionCases() {
		t.Run(name, func(t *testing.T) {
			require.True(t, isLocalAdmissionError(err))
			require.True(t, isLocalAdmissionError(fmt.Errorf("raw middleware: %w", err)))
			require.True(t, isLocalAdmissionError(errors.Join(errors.New("another cause"), err)))
		})
	}
	for _, err := range []error{nil, context.Canceled, errors.New("network failure"), &httpclient.Error{StatusCode: 429}, &llm.ResponseError{StatusCode: 429}} {
		require.False(t, isLocalAdmissionError(err), "ordinary/upstream errors are not local admission")
	}
}

func TestLocalAdmissionDoesNotPenalizeHealthOrOverwritePerformance(t *testing.T) {
	for name, err := range localAdmissionCases() {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			wrapped := fmt.Errorf("raw middleware: %w", err)
			previous := biz.PerformanceRecord{ChannelID: 7, Success: true, RequestCompleted: true, CompletionTokens: 12}
			perf := previous
			candidate := &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{ID: 42, Name: "next"}}}
			state := &PersistenceState{
				Perf: &perf, OriginalModel: "m", CurrentCandidate: candidate,
				ChannelModelsCandidates: []*ChannelModelsCandidate{candidate, {}},
				RoutingPolicy:           EffectiveRoutingPolicy{LoadBalancerStrategy: biz.LoadBalancerStrategyCircuitBreaker},
			}
			outbound := &PersistentOutboundTransformer{state: state}
			// The current attempt's request callback may not have run. A nil service
			// also ensures this path cannot enqueue an upstream failure as a side effect.
			(&performanceRecording{outbound: outbound}).OnOutboundRawError(ctx, wrapped)
			require.Equal(t, previous, perf)
			cb := biz.NewModelCircuitBreaker()
			cb.RecordError(ctx, 42, "m", false)
			circuit := withModelCircuitBreaker(outbound, cb).(*modelCircuitBreakerTracker)
			circuit.probeActive = true
			circuit.probeChannelID = 42
			circuit.probeModelID = "m"
			circuit.OnOutboundRawError(ctx, wrapped)
			require.False(t, circuit.probeActive, "admission failure still releases a probe lease")
			require.Equal(t, 1, cb.GetModelCircuitBreakerStats(ctx, 42, "m").ConsecutiveFailures)
			tracker := NewChannelRequestTracker()
			rate := &rateLimitTracking{outbound: outbound, tracker: tracker}
			// Even joined transport diagnostics must not turn a local rejection into cooldown.
			withHeaders := errors.Join(&httpclient.Error{StatusCode: 429, Headers: http.Header{"Retry-After": []string{"60"}}}, wrapped)
			rate.OnOutboundRawError(ctx, withHeaders)
			require.False(t, tracker.IsCoolingDown(42))
			require.False(t, outbound.CanRetry(wrapped))
			if isPluginOverloaded(wrapped) {
				state.PluginOverloaded = true
			}
			require.Equal(t, !isPluginOverloaded(wrapped), outbound.HasMoreChannels())
		})
	}
}

func TestAdmissionPersistenceDistinguishesEarlyPluginOverload(t *testing.T) {
	for name, admissionErr := range localAdmissionCases() {
		t.Run(name, func(t *testing.T) {
			ctx, client, state := newUpstreamModelPersistenceTest(t)
			execution := createUpstreamModelTestExecution(t, ctx, client, state.Request, "model", llm.APIFormatOpenAIChatCompletion, false)
			if isPluginOverloaded(admissionErr) {
				// This belongs to the preceding attempt, not the overloaded one.
				var err error
				execution, err = execution.Update().SetStatus(requestexecution.StatusFailed).SetErrorMessage("previous upstream failure").Save(ctx)
				require.NoError(t, err)
			}
			state.RequestExec = execution
			middleware := &persistRequestExecutionMiddleware{outbound: &PersistentOutboundTransformer{state: state}}
			middleware.OnOutboundRawError(ctx, fmt.Errorf("raw middleware: %w", admissionErr))
			saved, err := client.RequestExecution.Get(ctx, execution.ID)
			require.NoError(t, err)
			require.Equal(t, requestexecution.StatusFailed, saved.Status)
			if isPluginOverloaded(admissionErr) {
				require.Equal(t, "previous upstream failure", saved.ErrorMessage)
			} else {
				require.NotEmpty(t, saved.ErrorMessage)
			}
		})
	}
}

func TestLocalAdmissionDoesNotAdvanceCredentialDisableRules(t *testing.T) {
	ctx, client, state := newUpstreamModelPersistenceTest(t)
	entity, err := client.Channel.Get(ctx, state.Request.ChannelID)
	require.NoError(t, err)
	entity, err = entity.Update().SetCredentials(objects.ChannelCredentials{APIKeys: []string{"key1", "key2"}}).
		SetPolicies(objects.ChannelPolicies{
			APIKeyAutoDisableMode:  objects.APIKeyAutoDisableModeCustom,
			APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{StatusCodes: []int{429}, Times: 2, Action: objects.APIKeyAutoDisableActionTemporary, DisableDurationMinutes: lo.ToPtr(30)}},
		}).Save(ctx)
	require.NoError(t, err)
	svc := biz.NewChannelServiceForTest(client)
	ch := &biz.Channel{Channel: entity}
	svc.SetEnabledChannelsForTest([]*biz.Channel{ch})
	state.ChannelService = svc
	state.CurrentCandidate = &ChannelModelsCandidate{Channel: ch}
	outbound := &PersistentOutboundTransformer{state: state}
	ctx = contexts.WithChannelAPIKey(ctx, "key1")
	for _, localErr := range localAdmissionCases() {
		maybeEvaluateChannelAPIKeyRulesOnFailure(ctx, outbound, fmt.Errorf("wrapped: %w", localErr))
	}
	saved, err := client.Channel.Get(ctx, entity.ID)
	require.NoError(t, err)
	require.Empty(t, saved.DisabledAPIKeys)
	// A real upstream rejection must still count. Two real failures, not local
	// rejections, are required to reach the configured threshold.
	upstreamErr := &httpclient.Error{StatusCode: 429, Body: []byte(`{"error":{"message":"upstream limit"}}`)}
	maybeEvaluateChannelAPIKeyRulesOnFailure(ctx, outbound, upstreamErr)
	saved, err = client.Channel.Get(ctx, entity.ID)
	require.NoError(t, err)
	require.Empty(t, saved.DisabledAPIKeys)
	maybeEvaluateChannelAPIKeyRulesOnFailure(ctx, outbound, upstreamErr)
	saved, err = client.Channel.Get(ctx, entity.ID)
	require.NoError(t, err)
	require.Len(t, saved.DisabledAPIKeys, 1)
	require.Equal(t, "key1", saved.DisabledAPIKeys[0].Key)
}

func TestProcessPluginOverloadPreservesPreviousExecutionFailure(t *testing.T) {
	ctx := authz.WithTestBypass(context.Background())
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx = ent.NewContext(ctx, client)
	project := createTestProject(t, ctx, client)
	ctx = contexts.WithProjectID(ctx, project.ID)
	ch := createTestChannel(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	require.NoError(t, systemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled: true, MaxChannelRetries: 1, MaxSingleChannelRetries: 1,
		LoadBalancerStrategy: "adaptive", TraceStickyMode: biz.TraceStickyPreferPreviousChannel,
	}))
	provider, err := openai.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
	require.NoError(t, err)
	selector := &staticChannelSelector{candidates: []*ChannelModelsCandidate{{
		Channel: &biz.Channel{Channel: ch, Outbound: provider}, TraceSticky: true,
		Models: []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
	}}}
	executor := &sequenceExecutor{steps: []executorStep{
		{err: &httpclient.Error{StatusCode: 500, Body: []byte(`{"error":{"message":"first upstream failure"}}`)}},
		{resp: &httpclient.Response{StatusCode: 200, Body: buildMockOpenAIResponse("unexpected", "gpt-4", "not reached", 1, 1)}},
	}}
	hook := &overloadOnSecondAttempt{}
	processor := &ChatCompletionOrchestrator{
		channelSelector: selector, Inbound: openai.NewInboundTransformer(), RequestService: requestService, ChannelService: channelService,
		PromptProvider: &stubPromptProvider{}, SystemService: systemService, UsageLogService: usageLogService,
		PipelineFactory: pipeline.NewFactory(executor), ModelMapper: NewModelMapper(), channelLimiterManager: NewChannelLimiterManager(),
		Plugins: PluginIntegration{Hooks: hookProvider{middleware: hook}},
	}
	_, err = processor.Process(ctx, buildTestRequest("gpt-4", "Hello", false))
	var overloaded *PluginOverloadedError
	require.ErrorAs(t, err, &overloaded)
	require.Equal(t, 429, overloaded.Response.StatusCode)
	require.Equal(t, 2, hook.calls)
	require.Len(t, executor.requests, 1)
	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)
	require.Equal(t, requestexecution.StatusFailed, executions[0].Status)
	require.Contains(t, executions[0].ErrorMessage, "first upstream failure")
	require.NotContains(t, executions[0].ErrorMessage, "plugin_overloaded")
	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	require.Equal(t, request.StatusFailed, requests[0].Status)
}

// Attempt-only hooks do not own any request-scoped resources.
type overloadOnSecondAttempt struct {
	pipeline.DummyMiddleware

	calls int
}

func (h *overloadOnSecondAttempt) OnOutboundRawRequest(_ context.Context, req *httpclient.Request) (*httpclient.Request, error) {
	h.calls++
	if h.calls == 2 {
		return nil, NewPluginOverloadedError(&llm.ResponseError{StatusCode: 429, Detail: llm.ErrorDetail{Code: "plugin_overloaded", Message: "plugin at capacity"}})
	}
	return req, nil
}
