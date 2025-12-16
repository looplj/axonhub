package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// EmbeddingOutboundTransformer 实现 OpenAI Embedding API 的出站转换器。
type EmbeddingOutboundTransformer struct {
	config *Config
}

// NewEmbeddingOutboundTransformer 创建一个新的 EmbeddingOutboundTransformer。
func NewEmbeddingOutboundTransformer(baseURL, apiKey string) (*EmbeddingOutboundTransformer, error) {
	config := &Config{
		Type:    PlatformOpenAI,
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		APIKey:  apiKey,
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &EmbeddingOutboundTransformer{
		config: config,
	}, nil
}

// NewEmbeddingOutboundTransformerWithConfig 使用给定的配置创建转换器。
func NewEmbeddingOutboundTransformerWithConfig(config *Config) (*EmbeddingOutboundTransformer, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	config.BaseURL = strings.TrimSuffix(config.BaseURL, "/")

	return &EmbeddingOutboundTransformer{
		config: config,
	}, nil
}

func (t *EmbeddingOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIEmbedding
}

// TransformRequest 将统一的 llm.Request 转换为 HTTP embedding 请求。
func (t *EmbeddingOutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("llm request is nil")
	}

	// 从 ExtraBody 中解析 embedding 请求
	var embReq objects.EmbeddingRequest
	if len(llmReq.ExtraBody) > 0 {
		err := json.Unmarshal(llmReq.ExtraBody, &embReq)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal embedding request from ExtraBody: %w", err)
		}
	} else {
		return nil, fmt.Errorf("embedding request missing in ExtraBody")
	}

	// 重新序列化为 JSON（确保输出干净）
	body, err := json.Marshal(embReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	// 准备请求头
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// 构建 URL，复用与 chat 相同的逻辑
	url := t.buildEmbeddingURL()

	// 构建认证配置
	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: t.config.APIKey,
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

// buildEmbeddingURL 构建 embedding API 的 URL。
func (t *EmbeddingOutboundTransformer) buildEmbeddingURL() string {
	// 如果 BaseURL 已经包含 /v1，直接追加 /embeddings
	if strings.HasSuffix(t.config.BaseURL, "/v1") {
		return t.config.BaseURL + "/embeddings"
	}

	// 否则添加 /v1/embeddings
	return t.config.BaseURL + "/v1/embeddings"
}

// TransformResponse 将 HTTP embedding 响应转换为统一的 llm.Response。
func (t *EmbeddingOutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// 检查 HTTP 状态码，4xx/5xx 应该返回标准格式的错误
	// 注意：httpclient 通常已经在 4xx/5xx 时返回 *httpclient.Error，
	// 这里作为防御性代码，确保错误格式符合 OpenAI 规范
	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
		})
	}

	// 检查响应体是否为空
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	// 解析 OpenAI embedding 响应
	var embResp objects.EmbeddingResponse
	if err := json.Unmarshal(httpResp.Body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedding response: %w", err)
	}

	// 构建统一响应，使用上游返回的 ID（如果有的话）
	responseID := embResp.ID
	if responseID == "" {
		// 如果上游没有返回 ID，生成一个基于模型的 ID
		responseID = fmt.Sprintf("emb-%s-%d", embResp.Model, len(embResp.Data))
	}

	llmResp := &llm.Response{
		ID:           responseID,
		Object:       embResp.Object,
		Model:        embResp.Model,
		Choices:      nil, // Embedding 没有 choices
		ProviderData: embResp,
	}

	// 映射 usage 信息
	if embResp.Usage.PromptTokens > 0 || embResp.Usage.TotalTokens > 0 {
		llmResp.Usage = &llm.Usage{
			PromptTokens:     int64(embResp.Usage.PromptTokens),
			CompletionTokens: 0, // Embedding 没有 completion tokens
			TotalTokens:      int64(embResp.Usage.TotalTokens),
		}
	}

	return llmResp, nil
}

// TransformStream Embedding 不支持流式传输。
func (t *EmbeddingOutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("embeddings do not support streaming")
}

// AggregateStreamChunks Embedding 不支持流式传输。
func (t *EmbeddingOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("embeddings do not support streaming")
}

// TransformError 转换 HTTP 错误响应为统一的错误响应。
func (t *EmbeddingOutboundTransformer) TransformError(
	ctx context.Context,
	httpErr *httpclient.Error,
) *llm.ResponseError {
	if httpErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
				Type:    "api_error",
			},
		}
	}

	// 尝试解析 OpenAI 错误格式
	var openaiError struct {
		Error llm.ErrorDetail `json:"error"`
	}

	err := json.Unmarshal(httpErr.Body, &openaiError)
	if err == nil && openaiError.Error.Message != "" {
		return &llm.ResponseError{
			StatusCode: httpErr.StatusCode,
			Detail:     openaiError.Error,
		}
	}

	// 如果 JSON 解析失败，使用上游状态文本
	return &llm.ResponseError{
		StatusCode: httpErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: http.StatusText(httpErr.StatusCode),
			Type:    "api_error",
		},
	}
}
