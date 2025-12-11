package chat

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/pipeline/stream"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/server/biz"
)

func NewChatCompletionProcessor(
	channelService *biz.ChannelService,
	requestService *biz.RequestService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
	systemService *biz.SystemService,
	usageLogService *biz.UsageLogService,
) *ChatCompletionProcessor {
	connectionTracker := NewDefaultConnectionTracker(256)

	// Build strategies
	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(requestService),                         // Priority 1: Last successful channel from trace
		NewErrorAwareStrategy(channelService),                         // Priority 2: Health and error rate
		NewWeightRoundRobinStrategy(channelService),                   // Priority 3: Weight round robin
		NewConnectionAwareStrategy(channelService, connectionTracker), // Priority 4: Connection count
	}

	loadBalancer := NewLoadBalancer(systemService, strategies...)

	return &ChatCompletionProcessor{
		Inbound:         inbound,
		RequestService:  requestService,
		ChannelService:  channelService,
		SystemService:   systemService,
		UsageLogService: usageLogService,
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
		PipelineFactory:    pipeline.NewFactory(httpClient),
		ModelMapper:        NewModelMapper(),
		channelSelector:    NewDefaultSelector(channelService),
		selectedChannelIds: []int{},
		connectionTracker:  connectionTracker,
		loadBalancer:       loadBalancer,
		proxy:              nil,
	}
}

type ChatCompletionProcessor struct {
	Inbound         transformer.Inbound
	RequestService  *biz.RequestService
	ChannelService  *biz.ChannelService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	Middlewares     []pipeline.Middleware
	PipelineFactory *pipeline.Factory
	ModelMapper     *ModelMapper

	// The runtime fields.

	// The default channel selector.
	channelSelector ChannelSelector
	// The runtime selected channel ids.
	selectedChannelIds []int
	// The load balancer for channel load balancing.
	loadBalancer *LoadBalancer
	// The connection tracker for connection aware load balancing.
	connectionTracker ConnectionTracker

	// proxy is the proxy configuration for testing
	// If set, it will override the channel's default proxy configuration
	proxy *objects.ProxyConfig
}

func (processor *ChatCompletionProcessor) WithChannelSelector(selector ChannelSelector) *ChatCompletionProcessor {
	c := *processor
	c.channelSelector = selector

	return &c
}

func (processor *ChatCompletionProcessor) WithAllowedChannels(allowedChannelIDs []int) *ChatCompletionProcessor {
	c := *processor
	c.channelSelector = NewSelectedChannelsSelector(processor.channelSelector, allowedChannelIDs)

	return &c
}

func (processor *ChatCompletionProcessor) WithProxy(proxy *objects.ProxyConfig) *ChatCompletionProcessor {
	c := *processor
	c.proxy = proxy

	return &c
}

type ChatCompletionResult struct {
	ChatCompletion       *httpclient.Response
	ChatCompletionStream streams.Stream[*httpclient.StreamEvent]
}

func (processor *ChatCompletionProcessor) Process(ctx context.Context, request *httpclient.Request) (ChatCompletionResult, error) {
	apiKey, _ := contexts.GetAPIKey(ctx)
	user, _ := contexts.GetUser(ctx)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "request received", log.String("request_body", string(request.Body)))
	}

	state := &PersistenceState{
		APIKey:          apiKey,
		User:            user,
		RequestService:  processor.RequestService,
		UsageLogService: processor.UsageLogService,
		ChannelService:  processor.ChannelService,
		ChannelSelector: processor.channelSelector,
		LoadBalancer:    processor.loadBalancer,
		ModelMapper:     processor.ModelMapper,
		Proxy:           processor.proxy,
		ChannelIndex:    0,
	}

	var pipelineOpts []pipeline.Option

	// Get retry policy from system settings
	retryPolicy := processor.SystemService.RetryPolicyOrDefault(ctx)
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
		applyApiKeyModelMapping(inbound),
		selectChannels(inbound),
		persistRequest(inbound),
	)

	// Add outbound middlewares (executed after outbound.TransformRequest)
	middlewares = append(middlewares,
		applyOverrideRequestBody(outbound),
		applyOverrideRequestHeaders(outbound),

		// The request execution middleware must be the final middleware
		// to ensure that the request execution is created with the correct request bodys.
		persistRequestExecution(outbound),

		// Unified performance tracking middleware.
		withPerformanceRecording(outbound),

		// Connection tracking middleware for load balancing.
		withConnectionTracking(outbound, processor.connectionTracker),
	)

	pipelineOpts = append(pipelineOpts, pipeline.WithMiddlewares(middlewares...))

	pipe := processor.PipelineFactory.Pipeline(
		inbound,
		outbound,
		pipelineOpts...,
	)

	result, err := pipe.Process(ctx, request)
	if err != nil {
		// Update request status to failed when all retries are exhausted
		if outbound != nil {
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
