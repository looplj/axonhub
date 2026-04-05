package doubao

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

type seedanceCreateResponse struct {
	ID string `json:"id"`
}

func (t *OutboundTransformer) buildVideoGenerationAPIRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil || llmReq.Video == nil {
		return nil, fmt.Errorf("%w: video request is required", transformer.ErrInvalidRequest)
	}

	video := llmReq.Video

	if strings.TrimSpace(llmReq.Model) == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(video.Content) == 0 {
		return nil, fmt.Errorf("%w: content is required", transformer.ErrInvalidRequest)
	}

	// Seedance does not accept size; map common sizes to ratio+resolution if needed.
	ratio := strings.TrimSpace(video.Ratio)
	resolution := strings.TrimSpace(video.Resolution)
	if ratio == "" && resolution == "" && strings.TrimSpace(video.Size) != "" {
		r, res, ok := inferSeedanceRatioResolution(video.Size)
		if !ok {
			return nil, fmt.Errorf("%w: size %q cannot be mapped to ratio/resolution, please set ratio and resolution", transformer.ErrInvalidRequest, video.Size)
		}
		ratio, resolution = r, res
	}

	reqBody := map[string]any{
		"model":   llmReq.Model,
		"content": video.Content,
	}

	if video.Duration != nil {
		reqBody["duration"] = *video.Duration
	}
	if ratio != "" {
		reqBody["ratio"] = ratio
	}
	if resolution != "" {
		reqBody["resolution"] = resolution
	}
	if video.Frames != nil {
		reqBody["frames"] = *video.Frames
	}
	if video.Seed != nil {
		reqBody["seed"] = *video.Seed
	}
	if video.GenerateAudio != nil {
		reqBody["generate_audio"] = *video.GenerateAudio
	}
	if video.CameraFixed != nil {
		reqBody["camera_fixed"] = *video.CameraFixed
	}
	if video.Watermark != nil {
		reqBody["watermark"] = *video.Watermark
	}
	if video.Draft != nil {
		reqBody["draft"] = *video.Draft
	}
	if strings.TrimSpace(video.ServiceTier) != "" {
		reqBody["service_tier"] = video.ServiceTier
	}
	if video.ExecutionExpiresAfter != nil {
		reqBody["execution_expires_after"] = *video.ExecutionExpiresAfter
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal seedance video request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	apiKey := t.APIKeyProvider.Get(ctx)
	authConfig := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	url := t.BaseURL + "/contents/generations/tasks"

	req := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    authConfig,
	}

	req.RequestType = llm.RequestTypeVideo.String()
	req.APIFormat = llm.APIFormatSeedanceVideo.String()

	if req.TransformerMetadata == nil {
		req.TransformerMetadata = map[string]any{}
	}
	req.TransformerMetadata["model"] = llmReq.Model

	return req, nil
}

func inferSeedanceRatioResolution(size string) (string, string, bool) {
	w, h, ok := parseSize(size)
	if !ok {
		return "", "", false
	}

	switch {
	case w == 1280 && h == 720:
		return "16:9", "720p", true
	case w == 720 && h == 1280:
		return "9:16", "720p", true
	case w == 1920 && h == 1080:
		return "16:9", "1080p", true
	case w == 1080 && h == 1920:
		return "9:16", "1080p", true
	case w == 640 && h == 480:
		return "4:3", "480p", true
	case w == 480 && h == 640:
		return "3:4", "480p", true
	default:
		return "", "", false
	}
}

func parseSize(size string) (int, int, bool) {
	size = strings.TrimSpace(strings.ToLower(size))
	before, after, ok := strings.Cut(size, "x")
	if !ok {
		return 0, 0, false
	}

	w, err := strconv.Atoi(strings.TrimSpace(before))
	if err != nil || w <= 0 {
		return 0, 0, false
	}
	h, err := strconv.Atoi(strings.TrimSpace(after))
	if err != nil || h <= 0 {
		return 0, 0, false
	}

	return w, h, true
}

// BuildGetVideoTaskRequest implements transformer.VideoTaskOutbound.
func (t *OutboundTransformer) BuildGetVideoTaskRequest(ctx context.Context, providerTaskID string) (*httpclient.Request, error) {
	if strings.TrimSpace(providerTaskID) == "" {
		return nil, fmt.Errorf("%w: providerTaskID is required", transformer.ErrInvalidRequest)
	}

	apiKey := t.APIKeyProvider.Get(ctx)
	authConfig := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	return &httpclient.Request{
		Method:      http.MethodGet,
		URL:         t.BaseURL + "/contents/generations/tasks/" + providerTaskID,
		Headers:     http.Header{"Accept": []string{"application/json"}},
		Auth:        authConfig,
		RequestType: llm.RequestTypeVideo.String(),
		APIFormat:   llm.APIFormatSeedanceVideo.String(),
	}, nil
}

type seedanceGetResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model,omitempty"`
	Status string `json:"status,omitempty"`

	Content *struct {
		VideoURL string `json:"video_url,omitempty"`
	} `json:"content,omitempty"`

	Usage *struct {
		CompletionTokens int64 `json:"completion_tokens,omitempty"`
		TotalTokens      int64 `json:"total_tokens,omitempty"`
	} `json:"usage,omitempty"`

	CreatedAt       int64  `json:"created_at,omitempty"`
	UpdatedAt       int64  `json:"updated_at,omitempty"`
	Seed            *int64 `json:"seed,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
	Ratio           string `json:"ratio,omitempty"`
	Duration        *int64 `json:"duration,omitempty"`
	FramesPerSecond *int64 `json:"framespersecond,omitempty"`
	ServiceTier     string `json:"service_tier,omitempty"`
}

// ParseGetVideoTaskResponse implements transformer.VideoTaskOutbound.
func (t *OutboundTransformer) ParseGetVideoTaskResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("%w: http response is nil", transformer.ErrInvalidResponse)
	}

	var resp seedanceGetResponse
	if err := json.Unmarshal(httpResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal seedance video response: %w", err)
	}

	if strings.TrimSpace(resp.ID) == "" {
		return nil, fmt.Errorf("%w: missing id in seedance response", transformer.ErrInvalidResponse)
	}

	status := strings.ToLower(strings.TrimSpace(resp.Status))
	if status == "" {
		status = "queued"
	}

	var completedAt int64
	if status == "succeeded" || status == "failed" {
		if resp.UpdatedAt != 0 {
			completedAt = resp.UpdatedAt
		} else {
			completedAt = time.Now().Unix()
		}
	}

	v := &llm.VideoResponse{
		ID:         resp.ID,
		Status:     status,
		Model:      resp.Model,
		CreatedAt:  resp.CreatedAt,
		CompletedAt: completedAt,
		Ratio:      resp.Ratio,
		Resolution: resp.Resolution,
		Duration:   resp.Duration,
		FPS:        resp.FramesPerSecond,
		Seed:       resp.Seed,
	}

	if resp.Content != nil {
		v.VideoURL = resp.Content.VideoURL
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

// BuildDeleteVideoTaskRequest implements transformer.VideoTaskOutbound.
func (t *OutboundTransformer) BuildDeleteVideoTaskRequest(ctx context.Context, providerTaskID string) (*httpclient.Request, error) {
	if strings.TrimSpace(providerTaskID) == "" {
		return nil, fmt.Errorf("%w: providerTaskID is required", transformer.ErrInvalidRequest)
	}

	apiKey := t.APIKeyProvider.Get(ctx)
	authConfig := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	return &httpclient.Request{
		Method:      http.MethodDelete,
		URL:         t.BaseURL + "/contents/generations/tasks/" + providerTaskID,
		Headers:     http.Header{"Accept": []string{"application/json"}},
		Auth:        authConfig,
		RequestType: llm.RequestTypeVideo.String(),
		APIFormat:   llm.APIFormatSeedanceVideo.String(),
	}, nil
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	// Delegate to wrapped OpenAI transformer for non-video requests.
	if httpResp == nil || httpResp.Request == nil || httpResp.Request.RequestType != llm.RequestTypeVideo.String() {
		return t.Outbound.TransformResponse(ctx, httpResp)
	}

	// Video create returns {id}.
	var resp seedanceCreateResponse
	if err := json.Unmarshal(httpResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal seedance create response: %w", err)
	}

	if strings.TrimSpace(resp.ID) == "" {
		return nil, fmt.Errorf("%w: missing id in seedance create response", transformer.ErrInvalidResponse)
	}

	return &llm.Response{
		ID:          resp.ID, // provider task id for persistence
		Object:      "video.create",
		Created:     time.Now().Unix(),
		Model:       llmReqModelOrFallback(httpResp),
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatSeedanceVideo,
		Choices:     []llm.Choice{},
		Video: &llm.VideoResponse{
			ID:        resp.ID,
			Status:    "queued",
			Model:     llmReqModelOrFallback(httpResp),
			CreatedAt: time.Now().Unix(),
		},
	}, nil
}

func llmReqModelOrFallback(httpResp *httpclient.Response) string {
	if httpResp != nil && httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if m, ok := httpResp.Request.TransformerMetadata["model"].(string); ok && m != "" {
			return m
		}
	}

	return ""
}

var (
	_ transformer.VideoTaskOutbound     = (*OutboundTransformer)(nil)
)
