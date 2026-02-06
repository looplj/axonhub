package doubao

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config holds all configuration for the Doubao outbound transformer.
type Config struct {
	// API configuration
	BaseURL        string              `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKeyProvider auth.APIKeyProvider `json:"-"`                  // API key provider
}

// OutboundTransformer implements transformer.Outbound for Doubao format.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL        string
	APIKeyProvider auth.APIKeyProvider
}

// NewOutboundTransformer creates a new Doubao OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required for Doubao transformer")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for Doubao transformer")
	}

	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Doubao OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required for Doubao transformer")
	}

	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("API key provider is required for Doubao transformer")
	}

	baseURL := transformer.NormalizeBaseURL(config.BaseURL, "v3")

	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
	}

	outbound, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Doubao outbound transformer: %w", err)
	}

	return &OutboundTransformer{
		Outbound:       outbound,
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
	}, nil
}

type Request struct {
	openai.Request

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
		return t.buildImageGenerationAPIRequest(llmReq)
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Convert llm.Request to openai.Request first
	oaiReq := openai.RequestFromLLM(llmReq)

	// Create Doubao-specific request by adding request_id/user_id
	doubaoReq := Request{
		Request:   *oaiReq,
		UserID:    "",
		RequestID: "",
	}

	if llmReq.Metadata != nil {
		doubaoReq.UserID = llmReq.Metadata["user_id"]
		doubaoReq.RequestID = llmReq.Metadata["request_id"]
	}

	// Generate request ID if not provided
	if doubaoReq.RequestID == "" {
		// Use timestamp as fallback
		doubaoReq.RequestID = fmt.Sprintf("req_%d", time.Now().Unix())
	}

	// Doubao request does not support metadata (extracted to user_id/request_id)
	doubaoReq.Metadata = nil

	body, err := json.Marshal(doubaoReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Get API key from provider
	apiKey := t.APIKeyProvider.Get(ctx)

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: apiKey,
	}

	url := t.BaseURL + "/chat/completions"

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
func (t *OutboundTransformer) buildImageGenerationAPIRequest(llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq.Image == nil {
		return nil, fmt.Errorf("image request is required")
	}

	prompt := llmReq.Image.Prompt
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required for image generation")
	}

	hasImages := len(llmReq.Image.Images) > 0

	var images []string
	if hasImages {
		images = lo.Map(llmReq.Image.Images, func(b []byte, _ int) string {
			return encodeImageBytesToDataURL(b)
		})
	}

	// Build request body - Doubao uses /images/generations for both generation and editing
	reqBody := map[string]any{
		"model":           llmReq.Model,
		"prompt":          prompt,
		"response_format": "b64_json",
		"stream":          false,
	}

	// Add images if present (for editing)
	if hasImages {
		if len(images) == 1 {
			reqBody["image"] = images[0]
		} else {
			reqBody["image"] = images
		}
	}

	if llmReq.Image.N != nil {
		reqBody["n"] = *llmReq.Image.N
	}

	if llmReq.Image.Size != "" {
		reqBody["size"] = llmReq.Image.Size
	}

	switch llmReq.Image.Quality {
	case "hd":
		reqBody["guidance_scale"] = 7.5
	case "standard":
		reqBody["guidance_scale"] = 2.5
	}

	if llmReq.Image.ResponseFormat != "" {
		reqBody["response_format"] = llmReq.Image.ResponseFormat
	}

	if llmReq.Image.User != "" {
		reqBody["user"] = llmReq.Image.User
	}

	// Use JSON for generation only
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	url := t.BaseURL + "/images/generations"

	// Get API key from provider
	apiKey := t.APIKeyProvider.Get(context.Background())

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: apiKey,
	}

	request := &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		ContentType: "application/json",
		Headers:     headers,
		Body:        body,
		Auth:        auth,
		RequestType: string(llm.RequestTypeImage),
		APIFormat:   string(llm.APIFormatOpenAIImageGeneration),
	}

	// Add TransformerMetadata for response transformation
	if request.TransformerMetadata == nil {
		request.TransformerMetadata = map[string]any{}
	}
	request.TransformerMetadata["model"] = llmReq.Model

	return request, nil
}

func encodeImageBytesToDataURL(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}
