package doubao

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	oai "github.com/looplj/axonhub/llm/transformer/openai"
)

// VideoInboundTransformer handles Seedance-compatible API payloads.
type VideoInboundTransformer struct{}

func NewVideoInboundTransformer() *VideoInboundTransformer {
	return &VideoInboundTransformer{}
}

type seedanceCreateRequest struct {
	Model   string           `json:"model"`
	Content []llm.VideoContent `json:"content"`

	Duration   *int64 `json:"duration,omitempty"`
	Ratio      string `json:"ratio,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Frames     *int64 `json:"frames,omitempty"`
	Seed       *int64 `json:"seed,omitempty"`

	GenerateAudio         *bool  `json:"generate_audio,omitempty"`
	CameraFixed           *bool  `json:"camera_fixed,omitempty"`
	Watermark             *bool  `json:"watermark,omitempty"`
	Draft                 *bool  `json:"draft,omitempty"`
	ServiceTier           string `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *int64 `json:"execution_expires_after,omitempty"`
}

func (t *VideoInboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}
	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	contentType := httpReq.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	var req seedanceCreateRequest
	if err := json.Unmarshal(httpReq.Body, &req); err != nil {
		return nil, fmt.Errorf("%w: failed to decode seedance video request: %w", transformer.ErrInvalidRequest, err)
	}

	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}
	if len(req.Content) == 0 {
		return nil, fmt.Errorf("%w: content is required", transformer.ErrInvalidRequest)
	}

	videoReq := &llm.VideoRequest{
		Model:                 req.Model,
		Content:               req.Content,
		Duration:              req.Duration,
		Ratio:                 req.Ratio,
		Resolution:            req.Resolution,
		Frames:                req.Frames,
		Seed:                  req.Seed,
		GenerateAudio:         req.GenerateAudio,
		CameraFixed:           req.CameraFixed,
		Watermark:             req.Watermark,
		Draft:                 req.Draft,
		ServiceTier:           req.ServiceTier,
		ExecutionExpiresAfter: req.ExecutionExpiresAfter,
	}

	return &llm.Request{
		Model:       req.Model,
		Messages:    []llm.Message{},
		Modalities:  []string{"video"},
		Stream:      lo.ToPtr(false),
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatSeedanceVideo,
		Video:       videoReq,
	}, nil
}

type seedanceCreateResponseInbound struct {
	ID string `json:"id"`
}

type seedanceGetResponseInbound struct {
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

func (t *VideoInboundTransformer) TransformResponse(ctx context.Context, llmResp *llm.Response) (*httpclient.Response, error) {
	if llmResp == nil || llmResp.Video == nil {
		return nil, fmt.Errorf("%w: video response is nil", transformer.ErrInvalidResponse)
	}

	v := llmResp.Video

	// For Seedance endpoints, create response returns only {id}.
	if llmResp.Object == "video.create" {
		body, err := json.Marshal(seedanceCreateResponseInbound{ID: v.ID})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal seedance create response: %w", err)
		}

		return &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       body,
			Headers: http.Header{
				"Content-Type":  []string{"application/json"},
				"Cache-Control": []string{"no-cache"},
			},
		}, nil
	}

	resp := seedanceGetResponseInbound{
		ID:         v.ID,
		Model:      v.Model,
		Status:     v.Status,
		CreatedAt:  v.CreatedAt,
		UpdatedAt:  lo.Ternary(v.CompletedAt != 0, v.CompletedAt, time.Now().Unix()),
		Seed:       v.Seed,
		Resolution: v.Resolution,
		Ratio:      v.Ratio,
		Duration:   v.Duration,
		ServiceTier: "",
	}

	if v.VideoURL != "" {
		resp.Content = &struct {
			VideoURL string `json:"video_url,omitempty"`
		}{VideoURL: v.VideoURL}
	}

	if llmResp.Usage != nil {
		resp.Usage = &struct {
			CompletionTokens int64 `json:"completion_tokens,omitempty"`
			TotalTokens      int64 `json:"total_tokens,omitempty"`
		}{
			CompletionTokens: llmResp.Usage.CompletionTokens,
			TotalTokens:      llmResp.Usage.TotalTokens,
		}
	}

	if v.FPS != nil {
		resp.FramesPerSecond = v.FPS
	}

	body, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal seedance get response: %w", err)
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

func (t *VideoInboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: video request does not support streaming", transformer.ErrInvalidRequest)
}

func (t *VideoInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	// Reuse OpenAI error format for now.
	return oai.NewInboundTransformer().TransformError(ctx, rawErr)
}

func (t *VideoInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("%w: video request does not support streaming", transformer.ErrInvalidRequest)
}
