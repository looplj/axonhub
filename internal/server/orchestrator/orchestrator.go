package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

func NewChatCompletionOrchestrator(
	channelService *biz.ChannelService,
	modelService *biz.ModelService,
	requestService *biz.RequestService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
	systemService *biz.SystemService,
	usageLogService *biz.UsageLogService,
	promptService *biz.PromptService,
	quotaService *biz.QuotaService,
) *ChatCompletionOrchestrator {
	connectionTracker := NewDefaultConnectionTracker(256)

	// Initialize model circuit breaker
	modelCircuitBreaker := biz.NewModelCircuitBreaker()

	adaptiveLoadBalancer := NewLoadBalancer(systemService, channelService,
		NewTraceAwareStrategy(requestService),
		NewErrorAwareStrategy(channelService),
		NewWeightRoundRobinStrategy(channelService),
		NewConnectionAwareStrategy(channelService, connectionTracker),
	)

	weightedLoadBalancer := NewLoadBalancer(systemService, channelService,
		NewWeightStrategy(), NewRandomStrategy())

	circuitBreakerLoadBalancer := NewLoadBalancer(systemService, channelService,
		NewWeightStrategy(), NewModelAwareCircuitBreakerStrategy(modelCircuitBreaker))

	return &ChatCompletionOrchestrator{
		Inbound:         inbound,
		RequestService:  requestService,
		ChannelService:  channelService,
		SystemService:   systemService,
		UsageLogService: usageLogService,
		QuotaService:    quotaService,
		PromptProvider:  promptService,
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
		PipelineFactory:            pipeline.NewFactory(httpClient),
		ModelMapper:                NewModelMapper(),
		channelSelector:            NewDefaultSelector(channelService, modelService, systemService),
		selectedChannelIds:         []int{},
		connectionTracker:          connectionTracker,
		adaptiveLoadBalancer:       adaptiveLoadBalancer,
		failoverLoadBalancer:       weightedLoadBalancer,
		circuitBreakerLoadBalancer: circuitBreakerLoadBalancer,
		modelCircuitBreaker:        modelCircuitBreaker,
		proxy:                      nil,
	}
}

type ChatCompletionOrchestrator struct {
	Inbound         transformer.Inbound
	RequestService  *biz.RequestService
	ChannelService  *biz.ChannelService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	QuotaService    *biz.QuotaService
	PromptProvider  PromptProvider
	Middlewares     []pipeline.Middleware
	PipelineFactory *pipeline.Factory
	ModelMapper     *ModelMapper

	// The runtime fields.

	// The default channel selector.
	channelSelector CandidateSelector
	// The runtime selected channel ids.
	selectedChannelIds []int
	// The load balancer for channel load balancing.
	adaptiveLoadBalancer       *LoadBalancer
	failoverLoadBalancer       *LoadBalancer
	circuitBreakerLoadBalancer *LoadBalancer
	// The connection tracker for connection aware load balancing.
	connectionTracker ConnectionTracker
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
	apiKey, _ := contexts.GetAPIKey(ctx)

	// Get retry policy from system settings
	retryPolicy := processor.SystemService.RetryPolicyOrDefault(ctx)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "chat request received",
			log.String("request_body", string(request.Body)),
			log.Any("request_headers", request.Headers),
			log.Any("retry_policy", retryPolicy),
		)
	}

	loadBalancer := processor.adaptiveLoadBalancer

	switch retryPolicy.LoadBalancerStrategy {
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
	}

	var middlewares []pipeline.Middleware

	// Add global middlewares
	middlewares = append(middlewares, processor.Middlewares...)

	inbound, outbound := NewPersistentTransformers(state, processor.Inbound)

	// Add inbound middlewares (executed after inbound.TransformRequest)
	middlewares = append(middlewares,
		enforceQuota(inbound, processor.QuotaService),
		checkApiKeyModelAccess(inbound),
		applyModelMapping(inbound),
		selectCandidates(inbound),
		injectPrompts(inbound),
		persistRequest(inbound),
	)

	// Add outbound middlewares (executed after outbound.TransformRequest)
	middlewares = append(middlewares,
		applyOverrideRequestBody(outbound),
		applyOverrideRequestHeaders(outbound),

		// Unified performance tracking middleware.
		withPerformanceRecording(outbound),

		// The request execution middleware must be the final middleware
		// to ensure that the request execution is created with the correct request bodys.
		persistRequestExecution(outbound),

		// Connection tracking middleware for load balancing.
		withConnectionTracking(outbound, processor.connectionTracker),
	)

	if retryPolicy.LoadBalancerStrategy == biz.LoadBalancerStrategyCircuitBreaker {
		// Model circuit breaker middleware for circuit-breaker load balancing.
		middlewares = append(middlewares, withModelCircuitBreaker(outbound, processor.modelCircuitBreaker))
	}

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
