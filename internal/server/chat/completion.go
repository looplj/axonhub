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
	httpClient httpclient.HttpClient,
	inbound transformer.Inbound,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		ChannelService:  channelService,
		Inbound:         inbound,
		RequestService:  requestService,
		HttpClient:      httpClient,
		PipelineFactory: pipeline.NewFactory(httpClient),
	}
}

type ChatCompletionProcessor struct {
	ChannelService  *biz.ChannelService
	Inbound         transformer.Inbound
	RequestService  *biz.RequestService
	HttpClient      httpclient.HttpClient
	DecoratorChain  decorator.DecoratorChain
	PipelineFactory *pipeline.Factory
}

type ChatCompletionResult struct {
	ChatCompletion       *httpclient.Response
	ChatCompletionStream streams.Stream[*httpclient.StreamEvent]
}

func (processor *ChatCompletionProcessor) Process(
	ctx context.Context,
	request *httpclient.Request,
) (ChatCompletionResult, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok {
		log.Warn(ctx, "api key not found")
	}

	// Create enhanced persistent transformers with channel management and request creation
	// This now handles the inbound transformation internally
	inbound, outbound, err := NewPersistentTransformers(
		ctx,
		processor.Inbound,
		processor.ChannelService,
		processor.RequestService,
		apiKey,
		request,
		request.Body,
	)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	pipeline := processor.PipelineFactory.Pipeline(
		inbound,
		outbound,
		pipeline.WithRetry(
			3,
			0,
			"connection timeout",
			"rate limit exceeded",
			"temporary unavailable",
		),
	)

	result, err := pipeline.Process(ctx, request)
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
