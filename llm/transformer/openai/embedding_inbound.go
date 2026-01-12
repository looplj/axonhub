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

	var embReq EmbeddingRequest

	err := json.Unmarshal(httpReq.Body, &embReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode embedding request: %w", transformer.ErrInvalidRequest, err)
	}

	if embReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if err := validateEmbeddingInput(embReq.Input); err != nil {
		return nil, err
	}

	llmReq := &llm.Request{
		Model:       embReq.Model,
		Messages:    []llm.Message{}, // Embedding 不使用 messages
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatOpenAIEmbedding,
		Stream:      nil, // Embedding 不支持流式
		Embedding: &llm.EmbeddingRequest{
			Input:          embReq.Input,
			EncodingFormat: embReq.EncodingFormat,
			Dimensions:     embReq.Dimensions,
			User:           embReq.User,
		},
	}

	if embReq.User != "" {
		llmReq.User = &embReq.User
	}

	return llmReq, nil
}

func validateEmbeddingInput(input llm.EmbeddingInput) error {
	// Determine input type based on which field is set
	// JSON unmarshaling only sets one field based on the input type

	// String array input
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

	// Integer array input
	if input.IntArray != nil {
		if len(input.IntArray) == 0 {
			return fmt.Errorf("%w: input cannot be empty array", transformer.ErrInvalidRequest)
		}

		return nil
	}

	// Nested integer array input
	if input.IntArrayArray != nil {
		if len(input.IntArrayArray) == 0 {
			return fmt.Errorf("%w: input cannot be empty array", transformer.ErrInvalidRequest)
		}

		for i, innerArray := range input.IntArrayArray {
			if len(innerArray) == 0 {
				return fmt.Errorf("%w: input[%d] cannot be empty array", transformer.ErrInvalidRequest, i)
			}
		}

		return nil
	}

	// String input (default case)
	if strings.TrimSpace(input.String) == "" {
		return fmt.Errorf("%w: input cannot be empty string", transformer.ErrInvalidRequest)
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

	// 从 llm.Embedding 中提取 embedding 响应
	var body []byte

	if llmResp.Embedding != nil {
		// 将 llm.EmbeddingResponse 转换为 OpenAI EmbeddingResponse 格式
		embResp := EmbeddingResponse{
			Object: llmResp.Embedding.Object,
			Data:   make([]EmbeddingData, len(llmResp.Embedding.Data)),
			Model:  llmResp.Model,
		}

		// 转换 EmbeddingData
		for i, data := range llmResp.Embedding.Data {
			embResp.Data[i] = EmbeddingData{
				Object:    data.Object,
				Embedding: data.Embedding,
				Index:     data.Index,
			}
		}

		// 转换 Usage
		if llmResp.Embedding.Usage != nil {
			embResp.Usage = EmbeddingUsage{
				PromptTokens: llmResp.Embedding.Usage.PromptTokens,
				TotalTokens:  llmResp.Embedding.Usage.TotalTokens,
			}
		}

		var err error

		body, err = json.Marshal(embResp)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal embedding response: %w", err)
		}
	} else {
		return nil, fmt.Errorf("embedding response missing embedding data")
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
