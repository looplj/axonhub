package biz

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
)

func TestRequestToSegment_EmbeddingFormatSkipsSpans(t *testing.T) {
	now := time.Now()

	reqBody, err := json.Marshal(map[string]any{
		"model": "text-embedding-3-small",
		"input": "hello world",
	})
	require.NoError(t, err)

	respBody, err := json.Marshal(map[string]any{
		"object": "list",
		"data": []map[string]any{
			{"object": "embedding", "index": 0, "embedding": []float64{0.1, 0.2}},
		},
		"usage": map[string]any{"prompt_tokens": 3, "total_tokens": 3},
	})
	require.NoError(t, err)

	req := &ent.Request{
		ID:           42,
		ModelID:      "text-embedding-3-small",
		Format:       string(llm.APIFormatOpenAIEmbedding),
		Status:       request.StatusCompleted,
		CreatedAt:    now,
		UpdatedAt:    now,
		RequestBody:  reqBody,
		ResponseBody: respBody,
	}

	segment, err := requestToSegment(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, segment)
	require.Empty(t, segment.RequestSpans)
	require.Empty(t, segment.ResponseSpans)
}
