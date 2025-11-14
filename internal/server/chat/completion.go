package chat

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/pipeline/stream"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/server/biz"
)

// NewChatCompletionProcessor creates a new ChatCompletionProcessor.
func NewChatCompletionProcessor(
	channelService *biz.ChannelService,
	requestService *biz.RequestService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
	systemService *biz.SystemService,
) *ChatCompletionProcessor {
	return NewChatCompletionProcessorWithSelector(
		NewDefaultChannelSelector(channelService),
		requestService,
		httpClient,
		inbound,
		systemService,
	)
}

func NewChatCompletionProcessorWithSelector(
	channelSelector ChannelSelector,
	requestService *biz.RequestService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
	systemService *biz.SystemService,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		ChannelSelector: channelSelector,
		Inbound:         inbound,
		RequestService:  requestService,
		SystemService:   systemService,
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
		ModelMapper:     NewModelMapper(),
		PipelineFactory: pipeline.NewFactory(httpClient),
	}
}

type ChatCompletionProcessor struct {
	ChannelSelector ChannelSelector
	Inbound         transformer.Inbound
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	Middlewares     []pipeline.Middleware
	PipelineFactory *pipeline.Factory
	ModelMapper     *ModelMapper
}

type ChatCompletionResult struct {
	ChatCompletion       *httpclient.Response
	ChatCompletionStream streams.Stream[*httpclient.StreamEvent]
}

func (processor *ChatCompletionProcessor) Process(ctx context.Context, request *httpclient.Request) (ChatCompletionResult, error) {
	apiKey, _ := contexts.GetAPIKey(ctx)
	user, _ := contexts.GetUser(ctx)

	log.Debug(ctx, "request received", log.String("request_body", string(request.Body)))

	inbound, outbound := NewPersistentTransformersWithSelector(
		ctx,
		processor.Inbound,
		processor.RequestService,
		apiKey,
		user,
		request,
		processor.ModelMapper,
		processor.ChannelSelector,
	)

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
