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

const DefaultMaxConnectionsPerChannel = 256

type OrchestratorConfig struct {
	ChannelService              *biz.ChannelService
	DefaultSelector             *DefaultSelector
	RequestService              *biz.RequestService
	HttpClient                  *httpclient.HttpClient
	Inbound                     transformer.Inbound
	SystemService               *biz.SystemService
	UsageLogService             *biz.UsageLogService
	PromptService               *biz.PromptService
	QuotaService                *biz.QuotaService
	PromptProtectionRuleService *biz.PromptProtectionRuleService
	LiveStreamRegistry          *biz.LiveStreamRegistry
	RateLimitTracker            *ChannelRequestTracker
	ConnectionTracker           ConnectionTracker
	ModelConnectionTracker      *ModelConnectionTracker
	CostTracker                 *ChannelCostTracker
}

// WithInbound returns a copy of the config with the Inbound field replaced.
func (c OrchestratorConfig) WithInbound(inbound transformer.Inbound) OrchestratorConfig {
	cfg := c
	cfg.Inbound = inbound

	return cfg
}

func NewChatCompletionOrchestrator(cfg OrchestratorConfig) *ChatCompletionOrchestrator {
	var connectionTracker ConnectionTracker
	if cfg.ConnectionTracker != nil {
		connectionTracker = cfg.ConnectionTracker
	}

	var rateLimitTracker *ChannelRequestTracker
	if cfg.RateLimitTracker != nil {
		rateLimitTracker = cfg.RateLimitTracker
	} else {
		rateLimitTracker = NewChannelRequestTracker()
	}
	// Only start default (non-fx) trackers. When trackers are provided via fx,
	// the fx lifecycle hook handles Start/Stop.
	if cfg.RateLimitTracker == nil {
		rateLimitTracker.Start()
	}

	var modelConnectionTracker *ModelConnectionTracker
	if cfg.ModelConnectionTracker != nil {
		modelConnectionTracker = cfg.ModelConnectionTracker
	} else {
		modelConnectionTracker = NewModelConnectionTracker()
	}

	var costTracker *ChannelCostTracker
	if cfg.CostTracker != nil {
		costTracker = cfg.CostTracker
	} else {
		costTracker = NewChannelCostTracker()
	}
	if cfg.CostTracker == nil {
		costTracker.Start()
	}

	channelLimiterManager := NewChannelLimiterManager()
	cfg.ChannelService.SetChannelLimiterForgetter(channelLimiterManager)

	channelLimiterMetrics, err := NewChannelLimiterMetrics(metrics.Meter, channelLimiterManager)
	if err != nil {
		log.Warn(context.Background(), "failed to register channel limiter metrics, continuing without them", log.Cause(err))
		channelLimiterMetrics = nil
	}

	// Initialize model circuit breaker
	modelCircuitBreaker := biz.NewModelCircuitBreaker()
	rateLimitStrategy := NewRateLimitAwareStrategy(RateLimitProvider{
		RequestTracker:    rateLimitTracker,
		ConnectionTracker: connectionTracker,
		ModelConnTracker:  modelConnectionTracker,
		CostTracker:       costTracker,
		QuotaService:      cfg.QuotaService,
	})

	adaptiveLoadBalancer := NewLoadBalancer(cfg.SystemService, cfg.ChannelService,
		NewTraceAwareStrategy(cfg.RequestService),
		NewErrorAwareStrategy(cfg.ChannelService),
		NewWeightRoundRobinStrategy(cfg.ChannelService),
		NewLatencyAwareStrategy(cfg.ChannelService),
		rateLimitStrategy,
	)

	failoverLoadBalancer := NewLoadBalancer(cfg.SystemService, cfg.ChannelService,
		NewWeightStrategy(), NewRandomStrategy(), rateLimitStrategy)

	circuitBreakerLoadBalancer := NewLoadBalancer(cfg.SystemService, cfg.ChannelService,
		NewWeightStrategy(), NewModelAwareCircuitBreakerStrategy(modelCircuitBreaker), rateLimitStrategy)

	return &ChatCompletionOrchestrator{
		Inbound:            cfg.Inbound,
		RequestService:     cfg.RequestService,
		ChannelService:     cfg.ChannelService,
		SystemService:      cfg.SystemService,
		UsageLogService:    cfg.UsageLogService,
		QuotaService:       cfg.QuotaService,
		LiveStreamRegistry: cfg.LiveStreamRegistry,
		PromptProvider:     cfg.PromptService,
		PromptProtecter:    cfg.PromptProtectionRuleService,
		Middlewares: []pipeline.Middleware{
			cc.StripBillingHeaderCCH(),
			stream.EnsureUsage(),
		},
		PipelineFactory:            pipeline.NewFactory(cfg.HttpClient),
		ModelMapper:                NewModelMapper(),
		channelSelector:            cfg.DefaultSelector,
		channelLimiterManager:      channelLimiterManager,
		channelLimiterMetrics:      channelLimiterMetrics,
		connectionTracker:          connectionTracker,
		rateLimitTracker:           rateLimitTracker,
		modelConnectionTracker:     modelConnectionTracker,
		costTracker:                costTracker,
		adaptiveLoadBalancer:       adaptiveLoadBalancer,
		failoverLoadBalancer:       failoverLoadBalancer,
		circuitBreakerLoadBalancer: circuitBreakerLoadBalancer,
		modelCircuitBreaker:        modelCircuitBreaker,
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
	channelLimiterManager *ChannelLimiterManager
	channelLimiterMetrics *ChannelLimiterMetrics
	connectionTracker      ConnectionTracker
	rateLimitTracker       *ChannelRequestTracker
	// The model connection tracker for per-model connection tracking.
	modelConnectionTracker *ModelConnectionTracker
	// costTracker caches channel spend for cost-based rate-limit scoring.
	costTracker *ChannelCostTracker
	// The model circuit breaker for circuit-breaker load balancing.
	modelCircuitBreaker *biz.ModelCircuitBreaker

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
		selectCandidates(inbound),
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
		withRateLimitTracking(outbound, processor.rateLimitTracker, time.Now),
		// Connection tracking middleware for load balancing.
		withConnectionTracking(outbound, processor.connectionTracker),
		// Model connection tracking middleware for per-model concurrency tracking.
		withModelConnectionTracking(outbound, processor.modelConnectionTracker),
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

// Close stops the orchestrator's background workers.
func (processor *ChatCompletionOrchestrator) Close() {
	if processor.costTracker != nil {
		processor.costTracker.Stop()
	}
	if processor.rateLimitTracker != nil {
		processor.rateLimitTracker.Stop()
	}
}
