package doubao

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// Config holds all configuration for the Doubao outbound transformer.
type Config struct {
	// API configuration
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKey  string `json:"api_key,omitempty"`  // API key
}

// OutboundTransformer implements transformer.Outbound for Doubao format.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL string
	APIKey  string
}

// NewOutboundTransformer creates a new Doubao OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Doubao OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required for Doubao transformer")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for Doubao transformer")
	}

	baseURL := strings.TrimSuffix(config.BaseURL, "/")

	outbound, err := openai.NewOutboundTransformer(baseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Doubao outbound transformer: %w", err)
	}

	return &OutboundTransformer{
		Outbound: outbound,
		BaseURL:  baseURL,
		APIKey:   config.APIKey,
	}, nil
}

type Request struct {
	llm.Request

	UserID    string    `json:"user_id,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Thinking  *Thinking `json:"thinking,omitempty"`
}

type Thinking struct {
	// Enable or disable thinking.
	// enabled | disabled.
	Type string `json:"type"`
}

// TransformRequest transforms ChatCompletionRequest to Request.
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
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// If this is an image generation request, use the Doubao Image Generation API
	if chatReq.IsImageGenerationRequest() {
		return t.buildImageGenerationAPIRequest(chatReq)
	}

	// Create Doubao-specific request by removing Metadata and adding request_id/user_id
	doubaoReq := Request{
		Request:   *chatReq,
		UserID:    "",
		RequestID: "",
	}

	if chatReq.Metadata != nil {
		doubaoReq.UserID = chatReq.Metadata["user_id"]
		doubaoReq.RequestID = chatReq.Metadata["request_id"]
	}

	// Generate request ID if not provided
	if doubaoReq.RequestID == "" {
		// Use timestamp as fallback
		doubaoReq.RequestID = fmt.Sprintf("req_%d", time.Now().Unix())
	}

	// Doubao request does not support metadata
	doubaoReq.Metadata = nil

	body, err := json.Marshal(doubaoReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: t.APIKey,
	}

	baseURL := strings.TrimSuffix(t.BaseURL, "/")
	url := baseURL + "/chat/completions"

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

// buildImageGenerationAPIRequest builds the HTTP request to call the Doubao Image Generation API.
// Doubao uses only /images/generations API for both generation and editing.
func (t *OutboundTransformer) buildImageGenerationAPIRequest(chatReq *llm.Request) (*httpclient.Request, error) {
	chatReq.Stream = lo.ToPtr(false)

	// Extract prompt from messages
	prompt, err := extractPromptFromMessages(chatReq.Messages)
	if err != nil {
		return nil, err
	}

	// Check if there are images in the messages (for editing)
	hasImages := hasImagesInMessages(chatReq.Messages)

	// Build request body - Doubao uses /images/generations for both generation and editing
	reqBody := map[string]any{
		"model":           chatReq.Model,
		"prompt":          prompt,
		"response_format": "b64_json",
		"stream":          false,
	}

	// Add images if present (for editing)
	if hasImages {
		images, err := extractImages(chatReq)
		if err != nil {
			return nil, err
		}

		if len(images) > 0 {
			if len(images) == 1 {
				reqBody["image"] = images[0]
			} else {
				reqBody["image"] = images
			}
		}
	}

	// Extract image generation parameters from tools
	for _, tool := range chatReq.Tools {
		if tool.Type == "image_generation" && tool.ImageGeneration != nil {
			// Map OpenAI parameters to Doubao parameters
			if tool.ImageGeneration.Size != "" {
				reqBody["size"] = tool.ImageGeneration.Size
			}

			// Map quality to guidance_scale
			switch tool.ImageGeneration.Quality {
			case "hd":
				reqBody["guidance_scale"] = 7.5
			case "standard":
				reqBody["guidance_scale"] = 2.5
			}

			// Add watermark if specified
			if tool.ImageGeneration.Watermark {
				reqBody["watermark"] = true
			}

			break
		}
	}

	// Add user if specified
	if chatReq.User != nil {
		reqBody["user"] = *chatReq.User
	}

	var (
		body    []byte
		headers http.Header
	)

	// Use JSON for generation only
	body, err = json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	headers = make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	url := t.BaseURL + "/images/generations"

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: t.APIKey,
	}

	request := &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		ContentType: "application/json",
		Headers:     headers,
		Body:        body,
		Auth:        auth,
	}

	// Add metadata for response transformation
	if request.Metadata == nil {
		request.Metadata = map[string]string{}
	}

	request.Metadata["outbound_format_type"] = llm.APIFormatOpenAIImageGeneration.String()
	request.Metadata["model"] = chatReq.Model

	return request, nil
}

// Helper functions adapted from openai/image.go

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

// extractImages extracts images from messages and returns them as base64 data URLs.
func extractImages(chatReq *llm.Request) ([]string, error) {
	var images []string

	for _, msg := range chatReq.Messages {
		if len(msg.Content.MultipleContent) > 0 {
			for _, part := range msg.Content.MultipleContent {
				if part.Type == "image_url" && part.ImageURL != nil {
					// Convert to Doubao format if needed
					if strings.HasPrefix(part.ImageURL.URL, "data:") {
						// Already in data URL format, just validate and use as-is
						images = append(images, part.ImageURL.URL)
					} else {
						// Regular URL - Doubao supports both URL and base64
						images = append(images, part.ImageURL.URL)
					}
				}
			}
		}
	}

	return images, nil
}
