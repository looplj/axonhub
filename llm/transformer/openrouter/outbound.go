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
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
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
		ReasoningField: openai.ReasoningFieldReasoning,
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
		return t.buildImageGenerationRequest(ctx, llmReq)
	case llm.RequestTypeEmbedding,
		llm.RequestTypeSpeech,
		llm.RequestTypeTranscription,
		llm.RequestTypeTranslation:
		return t.Outbound.TransformRequest(ctx, llmReq)
	case llm.RequestTypeCompact:
		return nil, fmt.Errorf("%w: compact is only supported by OpenAI Responses API", transformer.ErrInvalidRequest)
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	body, err := json.Marshal(openai.RequestFromLLM(llmReq, openai.ReasoningFieldReasoning))
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
		APIFormat:   string(llm.APIFormatOpenAIChatCompletion),
	}, nil
}

// buildImageGenerationRequest builds the request for OpenRouter's image router.
// OpenRouter exposes image generation at /images rather than through chat
// completions. Input images are sent as input_references, which supports the
// same image_url content-part shape as the OpenAI-compatible APIs.
func (t *OutboundTransformer) buildImageGenerationRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if llmReq.Image == nil {
		return nil, fmt.Errorf("%w: image request is required", transformer.ErrInvalidRequest)
	}

	if llmReq.Image.Prompt == "" {
		return nil, fmt.Errorf("%w: prompt is required for image generation", transformer.ErrInvalidRequest)
	}

	if len(llmReq.Image.Images) > 16 {
		return nil, fmt.Errorf("%w: at most 16 input references are supported", transformer.ErrInvalidRequest)
	}

	if len(llmReq.Image.Mask) > 0 {
		return nil, fmt.Errorf("%w: masks are not supported by OpenRouter image generation", transformer.ErrInvalidRequest)
	}

	inputReferences := make([]openai.MessageContentPart, 0, len(llmReq.Image.Images))
	for _, imgData := range llmReq.Image.Images {
		inputReferences = append(inputReferences, openai.MessageContentPart{
			Type: "image_url",
			ImageURL: &openai.ImageURL{
				URL: encodeImageToBase64(imgData),
			},
		})
	}

	req := &ImageGenerationRequest{
		Model:             llmReq.Model,
		Prompt:            llmReq.Image.Prompt,
		N:                 llmReq.Image.N,
		Size:              llmReq.Image.Size,
		Quality:           llmReq.Image.Quality,
		Background:        llmReq.Image.Background,
		OutputFormat:      llmReq.Image.OutputFormat,
		OutputCompression: llmReq.Image.OutputCompression,
		Seed:              llmReq.Seed,
		InputReferences:   inputReferences,
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
	apiKey := t.APIKeyProvider.Get(ctx)

	auth := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	url := t.BaseURL + "/images"

	rawReq := &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		Headers:     headers,
		Body:        body,
		Auth:        auth,
		ContentType: "application/json",
		RequestType: llm.RequestTypeImage.String(),
		APIFormat:   llm.APIFormatOpenAIImageGeneration.String(),
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

	// Use xurl.BuildDataURL (single exact-size concat) instead of fmt.Sprintf to
	// avoid the printer's doubling-growth buffer churn on large base64 data.
	return xurl.BuildDataURL("image/"+format, base64Data, true)
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

	// Audio (speech/transcription/translation) responses are handled by the underlying
	// OpenAI transformer, which routes on APIFormat (binary audio for speech, JSON for STT).
	if httpResp.Request != nil {
		switch httpResp.Request.RequestType {
		case llm.RequestTypeSpeech.String(),
			llm.RequestTypeTranscription.String(),
			llm.RequestTypeTranslation.String():
			return t.Outbound.TransformResponse(ctx, httpResp)
		}
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

// transformImageGenerationResponse transforms the OpenRouter image-router
// response to the unified image response model.
func (t *OutboundTransformer) transformImageGenerationResponse(httpResp *httpclient.Response) (*llm.Response, error) {
	// Check for HTTP error status codes
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	// Read model from request TransformerMetadata
	model := "image-generation"

	if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if m, ok := httpResp.Request.TransformerMetadata["model"].(string); ok && m != "" {
			model = m
		}
	}

	var imageResp ImageGenerationResponse
	if err := json.Unmarshal(httpResp.Body, &imageResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal image generation response: %w", err)
	}

	resp := &llm.Response{
		ID:          fmt.Sprintf("img-%d", imageResp.Created),
		Object:      "chat.completion",
		Created:     imageResp.Created,
		Model:       model,
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
	}

	if imageResp.Usage != nil {
		resp.Usage = imageResp.Usage.ToLLMUsage()
	}

	imageResponse := &llm.ImageResponse{
		Created: imageResp.Created,
		Data:    make([]llm.ImageData, 0, len(imageResp.Data)),
	}

	// OpenRouter reports the encoding per image rather than once per response,
	// while ImageResponse.OutputFormat is response-scoped. Take the first image
	// that carries a usable media type instead of letting the last one win.
	for _, image := range imageResp.Data {
		data := llm.ImageData{B64JSON: image.B64JSON}
		if image.B64JSON != "" {
			data.URL = buildImageDataURL(image.MediaType, image.B64JSON)

			if imageResponse.OutputFormat == "" {
				imageResponse.OutputFormat = imageFormatFromMediaType(image.MediaType)
			}
		}

		imageResponse.Data = append(imageResponse.Data, data)
	}

	resp.Image = imageResponse

	return resp, nil
}

func (u *ImageGenerationUsage) ToLLMUsage() *llm.Usage {
	if u == nil {
		return nil
	}

	usage := &llm.Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}

	if u.PromptTokensDetails != nil {
		usage.PromptTokensDetails = &llm.PromptTokensDetails{
			ImageTokens:  u.PromptTokensDetails.ImageTokens,
			CachedTokens: u.PromptTokensDetails.CachedTokens,
		}
	}

	if u.CompletionTokensDetails != nil {
		usage.CompletionTokensDetails = &llm.CompletionTokensDetails{
			ReasoningTokens: u.CompletionTokensDetails.ReasoningTokens,
		}
	}

	return usage
}

func buildImageDataURL(mediaType, b64JSON string) string {
	if strings.HasPrefix(b64JSON, "data:") {
		return b64JSON
	}

	if mediaType == "" {
		mediaType = "image/png"
	}

	return xurl.BuildDataURL(mediaType, b64JSON, true)
}

func imageFormatFromMediaType(mediaType string) string {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType == "image/svg+xml" {
		return "svg"
	}

	if format, ok := strings.CutPrefix(mediaType, "image/"); ok {
		return format
	}

	return ""
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, req *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	// Audio streaming uses dedicated event schemas; delegate to the embedded OpenAI transformer.
	if req != nil {
		switch req.APIFormat {
		case string(llm.APIFormatOpenAISpeech),
			string(llm.APIFormatOpenAITranscription),
			string(llm.APIFormatOpenAITranslation):
			return t.Outbound.AggregateStreamChunks(ctx, req, chunks)
		}
	}

	return AggregateStreamChunks(ctx, chunks)
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	// Audio streaming uses dedicated event schemas; delegate to the embedded OpenAI transformer.
	if req != nil {
		switch req.APIFormat {
		case string(llm.APIFormatOpenAISpeech),
			string(llm.APIFormatOpenAITranscription),
			string(llm.APIFormatOpenAITranslation):
			return t.Outbound.TransformStream(ctx, req, stream)
		}
	}

	// Filter out upstream DONE events
	filteredStream := streams.Filter(stream, func(event *httpclient.StreamEvent) bool {
		return !bytes.HasPrefix(event.Data, []byte("[DONE]"))
	})

	// Transform remaining events
	transformedStream := streams.MapErr(filteredStream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	})

	// Always append our own DONE event at the end
	return streams.AppendStream(transformedStream, llm.DoneResponse), nil
}

func (t *OutboundTransformer) TransformStreamChunk(ctx context.Context, event *httpclient.StreamEvent) (*llm.Response, error) {
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
