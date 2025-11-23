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

// NewChatCompletionProcessor creates a new ChatCompletionProcessor.
func NewChatCompletionProcessor(
	channelService *biz.ChannelService,
	requestService *biz.RequestService,
	traceService *biz.TraceService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
	systemService *biz.SystemService,
	usageLogService *biz.UsageLogService,
) *ChatCompletionProcessor {
	connectionTracker := NewDefaultConnectionTracker(1024)

	return NewChatCompletionProcessorWithSelector(
		NewDefaultChannelSelector(channelService, systemService, traceService, connectionTracker),
		requestService,
		channelService,
		httpClient,
		inbound,
		systemService,
		usageLogService,
		connectionTracker,
	)
}

func NewChatCompletionProcessorWithSelector(
	channelSelector ChannelSelector,
	requestService *biz.RequestService,
	channelService *biz.ChannelService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
	systemService *biz.SystemService,
	usageLogService *biz.UsageLogService,
	connectionTracker *DefaultConnectionTracker,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		ChannelSelector: channelSelector,
		Inbound:         inbound,
		RequestService:  requestService,
		ChannelService:  channelService,
		SystemService:   systemService,
		UsageLogService: usageLogService,
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
		ModelMapper:       NewModelMapper(),
		PipelineFactory:   pipeline.NewFactory(httpClient),
		ConnectionTracker: connectionTracker,
	}
}

type ChatCompletionProcessor struct {
	ChannelSelector   ChannelSelector
	Inbound           transformer.Inbound
	RequestService    *biz.RequestService
	ChannelService    *biz.ChannelService
	SystemService     *biz.SystemService
	UsageLogService   *biz.UsageLogService
	Middlewares       []pipeline.Middleware
	PipelineFactory   *pipeline.Factory
	ModelMapper       *ModelMapper
	ConnectionTracker *DefaultConnectionTracker

	// Proxy is the proxy configuration for testing
	// If set, it will override the channel's default proxy configuration
	Proxy *objects.ProxyConfig
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
		ChannelSelector: processor.ChannelSelector,
		ChannelIndex:    0,
		ModelMapper:     processor.ModelMapper,
		Proxy:           processor.Proxy,
	}

	inbound, outbound := NewPersistentTransformers(state, processor.Inbound)

	// Get retry policy from system settings
	retryPolicy := processor.SystemService.RetryPolicyOrDefault(ctx)

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

	// Add inbound middlewares (executed after inbound.TransformRequest)
	middlewares = append(middlewares,
		applyApiKeyModelMapping(inbound),
		selectChannels(inbound),
		createRequest(inbound),
	)

	// Add outbound middlewares (executed after outbound.TransformRequest)
	middlewares = append(middlewares,
		applyOverrideParameters(outbound),

		// The request execution middleware must be the final middleware
		// to ensure that the request execution is created with the correct request bodys.
		createRequestExecution(outbound),

		// Unified performance tracking middleware.
		withPerformanceRecording(outbound),

		// Connection tracking middleware for load balancing.
		withConnectionTracking(outbound, processor.ConnectionTracker),
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
			if outbound.GetRequestExecution() != nil {
				if updateErr := processor.RequestService.UpdateRequestExecutionStatusFromError(
					persistCtx,
					outbound.GetRequestExecution().ID,
					err,
				); updateErr != nil {
					log.Warn(persistCtx, "Failed to update request execution status from error", log.Cause(updateErr))
				}
			}

			// Update the main request status based on error
			if outbound.GetRequest() != nil {
				if updateErr := processor.RequestService.UpdateRequestStatusFromError(
					persistCtx,
					outbound.GetRequest().ID,
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
