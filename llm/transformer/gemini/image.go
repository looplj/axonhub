package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// buildImageGenerationRequest builds the HTTP request for Gemini image generation/editing.
// Gemini uses the generateContent endpoint with responseModalities: ["TEXT", "IMAGE"].
func (t *OutboundTransformer) buildImageGenerationRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq.Image == nil {
		return nil, fmt.Errorf("%w: image request is required", transformer.ErrInvalidRequest)
	}

	if llmReq.Image.Prompt == "" {
		return nil, fmt.Errorf("%w: prompt is required for image generation", transformer.ErrInvalidRequest)
	}

	if llmReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	// Build content parts
	var parts []*Part

	// Add input images first (for image editing scenario)
	for _, imgData := range llmReq.Image.Images {
		mimeType := detectImageMIMEType(imgData)
		base64Data := base64.StdEncoding.EncodeToString(imgData)

		parts = append(parts, &Part{
			InlineData: &Blob{
				MIMEType: mimeType,
				Data:     base64Data,
			},
		})
	}

	// Add text prompt
	parts = append(parts, &Part{Text: llmReq.Image.Prompt})

	// Build Gemini request
	geminiReq := &GenerateContentRequest{
		Contents: []*Content{
			{
				Role:  "user",
				Parts: parts,
			},
		},
		GenerationConfig: &GenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
			ImageConfig:        buildImageConfig(llmReq.Image),
		},
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini image request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Prepare authentication
	var auth *httpclient.AuthConfig

	if t.config.APIKeyProvider != nil {
		apiKey := t.config.APIKeyProvider.Get(ctx)
		if apiKey != "" {
			auth = &httpclient.AuthConfig{
				Type:      "api_key",
				APIKey:    apiKey,
				HeaderKey: "x-goog-api-key",
			}
		}
	}

	// Build URL - image generation uses the same generateContent endpoint
	url := t.buildImageRequestURL(llmReq)

	rawReq := &httpclient.Request{
		Method:                http.MethodPost,
		URL:                   url,
		Headers:               headers,
		Body:                  body,
		Auth:                  auth,
		RequestType:           llm.RequestTypeImage.String(),
		SkipInboundQueryMerge: true,
	}

	// Save model to TransformerMetadata for response transformation
	rawReq.TransformerMetadata = map[string]any{"model": llmReq.Model}

	return rawReq, nil
}

// buildImageRequestURL constructs the URL for image generation requests.
// Image generation uses generateContent endpoint (not streaming).
func (t *OutboundTransformer) buildImageRequestURL(llmReq *llm.Request) string {
	version := t.config.APIVersion
	if version == "" {
		version = DefaultAPIVersion
	}

	// For Vertex AI platform
	if t.config.PlatformType == PlatformVertex {
		baseURL := strings.TrimSuffix(t.config.BaseURL, "/")
		if strings.Contains(baseURL, "/v1/") {
			return fmt.Sprintf("%s/publishers/google/models/%s:generateContent", baseURL, llmReq.Model)
		}

		return fmt.Sprintf("%s/v1/publishers/google/models/%s:generateContent", baseURL, llmReq.Model)
	}

	// Standard Gemini API format
	return fmt.Sprintf("%s/%s/models/%s:generateContent", t.config.BaseURL, version, llmReq.Model)
}

// buildImageConfig builds the ImageConfig from ImageRequest parameters.
func buildImageConfig(img *llm.ImageRequest) *ImageConfig {
	if img == nil {
		return nil
	}

	config := &ImageConfig{}

	// Map size to aspectRatio if provided
	if img.Size != "" {
		config.AspectRatio = mapSizeToAspectRatio(img.Size)
	}

	// Map size to imageSize if provided (e.g., "1024x1024" -> "1K", "2048x2048" -> "2K")
	if img.Size != "" {
		config.ImageSize = mapSizeToImageSize(img.Size)
	}

	return config
}

// mapSizeToAspectRatio maps OpenAI-style size (e.g., "1024x1024") to Gemini aspect ratio.
func mapSizeToAspectRatio(size string) string {
	switch size {
	case "1024x1024", "512x512", "256x256":
		return "1:1"
	case "1792x1024":
		return "16:9"
	case "1024x1792":
		return "9:16"
	case "1536x1024":
		return "3:2"
	case "1024x1536":
		return "2:3"
	case "1024x768":
		return "4:3"
	case "768x1024":
		return "3:4"
	default:
		// Check if it's already an aspect ratio format
		if strings.Contains(size, ":") {
			return size
		}

		return "1:1" // default
	}
}

// mapSizeToImageSize maps OpenAI-style size to Gemini imageSize ("1K", "2K", "4K").
func mapSizeToImageSize(size string) string {
	switch size {
	case "256x256", "512x512", "1024x1024",
		"1024x1536", "1024x1792", "1024x768",
		"1536x1024", "768x1024", "1792x1024":
		return "1K"
	case "2048x2048", "2048x1536", "2048x1152", "1536x2048", "1152x2048":
		return "2K"
	case "4096x4096", "4096x3072", "4096x2304", "3072x4096", "2304x4096":
		return "4K"
	default:
		// Check if size contains dimensions and calculate approximate pixel count
		var width, height int
		if _, err := fmt.Sscanf(size, "%dx%d", &width, &height); err == nil {
			pixels := width * height
			switch {
			case pixels <= 1024*1024:
				return "1K"
			case pixels <= 2048*2048:
				return "2K"
			default:
				return "4K"
			}
		}

		return "1K" // default
	}
}

// detectImageMIMEType detects image format from magic bytes.
func detectImageMIMEType(data []byte) string {
	if len(data) < 4 {
		return "image/png" // default
	}

	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// GIF: GIF87a or GIF89a
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}

	// WebP: RIFF....WEBP
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
	}

	return "image/png" // default
}

// transformImageGenerationResponse transforms Gemini image generation response to llm.Response.
// Gemini returns images in candidates[].content.parts[].inlineData.
func transformImageGenerationResponse(httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var geminiResp GenerateContentResponse
	if err := json.Unmarshal(httpResp.Body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gemini image response: %w", err)
	}

	// Read model from request TransformerMetadata
	model := "image-generation"

	if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if m, ok := httpResp.Request.TransformerMetadata["model"].(string); ok && m != "" {
			model = m
		}
	}

	// Build the base response
	created := time.Now().Unix()

	resp := &llm.Response{
		ID:          geminiResp.ResponseID,
		Object:      "chat.completion",
		Created:     created,
		Model:       model,
		RequestType: llm.RequestTypeImage,
	}

	// Convert usage information
	if geminiResp.UsageMetadata != nil {
		resp.Usage = convertToLLMUsage(geminiResp.UsageMetadata)
	}

	// Build ImageResponse from candidates
	imageResponse := &llm.ImageResponse{
		Created: created,
		Data:    make([]llm.ImageData, 0),
	}

	// Extract images from the response candidates
	for _, candidate := range geminiResp.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && strings.HasPrefix(part.InlineData.MIMEType, "image/") {
				imageResponse.Data = append(imageResponse.Data, llm.ImageData{
					B64JSON: part.InlineData.Data,
				})
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
