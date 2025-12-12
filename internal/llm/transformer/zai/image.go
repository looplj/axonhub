package zai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// buildImageGenerationAPIRequest builds the HTTP request to call the ZAI Image Generation API.
func (t *OutboundTransformer) buildImageGenerationAPIRequest(chatReq *llm.Request) (*httpclient.Request, error) {
	chatReq.Stream = lo.ToPtr(false)

	// Check if there are images in the messages - ZAI doesn't support image editing
	hasImages := hasImagesInMessages(chatReq.Messages)
	if hasImages {
		return nil, fmt.Errorf("%w: ZAI does not support image editing with input images", transformer.ErrInvalidRequest)
	}

	// Use Image Generation API only
	rawReq, err := t.buildImageGenerateRequest(chatReq)
	if err != nil {
		return nil, err
	}

	if rawReq.TransformerMetadata == nil {
		rawReq.TransformerMetadata = map[string]any{}
	}

	rawReq.TransformerMetadata["outbound_format_type"] = string(llm.APIFormatOpenAIImageGeneration)
	// Save model to TransformerMetadata for response transformation
	rawReq.TransformerMetadata["model"] = chatReq.Model

	return rawReq, nil
}

// buildImageGenerateRequest builds request for ZAI Image Generation API.
func (t *OutboundTransformer) buildImageGenerateRequest(chatReq *llm.Request) (*httpclient.Request, error) {
	// Extract prompt from messages
	prompt, err := extractPromptFromMessages(chatReq.Messages)
	if err != nil {
		return nil, err
	}

	// Build request body according to ZAI API documentation
	reqBody := map[string]any{
		"model":  chatReq.Model,
		"prompt": prompt,
	}

	// Extract image generation parameters from tools
	for _, tool := range chatReq.Tools {
		if tool.Type == "image_generation" && tool.ImageGeneration != nil {
			// Map quality parameter
			if tool.ImageGeneration.Quality != "" {
				// ZAI supports: hd, standard
				quality := tool.ImageGeneration.Quality
				switch quality {
				case "high":
					quality = "hd"
				case "low", "":
					quality = "standard"
				}

				reqBody["quality"] = quality
			} else {
				// Default to standard
				reqBody["quality"] = "standard"
			}

			// Map size parameter
			if tool.ImageGeneration.Size != "" {
				reqBody["size"] = tool.ImageGeneration.Size
			} else {
				// Default to 1024x1024
				reqBody["size"] = "1024x1024"
			}

			// Map watermark parameter (ZAI uses watermark_enabled, we use Watermark)
			// ZAI default is true, our default is false, so we need to handle this
			if tool.ImageGeneration.Watermark {
				reqBody["watermark_enabled"] = true
			} else {
				// Only set to false if explicitly disabled
				reqBody["watermark_enabled"] = false
			}

			break
		}
	}

	// User ID from metadata (following the pattern from TransformRequest)
	if chatReq.Metadata != nil {
		if userID, exists := chatReq.Metadata["user_id"]; exists && userID != "" {
			userIDStr := userID
			if len(userIDStr) < 6 || len(userIDStr) > 128 {
				return nil, fmt.Errorf("user_id must be between 6 and 128 characters, got %d", len(userIDStr))
			}

			reqBody["user_id"] = userIDStr
		}
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

	// Build auth config
	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: t.APIKey,
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
	var imgResp ZAIImagesResponse
	if err := json.Unmarshal(httpResp.Body, &imgResp); err != nil {
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
	resp := &llm.Response{
		ID:      fmt.Sprintf("zai-img-%s", uuid.NewString()),
		Object:  "chat.completion",
		Created: imgResp.Created,
		Model:   model,
		Choices: make([]llm.Choice, 0, len(imgResp.Data)),
	}

	// Convert each image to a choice with image_url content
	for i, img := range imgResp.Data {
		// Download image and convert to base64 data URL
		imageDataURL, err := downloadImageToDataURL(ctx, img.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to download and convert image: %w", err)
		}

		choice := llm.Choice{
			Index: i,
			Message: &llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: imageDataURL,
							},
						},
					},
				},
			},
			FinishReason: lo.ToPtr("stop"),
		}

		resp.Choices = append(resp.Choices, choice)
	}

	return resp, nil
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
			log.Error(ctx, "failed to close response body", log.Cause(err))
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

// hasImagesInMessages checks if any message contains image content.
func hasImagesInMessages(messages []llm.Message) bool {
	for _, msg := range messages {
		if len(msg.Content.MultipleContent) > 0 {
			for _, part := range msg.Content.MultipleContent {
				if part.Type == "image_url" {
					return true
				}
			}
		}
	}

	return false
}

// extractPromptFromMessages extracts the text prompt from messages.
func extractPromptFromMessages(messages []llm.Message) (string, error) {
	for _, msg := range messages {
		if msg.Content.Content != nil {
			return *msg.Content.Content, nil
		}

		if len(msg.Content.MultipleContent) > 0 {
			for _, part := range msg.Content.MultipleContent {
				if part.Type == "text" && part.Text != nil {
					return *part.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("prompt is required for image generation")
}
