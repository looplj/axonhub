package jina

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

type EmbeddingInboundTransformer struct{}

func NewEmbeddingInboundTransformer() *EmbeddingInboundTransformer {
	return &EmbeddingInboundTransformer{}
}

func (t *EmbeddingInboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatJinaEmbedding
}

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
		Messages:    []llm.Message{},
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatJinaEmbedding,
		Stream:      nil,
		Embedding: &llm.EmbeddingRequest{
			Input:          embReq.Input,
			Task:           embReq.Task,
			EncodingFormat: embReq.EncodingFormat,
			Dimensions:     embReq.Dimensions,
			User:           embReq.User,
		},
	}

	return llmReq, nil
}

func validateEmbeddingInput(input llm.EmbeddingInput) error {
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

	if input.IntArray != nil {
		if len(input.IntArray) == 0 {
			return fmt.Errorf("%w: input cannot be empty array", transformer.ErrInvalidRequest)
		}

		return nil
	}

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

	if strings.TrimSpace(input.String) == "" {
		return fmt.Errorf("%w: input cannot be empty string", transformer.ErrInvalidRequest)
	}

	return nil
}

func (t *EmbeddingInboundTransformer) TransformResponse(
	ctx context.Context,
	llmResp *llm.Response,
) (*httpclient.Response, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("embedding response is nil")
	}

	var body []byte

	if llmResp.Embedding != nil {
		embResp := EmbeddingResponse{
			Object: llmResp.Embedding.Object,
			Data:   make([]EmbeddingData, len(llmResp.Embedding.Data)),
			Model:  llmResp.Model,
		}

		for i, data := range llmResp.Embedding.Data {
			embResp.Data[i] = EmbeddingData{
				Object:    data.Object,
				Embedding: data.Embedding,
				Index:     data.Index,
			}
		}

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

func (t *EmbeddingInboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: embeddings do not support streaming", transformer.ErrInvalidRequest)
}

func (t *EmbeddingInboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("embeddings do not support streaming")
}

func (t *EmbeddingInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	rerankInbound := NewRerankInboundTransformer()
	return rerankInbound.TransformError(ctx, rawErr)
}
