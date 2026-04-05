package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

const (
	maxVideoBodySize        = 20 * 1024 * 1024
	maxVideoReferenceSize   = 4 * 1024 * 1024
	maxVideoFieldValueSize  = 4 * 1024 * 1024
	videoReferenceFieldName = "input_reference"
)

type VideoCreateRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	InputReference string `json:"input_reference,omitempty"`
	Seconds        *int64 `json:"seconds,omitempty"`
	Size           string `json:"size,omitempty"`
}

type VideoInboundTransformer struct{}

func NewVideoInboundTransformer() *VideoInboundTransformer {
	return &VideoInboundTransformer{}
}

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
		if err := json.Unmarshal(httpReq.Body, &req); err != nil {
			return nil, fmt.Errorf("%w: failed to decode video request: %w", transformer.ErrInvalidRequest, err)
		}
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
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("%w: prompt is required", transformer.ErrInvalidRequest)
	}

	content := []llm.VideoContent{
		{Type: "text", Text: req.Prompt},
	}
	if strings.TrimSpace(req.InputReference) != "" {
		content = append(content, llm.VideoContent{
			Type:     "image_url",
			ImageURL: &llm.VideoImageURL{URL: req.InputReference},
			Role:     "first_frame",
		})
	}

	videoReq := &llm.VideoRequest{
		Model:    req.Model,
		Content:  content,
		Duration: req.Seconds,
		Size:     req.Size,
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
	}, nil
}

func parseVideoMultipartRequest(httpReq *httpclient.Request) (*VideoCreateRequest, error) {
	if len(httpReq.Body) > maxVideoBodySize {
		return nil, fmt.Errorf("%w: request body too large", transformer.ErrInvalidRequest)
	}

	contentType := httpReq.Headers.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid content-type", transformer.ErrInvalidRequest)
	}

	if !strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		return nil, fmt.Errorf("%w: expected multipart/form-data", transformer.ErrInvalidRequest)
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("%w: missing boundary in content-type", transformer.ErrInvalidRequest)
	}

	reader := multipart.NewReader(bytes.NewReader(httpReq.Body), boundary)

	fields := map[string]string{}
	var referenceFile *multipartFile

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read multipart", transformer.ErrInvalidRequest)
		}

		fieldName := part.FormName()
		filename := part.FileName()

		// Regular field.
		if filename == "" {
			value, err := io.ReadAll(io.LimitReader(part, maxVideoFieldValueSize+1))
			if err != nil {
				return nil, fmt.Errorf("%w: failed to read multipart field", transformer.ErrInvalidRequest)
			}
			if len(value) > maxVideoFieldValueSize {
				return nil, fmt.Errorf("%w: multipart field too large", transformer.ErrInvalidRequest)
			}
			fields[fieldName] = string(value)
			continue
		}

		// File field.
		if fieldName != videoReferenceFieldName {
			continue
		}

		if referenceFile != nil {
			return nil, fmt.Errorf("%w: multiple input_reference files are not supported", transformer.ErrInvalidRequest)
		}

		contentType := strings.TrimSpace(part.Header.Get("Content-Type"))
		data, err := io.ReadAll(io.LimitReader(part, maxVideoReferenceSize+1))
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read multipart file", transformer.ErrInvalidRequest)
		}
		if len(data) > maxVideoReferenceSize {
			return nil, fmt.Errorf("%w: file too large", transformer.ErrInvalidRequest)
		}

		if contentType == "" {
			contentType = http.DetectContentType(lo.Ternary(len(data) > 512, data[:512], data))
		}
		if !isAllowedImageType(contentType) {
			return nil, fmt.Errorf("%w: unsupported image type", transformer.ErrInvalidRequest)
		}

		referenceFile = &multipartFile{
			ContentType: contentType,
			Data:        data,
		}
	}

	model := strings.TrimSpace(fields["model"])
	prompt := strings.TrimSpace(fields["prompt"])
	size := strings.TrimSpace(fields["size"])

	var seconds *int64
	if s := strings.TrimSpace(fields["seconds"]); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			seconds = &v
		} else {
			return nil, fmt.Errorf("%w: invalid seconds", transformer.ErrInvalidRequest)
		}
	}

	inputReference := strings.TrimSpace(fields[videoReferenceFieldName])
	if referenceFile != nil {
		inputReference = buildImageDataURL(referenceFile.ContentType, referenceFile.Data)
	}

	return &VideoCreateRequest{
		Model:          model,
		Prompt:         prompt,
		InputReference: inputReference,
		Seconds:        seconds,
		Size:           size,
	}, nil
}

func buildImageDataURL(contentType string, data []byte) string {
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data))
}

type OpenAIVideoError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type OpenAIVideoUsage struct {
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens,omitempty"`
}

type OpenAIVideoObject struct {
	ID          string            `json:"id"`
	Object      string            `json:"object"`
	Status      string            `json:"status"`
	Model       string            `json:"model,omitempty"`
	Prompt      string            `json:"prompt,omitempty"`
	Seconds     *int64            `json:"seconds,omitempty"`
	Size        string            `json:"size,omitempty"`
	Progress    *float64          `json:"progress,omitempty"`
	VideoURL    string            `json:"video_url,omitempty"`
	CreatedAt   int64             `json:"created_at,omitempty"`
	CompletedAt *int64            `json:"completed_at,omitempty"`
	ExpiresAt   *int64            `json:"expires_at,omitempty"`
	Error       *OpenAIVideoError `json:"error,omitempty"`
	Usage       *OpenAIVideoUsage `json:"usage,omitempty"`
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
		ID:          v.ID,
		Object:      "video",
		Status:      status,
		Model:       v.Model,
		Prompt:      v.Prompt,
		Seconds:     v.Duration,
		Size:        v.Size,
		Progress:    v.Progress,
		VideoURL:    v.VideoURL,
		CreatedAt:   createdAt,
		CompletedAt: completedAt,
		ExpiresAt:   expiresAt,
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
