package gemini

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
)

// countingKeyProvider counts how many times the outbound transformer selects a
// key, so a request that looks the key up twice (which would consume two
// round-robin slots) is caught.
type countingKeyProvider struct {
	calls atomic.Int64
}

func (p *countingKeyProvider) Get(context.Context) string {
	p.calls.Add(1)

	return "test-key"
}

func TestTransformRequest_SelectsKeyOncePerAttempt(t *testing.T) {
	newTransformer := func(provider auth.APIKeyProvider) *OutboundTransformer {
		tr, err := NewOutboundTransformerWithConfig(Config{
			BaseURL:        "https://generativelanguage.googleapis.com",
			APIKeyProvider: provider,
		})
		require.NoError(t, err)

		return tr.(*OutboundTransformer)
	}

	t.Run("embedding request consumes one key", func(t *testing.T) {
		provider := &countingKeyProvider{}
		tr := newTransformer(provider)

		_, err := tr.TransformRequest(context.Background(), &llm.Request{
			Model:       "gemini-embedding-001",
			RequestType: llm.RequestTypeEmbedding,
			Embedding:   &llm.EmbeddingRequest{Input: llm.EmbeddingInput{String: "hello"}},
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), provider.calls.Load(), "a round-robin key must be selected once per attempt")
	})

	t.Run("image request consumes one key", func(t *testing.T) {
		provider := &countingKeyProvider{}
		tr := newTransformer(provider)

		_, err := tr.TransformRequest(context.Background(), &llm.Request{
			Model:       "gemini-2.5-flash-image",
			RequestType: llm.RequestTypeImage,
			Image:       &llm.ImageRequest{Prompt: "a cat"},
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), provider.calls.Load(), "a round-robin key must be selected once per attempt")
	})
}