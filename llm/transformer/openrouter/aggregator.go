package openrouter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TransformChunk(ctx context.Context, chunk *httpclient.StreamEvent) (*openai.Response, error) {
	var chatResp Response

	err := json.Unmarshal(chunk.Data, &chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %w", err)
	}

	return chatResp.ToOpenAIResponse(), nil
}

// AggregateStreamChunks aggregates OpenRouter streaming response chunks into a complete response.
func AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return openai.AggregateStreamChunks(ctx, chunks, TransformChunk)
}
