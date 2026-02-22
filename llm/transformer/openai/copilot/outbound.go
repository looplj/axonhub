package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

const (
	// DefaultCopilotBaseURL is the base URL for GitHub Copilot API.
	DefaultCopilotBaseURL = "https://api.githubcopilot.com"

	// CopilotChatCompletionsEndpoint is the endpoint for chat completions.
	CopilotChatCompletionsEndpoint = "/chat/completions"

	// LiteLLM-style editor headers (from litellm/llms/github_copilot/common_utils.py)
	EditorVersionHeader        = "editor-version"
	EditorPluginVersionHeader  = "editor-plugin-version"
	UserAgentHeader            = "user-agent"
	XInitiatorHeader           = "x-initiator"
	OpenAIIntentHeader         = "Openai-Intent"
	CopilotVisionRequestHeader = "Copilot-Vision-Request"

	// Default editor header values (VSCode pattern)
	DefaultEditorVersion       = "vscode/1.85.0"
	DefaultEditorPluginVersion = "copilot-chat/0.11.0"
	DefaultUserAgent           = "GitHubCopilotChat/0.11.0"
	DefaultInitiator           = "github/copilot"
	DefaultOpenAIIntent        = "conversation"
)

// TokenProvider defines the interface for getting Copilot tokens.
// This is typically implemented by CopilotTokenProvider.
type TokenProvider interface {
	// GetToken returns a valid Copilot token for API authentication.
	GetToken(ctx context.Context) (string, error)
}

// OutboundTransformer implements transformer.Outbound for GitHub Copilot.
// It transforms unified LLM requests to GitHub Copilot API format with LiteLLM-style headers.
type OutboundTransformer struct {
	tokenProvider TokenProvider
	baseURL       string
}

// OutboundTransformerParams contains the parameters for creating a new OutboundTransformer.
type OutboundTransformerParams struct {
	// TokenProvider provides Copilot tokens for authentication (required).
	TokenProvider TokenProvider

	// BaseURL is the base URL for the Copilot API (optional, defaults to DefaultCopilotBaseURL).
	BaseURL string
}

// Compile-time interface check.
var _ transformer.Outbound = (*OutboundTransformer)(nil)

// NewOutboundTransformer creates a new GitHub Copilot outbound transformer.
func NewOutboundTransformer(params OutboundTransformerParams) (*OutboundTransformer, error) {
	if params.TokenProvider == nil {
		return nil, errors.New("token provider is required")
	}

	baseURL := params.BaseURL
	if baseURL == "" {
		baseURL = DefaultCopilotBaseURL
	}

	// Normalize base URL (remove trailing slash)
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &OutboundTransformer{
		tokenProvider: params.TokenProvider,
		baseURL:       baseURL,
	}, nil
}

// APIFormat returns the API format for this transformer.
func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIChatCompletion
}

// TransformRequest transforms a unified LLM request to a GitHub Copilot HTTP request.
// It adds LiteLLM-style editor headers required by the Copilot API.
func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, errors.New("request is nil")
	}

	if llmReq.Model == "" {
		return nil, errors.New("model is required")
	}

	if len(llmReq.Messages) == 0 {
		return nil, errors.New("messages are required")
	}

	// Get Copilot token from token provider.
	token, err := t.tokenProvider.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get copilot token: %w", err)
	}

	// Convert to OpenAI request format.
	oaiReq := openai.RequestFromLLM(llmReq)

	// Marshal request body.
	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL.
	url := t.baseURL + CopilotChatCompletionsEndpoint

	// Prepare headers with LiteLLM-style editor headers.
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Add LiteLLM-style editor headers (required by Copilot).
	headers.Set(EditorVersionHeader, DefaultEditorVersion)
	headers.Set(EditorPluginVersionHeader, DefaultEditorPluginVersion)
	headers.Set(UserAgentHeader, DefaultUserAgent)
	headers.Set(XInitiatorHeader, DefaultInitiator)
	headers.Set(OpenAIIntentHeader, DefaultOpenAIIntent)

	// Add vision header if request contains image content.
	if hasVisionContent(llmReq) {
		headers.Set(CopilotVisionRequestHeader, "true")
	}

	// Build authentication config.
	authConfig := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: token,
	}

	return &httpclient.Request{
		Method:    http.MethodPost,
		URL:       url,
		Headers:   headers,
		Body:      body,
		Auth:      authConfig,
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
	}, nil
}

// hasVisionContent checks if the request contains image content (vision capabilities).
// It returns true if any message contains image_url content or data URLs.
func hasVisionContent(llmReq *llm.Request) bool {
	for _, msg := range llmReq.Messages {
		// Check single content.
		if msg.Content.Content != nil {
			content := *msg.Content.Content
			if isImageDataURL(content) {
				return true
			}
		}

		// Check multiple content parts.
		for _, part := range msg.Content.MultipleContent {
			// Check for image_url type.
			if part.Type == "image_url" || part.ImageURL != nil {
				return true
			}

			// Check for data URLs in text.
			if part.Text != nil && isImageDataURL(*part.Text) {
				return true
			}
		}
	}

	return false
}

// isImageDataURL checks if the content is an image data URL.
func isImageDataURL(content string) bool {
	return strings.HasPrefix(content, "data:image/")
}

// TransformResponse transforms a GitHub Copilot HTTP response to a unified LLM response.
func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, errors.New("http response is nil")
	}

	// Check for HTTP error status codes.
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body.
	if len(httpResp.Body) == 0 {
		return nil, errors.New("response body is empty")
	}

	// Parse into OpenAI Response type.
	var oaiResp openai.Response

	err := json.Unmarshal(httpResp.Body, &oaiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert to unified llm.Response.
	return oaiResp.ToLLMResponse(), nil
}

// TransformStream transforms an HTTP stream to a unified LLM response stream.
func (t *OutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.transformStreamChunk(ctx, event)
	}), nil
}

// transformStreamChunk transforms a single stream chunk to a unified LLM response.
func (t *OutboundTransformer) transformStreamChunk(ctx context.Context, event *httpclient.StreamEvent) (*llm.Response, error) {
	if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
		return llm.DoneResponse, nil
	}

	// Check for errors in the stream.
	ep := gjson.GetBytes(event.Data, "error")
	if ep.Exists() {
		return nil, &llm.ResponseError{
			Detail: llm.ErrorDetail{
				Message: ep.String(),
			},
		}
	}

	// Create a synthetic HTTP response for compatibility.
	httpResp := &httpclient.Response{
		Body: event.Data,
	}

	return t.TransformResponse(ctx, httpResp)
}

// TransformError transforms an HTTP error to a unified response error.
func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	if rawErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
				Type:    "api_error",
			},
		}
	}

	// Try to parse as OpenAI error format.
	var openaiError struct {
		Error  llm.ErrorDetail `json:"error"`
		Errors llm.ErrorDetail `json:"errors"`
	}

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil && (openaiError.Error.Message != "" || openaiError.Errors.Message != "") {
		errDetail := openaiError.Error
		if errDetail.Message == "" {
			errDetail = openaiError.Errors
		}

		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail:     errDetail,
		}
	}

	// If JSON parsing fails, use the upstream status text.
	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: http.StatusText(rawErr.StatusCode),
			Type:    "api_error",
		},
	}
}

// AggregateStreamChunks aggregates streaming chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return openai.AggregateStreamChunks(ctx, chunks, openai.DefaultTransformChunk)
}
