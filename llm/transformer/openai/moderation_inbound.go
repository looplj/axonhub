package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// defaultModerationModel is used when the client omits model on /v1/moderations.
// Official OpenAI allows omitting model; this default keeps routing and upstream
// compatibility consistent with omni-moderation-latest.
const defaultModerationModel = "omni-moderation-latest"

// ModerationInboundTransformer 实现 OpenAI /v1/moderations 入站转换。
type ModerationInboundTransformer struct{}

// NewModerationInboundTransformer 创建 Moderations 入站转换器。
func NewModerationInboundTransformer() *ModerationInboundTransformer {
	return &ModerationInboundTransformer{}
}

// TransformRequest 将 HTTP moderation 请求转为统一 llm.Request。
func (t *ModerationInboundTransformer) TransformRequest(
	ctx context.Context,
	httpReq *httpclient.Request,
) (*llm.Request, error) {
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

	var modReq ModerationCreateRequest
	if err := json.Unmarshal(httpReq.Body, &modReq); err != nil {
		return nil, fmt.Errorf("%w: failed to decode moderation request: %w", transformer.ErrInvalidRequest, err)
	}

	if err := validateModerationInput(modReq.Input); err != nil {
		return nil, err
	}

	// 官方允许省略 model；未指定时使用 defaultModerationModel，便于渠道路由与上游兼容。
	model := strings.TrimSpace(modReq.Model)
	if model == "" {
		model = defaultModerationModel
	}

	return &llm.Request{
		Model:       model,
		Messages:    []llm.Message{},
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeModeration,
		APIFormat:   llm.APIFormatOpenAIModeration,
		Stream:      nil,
		Moderation: &llm.ModerationRequest{
			Input: modReq.Input,
		},
	}, nil
}

func validateModerationInput(input llm.ModerationInput) error {
	if len(input.Parts) > 0 {
		for i, part := range input.Parts {
			switch strings.TrimSpace(part.Type) {
			case "text":
				if strings.TrimSpace(part.Text) == "" {
					return fmt.Errorf("%w: input[%d].text cannot be empty", transformer.ErrInvalidRequest, i)
				}
			case "image_url":
				if part.ImageURL == nil || strings.TrimSpace(part.ImageURL.URL) == "" {
					return fmt.Errorf("%w: input[%d].image_url.url is required", transformer.ErrInvalidRequest, i)
				}
			default:
				return fmt.Errorf("%w: input[%d].type must be text or image_url", transformer.ErrInvalidRequest, i)
			}
		}

		return nil
	}

	if input.StringArray != nil {
		if len(input.StringArray) == 0 {
			return fmt.Errorf("%w: input cannot be empty array", transformer.ErrInvalidRequest)
		}

		for i, str := range input.StringArray {
			if strings.TrimSpace(str) == "" {
				return fmt.Errorf("%w: input[%d] cannot be empty string", transformer.ErrInvalidRequest, i)
			}
		}

		return nil
	}

	if strings.TrimSpace(input.String) == "" {
		return fmt.Errorf("%w: input cannot be empty string", transformer.ErrInvalidRequest)
	}

	return nil
}

// TransformResponse 将统一 llm.Response 转回 HTTP 响应。
func (t *ModerationInboundTransformer) TransformResponse(
	ctx context.Context,
	llmResp *llm.Response,
) (*httpclient.Response, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("moderation response is nil")
	}

	if llmResp.Moderation == nil {
		return nil, fmt.Errorf("moderation response missing moderation data")
	}

	modResp := ModerationCreateResponse{
		ID:      llmResp.ID,
		Model:   llmResp.Model,
		Results: llmResp.Moderation.Results,
	}

	body, err := json.Marshal(modResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal moderation response: %w", err)
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

// TransformStream Moderations 不支持流式。
func (t *ModerationInboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: moderations do not support streaming", transformer.ErrInvalidRequest)
}

// AggregateStreamChunks Moderations 不支持流式。
func (t *ModerationInboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("moderations do not support streaming")
}

// TransformError 复用标准 OpenAI 错误格式。
func (t *ModerationInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	return NewInboundTransformer().TransformError(ctx, rawErr)
}
