package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/llm/decorator"
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
	return &ChatCompletionProcessor{
		ChannelService:  channelService,
		Inbound:         inbound,
		RequestService:  requestService,
		PipelineFactory: pipeline.NewFactory(httpClient),
	}
}

type ChatCompletionProcessor struct {
	ChannelService  *biz.ChannelService
	Inbound         transformer.Inbound
	RequestService  *biz.RequestService
	DecoratorChain  decorator.DecoratorChain
	PipelineFactory *pipeline.Factory
}

type ChatCompletionResult struct {
	ChatCompletion       *httpclient.Response
	ChatCompletionStream streams.Stream[*httpclient.StreamEvent]
}

func (processor *ChatCompletionProcessor) Process(ctx context.Context, request *httpclient.Request) (ChatCompletionResult, error) {
	apiKey, _ := contexts.GetAPIKey(ctx)
	user, _ := contexts.GetUser(ctx)

	log.Debug(ctx, "request received", log.String("request_body", string(request.Body)))

	inbound, outbound := NewPersistentTransformers(
		ctx,
		processor.Inbound,
		processor.ChannelService,
		processor.RequestService,
		apiKey,
		user,
		request,
	)

	pipe := processor.PipelineFactory.Pipeline(
		inbound,
		outbound,
		pipeline.WithRetry(3, 0),
	)

	result, err := pipe.Process(ctx, request)
	if err != nil {
		log.Error(ctx, "Pipeline processing failed", log.Cause(err))
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
