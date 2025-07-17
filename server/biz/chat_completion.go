package biz

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/contexts"
	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/objects"
	"github.com/looplj/axonhub/pkg/streams"
)

// NewChatCompletionProcessor creates a new ChatCompletionProcessor
func NewChatCompletionProcessor(
	channelService *ChannelService,
	requestService *RequestService,
	httpClient httpclient.HttpClient,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		ChannelService:     channelService,
		InboundTransformer: openai.NewInboundTransformer(),
		RequestService:     requestService,
		HttpClient:         httpClient,
	}
}

type ChatCompletionProcessor struct {
	ChannelService     *ChannelService
	InboundTransformer transformer.Inbound
	RequestService     *RequestService
	HttpClient         httpclient.HttpClient
}

type ChatCompletionResult struct {
	ChatCompletion       *llm.GenericHttpResponse
	ChatCompletionStream streams.Stream[*llm.GenericHttpResponse]
}

// TrackedStream wraps a stream and tracks all responses for final saving
type TrackedStream struct {
	ctx                 context.Context
	stream              streams.Stream[*llm.GenericHttpResponse]
	request             *ent.Request
	requestExec         *ent.RequestExecution
	requestService      *RequestService
	outboundTransformer transformer.Outbound
	responseChunks      []objects.JSONRawMessage
	closed              bool
}

// Ensure TrackedStream implements Stream interface
var _ streams.Stream[*llm.GenericHttpResponse] = (*TrackedStream)(nil)

func NewTrackedStream(
	ctx context.Context,
	stream streams.Stream[*llm.GenericHttpResponse],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *RequestService,
	outboundTransformer transformer.Outbound,
) *TrackedStream {
	return &TrackedStream{
		ctx:                 ctx,
		stream:              stream,
		request:             request,
		requestExec:         requestExec,
		requestService:      requestService,
		outboundTransformer: outboundTransformer,
	}
}

func (ts *TrackedStream) Next() bool {
	return ts.stream.Next()
}

func (ts *TrackedStream) Current() *llm.GenericHttpResponse {
	resp := ts.stream.Current()
	if resp != nil && resp.Body != nil {
		// Save each chunk to response_chunks field
		chunk := objects.JSONRawMessage(resp.Body)
		ts.responseChunks = append(ts.responseChunks, chunk)

		// Add options to control if save chunk to database
		err := ts.requestService.AppendRequestExecutionChunk(ts.ctx, ts.requestExec.ID, chunk)
		if err != nil {
			log.Warn(ts.ctx, "Failed to save response chunk", log.Cause(err))
		}
	}
	return resp
}

func (ts *TrackedStream) Err() error {
	return ts.stream.Err()
}

func (ts *TrackedStream) Close() error {
	if ts.closed {
		return nil
	}
	ts.closed = true

	// Save final response body and update status
	ctx := ts.ctx

	// Update request execution
	if ts.stream.Err() != nil {
		err := ts.requestService.UpdateRequestExecutionFailed(ctx, ts.requestExec.ID, ts.stream.Err().Error())
		if err != nil {
			log.Warn(ctx, "Failed to update request execution status", log.Cause(err))
		}
		err = ts.requestService.UpdateRequestFailed(ctx, ts.request.ID)
		if err != nil {
			log.Warn(ctx, "Failed to update request status", log.Cause(err))
		}
	} else {
		// Use the new method to aggregate chunks and update status
		err := ts.requestService.UpdateRequestExecutionCompletedWithChunks(ctx, ts.requestExec.ID, ts.responseChunks, ts.outboundTransformer)
		if err != nil {
			log.Warn(ctx, "Failed to update request execution with aggregated chunks", log.Cause(err))
		}

		// For the main request, we need to get the aggregated response using transformer
		aggregatedResponse, err := ts.requestService.AggregateChunksToResponseWithTransformer(ctx, ts.responseChunks, ts.outboundTransformer)
		if err != nil {
			log.Warn(ctx, "Failed to aggregate chunks for request update", log.Cause(err))
			aggregatedResponse = objects.JSONRawMessage("{}")
		}
		err = ts.requestService.UpdateRequestCompleted(ctx, ts.request.ID, aggregatedResponse)
		if err != nil {
			log.Warn(ctx, "Failed to update request status to completed", log.Cause(err))
		}
	}

	return ts.stream.Close()
}

func (processor *ChatCompletionProcessor) Process(ctx context.Context, genericReq *llm.GenericHttpRequest) (ChatCompletionResult, error) {
	chatReq, err := processor.InboundTransformer.TransformRequest(ctx, genericReq)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	log.Debug(ctx, "receive chat request", log.Any("request", chatReq))

	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil {
		return ChatCompletionResult{}, errors.New("API key not found in context")
	}

	req, err := processor.RequestService.CreateRequest(ctx, apiKey, chatReq)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	// TODO - Apply decorators (rate limiting, authentication, etc.)

	channels, err := processor.ChannelService.ChooseChannels(ctx, chatReq)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	for _, channel := range channels {
		log.Info(ctx, "Using channel", log.Any("channel", channel.Name), log.Any("model", chatReq.Model))

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
	return ChatCompletionResult{}, errors.New("no provider available")
}

func (processor *ChatCompletionProcessor) handleNonStreamingResponse(ctx context.Context, channel *Channel, chatReq *llm.ChatCompletionRequest, req *ent.Request) (*llm.GenericHttpResponse, error) {
	// Transform ChatCompletionRequest to HTTP request
	httpReq, err := channel.Transformer.TransformRequest(ctx, chatReq)
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

	// Transform HTTP response to ChatCompletionResponse
	chatResp, err := channel.Transformer.TransformResponse(ctx, httpResp)
	if err != nil {
		log.Error(ctx, "Failed to transform response", log.Cause(err))
		innerErr := processor.RequestService.UpdateRequestExecutionFailed(ctx, requestExec.ID, err.Error())
		if innerErr != nil {
			log.Warn(ctx, "Failed to update request execution status", log.Cause(innerErr))
		}
		return nil, innerErr
	}

	log.Debug(ctx, "Chat completion response", log.Any("response", chatResp))

	transformedResp, err := processor.InboundTransformer.TransformResponse(ctx, chatResp)
	if err != nil {
		innerErr := processor.RequestService.UpdateRequestExecutionFailed(ctx, requestExec.ID, err.Error())
		if innerErr != nil {
			log.Warn(ctx, "Failed to update request execution status to completed", log.Cause(innerErr))
		}
		return nil, err
	}

	err = processor.RequestService.UpdateRequestExecutionCompleted(ctx, requestExec.ID, chatReq)
	if err != nil {
		log.Warn(ctx, "Failed to update request execution status to completed", log.Cause(err))
	}
	err = processor.RequestService.UpdateRequestCompleted(ctx, req.ID, transformedResp.Body)
	if err != nil {
		log.Warn(ctx, "Failed to update request status to completed", log.Cause(err))
	}
	return transformedResp, nil
}

func (processor *ChatCompletionProcessor) handleStreamingResponse(ctx context.Context, channel *Channel, chatReq *llm.ChatCompletionRequest, req *ent.Request) (streams.Stream[*llm.GenericHttpResponse], error) {
	// Transform ChatCompletionRequest to HTTP request
	httpReq, err := channel.Transformer.TransformRequest(ctx, chatReq)
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

	// Transform the stream: HTTP responses -> ChatCompletionResponse -> final HTTP responses
	transformedStream := streams.MapErr(stream, func(httpResp *llm.GenericHttpResponse) (*llm.GenericHttpResponse, error) {
		// Transform HTTP response to ChatCompletionResponse
		chatResp, err := channel.Transformer.TransformResponse(ctx, httpResp)
		if err != nil {
			return nil, err
		}

		// Transform ChatCompletionResponse to final HTTP response using inbound transformer
		return processor.InboundTransformer.TransformResponse(ctx, chatResp)
	})

	// Wrap with tracked stream to save final response
	trackedStream := NewTrackedStream(ctx, transformedStream, req, requestExec, processor.RequestService, channel.Transformer)
	return trackedStream, nil
}
