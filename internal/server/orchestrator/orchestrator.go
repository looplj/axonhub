package orchestrator

import (
	"context"
	"errors"
	"time"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/events"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/cc"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func NewChatCompletionOrchestrator(
	channelService *biz.ChannelService,
	defaultSelector *DefaultSelector,
	requestService *biz.RequestService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
	systemService *biz.SystemService,
	usageLogService *biz.UsageLogService,
	promptService *biz.PromptService,
	quotaService *biz.QuotaService,
	promptProtectionRuleService *biz.PromptProtectionRuleService,
	liveStreamRegistry *biz.LiveStreamRegistry,
	channelLimiterManager *ChannelLimiterManager,
	quotaProvider ProviderQuotaStatusProvider,
	plugins PluginIntegration,
) *ChatCompletionOrchestrator {
	responsesStore := newResponsesSessionStore(requestService.LoadCompletedResponsesSession)
	responsesStore.known = requestService.HasCompletedResponsesOrigin
	rateLimitTracker := NewChannelRequestTracker()

	channelService.SetChannelLimiterForgetter(channelLimiterManager)

	channelLimiterMetrics, err := NewChannelLimiterMetrics(metrics.Meter, channelLimiterManager)
	if err != nil {
		log.Warn(context.Background(), "failed to register channel limiter metrics, continuing without them", log.Cause(err))
		channelLimiterMetrics = nil
	}

	// Initialize model circuit breaker
	modelCircuitBreaker := biz.NewModelCircuitBreaker()

	rateLimitStrategy := NewRateLimitAwareStrategy(rateLimitTracker, channelLimiterManager)

	adaptiveLoadBalancer := NewLoadBalancer(channelService, channelService,
		NewErrorAwareStrategy(),
		NewWeightRoundRobinStrategy(),
		NewLatencyAwareStrategy(),
		rateLimitStrategy,
	)

	// Failover and circuit-breaker strategies never read channel metrics, so
	// these load balancers skip the per-candidate metrics fetch.
	failoverLoadBalancer := NewLoadBalancer(nil, channelService,
		NewWeightStrategy(), NewRandomStrategy(), rateLimitStrategy)

	circuitBreakerLoadBalancer := NewLoadBalancer(nil, channelService,
		NewWeightStrategy(), NewModelAwareCircuitBreakerStrategy(modelCircuitBreaker), rateLimitStrategy)

	roundRobinHealthFilter := NewRoundRobinHealthStrategy()
	roundRobinLoadBalancer := NewLoadBalancer(channelService, channelService,
		NewRoundRobinStrategy(),
		rateLimitStrategy,
	).WithoutWeightTieBreaker().WithRoundRobinHealthFilter(roundRobinHealthFilter)

	return &ChatCompletionOrchestrator{
		Inbound:            inbound,
		RequestService:     requestService,
		ChannelService:     channelService,
		SystemService:      systemService,
		UsageLogService:    usageLogService,
		QuotaService:       quotaService,
		LiveStreamRegistry: liveStreamRegistry,
		PromptProvider:     promptService,
		PromptProtecter:    promptProtectionRuleService,
		Middlewares: []pipeline.Middleware{
			cc.SystemCacheCompatibility(),
			stream.EnsureUsage(),
		},
		PipelineFactory:            pipeline.NewFactory(httpClient),
		ModelMapper:                NewModelMapper(),
		channelSelector:            defaultSelector,
		channelLimiterManager:      channelLimiterManager,
		channelLimiterMetrics:      channelLimiterMetrics,
		rateLimitTracker:           rateLimitTracker,
		adaptiveLoadBalancer:       adaptiveLoadBalancer,
		failoverLoadBalancer:       failoverLoadBalancer,
		circuitBreakerLoadBalancer: circuitBreakerLoadBalancer,
		roundRobinLoadBalancer:     roundRobinLoadBalancer,
		modelCircuitBreaker:        modelCircuitBreaker,
		quotaProvider:              quotaProvider,
		Plugins:                    plugins,
		proxy:                      nil,
		responsesSessions:          responsesStore,
		EventBus:                   channelService.EventBus,
	}
}

type ChatCompletionOrchestrator struct {
	Inbound            transformer.Inbound
	RequestService     *biz.RequestService
	ChannelService     *biz.ChannelService
	SystemService      *biz.SystemService
	UsageLogService    *biz.UsageLogService
	QuotaService       *biz.QuotaService
	LiveStreamRegistry *biz.LiveStreamRegistry
	PromptProvider     PromptProvider
	PromptProtecter    PromptProtecter
	Middlewares        []pipeline.Middleware
	PipelineFactory    *pipeline.Factory
	ModelMapper        *ModelMapper

	// The runtime fields.

	// The default channel selector.
	channelSelector CandidateSelector
	// The load balancer for channel load balancing.
	adaptiveLoadBalancer       *LoadBalancer
	failoverLoadBalancer       *LoadBalancer
	circuitBreakerLoadBalancer *LoadBalancer
	roundRobinLoadBalancer     *LoadBalancer
	// channelLimiterManager owns per-channel concurrency admission control and
	// supplies in-flight / queue stats to the rate-limit-aware load-balancer strategy.
	channelLimiterManager *ChannelLimiterManager
	// channelLimiterMetrics emits OTel metrics for the limiter (gauges + counters
	// + histogram). May be nil in test setups that skip metric registration.
	channelLimiterMetrics *ChannelLimiterMetrics
	// The rate limit tracker for rate limit aware load balancing.
	rateLimitTracker *ChannelRequestTracker
	// The model circuit breaker for circuit-breaker load balancing.
	modelCircuitBreaker *biz.ModelCircuitBreaker
	// The provider quota status provider for quota-aware load balancing and selection.
	quotaProvider ProviderQuotaStatusProvider

	// Plugins carries request hooks. A zero value adds no middleware.
	Plugins PluginIntegration

	// proxy is the proxy configuration for testing
	// If set, it will override the channel's default proxy configuration
	proxy *httpclient.ProxyConfig

	responsesSessions *responsesSessionStore

	// EventBus publishes plugin events. Nil skips publication. FX threads the
	// process bus through ChannelService, and the constructor copies it here.
	EventBus *events.Bus
}

func (processor *ChatCompletionOrchestrator) WithChannelSelector(selector CandidateSelector) *ChatCompletionOrchestrator {
	c := *processor
	c.channelSelector = selector

	return &c
}

func (processor *ChatCompletionOrchestrator) WithAllowedChannels(allowedChannelIDs []int) *ChatCompletionOrchestrator {
	c := *processor
	c.channelSelector = WithSelectedChannelsSelector(processor.channelSelector, allowedChannelIDs)

	return &c
}

func (processor *ChatCompletionOrchestrator) WithProxy(proxy *httpclient.ProxyConfig) *ChatCompletionOrchestrator {
	c := *processor
	c.proxy = proxy

	return &c
}

// WithEventBus returns a copy that publishes plugin events to bus.
// A nil bus skips publication. The bus is passed on the orchestrator, not on context.
func (processor *ChatCompletionOrchestrator) WithEventBus(bus *events.Bus) *ChatCompletionOrchestrator {
	c := *processor
	c.EventBus = bus

	return &c
}

// ResponseRoute identifies the committed upstream attempt without retaining
// mutable persistence state or exposing provider credentials.
type ResponseRoute struct {
	ChannelID int
	Model     string
	APIFormat string
}

type ChatCompletionResult struct {
	Route                         ResponseRoute
	ChatCompletion                *httpclient.Response
	ChatCompletionStream          streams.Stream[*httpclient.StreamEvent]
	CodexResponseHeadersSupported bool
}

func (processor *ChatCompletionOrchestrator) Process(ctx context.Context, request *httpclient.Request) (ChatCompletionResult, error) {
	lifecycleOwner, executionCtx := newStreamLifecycle(ctx)
	ctx = executionCtx

	var continuation *responsesContinuation
	if shared.IsResponsesAPI(ctx) && request != nil && processor.responsesSessions != nil {
		continuation = newResponsesContinuation(processor.responsesSessions, request.Body)
		defer func() {
			if continuation != nil {
				continuation.releaseRetention()
				continuation.releaseCapture()
			}
		}()
		ctx = shared.WithResponsesAttempt(ctx, &shared.ResponsesAttempt{})
		if shared.GetResponsesLane(ctx) == nil {
			sessionID, _ := shared.GetSessionID(ctx)
			if record := processor.responsesSessions.lookup(ctx, continuation.previous); record != nil && record.sessionID != "" {
				sessionID = record.sessionID
			}
			ctx = shared.WithSessionID(ctx, responseSessionID(sessionID))
		}
	}

	// API key providers cannot return a derived context, so install the shared
	// request container before provider selection mutates it.
	ctx = contexts.EnsureContainer(ctx)

	// Install a request-scoped memo so cost injection and usage-log creation share
	// a single model-library price lookup instead of repeating it per calculation.
	ctx = biz.WithModelLibraryPriceMemo(ctx)

	// The context is system bypassed to allow the orchestrator to access the system settings.
	ctx = authz.WithSystemBypass(ctx, "process-chat-completion")

	apiKey, _ := contexts.GetAPIKey(ctx)

	settings := processor.loadRequestSettings(ctx)
	ctx = withRequestSettings(ctx, settings)
	retryPolicy := &settings.RetryPolicy

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "chat request received",
			inboundRequestDebugFields(request, retryPolicy)...,
		)
	}

	state := &PersistenceState{
		ResponsesContinuation: continuation,
		APIKey:                apiKey,
		RequestService:        processor.RequestService,
		UsageLogService:       processor.UsageLogService,
		ChannelService:        processor.ChannelService,
		PromptProvider:        processor.PromptProvider,
		PromptProtecter:       processor.PromptProtecter,
		CandidateSelector:     processor.channelSelector,
		LoadBalancers: map[string]*LoadBalancer{
			biz.LoadBalancerStrategyAdaptive:       processor.adaptiveLoadBalancer,
			biz.LoadBalancerStrategyFailover:       processor.failoverLoadBalancer,
			biz.LoadBalancerStrategyCircuitBreaker: processor.circuitBreakerLoadBalancer,
			biz.LoadBalancerStrategyRoundRobin:     processor.roundRobinLoadBalancer,
		},
		RoutingPolicy:         deriveRoutingPolicy(retryPolicy, apiKey, nil),
		Settings:              settings,
		ModelMapper:           processor.ModelMapper,
		Proxy:                 processor.proxy,
		CurrentCandidateIndex: 0,
		EventBus:              processor.EventBus,
	}

	var pipelineOpts []pipeline.Option

	// Only apply retry if policy is enabled
	if retryPolicy.Enabled {
		pipelineOpts = append(pipelineOpts, pipeline.WithRetry(
			retryPolicy.MaxChannelRetries,
			retryPolicy.MaxSingleChannelRetries,
			time.Duration(retryPolicy.RetryDelayMs)*time.Millisecond,
		))

		if retryPolicy.EmptyResponseDetection {
			pipelineOpts = append(pipelineOpts, pipeline.WithEmptyResponseDetection())
		}

		pipelineOpts = append(pipelineOpts, pipeline.WithResponseTimeouts(
			time.Duration(retryPolicy.StreamFirstEventTimeoutSeconds)*time.Second,
			time.Duration(retryPolicy.NonStreamResponseTimeoutSeconds)*time.Second,
		))
	} else {
		// Retries are switched off globally: a channel-level override must not
		// turn same-channel retries back on.
		pipelineOpts = append(pipelineOpts, pipeline.WithRetriesDisabled())
	}

	var middlewares []pipeline.Middleware

	// Add global middlewares
	middlewares = append(middlewares, processor.Middlewares...)
	middlewares = append(middlewares, newBillingSystemMessageMiddleware(state))
	hooks, err := processor.Plugins.RequestHooks(ctx)
	if err != nil {
		lifecycleOwner.Close()
		return ChatCompletionResult{}, err
	}
	middlewares = append(middlewares, hooks.requestStage()...)

	inbound, outbound := NewPersistentTransformers(state, processor.Inbound, middlewares...)

	// Add inbound middlewares (executed after inbound.TransformRequest)
	middlewares = append(middlewares,
		enforceQuota(inbound, processor.QuotaService),
		applyAutoReasoningEffort(settings),
		checkApiKeyModelAccess(inbound),
		applyModelMapping(inbound),
		selectCandidates(inbound, processor.quotaProvider),
		injectPrompts(inbound),
		protectPrompts(inbound),
		// Response pass-through middlewares run before persistRequest so the raw provider
		// response is saved when pass-through is enabled.
		applyPassThroughResponse(outbound),
		applyPassThroughStream(outbound),
		persistRequest(inbound),
	)

	// Add outbound middlewares (executed after outbound.TransformRequest)
	middlewares = append(middlewares,
		// applyPassThroughBody runs first so that override operations can still modify the pass-through body.
		applyPassThroughRequestBody(outbound, processor.Plugins.Hooks),
		// Responses HTTP/WS adaptation reuses the effective body before continuation and overrides.
		applyResponsesTransportRequestBody(outbound),
		// Codex Responses metadata must travel with a pass-through body so compatible
		// upstreams preserve the client's protocol behavior.
		applyPassThroughRequestHeaders(outbound),
		applyResponsesContinuationBody(outbound),
	)
	middlewares = append(middlewares, hooks.attemptStage(outbound)...)
	middlewares = append(middlewares,
		applyOverrideRequestBody(outbound),
		// applyUserAgentPassThrough runs before header overrides to set the initial
		// User-Agent value (either from client pass-through or default "axonhub/1.0").
		// A provider-required User-Agent already set by the outbound transformer
		// (e.g. GitHubCopilotChat on Copilot channels) is preserved when
		// pass-through is disabled. Override headers can still modify the
		// User-Agent if configured.
		applyUserAgentPassThrough(outbound),
		applyOverrideRequestHeaders(outbound),
		// Remove transport-incompatible fields after pass-through and overrides,
		// so persistence and execution observe the same provider request.
		verifyResponsesAttempt(outbound),
		finalizeTransportRequest(outbound),

		// Unified performance tracking middleware.
		withoutResponsesRecoveryPenalty(withPerformanceRecording(outbound)),

		withoutResponsesRecoveryPenalty(withModelCircuitBreaker(outbound, processor.modelCircuitBreaker)),

		// The request execution middleware must be the final middleware
		// to ensure that the request execution is created with the correct request bodys.
		persistRequestExecution(outbound),

		// Forward the events to the live streaming.
		withLivePreview(state, processor.LiveStreamRegistry),

		// Per-channel concurrency admission runs before RPM admission so a locally
		// rejected queue attempt does not consume RPM for a request that never
		// reached upstream.
		withChannelLimiter(outbound, processor.channelLimiterManager, processor.channelLimiterMetrics),
		// Strict single-instance RPM admission for every outbound attempt.
		withRateLimitAdmission(outbound, processor.rateLimitTracker),
		// Rate limit tracking middleware for TPM and provider cooldown signals.
		withRateLimitTracking(outbound, processor.rateLimitTracker),

		// Response pass-through capture middlewares must be last in the outbound list
		// so they run first in reverse order (before any other OnOutboundRawResponse/OnOutboundRawStream handlers).
		captureRawProviderResponse(outbound),
		captureRawProviderStream(outbound),
		captureResponsesProvider(outbound),
		// Run first on raw responses (reverse order), before pass-through capture.
		closeCompletedAnthropicStream(outbound),
	)

	pipelineOpts = append(pipelineOpts, pipeline.WithMiddlewares(middlewares...))

	pipe := processor.PipelineFactory.Pipeline(
		inbound,
		outbound,
		pipelineOpts...,
	)

	result, err := pipe.Process(ctx, request)
	if err != nil {
		lifecycleOwner.Close()
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, time.Second*10)
		defer cancel()

		// Finish the failed execution, unless plugin admission stopped this attempt
		// before persistence ran: this attempt has no execution record.
		if requestExec := outbound.GetRequestExecution(); requestExec != nil && !isPluginOverloaded(err) {
			if updateErr := persistRequestExecutionFailure(
				persistCtx,
				processor.RequestService,
				requestExec.ID,
				err,
				nil,
				"",
			); updateErr != nil {
				log.Warn(persistCtx, "Failed to update request execution status from error", log.Cause(updateErr))
			}
			if !errors.Is(err, shared.ErrResponsesContinuationUnavailable) {
				maybeEvaluateChannelAPIKeyRulesOnFailure(persistCtx, outbound, err)
			}
		}

		// Update the main request status based on error
		if request := outbound.GetRequest(); request != nil {
			if updateErr := processor.RequestService.UpdateRequestStatusFromError(
				persistCtx,
				request.ID,
				err,
			); updateErr != nil {
				log.Warn(persistCtx, "Failed to update request status from error", log.Cause(updateErr))
			}
		}

		publishRequestFailed(persistCtx, state, err)

		return ChatCompletionResult{}, err
	}

	// Pipeline retries finish before the response is returned. Snapshot the final
	// route so downstream display estimates cannot share calibration across routes.
	var route ResponseRoute
	if channel := outbound.GetCurrentChannel(); channel != nil {
		route.ChannelID = channel.ID
	}
	if state.RawProviderRequest != nil {
		route.Model = gjson.GetBytes(state.RawProviderRequest.Body, "model").String()
		route.APIFormat = state.RawProviderRequest.APIFormat
	}
	if route.Model == "" && state.RequestExec != nil {
		route.Model = state.RequestExec.ModelID
	}

	// Return result based on stream type
	if result.Stream {
		if continuation != nil {
			source := result.EventStream
			accepted := &responsesAcceptedStream{Stream: source, ctx: ctx, continuation: continuation}
			result.EventStream = httpclient.WithResponseHeaders(preserveDeliveryCapabilities(source, accepted), httpclient.GetResponseHeaders(source))
			continuation = nil
		}
		return ChatCompletionResult{
			Route:                         route,
			ChatCompletion:                nil,
			ChatCompletionStream:          result.EventStream,
			CodexResponseHeadersSupported: outbound.SupportsCodexResponseHeaders(),
		}, nil
	}
	if continuation != nil && result.Response != nil {
		continuation.accept(ctx)
	}
	lifecycleOwner.Close()

	return ChatCompletionResult{
		Route:                         route,
		ChatCompletion:                result.Response,
		ChatCompletionStream:          nil,
		CodexResponseHeadersSupported: outbound.SupportsCodexResponseHeaders(),
	}, nil
}

func maybeEvaluateChannelAPIKeyRulesOnFailure(
	ctx context.Context,
	outbound *PersistentOutboundTransformer,
	rawErr error,
) {
	if isLocalAdmissionError(rawErr) {
		return
	}
	if outbound == nil || outbound.state == nil || outbound.state.ChannelService == nil {
		return
	}
	if outbound.state.Perf != nil && outbound.state.Perf.RequestCompleted {
		return
	}

	channel := outbound.GetCurrentChannel()
	if channel == nil {
		return
	}

	apiKey, ok := contexts.GetChannelAPIKey(ctx)
	if !ok && outbound.state.Perf != nil {
		apiKey = outbound.state.Perf.APIKey
	}
	if apiKey == "" {
		return
	}

	outbound.state.ChannelService.EvaluateAPIKeyRulesForFailure(
		ctx,
		channel.ID,
		apiKey,
		ExtractErrorCode(rawErr),
		extractErrorMessageForMatching(rawErr),
	)
}
