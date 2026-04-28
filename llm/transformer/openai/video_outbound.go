package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

type openAIVideoCreateResponse struct {
	ID     string `json:"id"`
	Object string `json:"object,omitempty"`
	Status string `json:"status,omitempty"`
	Error  *struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
}

func (t *OutboundTransformer) buildVideoGenerationAPIRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil || llmReq.Video == nil {
		return nil, fmt.Errorf("%w: video request is required", transformer.ErrInvalidRequest)
	}

	video := llmReq.Video

	if strings.TrimSpace(llmReq.Model) == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	prompt := firstVideoText(video.Content)
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("%w: prompt is required", transformer.ErrInvalidRequest)
	}

	if llmReq.TransformerMetadata == nil {
		llmReq.TransformerMetadata = map[string]any{}
	}

	llmReq.TransformerMetadata["video_prompt"] = prompt

	bodyMap := map[string]any{
		"model":  llmReq.Model,
		"prompt": prompt,
	}

	if ref := firstVideoImageURL(video.Content); ref != "" {
		bodyMap["input_reference"] = ref
	}

	if video.Duration != nil {
		bodyMap["seconds"] = *video.Duration
	}

	if video.Size != "" {
		bodyMap["size"] = video.Size
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal video request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	apiKey := t.config.APIKeyProvider.Get(ctx)
	authConfig := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	url := t.config.BaseURL + "/videos"

	req := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    authConfig,
	}

	req.RequestType = llm.RequestTypeVideo.String()
	req.APIFormat = llm.APIFormatOpenAIVideo.String()

	if req.TransformerMetadata == nil {
		req.TransformerMetadata = map[string]any{}
	}

	req.TransformerMetadata["model"] = llmReq.Model
	req.TransformerMetadata["video_prompt"] = prompt

	return req, nil
}

func firstVideoText(content []llm.VideoContent) string {
	for _, c := range content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			return c.Text
		}
	}

	return ""
}

func firstVideoImageURL(content []llm.VideoContent) string {
	for _, c := range content {
		if c.Type == "image_url" && c.ImageURL != nil && strings.TrimSpace(c.ImageURL.URL) != "" {
			return c.ImageURL.URL
		}
	}

	return ""
}

func transformVideoResponse(httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("%w: http response is nil", transformer.ErrInvalidResponse)
	}

	var createResp openAIVideoCreateResponse
	if err := json.Unmarshal(httpResp.Body, &createResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal openai video response: %w", err)
	}

	if createResp.ID == "" {
		return nil, fmt.Errorf("%w: missing id in openai video response", transformer.ErrInvalidResponse)
	}

	if createResp.Error != nil && strings.TrimSpace(createResp.Error.Message) != "" {
		return nil, &llm.ResponseError{
			StatusCode: http.StatusBadRequest,
			Detail: llm.ErrorDetail{
				Code:    createResp.Error.Code,
				Message: createResp.Error.Message,
				Type:    "video_generation_error",
			},
		}
	}

	model := "video-generation"

	var prompt string

	if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if m, ok := httpResp.Request.TransformerMetadata["model"].(string); ok && m != "" {
			model = m
		}

		if p, ok := httpResp.Request.TransformerMetadata["video_prompt"].(string); ok && p != "" {
			prompt = p
		}
	}

	status := normalizeVideoStatusOpenAI(createResp.Status)

	videoResp := &llm.VideoResponse{
		ID:        createResp.ID,
		Status:    status,
		Model:     model,
		Prompt:    prompt,
		CreatedAt: time.Now().Unix(),
	}

	return &llm.Response{
		ID:          createResp.ID,
		Object:      "video",
		Created:     time.Now().Unix(),
		Model:       model,
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatOpenAIVideo,
		Video:       videoResp,
		Choices:     []llm.Choice{},
	}, nil
}

func normalizeVideoStatusOpenAI(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued":
		return "queued"
	case "in_progress":
		return "running"
	case "completed":
		return "succeeded"
	case "failed":
		return "failed"
	default:
		if status == "" {
			return "queued"
		}

		return strings.ToLower(status)
	}
}

func (t *OutboundTransformer) BuildGetVideoTaskRequest(ctx context.Context, providerTaskID string) (*httpclient.Request, error) {
	if strings.TrimSpace(providerTaskID) == "" {
		return nil, fmt.Errorf("%w: providerTaskID is required", transformer.ErrInvalidRequest)
	}

	apiKey := t.config.APIKeyProvider.Get(ctx)
	authConfig := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	url := t.config.BaseURL + "/videos/" + providerTaskID

	return &httpclient.Request{
		Method:      http.MethodGet,
		URL:         url,
		Headers:     http.Header{"Accept": []string{"application/json"}},
		Auth:        authConfig,
		RequestType: llm.RequestTypeVideo.String(),
		APIFormat:   llm.APIFormatOpenAIVideo.String(),
	}, nil
}

type openAIVideoGetResponse struct {
	ID          string   `json:"id"`
	Object      string   `json:"object,omitempty"`
	Status      string   `json:"status,omitempty"`
	Model       string   `json:"model,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
	Seconds     *int64   `json:"seconds,omitempty"`
	Size        string   `json:"size,omitempty"`
	Progress    *float64 `json:"progress,omitempty"`
	VideoURL    string   `json:"video_url,omitempty"`
	CreatedAt   int64    `json:"created_at,omitempty"`
	CompletedAt *int64   `json:"completed_at,omitempty"`
	ExpiresAt   *int64   `json:"expires_at,omitempty"`
	Error       *struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
	Usage *struct {
		CompletionTokens int64 `json:"completion_tokens,omitempty"`
		TotalTokens      int64 `json:"total_tokens,omitempty"`
	} `json:"usage,omitempty"`
}

func (t *OutboundTransformer) ParseGetVideoTaskResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("%w: http response is nil", transformer.ErrInvalidResponse)
	}

	var resp openAIVideoGetResponse
	if err := json.Unmarshal(httpResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal openai video get response: %w", err)
	}

	if resp.ID == "" {
		return nil, fmt.Errorf("%w: missing id in openai video response", transformer.ErrInvalidResponse)
	}

	status := normalizeVideoStatusOpenAI(resp.Status)

	var completedAt int64
	if resp.CompletedAt != nil {
		completedAt = *resp.CompletedAt
	}

	var expiresAt int64
	if resp.ExpiresAt != nil {
		expiresAt = *resp.ExpiresAt
	}

	v := &llm.VideoResponse{
		ID:          resp.ID,
		Status:      status,
		VideoURL:    resp.VideoURL,
		Progress:    resp.Progress,
		Model:       resp.Model,
		Prompt:      resp.Prompt,
		Duration:    resp.Seconds,
		Size:        resp.Size,
		CreatedAt:   resp.CreatedAt,
		CompletedAt: completedAt,
		ExpiresAt:   expiresAt,
	}

	if resp.Error != nil && strings.TrimSpace(resp.Error.Message) != "" {
		v.Error = &llm.VideoError{
			Code:    resp.Error.Code,
			Message: resp.Error.Message,
		}
	}

	llmResp := &llm.Response{
		RequestType: llm.RequestTypeVideo,
		Video:       v,
	}

	if resp.Usage != nil {
		llmResp.Usage = &llm.Usage{
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	return llmResp, nil
}

func (t *OutboundTransformer) BuildDeleteVideoTaskRequest(ctx context.Context, providerTaskID string) (*httpclient.Request, error) {
	if strings.TrimSpace(providerTaskID) == "" {
		return nil, fmt.Errorf("%w: providerTaskID is required", transformer.ErrInvalidRequest)
	}

	apiKey := t.config.APIKeyProvider.Get(ctx)
	authConfig := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	url := t.config.BaseURL + "/videos/" + providerTaskID

	return &httpclient.Request{
		Method:      http.MethodDelete,
		URL:         url,
		Headers:     http.Header{"Accept": []string{"application/json"}},
		Auth:        authConfig,
		RequestType: llm.RequestTypeVideo.String(),
		APIFormat:   llm.APIFormatOpenAIVideo.String(),
	}, nil
}

var _ transformer.VideoTaskOutbound = (*OutboundTransformer)(nil)
