// Package minimax implements MiniMax's OpenAI-compatible chat and image APIs.
package minimax

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config configures the MiniMax outbound transformer.
type Config struct {
	BaseURL        string              `json:"base_url,omitempty"`
	EndpointPath   string              `json:"endpoint_path,omitempty"`
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

// OutboundTransformer handles chat through OpenAI compatibility and MiniMax image generation.
type OutboundTransformer struct {
	transformer.Outbound
	baseURL      string
	endpointPath string
	apiKeys      auth.APIKeyProvider
}

func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	return NewOutboundTransformerWithConfig(&Config{BaseURL: baseURL, APIKeyProvider: auth.NewStaticKeyProvider(apiKey)})
}

func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config == nil || strings.TrimSpace(config.BaseURL) == "" {
		return nil, fmt.Errorf("base URL is required for MiniMax transformer")
	}
	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("API key provider is required for MiniMax transformer")
	}
	oai, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
		ReasoningField: openai.ReasoningFieldContent,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid MiniMax transformer configuration: %w", err)
	}
	return &OutboundTransformer{Outbound: oai, baseURL: transformer.NormalizeBaseURL(config.BaseURL, "v1"), endpointPath: config.EndpointPath, apiKeys: config.APIKeyProvider}, nil
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	if req != nil && req.RequestType == llm.RequestTypeImage {
		return t.buildImageRequest(ctx, req)
	}
	return t.Outbound.TransformRequest(ctx, req)
}

func (t *OutboundTransformer) buildImageRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	if req.Image == nil {
		return nil, fmt.Errorf("%w: image request is required", transformer.ErrInvalidRequest)
	}
	if strings.TrimSpace(req.Image.Prompt) == "" {
		return nil, fmt.Errorf("%w: prompt is required for image generation", transformer.ErrInvalidRequest)
	}
	model := req.Model
	if model == "" || model == "dall-e-2" && !req.Image.ModelSpecified {
		model = "image-01"
	}
	body := map[string]any{"model": model, "prompt": req.Image.Prompt}
	if len(req.Image.SubjectReference) > 0 {
		var refs any
		if err := json.Unmarshal(req.Image.SubjectReference, &refs); err != nil {
			return nil, fmt.Errorf("%w: invalid subject_reference: %v", transformer.ErrInvalidRequest, err)
		}
		body["subject_reference"] = refs
	}
	if req.Image.AspectRatio != "" {
		body["aspect_ratio"] = req.Image.AspectRatio
	}
	if req.Image.Size != "" && (req.Image.Width == nil || req.Image.Height == nil) {
		widthText, heightText, ok := strings.Cut(req.Image.Size, "x")
		width, widthErr := strconv.ParseInt(widthText, 10, 64)
		height, heightErr := strconv.ParseInt(heightText, 10, 64)
		if !ok || widthErr != nil || heightErr != nil || width < 512 || width > 2048 || width%8 != 0 || height < 512 || height > 2048 || height%8 != 0 {
			return nil, fmt.Errorf("%w: unsupported MiniMax image size %q", transformer.ErrInvalidRequest, req.Image.Size)
		}
		if req.Image.Width == nil {
			body["width"] = width
		}
		if req.Image.Height == nil {
			body["height"] = height
		}
	}
	if req.Image.Width != nil {
		body["width"] = *req.Image.Width
	}
	if req.Image.Height != nil {
		body["height"] = *req.Image.Height
	}
	if req.Seed != nil {
		body["seed"] = *req.Seed
	}
	if req.Image.N != nil {
		body["n"] = *req.Image.N
	}
	if req.Image.PromptOptimizer != nil {
		body["prompt_optimizer"] = *req.Image.PromptOptimizer
	}
	format := req.Image.ResponseFormat
	if format == "b64_json" {
		format = "base64"
	}
	if format == "" {
		format = "url"
	}
	body["response_format"] = format
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MiniMax image request: %w", err)
	}
	path := t.endpointPath
	if path == "" {
		path = "/image_generation"
	}
	url := t.baseURL + path
	if strings.HasPrefix(path, "/v1/") && strings.HasSuffix(t.baseURL, "/v1") {
		url = strings.TrimSuffix(t.baseURL, "/v1") + path
	}
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "application/json")
	return &httpclient.Request{
		Method: http.MethodPost, URL: url, Headers: h, Body: raw,
		Auth:        &httpclient.AuthConfig{Type: "bearer", APIKey: t.apiKeys.Get(ctx)},
		RequestType: llm.RequestTypeImage.String(), APIFormat: llm.APIFormatOpenAIImageGeneration.String(),
		TransformerMetadata: map[string]any{"model": model},
	}, nil
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
	if resp == nil || resp.Request == nil || resp.Request.RequestType != llm.RequestTypeImage.String() {
		return t.Outbound.TransformResponse(ctx, resp)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("MiniMax image API returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		ID       string `json:"id"`
		Created  int64  `json:"created"`
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
		Data struct {
			ImageURLs   []string `json:"image_urls"`
			ImageBase64 []string `json:"image_base64"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode MiniMax image response: %w", err)
	}
	if payload.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("%w: MiniMax image API status %d: %s", transformer.ErrInvalidResponse, payload.BaseResp.StatusCode, payload.BaseResp.StatusMsg)
	}
	created := payload.Created
	if created == 0 {
		created = time.Now().Unix()
	}
	model := "image-01"
	if m, ok := resp.Request.TransformerMetadata["model"].(string); ok && m != "" {
		model = m
	}
	data := make([]llm.ImageData, 0, len(payload.Data.ImageURLs)+len(payload.Data.ImageBase64))
	for _, url := range payload.Data.ImageURLs {
		data = append(data, llm.ImageData{URL: url})
	}
	for _, b64 := range payload.Data.ImageBase64 {
		data = append(data, llm.ImageData{B64JSON: b64})
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: MiniMax image response contained no images", transformer.ErrInvalidResponse)
	}
	id := payload.ID
	if id == "" {
		id = fmt.Sprintf("minimax-img-%d", created)
	}
	return &llm.Response{ID: id, Object: "image.generation", Created: created, Model: model, RequestType: llm.RequestTypeImage, APIFormat: llm.APIFormatOpenAIImageGeneration, Image: &llm.ImageResponse{Created: created, Data: data}}, nil
}
