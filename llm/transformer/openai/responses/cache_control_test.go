package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// TestCacheControl_ResponsesInboundCaptures covers #11: responses top-level
// cache_control must be captured into TransformerMetadata (was dropped).
func TestCacheControl_ResponsesInboundCaptures(t *testing.T) {
	inbound := NewInboundTransformer()
	req := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/responses",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-4o","input":"hi","cache_control":{"type":"ephemeral","ttl":"1h"}}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	raw, ok := llmReq.TransformerMetadata[shared.TransformerMetadataKeyCacheControl].(json.RawMessage)
	require.True(t, ok, "#11: responses cache_control must be stashed as json.RawMessage")
	var cc CacheControl
	require.NoError(t, json.Unmarshal(raw, &cc))
	require.Equal(t, "ephemeral", cc.Type)
	require.Equal(t, "1h", cc.TTL)
}

// TestCacheControl_ResponsesToAnthropic covers #11: responses cache_control must
// reach the anthropic provider (cross-format bridge).
func TestCacheControl_ResponsesToAnthropic(t *testing.T) {
	inbound := NewInboundTransformer()
	req := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/responses",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-4o","input":"hi","cache_control":{"type":"ephemeral"}}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	transformer, err := anthropic.NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	result, err := transformer.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var anthropicReq anthropic.MessageRequest
	require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
	require.NotNil(t, anthropicReq.CacheControl, "#11: responses cache_control must bridge to anthropic upstream")
	require.Equal(t, "ephemeral", anthropicReq.CacheControl.Type)
}

// TestCacheControl_DefaultGuard: no cache_control means no metadata stashed.
func TestCacheControl_DefaultGuard(t *testing.T) {
	inbound := NewInboundTransformer()
	req := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/responses",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-4o","input":"hi"}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	_, ok := llmReq.TransformerMetadata[shared.TransformerMetadataKeyCacheControl]
	require.False(t, ok, "#11: no cache_control must not stash metadata")
}
