package biz

import (
	"context"
	"net/http"

	"github.com/looplj/axonhub/llm/provider"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/types"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/pkg/streams"
)

type ChatCompletionProcessor struct {
	Transformer      transformer.Transformer
	ProviderRegistry provider.ProviderRegistry
}

// NewChatCompletionProcessor creates a new ChatCompletionProcessor
func NewChatCompletionProcessor(
	inboundTransformer transformer.Transformer,
	providerRegistry provider.ProviderRegistry,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		Transformer:      inboundTransformer,
		ProviderRegistry: providerRegistry,
	}
}

type ChatCompletionResult struct {
	ChatCompletion       *types.GenericHttpResponse
	ChatCompletionStream streams.Stream[*types.GenericHttpResponse]
}

func (processor *ChatCompletionProcessor) Process(ctx context.Context, rawRequest *http.Request) (ChatCompletionResult, error) {
	// Step 1: Inbound transformation - Convert HTTP request to ChatCompletionRequest
	chatReq, err := processor.Transformer.TransformRequest(ctx, rawRequest)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	log.Debug(ctx, "receive chat request", log.Any("request", chatReq))

	// Step 2: TODO - Apply decorators (rate limiting, authentication, etc.)
	// This would be where decorator chain processing happens

	// Step 3: Get provider for the model
	prov, err := processor.ProviderRegistry.GetProviderForModel(chatReq.Model)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	log.Info(ctx, "Using provider", log.Any("provider", prov.GetConfig().Name), log.Any("model", chatReq.Model))

	// Step 4: Handle streaming vs non-streaming responses
	if chatReq.Stream != nil && *chatReq.Stream {
		stream, err := processor.handleStreamingResponse(ctx, prov, chatReq)
		if err != nil {
			return ChatCompletionResult{}, err
		}
		return ChatCompletionResult{
			ChatCompletion:       nil,
			ChatCompletionStream: stream,
		}, nil
	}

	resp, err := processor.handleNonStreamingResponse(ctx, prov, chatReq)
	if err != nil {
		return ChatCompletionResult{}, err
	}
	return ChatCompletionResult{
		ChatCompletion:       resp,
		ChatCompletionStream: nil,
	}, nil
}

func (processor *ChatCompletionProcessor) handleNonStreamingResponse(ctx context.Context, prov provider.Provider, chatReq *types.ChatCompletionRequest) (*types.GenericHttpResponse, error) {
	chatResp, err := prov.ChatCompletion(ctx, chatReq)
	if err != nil {
		log.Error(ctx, "Provider chat completion failed", log.Cause(err))
		return nil, err
	}

	log.Info(ctx, "Chat completion response", log.Any("response", chatResp))

	return processor.Transformer.TransformResponse(ctx, chatResp)
}

func (processor *ChatCompletionProcessor) handleStreamingResponse(ctx context.Context, prov provider.Provider, chatReq *types.ChatCompletionRequest) (streams.Stream[*types.GenericHttpResponse], error) {
	stream, err := prov.ChatCompletionStream(ctx, chatReq)
	if err != nil {
		log.Error(ctx, "Provider streaming failed", log.Cause(err))
		return nil, err
	}

	return streams.MapErr(stream, func(resp *types.ChatCompletionResponse) (*types.GenericHttpResponse, error) {
		return processor.Transformer.TransformResponse(ctx, resp)
	}), nil
}
