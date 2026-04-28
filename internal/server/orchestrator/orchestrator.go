package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/cc"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
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
) *ChatCompletionOrchestrator {
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
	quotaStrategy := NewQuotaAwareStrategy(quotaProvider, systemService)

	adaptiveLoadBalancer := NewLoadBalancer(systemService, channelService,
		NewTraceAwareStrategy(requestService),
		NewErrorAwareStrategy(channelService),
		NewWeightRoundRobinStrategy(channelService),
		NewLatencyAwareStrategy(channelService),
		rateLimitStrategy,
		quotaStrategy,
	)

	failoverLoadBalancer := NewLoadBalancer(systemService, channelService,
		NewWeightStrategy(), NewRandomStrategy(), rateLimitStrategy, quotaStrategy)

	circuitBreakerLoadBalancer := NewLoadBalancer(systemService, channelService,
		NewWeightStrategy(), NewModelAwareCircuitBreakerStrategy(modelCircuitBreaker), rateLimitStrategy, quotaStrategy)

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
			cc.StripBillingHeaderCCH(),
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
		modelCircuitBreaker:        modelCircuitBreaker,
		quotaProvider:              quotaProvider,
		proxy:                      nil,
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

	// proxy is the proxy configuration for testing
	// If set, it will override the channel's default proxy configuration
	proxy *httpclient.ProxyConfig
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

type ChatCompletionResult struct {
	ChatCompletion       *httpclient.Response
	ChatCompletionStream streams.Stream[*httpclient.StreamEvent]
}

func (processor *ChatCompletionOrchestrator) Process(ctx context.Context, request *httpclient.Request) (ChatCompletionResult, error) {
	// The context is system bypassed to allow the orchestrator to access the system settings.
	ctx = authz.WithSystemBypass(ctx, "process-chat-completion")

	apiKey, _ := contexts.GetAPIKey(ctx)

	// Get retry policy from system settings
	retryPolicy := processor.SystemService.RetryPolicyOrDefault(ctx)

	strategy := deriveLoadBalancerStrategy(retryPolicy, apiKey)
	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "chat request received",
			log.String("request_body", string(request.Body)),
			log.Any("request_headers", request.Headers),
			log.Any("retry_policy", retryPolicy),
			log.String("system_load_balance_strategy", retryPolicy.LoadBalancerStrategy),
			log.String("load_balance_strategy", strategy),
		)
	}

	loadBalancer := processor.adaptiveLoadBalancer

	switch strategy {
	case biz.LoadBalancerStrategyAdaptive:
		loadBalancer = processor.adaptiveLoadBalancer
	case biz.LoadBalancerStrategyFailover:
		loadBalancer = processor.failoverLoadBalancer
	case biz.LoadBalancerStrategyCircuitBreaker:
		loadBalancer = processor.circuitBreakerLoadBalancer
	default:
		// Default to adaptive load balancer
	}

	state := &PersistenceState{
		APIKey:                apiKey,
		RequestService:        processor.RequestService,
		UsageLogService:       processor.UsageLogService,
		ChannelService:        processor.ChannelService,
		PromptProvider:        processor.PromptProvider,
		PromptProtecter:       processor.PromptProtecter,
		RetryPolicyProvider:   processor.SystemService,
		CandidateSelector:     processor.channelSelector,
		LoadBalancer:          loadBalancer,
		ModelMapper:           processor.ModelMapper,
		Proxy:                 processor.proxy,
		CurrentCandidateIndex: 0,
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
	}

	var middlewares []pipeline.Middleware

	// Add global middlewares
	middlewares = append(middlewares, processor.Middlewares...)

	inbound, outbound := NewPersistentTransformers(state, processor.Inbound)

	// Add inbound middlewares (executed after inbound.TransformRequest)
	middlewares = append(middlewares,
		enforceQuota(inbound, processor.QuotaService),
		applyAutoReasoningEffort(processor.SystemService),
		checkApiKeyModelAccess(inbound),
		applyModelMapping(inbound),
		selectCandidates(inbound, processor.quotaProvider, processor.SystemService),
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
		applyPassThroughRequestBody(outbound),
		applyOverrideRequestBody(outbound),
		// applyUserAgentPassThrough runs before header overrides to set the initial
		// User-Agent value (either from client pass-through or default "axonhub/1.0").
		// This allows override headers to modify the User-Agent if configured.
		applyUserAgentPassThrough(outbound, processor.SystemService),
		applyOverrideRequestHeaders(outbound),

		// Unified performance tracking middleware.
		withPerformanceRecording(outbound),

		withModelCircuitBreaker(outbound, processor.modelCircuitBreaker, strategy),

		// The request execution middleware must be the final middleware
		// to ensure that the request execution is created with the correct request bodys.
		persistRequestExecution(outbound),

		// Forward the events to the live streaming.
		withLivePreview(state, processor.SystemService, processor.LiveStreamRegistry),

		// Per-channel admission control. Must run before rate-limit tracking so a
		// locally rejected (queue full / queue timeout) request does not consume
		// RPM budget for a request that never reached upstream.
		withChannelLimiter(outbound, processor.channelLimiterManager, processor.channelLimiterMetrics),
		// Rate limit tracking middleware for load balancing.
		withRateLimitTracking(outbound, processor.rateLimitTracker),

		// Response pass-through capture middlewares must be last in the outbound list
		// so they run first in reverse order (before any other OnOutboundRawResponse/OnOutboundRawStream handlers).
		captureRawProviderResponse(outbound),
		captureRawProviderStream(outbound),
	)

	pipelineOpts = append(pipelineOpts, pipeline.WithMiddlewares(middlewares...))

	pipe := processor.PipelineFactory.Pipeline(
		inbound,
		outbound,
		pipelineOpts...,
	)

	result, err := pipe.Process(ctx, request)
	if err != nil {
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, time.Second*10)
		defer cancel()

		// Update the last request execution status based on error if it exists
		// This ensures that when retry fails completely, the last execution is properly marked
		if requestExec := outbound.GetRequestExecution(); requestExec != nil {
			if updateErr := processor.RequestService.UpdateRequestExecutionStatusFromError(
				persistCtx,
				requestExec.ID,
				err,
			); updateErr != nil {
				log.Warn(persistCtx, "Failed to update request execution status from error", log.Cause(updateErr))
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

		return ChatCompletionResult{}, err
	}

	// Return result based on stream type
	if result.Stream {
		return ChatCompletionResult{
			ChatCompletion:       nil,
			ChatCompletionStream: result.EventStream,
		}, nil
	}

	return ChatCompletionResult{
		ChatCompletion:       result.Response,
		ChatCompletionStream: nil,
	}, nil
}
