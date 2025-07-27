package biz

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/contexts"
	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/pkg/streams"
)

// NewChatCompletionProcessor creates a new ChatCompletionProcessor
func NewChatCompletionProcessor(
	channelService *ChannelService,
	requestService *RequestService,
	httpClient httpclient.HttpClient,
	inbound transformer.Inbound,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		ChannelService: channelService,
		Inbound:        inbound,
		RequestService: requestService,
		HttpClient:     httpClient,
	}
}

type ChatCompletionProcessor struct {
	ChannelService *ChannelService
	Inbound        transformer.Inbound
	RequestService *RequestService
	HttpClient     httpclient.HttpClient
}

type ChatCompletionResult struct {
	ChatCompletion       *llm.GenericHttpResponse
	ChatCompletionStream streams.Stream[*llm.GenericStreamEvent]
}

func (processor *ChatCompletionProcessor) Process(ctx context.Context, genericReq *llm.GenericHttpRequest) (ChatCompletionResult, error) {
	chatReq, err := processor.Inbound.TransformRequest(ctx, genericReq)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	log.Debug(ctx, "receive chat request", log.Any("request", chatReq))

	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok {
		log.Warn(ctx, "api key not found")
	}

	req, err := processor.RequestService.CreateRequest(ctx, apiKey, chatReq, genericReq.Body)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	// TODO - Apply decorators (rate limiting, authentication, etc.)

	channels, err := processor.ChannelService.ChooseChannels(ctx, chatReq)
	if err != nil {
		return ChatCompletionResult{}, err
	}
	log.Debug(ctx, "choose channels", log.Any("channels", channels), log.Any("model", chatReq.Model))
	if len(channels) == 0 {
		return ChatCompletionResult{}, errors.New("no provider available")
	}

	for _, channel := range channels {
		log.Debug(ctx, "using channel", log.Any("channel", channel.Name), log.Any("model", chatReq.Model))

		// Handle streaming vs non-streaming responses
		if chatReq.Stream != nil && *chatReq.Stream {
			stream, err := processor.handleStreamingResponse(ctx, channel, chatReq, req)
			if err != nil {
				log.Warn(ctx, "Provider streaming failed", log.Cause(err))
				continue
			}
			return ChatCompletionResult{
				ChatCompletion:       nil,
				ChatCompletionStream: stream,
			}, nil
		}

		resp, err := processor.handleNonStreamingResponse(ctx, channel, chatReq, req)
		if err != nil {
			log.Warn(ctx, "Provider non-streaming failed", log.Cause(err))
			continue
		}
		return ChatCompletionResult{
			ChatCompletion:       resp,
			ChatCompletionStream: nil,
		}, nil
	}
	err = processor.RequestService.UpdateRequestFailed(ctx, req.ID)
	if err != nil {
		log.Warn(ctx, "Failed to update request status to failed", log.Cause(err))
	}
	return ChatCompletionResult{}, errors.New("Failed to reqeust provider")
}

func (processor *ChatCompletionProcessor) handleNonStreamingResponse(ctx context.Context, channel *Channel, chatReq *llm.ChatCompletionRequest, req *ent.Request) (*llm.GenericHttpResponse, error) {
	httpReq, err := channel.Outbound.TransformRequest(ctx, chatReq)
	if err != nil {
		log.Error(ctx, "Failed to transform request", log.Cause(err))
		return nil, err
	}

	requestExec, err := processor.RequestService.CreateRequestExecution(ctx, channel, req, httpReq.Body)
	if err != nil {
		return nil, err
	}

	// Execute HTTP request
	httpResp, err := processor.HttpClient.Do(ctx, httpReq)
	if err != nil {
		log.Error(ctx, "HTTP request failed", log.Cause(err))
		innerErr := processor.RequestService.UpdateRequestExecutionFailed(ctx, requestExec.ID, err.Error())
		if innerErr != nil {
			log.Warn(ctx, "Failed to update request execution status", log.Cause(innerErr))
		}
		return nil, err
	}

	err = processor.RequestService.UpdateRequestExecutionCompleted(ctx, requestExec.ID, httpResp.Body)
	if err != nil {
		log.Warn(ctx, "Failed to update request execution status to completed", log.Cause(err))
	}

	// Transform HTTP response to ChatCompletionResponse
	chatResp, err := channel.Outbound.TransformResponse(ctx, httpResp)
	if err != nil {
		log.Error(ctx, "Failed to transform response", log.Cause(err))
		innerErr := processor.RequestService.UpdateRequestExecutionFailed(ctx, requestExec.ID, err.Error())
		if innerErr != nil {
			log.Warn(ctx, "Failed to update request execution status", log.Cause(innerErr))
		}
		return nil, innerErr
	}

	log.Debug(ctx, "Chat completion response", log.Any("response", chatResp))

	transformedResp, err := processor.Inbound.TransformResponse(ctx, chatResp)
	if err != nil {
		innerErr := processor.RequestService.UpdateRequestExecutionFailed(ctx, requestExec.ID, err.Error())
		if innerErr != nil {
			log.Warn(ctx, "Failed to update request execution status to completed", log.Cause(innerErr))
		}
		return nil, err
	}

	err = processor.RequestService.UpdateRequestCompleted(ctx, req.ID, transformedResp.Body)
	if err != nil {
		log.Warn(ctx, "Failed to update request status to completed", log.Cause(err))
	}
	return transformedResp, nil
}

func (processor *ChatCompletionProcessor) handleStreamingResponse(ctx context.Context, channel *Channel, chatReq *llm.ChatCompletionRequest, req *ent.Request) (streams.Stream[*llm.GenericStreamEvent], error) {
	// Transform ChatCompletionRequest to HTTP request
	httpReq, err := channel.Outbound.TransformRequest(ctx, chatReq)
	if err != nil {
		log.Error(ctx, "Failed to transform streaming request", log.Cause(err))
		return nil, err
	}

	requestExec, err := processor.RequestService.CreateRequestExecution(ctx, channel, req, httpReq.Body)
	if err != nil {
		return nil, err
	}

	// Execute streaming HTTP request
	stream, err := processor.HttpClient.DoStream(ctx, httpReq)
	if err != nil {
		log.Error(ctx, "HTTP streaming request failed", log.Cause(err))
		innerErr := processor.RequestService.UpdateRequestExecutionFailed(ctx, requestExec.ID, err.Error())
		if innerErr != nil {
			log.Warn(ctx, "Failed to update request execution status", log.Cause(innerErr))
		}
		return nil, err
	}

	stream = NewTrackedStream(ctx, stream, req, requestExec, processor.RequestService, channel.Outbound)

	// Transform the stream: HTTP responses -> ChatCompletionResponse -> final HTTP responses
	transformedStream := streams.MapErr(stream, func(httpResp *llm.GenericHttpResponse) (*llm.GenericStreamEvent, error) {
		chatResp, err := channel.Outbound.TransformStreamChunk(ctx, httpResp)
		if err != nil {
			return nil, err
		}
		log.Debug(ctx, "chat stream response", log.Any("response", chatResp))
		streamEvent, err := processor.Inbound.TransformStreamChunk(ctx, chatResp)
		if err != nil {
			return nil, err
		}
		log.Debug(ctx, "transformed stream event", log.Any("event", streamEvent))

		return streamEvent, nil
	})
	return transformedStream, nil
}
