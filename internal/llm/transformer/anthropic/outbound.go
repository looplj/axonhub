package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/bedrock"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/vertex"
	"github.com/looplj/axonhub/internal/pkg/xjson"
)

// PlatformType represents the platform type for Anthropic API.
type PlatformType string

const (
	PlatformDirect  PlatformType = "direct"  // Direct Anthropic API
	PlatformBedrock PlatformType = "bedrock" // AWS Bedrock
	PlatformVertex  PlatformType = "vertex"  // Google Vertex AI
)

// Config holds all configuration for the Anthropic outbound transformer.
type Config struct {
	// Platform configuration
	Type PlatformType `json:"type"`

	Region          string `json:"region,omitempty"`          // For Bedrock and Vertex
	AccessKeyID     string `json:"accessKeyID,omitempty"`     // For Bedrock
	SecretAccessKey string `json:"secretAccessKey,omitempty"` // For Bedrock

	ProjectID string `json:"project_id,omitempty"` // For Vertex
	JSONData  string `json:"json_data,omitempty"`  // For Vertex

	// API configuration
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKey  string `json:"api_key,omitempty"`  // API key for direct Anthropic API
}

// OutboundTransformer implements transformer.Outbound for Anthropic format.
type OutboundTransformer struct {
	config *Config
}

// NewOutboundTransformer creates a new Anthropic OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		Type:    PlatformDirect,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Anthropic OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config.BaseURL == "" {
		config.BaseURL = getDefaultBaseURL(config)
	}

	var t transformer.Outbound = &OutboundTransformer{
		config: config,
	}

	if config.Type == PlatformBedrock {
		executor, err := bedrock.NewExecutor(config.Region, config.AccessKeyID, config.SecretAccessKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create bedrock executor: %w", err)
		}

		t = &BedrockTransformer{
			Outbound: t,
			bedrock:  executor,
		}
	}

	if config.Type == PlatformVertex {
		executor, err := vertex.NewExecutorFromJSON(config.Region, config.ProjectID, config.JSONData)
		if err != nil {
			return nil, fmt.Errorf("failed to create vertex transformer: %w", err)
		}

		t = &VertexTransformer{
			Outbound: t,
			executor: executor,
		}
	}

	return t, nil
}

// getDefaultBaseURL returns the default base URL for the given platform configuration.
func getDefaultBaseURL(config *Config) string {
	//nolint:exhaustive // Checked.
	switch config.Type {
	case PlatformBedrock:
		if config.Region != "" {
			return fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", config.Region)
		}

		return "https://bedrock-runtime.us-east-1.amazonaws.com"
	case PlatformVertex:
		if config.Region != "" {
			if config.Region == "global" {
				return "https://aiplatform.googleapis.com"
			}

			return fmt.Sprintf("https://%s-aiplatform.googleapis.com", config.Region)
		}

		return "https://us-central1-aiplatform.googleapis.com"
	default:
		return "https://api.anthropic.com"
	}
}

// Name returns the name of the transformer.
func (t *OutboundTransformer) Name() string {
	return "claude/messages"
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

	// Apply platform-specific transformations
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request for platform: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	//nolint:exhaustive // Checked.
	switch t.config.Type {
	case PlatformBedrock:
		headers.Set("Anthropic-Version", "bedrock-2023-05-31")
	case PlatformVertex:
		headers.Set("Anthropic-Version", "vertex-2023-10-16")
	default:
		headers.Set("Anthropic-Version", "2023-06-01")
	}

	// Prepare authentication
	var auth *httpclient.AuthConfig
	if t.config.APIKey != "" && t.config.Type == PlatformDirect {
		auth = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    t.config.APIKey,
			HeaderKey: "X-API-Key",
		}
	}

	// Determine endpoint based on platform
	url, err := t.buildFullRequestURL(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to build platform URL: %w", err)
	}

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

// buildFullRequestURL constructs the appropriate URL based on the platform.
func (t *OutboundTransformer) buildFullRequestURL(chatReq *llm.Request) (string, error) {
	baseURL := strings.TrimSuffix(t.config.BaseURL, "/")

	//nolint:exhaustive // Checked.
	switch t.config.Type {
	case PlatformBedrock:
		// Bedrock URL format: /model/{model}/invoke or /model/{model}/invoke-with-response-stream
		var endpoint string
		if chatReq.Stream != nil && *chatReq.Stream {
			endpoint = fmt.Sprintf("/model/%s/invoke-with-response-stream", chatReq.Model)
		} else {
			endpoint = fmt.Sprintf("/model/%s/invoke", chatReq.Model)
		}

		return baseURL + endpoint, nil

	case PlatformVertex:
		// Vertex AI URL format: /v1/projects/{project}/locations/{region}/publishers/anthropic/models/{model}:rawPredict
		if t.config.ProjectID == "" {
			return "", fmt.Errorf("project ID is required for Vertex AI")
		}

		if t.config.Region == "" {
			return "", fmt.Errorf("region is required for Vertex AI")
		}

		var specifier string
		if chatReq.Stream != nil && *chatReq.Stream {
			specifier = "streamRawPredict"
		} else {
			specifier = "rawPredict"
		}

		endpoint := fmt.Sprintf("/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:%s",
			t.config.ProjectID, t.config.Region, chatReq.Model, specifier)

		return baseURL + endpoint, nil

	default:
		// Direct Anthropic API
		return baseURL + "/v1/messages", nil
	}
}

// TransformResponse transforms Anthropic HTTP response to ChatCompletionResponse.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check for HTTP error status
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body
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
	t.config.APIKey = apiKey
}

// SetBaseURL updates the base URL.
func (t *OutboundTransformer) SetBaseURL(baseURL string) {
	t.config.BaseURL = baseURL
}

// SetConfig updates the entire configuration.
func (t *OutboundTransformer) SetConfig(config *Config) {
	if config == nil {
		config = &Config{Type: PlatformDirect}
	}

	t.config = config
}

// ConfigureForBedrock configures the transformer for AWS Bedrock.
func (t *OutboundTransformer) ConfigureForBedrock(region string) {
	if region == "" {
		region = "us-east-1"
	}

	t.config.Type = PlatformBedrock
	t.config.Region = region
	t.config.ProjectID = "" // Clear project ID for Bedrock

	// Update base URL for Bedrock
	t.config.BaseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region)
}

// ConfigureForVertex configures the transformer for Google Vertex AI.
func (t *OutboundTransformer) ConfigureForVertex(region, projectID string) error {
	if region == "" {
		return fmt.Errorf("region is required for Vertex AI")
	}

	if projectID == "" {
		return fmt.Errorf("project ID is required for Vertex AI")
	}

	t.config.Type = PlatformVertex
	t.config.Region = region
	t.config.ProjectID = projectID

	// Update base URL for Vertex AI
	if region == "global" {
		t.config.BaseURL = "https://aiplatform.googleapis.com"
	} else {
		t.config.BaseURL = fmt.Sprintf("https://%s-aiplatform.googleapis.com", region)
	}

	return nil
}

// GetConfig returns the current configuration.
func (t *OutboundTransformer) GetConfig() *Config {
	return t.config
}

// GetPlatformConfig returns the current platform configuration (for backward compatibility).
// Deprecated: Use GetConfig instead.
func (t *OutboundTransformer) GetPlatformConfig() *Config {
	return t.config
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
func (s *outboundStream) transformStreamChunk(event *httpclient.StreamEvent) (*llm.Response, error) {
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

	if event.Type == "error" {
		return nil, &llm.ResponseError{
			Detail: llm.ErrorDetail{
				Message: fmt.Sprintf("received error while streaming: %s", string(event.Data)),
			},
		}
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

// TransformError transforms HTTP error response to unified error response for Anthropic.
func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	if rawErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: "Request failed.",
				Type:    "api_error",
			},
		}
	}

	aErr, err := xjson.To[AnthropicErr](rawErr.Body)
	if err == nil && aErr.RequestID != "" {
		// Successfully parsed as Anthropic error format
		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail: llm.ErrorDetail{
				Type:    "api_error",
				Message: fmt.Sprintf("Request failed. Request_id: %s", aErr.RequestID),
			},
		}
	}

	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message:   fmt.Sprintf("Request failed. Status_code: %d, body: %s", rawErr.StatusCode, string(rawErr.Body)),
			Type:      "api_error",
			Code:      "",
			Param:     "",
			RequestID: aErr.RequestID,
		},
	}
}
