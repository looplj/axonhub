package zai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// buildImageGenerationAPIRequest builds the HTTP request to call the ZAI Image Generation API.
func (t *OutboundTransformer) buildImageGenerationAPIRequest(ctx context.Context, chatReq *llm.Request) (*httpclient.Request, error) {
	if chatReq.Image == nil {
		return nil, fmt.Errorf("%w: image request is required", transformer.ErrInvalidRequest)
	}

	if chatReq.APIFormat != "" && chatReq.APIFormat != llm.APIFormatOpenAIImageGeneration {
		return nil, fmt.Errorf("%w: ZAI only supports image generation", transformer.ErrInvalidRequest)
	}

	if len(chatReq.Image.Images) > 0 || len(chatReq.Image.Mask) > 0 {
		return nil, fmt.Errorf("%w: ZAI does not support image editing with input images", transformer.ErrInvalidRequest)
	}

	// Use Image Generation API only
	rawReq, err := t.buildImageGenerateRequest(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	rawReq.RequestType = llm.RequestTypeImage.String()
	rawReq.APIFormat = llm.APIFormatOpenAIImageGeneration.String()
	// Save model to TransformerMetadata for response transformation
	if rawReq.TransformerMetadata == nil {
		rawReq.TransformerMetadata = map[string]any{}
	}
	rawReq.TransformerMetadata["model"] = chatReq.Model

	return rawReq, nil
}

// buildImageGenerateRequest builds request for ZAI Image Generation API.
func (t *OutboundTransformer) buildImageGenerateRequest(ctx context.Context, chatReq *llm.Request) (*httpclient.Request, error) {
	prompt := chatReq.Image.Prompt
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required for image generation")
	}

	// Build request body according to ZAI API documentation
	reqBody := map[string]any{
		"model":  chatReq.Model,
		"prompt": prompt,
	}

	quality := chatReq.Image.Quality
	switch quality {
	case "high":
		quality = "hd"
	case "low", "":
		quality = "standard"
	}

	if quality == "" {
		quality = "standard"
	}

	reqBody["quality"] = quality

	if chatReq.Image.Size != "" {
		reqBody["size"] = chatReq.Image.Size
	} else {
		reqBody["size"] = "1024x1024"
	}

	reqBody["watermark_enabled"] = false

	// User ID from metadata (following the pattern from TransformRequest)
	if chatReq.Image.User != "" {
		reqBody["user_id"] = chatReq.Image.User
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Build URL
	url := t.BaseURL + "/images/generations"

	// Get API key from provider
	apiKey := t.APIKeyProvider.Get(ctx)

	// Build auth config
	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: apiKey,
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

// transformImageGenerationResponse transforms the ZAI Image Generation API response
// to the unified llm.Response format.
func transformImageGenerationResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	// Parse the ZAI ImagesResponse
	var zaiResp ZAIImagesResponse
	if err := json.Unmarshal(httpResp.Body, &zaiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal images response: %w", err)
	}

	// Read model from request metadata
	model := "image-generation"

	if httpResp.Request != nil && httpResp.Request.Metadata != nil {
		if m, ok := httpResp.Request.Metadata["model"]; ok && m != "" {
			model = m
		}
	}

	// Convert to llm.Response format
	llmResp := &llm.Response{
		ID:          fmt.Sprintf("zai-img-%s", uuid.NewString()),
		Object:      "chat.completion",
		Created:     zaiResp.Created,
		Model:       model,
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
	}

	imageResp := &llm.ImageResponse{
		Created: zaiResp.Created,
		Data:    make([]llm.ImageData, 0, len(zaiResp.Data)),
	}

	// Convert each image to ImageData
	for _, img := range zaiResp.Data {
		// Download image and convert to base64 data URL
		imageDataURL, err := downloadImageToDataURL(ctx, img.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to download and convert image: %w", err)
		}

		var b64 string
		if _, after, ok := strings.Cut(imageDataURL, "base64,"); ok {
			b64 = after
		}

		imageResp.Data = append(imageResp.Data, llm.ImageData{
			B64JSON: b64,
			URL:     img.URL,
		})
	}

	llmResp.Image = imageResp

	return llmResp, nil
}

// downloadImageToDataURL downloads an image from a URL and converts it to a base64 data URL.
func downloadImageToDataURL(ctx context.Context, imageURL string) (string, error) {
	// Check if it's already a data URL
	if isDataURL(imageURL) {
		return imageURL, nil
	}

	// Download the image
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.ErrorContext(ctx, "failed to close response body", slog.Any("error", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download image: HTTP %d", resp.StatusCode)
	}

	// Read the image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}

	// Detect image format from Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// Default to png if content type is not available
		contentType = "image/png"
	}

	// Convert to base64 data URL
	base64Data := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, base64Data)

	return dataURL, nil
}

// isDataURL checks if the given URL is a data URL.
func isDataURL(url string) bool {
	return len(url) > 5 && url[:5] == "data:"
}

// ZAIImagesResponse represents the response from ZAI Image Generation API.
type ZAIImagesResponse struct {
	Created int64          `json:"created"`
	Data    []ZAIImageData `json:"data"`
}

// ZAIImageData represents a single image in the response.
type ZAIImageData struct {
	URL string `json:"url"`
}

// ZAIContentFilter represents content filter information.
type ZAIContentFilter struct {
	Role  string `json:"role"`
	Level int    `json:"level"`
}
