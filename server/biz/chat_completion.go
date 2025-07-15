package biz

import (
	"context"
	"errors"
	"net/http"

	"github.com/looplj/axonhub/llm/provider"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/types"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/pkg/streams"
)

// NewChatCompletionProcessor creates a new ChatCompletionProcessor
func NewChatCompletionProcessor(
	channelService *ChannelService,
	transformer transformer.Transformer,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		ChannelService: channelService,
		Transformer:    transformer,
	}
}

type ChatCompletionProcessor struct {
	ChannelService *ChannelService
	Transformer    transformer.Transformer
}

type ChatCompletionResult struct {
	ChatCompletion       *types.GenericHttpResponse
	ChatCompletionStream streams.Stream[*types.GenericHttpResponse]
}

func (processor *ChatCompletionProcessor) Process(ctx context.Context, rawRequest *http.Request) (ChatCompletionResult, error) {
	chatReq, err := processor.Transformer.TransformRequest(ctx, rawRequest)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	log.Debug(ctx, "receive chat request", log.Any("request", chatReq))

	// TODO - Apply decorators (rate limiting, authentication, etc.)

	channels, err := processor.ChannelService.ChooseChannels(ctx, chatReq)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	for _, channel := range channels {
		log.Info(ctx, "Using channel", log.Any("channel", channel.Name), log.Any("model", chatReq.Model))

		prov, err := processor.ChannelService.GetProvider(ctx, channel.Name)
		if err != nil {
			return ChatCompletionResult{}, err
		}

		// Step 4: Handle streaming vs non-streaming responses
		if chatReq.Stream != nil && *chatReq.Stream {
			stream, err := processor.handleStreamingResponse(ctx, prov, chatReq)
			if err != nil {
				log.Warn(ctx, "Provider streaming failed", log.Cause(err))
				continue
			}
			return ChatCompletionResult{
				ChatCompletion:       nil,
				ChatCompletionStream: stream,
			}, nil
		}

		resp, err := processor.handleNonStreamingResponse(ctx, prov, chatReq)
		if err != nil {
			log.Warn(ctx, "Provider non-streaming failed", log.Cause(err))
			continue
		}
		return ChatCompletionResult{
			ChatCompletion:       resp,
			ChatCompletionStream: nil,
		}, nil
	}
	return ChatCompletionResult{}, errors.New("no provider available")
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
