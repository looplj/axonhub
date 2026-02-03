package openrouter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cast"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config holds all configuration for the OpenRouter outbound transformer.
type Config struct {
	// API configuration
	BaseURL        string              `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKeyProvider auth.APIKeyProvider `json:"-"`                  // API key provider
}

// OutboundTransformer implements transformer.Outbound for OpenRouter format.
// OpenRouter is mostly compatible with OpenAI(DeepSeek) API, but there are some differences for the reasoning content.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL        string
	APIKeyProvider auth.APIKeyProvider
}

// NewOutboundTransformer creates a new OpenRouter OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new OpenRouter OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
	}

	t, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenRouter transformer configuration: %w", err)
	}

	baseURL := strings.TrimSuffix(config.BaseURL, "/")

	return &OutboundTransformer{
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
		Outbound:       t,
	}, nil
}

// TransformRequest transforms ChatCompletionRequest to Request.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Validate required fields
	if llmReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeChat, "":
		// continue
	case llm.RequestTypeImage:
		return t.buildImageGenerationRequest(llmReq)
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	body, err := json.Marshal(openai.RequestFromLLM(llmReq))
	if err != nil {
		return nil, fmt.Errorf("%w: failed to transform request: %w", transformer.ErrInvalidRequest, err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Get API key from provider
	apiKey := t.APIKeyProvider.Get(ctx)

	auth := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	url := t.BaseURL + "/chat/completions"

	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		Headers:     headers,
		Body:        body,
		Auth:        auth,
		ContentType: "application/json",
	}, nil
}

// buildImageGenerationRequest builds the request for OpenRouter image generation.
// OpenRouter uses the chat completions endpoint with modalities: ["image", "text"].
// Supports image editing when llmReq.Image.Images is provided.
func (t *OutboundTransformer) buildImageGenerationRequest(llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if llmReq.Image == nil {
		return nil, fmt.Errorf("%w: image request is required", transformer.ErrInvalidRequest)
	}

	if llmReq.Image.Prompt == "" {
		return nil, fmt.Errorf("%w: prompt is required for image generation", transformer.ErrInvalidRequest)
	}

	prompt := llmReq.Image.Prompt

	// Build message content parts
	var contentParts []openai.MessageContentPart

	// Add input images if provided (for image editing)
	for _, imgData := range llmReq.Image.Images {
		base64Image := encodeImageToBase64(imgData)
		contentParts = append(contentParts, openai.MessageContentPart{
			Type: "image_url",
			ImageURL: &openai.ImageURL{
				URL: base64Image,
			},
		})
	}

	// Add the text prompt
	contentParts = append(contentParts, openai.MessageContentPart{
		Type: "text",
		Text: &prompt,
	})

	// Build messages with the content parts
	messages := []openai.Message{
		{
			Role: "user",
			Content: openai.MessageContent{
				MultipleContent: contentParts,
			},
		},
	}

	// Build the OpenAI request
	req := &openai.Request{
		Model:      llmReq.Model,
		Messages:   messages,
		Modalities: []string{"image", "text"},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to marshal request: %w", transformer.ErrInvalidRequest, err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Get API key from provider
	apiKey := t.APIKeyProvider.Get(context.Background())

	auth := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	url := t.BaseURL + "/chat/completions"

	rawReq := &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		Headers:     headers,
		Body:        body,
		Auth:        auth,
		ContentType: "application/json",
		RequestType: llm.RequestTypeImage.String(),
	}

	// Save model to TransformerMetadata for response transformation
	rawReq.TransformerMetadata = map[string]any{"model": llmReq.Model}

	return rawReq, nil
}

// encodeImageToBase64 encodes image bytes to a base64 data URL.
func encodeImageToBase64(data []byte) string {
	// Detect image format from magic bytes
	format := detectImageFormat(data)
	base64Data := base64.StdEncoding.EncodeToString(data)

	return fmt.Sprintf("data:image/%s;base64,%s", format, base64Data)
}

// detectImageFormat detects image format from magic bytes.
func detectImageFormat(data []byte) string {
	if len(data) < 4 {
		return "png" // default
	}

	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "png"
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpeg"
	}

	// GIF: GIF87a or GIF89a
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "gif"
	}

	// WebP: RIFF....WEBP
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "webp"
		}
	}

	return "png" // default
}

func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check if this is an image generation request
	if httpResp.Request != nil && httpResp.Request.RequestType == llm.RequestTypeImage.String() {
		return t.transformImageGenerationResponse(httpResp)
	}

	// Check for HTTP error status codes
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var chatResp Response

	err := json.Unmarshal(httpResp.Body, &chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %w", err)
	}

	return chatResp.ToOpenAIResponse().ToLLMResponse(), nil
}

// transformImageGenerationResponse transforms OpenRouter image generation response to llm.Response.
// OpenRouter returns images in message.images array with base64 data URLs.
func (t *OutboundTransformer) transformImageGenerationResponse(httpResp *httpclient.Response) (*llm.Response, error) {
	// Check for HTTP error status codes
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var chatResp Response

	err := json.Unmarshal(httpResp.Body, &chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal image generation response: %w", err)
	}

	// Read model from request TransformerMetadata
	model := "image-generation"

	if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if m, ok := httpResp.Request.TransformerMetadata["model"].(string); ok && m != "" {
			model = m
		}
	}

	// Build the base response
	resp := &llm.Response{
		ID:          chatResp.ID,
		Object:      "chat.completion",
		Created:     chatResp.Created,
		Model:       model,
		RequestType: llm.RequestTypeImage,
	}

	// Convert usage information
	if chatResp.Usage != nil {
		resp.Usage = chatResp.Usage.ToLLMUsage()
	}

	// Build ImageResponse from the message images
	imageResponse := &llm.ImageResponse{
		Created: chatResp.Created,
		Data:    make([]llm.ImageData, 0),
	}

	// Extract images from the response
	for _, choice := range chatResp.Choices {
		if choice.Message != nil && len(choice.Message.Images) > 0 {
			for _, img := range choice.Message.Images {
				if img.ImageURL != nil && img.ImageURL.URL != "" {
					imageResponse.Data = append(imageResponse.Data, llm.ImageData{
						B64JSON: extractBase64FromDataURL(img.ImageURL.URL),
						URL:     img.ImageURL.URL,
					})
				}
			}
		}
	}

	// If no images found in Images field, check Content.MultipleContent
	if len(imageResponse.Data) == 0 {
		for _, choice := range chatResp.Choices {
			if choice.Message != nil && len(choice.Message.Content.MultipleContent) > 0 {
				for _, part := range choice.Message.Content.MultipleContent {
					if part.Type == "image_url" && part.ImageURL != nil && part.ImageURL.URL != "" {
						imageResponse.Data = append(imageResponse.Data, llm.ImageData{
							B64JSON: extractBase64FromDataURL(part.ImageURL.URL),
							URL:     part.ImageURL.URL,
						})
					}
				}
			}
		}
	}

	// Set usage info for image generation
	if resp.Usage != nil {
		imageResponse.Usage = &llm.ImageUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	resp.Image = imageResponse

	return resp, nil
}

// extractBase64FromDataURL extracts base64 data from a data URL.
// e.g., "data:image/png;base64,iVBORw0KGgo..." -> "iVBORw0KGgo...".
func extractBase64FromDataURL(dataURL string) string {
	const prefix = "base64,"

	_, after, ok := strings.Cut(dataURL, prefix)
	if !ok {
		return ""
	}

	return after
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	}), nil
}

func (t *OutboundTransformer) TransformStreamChunk(ctx context.Context, event *httpclient.StreamEvent) (*llm.Response, error) {
	if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
		return llm.DoneResponse, nil
	}

	ep := gjson.GetBytes(event.Data, "error")
	if ep.Exists() {
		return nil, &llm.ResponseError{
			Detail: llm.ErrorDetail{
				Message: ep.String(),
			},
		}
	}

	// Create a synthetic HTTP response for compatibility with existing logic
	httpResp := &httpclient.Response{
		Body: event.Data,
	}

	return t.TransformResponse(ctx, httpResp)
}

type openRouterError struct {
	Error struct {
		Message  string `json:"message"`
		Code     int    `json:"code"`
		Metadata struct {
			Raw any `json:"raw"`
		} `json:"metadata"`
	} `json:"error"`
}

func (e openRouterError) ToLLMError() llm.ErrorDetail {
	message := cast.ToString(e.Error.Metadata.Raw)
	if message == "" {
		message = e.Error.Message
	}

	if message == "" && e.Error.Code != 0 {
		message = http.StatusText(e.Error.Code)
	}

	return llm.ErrorDetail{
		Message: message,
		Code:    fmt.Sprintf("%d", e.Error.Code),
		Type:    "api_error",
	}
}

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

	// Try to parse as OpenRouter error format first
	var openaiError openRouterError

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil {
		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail:     openaiError.ToLLMError(),
		}
	}

	// If JSON parsing fails, use the upstream status text
	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: http.StatusText(rawErr.StatusCode),
			Type:    "api_error",
		},
	}
}
