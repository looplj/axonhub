package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/llm/decorator"
	"github.com/looplj/axonhub/internal/llm/decorator/stream"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
)

// NewChatCompletionProcessor creates a new ChatCompletionProcessor.
func NewChatCompletionProcessor(
	channelService *biz.ChannelService,
	requestService *biz.RequestService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
) *ChatCompletionProcessor {
	return NewChatCompletionProcessorWithSelector(
		NewDefaultChannelSelector(channelService),
		requestService,
		httpClient,
		inbound,
	)
}

func NewChatCompletionProcessorWithSelector(
	channelSelector ChannelSelector,
	requestService *biz.RequestService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		ChannelSelector: channelSelector,
		Inbound:         inbound,
		RequestService:  requestService,
		Decorators: []decorator.Decorator{
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
	Decorators      []decorator.Decorator
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

	pipe := processor.PipelineFactory.Pipeline(
		inbound,
		outbound,
		pipeline.WithRetry(3, 0),
		pipeline.WithDecorators(processor.Decorators...),
	)

	result, err := pipe.Process(ctx, request)
	if err != nil {
		log.Error(ctx, "Pipeline processing failed", log.Cause(err))

		// Update request status to failed when all retries are exhausted
		if outbound != nil {
			persistCtx := context.WithoutCancel(ctx)

			// Update the last request execution status based on error if it exists
			// This ensures that when retry fails completely, the last execution is properly marked
			if outbound.GetRequestExecution() != nil {
				if execUpdateErr := processor.RequestService.UpdateRequestExecutionStatusFromError(
					persistCtx,
					outbound.GetRequestExecution().ID,
					err,
				); execUpdateErr != nil {
					log.Warn(persistCtx, "Failed to update request execution status from error", log.Cause(execUpdateErr))
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
