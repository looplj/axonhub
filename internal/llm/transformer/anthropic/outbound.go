package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// OutboundTransformer implements transformer.Outbound for Anthropic format.
type OutboundTransformer struct {
	baseURL string
	apiKey  string
}

// NewOutboundTransformer creates a new Anthropic OutboundTransformer.
func NewOutboundTransformer(baseURL, apiKey string) *OutboundTransformer {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	return &OutboundTransformer{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// TransformRequest transforms ChatCompletionRequest to Anthropic HTTP request.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	chatReq *llm.Request,
) (*httpclient.Request, error) {
	if chatReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Validate required fields
	if chatReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if len(chatReq.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	// Validate max_tokens
	if chatReq.MaxTokens != nil && *chatReq.MaxTokens <= 0 {
		return nil, fmt.Errorf("max_tokens must be positive")
	}

	// Convert to Anthropic request format
	anthropicReq := t.convertToAnthropicRequest(chatReq)

	// Marshal the request body
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	headers.Set("Anthropic-Version", "2023-06-01")

	// Prepare authentication
	var auth *httpclient.AuthConfig
	if t.apiKey != "" {
		auth = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    t.apiKey,
			HeaderKey: "x-api-key",
		}
	}

	// Determine endpoint
	endpoint := "/v1/messages"
	url := strings.TrimSuffix(t.baseURL, "/") + endpoint

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

func (t *OutboundTransformer) convertToAnthropicRequest(chatReq *llm.Request) *MessageRequest {
	return convertToAnthropicRequest(chatReq)
}

// TransformResponse transforms Anthropic HTTP response to ChatCompletionResponse.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check for HTTP errors
	if httpResp.StatusCode >= 400 {
		if httpResp.Error != nil {
			return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, httpResp.Error.Message)
		}

		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var anthropicResp Message

	err := json.Unmarshal(httpResp.Body, &anthropicResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal anthropic response: %w", err)
	}

	// Convert to ChatCompletionResponse
	chatResp := convertToChatCompletionResponse(&anthropicResp)

	return chatResp, nil
}

func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	// Filter out unnecessary stream events to optimize performance
	filteredStream := streams.Filter(stream, filterStreamEvent)

	doneEvent := lo.ToPtr(llm.DoneStreamEvent)
	// Append the DONE event to the filtered stream
	streamWithDone := streams.AppendStream(filteredStream, doneEvent)

	return newOutboundStream(streamWithDone), nil
}

// filterStreamEvent determines if a stream event should be processed
// Filters out unnecessary events like ping, content_block_start, and content_block_stop.
func filterStreamEvent(event *httpclient.StreamEvent) bool {
	if event == nil || len(event.Data) == 0 {
		return false
	}

	// Only process events that contribute to the OpenAI response format
	switch event.Type {
	case "message_start", "content_block_delta", "message_delta", "message_stop":
		return true
	case "ping", "content_block_start", "content_block_stop":
		return false // Skip these events as they're not needed for OpenAI format
	default:
		return false // Skip unknown event types
	}
}

// AggregateStreamChunks aggregates Anthropic streaming response chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, error) {
	return AggregateStreamChunks(ctx, chunks)
}

// SetAPIKey updates the API key.
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.apiKey = apiKey
}

// SetBaseURL updates the base URL.
func (t *OutboundTransformer) SetBaseURL(baseURL string) {
	t.baseURL = baseURL
}

// streamState holds the state for a streaming session.
type streamState struct {
	streamID    string
	streamModel string
	streamUsage *llm.Usage
}

// outboundStream wraps a stream and maintains state during processing.
type outboundStream struct {
	stream  streams.Stream[*httpclient.StreamEvent]
	state   *streamState
	current *llm.Response
	err     error
}

func newOutboundStream(stream streams.Stream[*httpclient.StreamEvent]) *outboundStream {
	return &outboundStream{
		stream: stream,
		state:  &streamState{},
	}
}

func (s *outboundStream) Next() bool {
	if s.stream.Next() {
		event := s.stream.Current()

		resp, err := s.transformStreamChunk(event)
		if err != nil {
			s.err = err
			return false
		}

		s.current = resp

		return true
	}

	return false
}

// transformStreamChunk transforms a single Anthropic streaming chunk to ChatCompletionResponse with state.
func (s *outboundStream) transformStreamChunk(
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	if event == nil {
		return nil, fmt.Errorf("stream event is nil")
	}

	if len(event.Data) == 0 {
		return nil, fmt.Errorf("event data is empty")
	}

	// Handle DONE event specially
	if string(event.Data) == "[DONE]" {
		return llm.DoneResponse, nil
	}

	state := s.state

	// Parse the streaming event
	var streamEvent StreamEvent

	err := json.Unmarshal(event.Data, &streamEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal anthropic stream event: %w", err)
	}

	// Convert the stream event to ChatCompletionResponse
	resp := &llm.Response{
		Object: "chat.completion.chunk",
		ID:     state.streamID,    // Use stored ID from message_start
		Model:  state.streamModel, // Use stored model from message_start
	}

	switch streamEvent.Type {
	case "message_start":
		if streamEvent.Message != nil {
			// Store ID, model, and usage for subsequent events
			state.streamID = streamEvent.Message.ID
			state.streamModel = streamEvent.Message.Model

			// Update response with stored values
			resp.ID = state.streamID
			resp.Model = state.streamModel

			if streamEvent.Message.Usage != nil {
				state.streamUsage = lo.ToPtr(convertUsage(*streamEvent.Message.Usage))
				resp.Usage = state.streamUsage
			}

			resp.Created = 0
			if streamEvent.Message.Usage != nil {
				resp.ServiceTier = streamEvent.Message.Usage.ServiceTier
			}
		}
		// For message_start, we return an empty choice to indicate the start
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
				},
			},
		}

	case "content_block_delta":
		resp.Usage = state.streamUsage
		if streamEvent.Delta != nil && streamEvent.Delta.Text != nil {
			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: streamEvent.Delta.Text,
						},
					},
				},
			}
		}

	case "message_delta":
		// Update stored usage if available (final usage information)
		if streamEvent.Usage != nil {
			usage := convertUsage(*streamEvent.Usage)
			if state.streamUsage != nil {
				usage.PromptTokens = state.streamUsage.PromptTokens
				if state.streamUsage.PromptTokensDetails != nil {
					usage.PromptTokensDetails = state.streamUsage.PromptTokensDetails
				}
				// Recalculate total tokens
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}

			state.streamUsage = &usage
		}

		resp.Usage = state.streamUsage

		if streamEvent.Delta != nil && streamEvent.Delta.StopReason != nil {
			// Determine finish reason
			var finishReason *string

			switch *streamEvent.Delta.StopReason {
			case "end_turn":
				reason := "stop"
				finishReason = &reason
			case "max_tokens":
				reason := "length"
				finishReason = &reason
			case "stop_sequence":
				reason := "stop"
				finishReason = &reason
			case "tool_use":
				reason := "tool_calls"
				finishReason = &reason
			default:
				finishReason = streamEvent.Delta.StopReason
			}

			resp.Choices = []llm.Choice{
				{
					Index:        0,
					FinishReason: finishReason,
				},
			}
		}

	case "message_stop":
		// Final event - return empty response to indicate completion
		resp.Choices = []llm.Choice{}
		// Include final usage information
		if state.streamUsage != nil {
			resp.Usage = state.streamUsage
		}

	default:
		// This should not happen due to filtering, but handle gracefully
		return nil, fmt.Errorf("unexpected stream event type: %s", streamEvent.Type)
	}

	return resp, nil
}

func (s *outboundStream) Current() *llm.Response {
	return s.current
}

func (s *outboundStream) Err() error {
	if s.err != nil {
		return s.err
	}

	return s.stream.Err()
}

func (s *outboundStream) Close() error {
	return s.stream.Close()
}
