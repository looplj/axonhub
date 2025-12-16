package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// EmbeddingInboundTransformer 实现 OpenAI embeddings 端点的入站转换器。
type EmbeddingInboundTransformer struct{}

// NewEmbeddingInboundTransformer 创建一个新的 EmbeddingInboundTransformer。
func NewEmbeddingInboundTransformer() *EmbeddingInboundTransformer {
	return &EmbeddingInboundTransformer{}
}

func (t *EmbeddingInboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIEmbedding
}

// TransformRequest 将 HTTP embedding 请求转换为统一的 llm.Request 格式。
// 由于 embedding 不使用 messages，我们将 input 作为 JSON 存储在 ExtraBody 中。
func (t *EmbeddingInboundTransformer) TransformRequest(
	ctx context.Context,
	httpReq *httpclient.Request,
) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	// 检查 Content-Type
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}

	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	var embReq objects.EmbeddingRequest

	err := json.Unmarshal(httpReq.Body, &embReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode embedding request: %w", transformer.ErrInvalidRequest, err)
	}

	// 验证必填字段
	if embReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if embReq.Input == nil {
		return nil, fmt.Errorf("%w: input is required", transformer.ErrInvalidRequest)
	}

	// 验证 input 不为空字符串或空数组
	if err := validateEmbeddingInput(embReq.Input); err != nil {
		return nil, err
	}

	// 构建统一请求
	// Embedding 不使用 chat messages，所以将 embedding 参数存储在 ExtraBody 中
	extraBody, err := json.Marshal(embReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request to ExtraBody: %w", err)
	}

	llmReq := &llm.Request{
		Model:        embReq.Model,
		Messages:     []llm.Message{}, // Embedding 不使用 messages
		RawRequest:   httpReq,
		RawAPIFormat: llm.APIFormatOpenAIEmbedding,
		ExtraBody:    extraBody,
		Stream:       nil, // Embedding 不支持流式
	}

	if embReq.User != "" {
		llmReq.User = &embReq.User
	}

	return llmReq, nil
}

// validateEmbeddingInput 验证 embedding input 不为空。
// OpenAI 规范支持以下输入类型：
// - string: 单个字符串
// - []string: 字符串数组（JSON 解析后为 []any）
// - []int: token IDs 数组（JSON 解析后为 []any，元素为 float64）
// - [][]int: 多个 token IDs 数组（JSON 解析后为 []any，元素为 []any）
//
// 注意：由于 Input 字段类型为 any，json.Unmarshal 会将所有数组解析为 []any，
// 因此只需处理 string 和 []any 两种情况。
func validateEmbeddingInput(input any) error {
	switch v := input.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%w: input cannot be empty string", transformer.ErrInvalidRequest)
		}
	case []any:
		if len(v) == 0 {
			return fmt.Errorf("%w: input cannot be empty array", transformer.ErrInvalidRequest)
		}
		// 检查数组中的每个元素
		for i, item := range v {
			switch elem := item.(type) {
			case string:
				// 字符串数组：检查每个字符串不为空
				if strings.TrimSpace(elem) == "" {
					return fmt.Errorf("%w: input[%d] cannot be empty string", transformer.ErrInvalidRequest, i)
				}
			case float64:
				// token ID 数组：数字类型，不需要额外校验
				// JSON 解析后整数会变成 float64
				continue
			case []any:
				// 嵌套数组：[][]int 的情况
				if len(elem) == 0 {
					return fmt.Errorf("%w: input[%d] cannot be empty array", transformer.ErrInvalidRequest, i)
				}
			default:
				// 其他类型，透传给上游处理
				continue
			}
		}
	}

	return nil
}

// TransformResponse 将统一的 llm.Response 转换回 HTTP 响应。
func (t *EmbeddingInboundTransformer) TransformResponse(
	ctx context.Context,
	llmResp *llm.Response,
) (*httpclient.Response, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("embedding response is nil")
	}

	// 从 ProviderData 中提取 embedding 响应
	var body []byte

	if llmResp.ProviderData != nil {
		var embResp objects.EmbeddingResponse

		switch v := llmResp.ProviderData.(type) {
		case objects.EmbeddingResponse:
			embResp = v
		case *objects.EmbeddingResponse:
			if v == nil {
				return nil, fmt.Errorf("embedding response provider data is nil")
			}

			embResp = *v
		default:
			return nil, fmt.Errorf("invalid provider data for embedding response")
		}

		var err error

		body, err = json.Marshal(embResp)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal embedding response: %w", err)
		}
	} else {
		return nil, fmt.Errorf("embedding response missing provider data")
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

// TransformStream Embedding 不支持流式传输。
func (t *EmbeddingInboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: embeddings do not support streaming", transformer.ErrInvalidRequest)
}

// AggregateStreamChunks Embedding 不支持流式传输。
func (t *EmbeddingInboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("embeddings do not support streaming")
}

// TransformError 复用标准 OpenAI 错误格式化。
func (t *EmbeddingInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	// 委托给标准 chat inbound transformer 以保持一致的错误处理
	chatInbound := NewInboundTransformer()
	return chatInbound.TransformError(ctx, rawErr)
}
