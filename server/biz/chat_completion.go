package biz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/contexts"
	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/provider"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/pkg/streams"
)

// NewChatCompletionProcessor creates a new ChatCompletionProcessor
func NewChatCompletionProcessor(
	channelService *ChannelService,
	requestService *RequestService,
) *ChatCompletionProcessor {
	return &ChatCompletionProcessor{
		ChannelService: channelService,
		Transformer:    openai.NewTransformer(), // Use OpenAI transformer directly
		RequestService: requestService,
	}
}

type ChatCompletionProcessor struct {
	ChannelService *ChannelService
	Transformer    transformer.Transformer
	RequestService *RequestService
}

type ChatCompletionResult struct {
	ChatCompletion       *llm.GenericHttpResponse
	ChatCompletionStream streams.Stream[*llm.GenericHttpResponse]
}

// TrackedStream wraps a stream and tracks all responses for final saving
type TrackedStream struct {
	stream          streams.Stream[*llm.GenericHttpResponse]
	request         *ent.Request
	requestExec     *ent.RequestExecution
	requestService  *RequestService
	responseBuilder strings.Builder
	closed          bool
}

// Ensure TrackedStream implements Stream interface
var _ streams.Stream[*llm.GenericHttpResponse] = (*TrackedStream)(nil)

func NewTrackedStream(
	stream streams.Stream[*llm.GenericHttpResponse],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *RequestService,
) *TrackedStream {
	return &TrackedStream{
		stream:         stream,
		request:        request,
		requestExec:    requestExec,
		requestService: requestService,
	}
}

func (ts *TrackedStream) Next() bool {
	return ts.stream.Next()
}

func (ts *TrackedStream) Current() *llm.GenericHttpResponse {
	resp := ts.stream.Current()
	if resp != nil && resp.Body != nil {
		// Accumulate response body for final saving
		ts.responseBuilder.Write(resp.Body)
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
	ctx := context.Background()
	finalResponseBody := ts.responseBuilder.String()

	// Update request execution
	if ts.stream.Err() != nil {
		ts.requestService.UpdateRequestExecutionFailed(ctx, ts.requestExec.ID, ts.stream.Err().Error())
		ts.requestService.UpdateRequestFailed(ctx, ts.request.ID)
	} else {
		ts.requestService.UpdateRequestExecutionCompleted(ctx, ts.requestExec.ID, finalResponseBody)
		ts.requestService.UpdateRequestCompleted(ctx, ts.request.ID, finalResponseBody)
	}

	return ts.stream.Close()
}

func (processor *ChatCompletionProcessor) Process(ctx context.Context, rawRequest *http.Request) (ChatCompletionResult, error) {
	chatReq, err := processor.Transformer.TransformRequest(ctx, rawRequest)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	log.Debug(ctx, "receive chat request", log.Any("request", chatReq))

	// Get API key from context
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil {
		return ChatCompletionResult{}, errors.New("API key not found in context")
	}

	// Create request record
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

		// Serialize request body for execution record
		requestBodyBytes, _ := json.Marshal(chatReq)
		requestBody := string(requestBodyBytes)

		// Create request execution record
		requestExec, err := processor.RequestService.CreateRequestExecution(ctx, req.ID, apiKey.UserID, 0, 0, requestBody)
		if err != nil {
			continue
		}

		prov, err := processor.ChannelService.GetProvider(ctx, channel.Name)
		if err != nil {
			return ChatCompletionResult{}, err
		}

		// Handle streaming vs non-streaming responses
		if chatReq.Stream != nil && *chatReq.Stream {
			stream, err := processor.handleStreamingResponse(ctx, prov, chatReq, req, requestExec)
			if err != nil {
				log.Warn(ctx, "Provider streaming failed", log.Cause(err))
				// Update failed status for this execution only
				processor.RequestService.UpdateRequestExecutionFailed(ctx, requestExec.ID, err.Error())
				continue
			}
			return ChatCompletionResult{
				ChatCompletion:       nil,
				ChatCompletionStream: stream,
			}, nil
		}

		resp, err := processor.handleNonStreamingResponse(ctx, prov, chatReq, req, requestExec)
		if err != nil {
			log.Warn(ctx, "Provider non-streaming failed", log.Cause(err))
			// Update failed status for this execution only
			processor.RequestService.UpdateRequestExecutionFailed(ctx, requestExec.ID, err.Error())
			continue
		}
		return ChatCompletionResult{
			ChatCompletion:       resp,
			ChatCompletionStream: nil,
		}, nil
	}
	// All providers failed, update request status to failed
	processor.RequestService.UpdateRequestFailed(ctx, req.ID)
	return ChatCompletionResult{}, errors.New("no provider available")
}

func (processor *ChatCompletionProcessor) handleNonStreamingResponse(ctx context.Context, prov provider.Provider, chatReq *llm.ChatCompletionRequest, req *ent.Request, requestExec *ent.RequestExecution) (*llm.GenericHttpResponse, error) {
	chatResp, err := prov.ChatCompletion(ctx, chatReq)
	if err != nil {
		log.Error(ctx, "Provider chat completion failed", log.Cause(err))
		return nil, err
	}

	log.Info(ctx, "Chat completion response", log.Any("response", chatResp))

	transformedResp, err := processor.Transformer.TransformResponse(ctx, chatResp)
	if err != nil {
		return nil, err
	}

	// Update request status to completed
	processor.RequestService.UpdateRequestCompleted(ctx, req.ID, chatResp)
	// Update request execution status to completed
	processor.RequestService.UpdateRequestExecutionCompleted(ctx, requestExec.ID, chatResp)

	return transformedResp, nil
}

func (processor *ChatCompletionProcessor) handleStreamingResponse(ctx context.Context, prov provider.Provider, chatReq *llm.ChatCompletionRequest, req *ent.Request, requestExec *ent.RequestExecution) (streams.Stream[*llm.GenericHttpResponse], error) {
	stream, err := prov.ChatCompletionStream(ctx, chatReq)
	if err != nil {
		log.Error(ctx, "Provider streaming failed", log.Cause(err))
		return nil, err
	}

	transformedStream := streams.MapErr(stream, func(resp *llm.ChatCompletionResponse) (*llm.GenericHttpResponse, error) {
		return processor.Transformer.TransformResponse(ctx, resp)
	})

	// Wrap with tracked stream to save final response
	trackedStream := NewTrackedStream(transformedStream, req, requestExec, processor.RequestService)

	return trackedStream, nil
}
