package openai

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
)

type VideoCreateRequest struct {
	Model                 string             `json:"model"`
	Prompt                string             `json:"prompt"`
	InputReference        string             `json:"input_reference,omitempty"`
	Content               []llm.VideoContent `json:"content,omitempty"`
	Seconds               *string            `json:"seconds,omitempty"`
	Size                  string             `json:"size,omitempty"`
	Ratio                 string             `json:"ratio,omitempty"`
	Resolution            string             `json:"resolution,omitempty"`
	ServiceTier           string             `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *int64             `json:"execution_expires_after,omitempty"`
	CallbackURL           string             `json:"callback_url,omitempty"`
	ReturnLastFrame       *bool              `json:"return_last_frame,omitempty"`
	Tools                 []json.RawMessage  `json:"tools,omitempty"`
	ExtraBody             json.RawMessage    `json:"extra_body,omitempty"`
}

type VideoInboundTransformer struct{}

func NewVideoInboundTransformer() *VideoInboundTransformer {
	return &VideoInboundTransformer{}
}

// APIFormat returns the API format of the transformer.
func (t *VideoInboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIVideo
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

	var req VideoCreateRequest

	switch {
	case strings.Contains(strings.ToLower(contentType), "application/json"):
		parsed, err := parseVideoJSONRequest(httpReq.Body)
		if err != nil {
			return nil, err
		}
		req = *parsed
	case strings.HasPrefix(strings.ToLower(contentType), "multipart/"):
		parsed, err := parseVideoMultipartRequest(httpReq)
		if err != nil {
			return nil, err
		}

		req = *parsed
	default:
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(req.Content) == 0 && strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("%w: prompt is required", transformer.ErrInvalidRequest)
	}

	content := req.Content
	if len(content) == 0 {
		content = []llm.VideoContent{{Type: "text", Text: req.Prompt}}
		if strings.TrimSpace(req.InputReference) != "" {
			content = append(content, llm.VideoContent{
				Type:     "image_url",
				ImageURL: &llm.VideoImageURL{URL: req.InputReference},
				Role:     "first_frame",
			})
		}
	}

	videoReq := &llm.VideoRequest{
		Model:                 req.Model,
		Content:               content,
		Duration:              req.Seconds,
		Size:                  req.Size,
		Ratio:                 req.Ratio,
		Resolution:            req.Resolution,
		ServiceTier:           req.ServiceTier,
		ExecutionExpiresAfter: req.ExecutionExpiresAfter,
		CallbackURL:           req.CallbackURL,
		ReturnLastFrame:       req.ReturnLastFrame,
		Tools:                 req.Tools,
	}

	return &llm.Request{
		Model:       req.Model,
		Messages:    []llm.Message{},
		Modalities:  []string{"video"},
		Stream:      lo.ToPtr(false),
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatOpenAIVideo,
		Video:       videoReq,
		ExtraBody:   req.ExtraBody,
	}, nil
}

type OpenAIVideoError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type OpenAIVideoUsage struct {
	CompletionTokens int64    `json:"completion_tokens,omitempty"`
	TotalTokens      int64    `json:"total_tokens,omitempty"`
	Cost             *float64 `json:"cost,omitempty"`
}

type OpenAIVideoObject struct {
	ID           string            `json:"id"`
	Object       string            `json:"object"`
	Status       string            `json:"status"`
	Model        string            `json:"model,omitempty"`
	Prompt       string            `json:"prompt,omitempty"`
	Seconds      *string           `json:"seconds,omitempty"`
	Size         string            `json:"size,omitempty"`
	Progress     *float64          `json:"progress,omitempty"`
	VideoURL     string            `json:"video_url,omitempty"`
	LastFrameURL string            `json:"last_frame_url,omitempty"`
	Content      json.RawMessage   `json:"content,omitempty"`
	CreatedAt    int64             `json:"created_at,omitempty"`
	CompletedAt  *int64            `json:"completed_at,omitempty"`
	ExpiresAt    *int64            `json:"expires_at,omitempty"`
	Error        *OpenAIVideoError `json:"error,omitempty"`
	Usage        *OpenAIVideoUsage `json:"usage,omitempty"`
}

func (t *VideoInboundTransformer) TransformResponse(ctx context.Context, llmResp *llm.Response) (*httpclient.Response, error) {
	if llmResp == nil || llmResp.Video == nil {
		return nil, fmt.Errorf("%w: video response is nil", transformer.ErrInvalidResponse)
	}

	v := llmResp.Video

	status := v.Status
	switch status {
	case "running":
		status = "in_progress"
	case "succeeded":
		status = "completed"
	}

	var completedAt *int64
	if v.CompletedAt != 0 {
		completedAt = lo.ToPtr(v.CompletedAt)
	}

	var expiresAt *int64
	if v.ExpiresAt != 0 {
		expiresAt = lo.ToPtr(v.ExpiresAt)
	}

	createdAt := v.CreatedAt
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}

	oai := OpenAIVideoObject{
		ID:           v.ID,
		Object:       "video",
		Status:       status,
		Model:        v.Model,
		Prompt:       v.Prompt,
		Seconds:      v.Duration,
		Size:         v.Size,
		Progress:     v.Progress,
		VideoURL:     v.VideoURL,
		LastFrameURL: v.LastFrameURL,
		Content:      v.Content,
		CreatedAt:    createdAt,
		CompletedAt:  completedAt,
		ExpiresAt:    expiresAt,
	}

	if v.Error != nil {
		oai.Error = &OpenAIVideoError{
			Code:    v.Error.Code,
			Message: v.Error.Message,
		}
	}

	if llmResp.Usage != nil {
		oai.Usage = &OpenAIVideoUsage{
			CompletionTokens: llmResp.Usage.CompletionTokens,
			TotalTokens:      llmResp.Usage.TotalTokens,
			Cost:             llmResp.Usage.Cost,
		}
	}

	body, err := json.Marshal(oai)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal openai video response: %w", err)
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
	chatInbound := NewInboundTransformer()
	return chatInbound.TransformError(ctx, rawErr)
}

func (t *VideoInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("%w: video request does not support streaming", transformer.ErrInvalidRequest)
}
