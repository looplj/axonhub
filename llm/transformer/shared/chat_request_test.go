package shared

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestBuildChatCompletionHTTPRequest(t *testing.T) {
	ctx := context.Background()
	provider := auth.NewStaticKeyProvider("test-key-123")
	body := []byte(`{"model":"gpt-4"}`)
	llmReq := &llm.Request{
		Model:               "gpt-4",
		TransformerMetadata: map[string]any{"foo": "bar"},
	}

	req := BuildChatCompletionHTTPRequest(ctx, provider, "https://api.example.com/v1", body, llmReq)

	require.Equal(t, http.MethodPost, req.Method)
	require.Equal(t, "https://api.example.com/v1/chat/completions", req.URL)
	require.Equal(t, "application/json", req.Headers.Get("Content-Type"))
	require.Equal(t, "application/json", req.Headers.Get("Accept"))

	require.NotNil(t, req.Auth)
	require.Equal(t, httpclient.AuthTypeBearer, req.Auth.Type)
	require.Equal(t, "test-key-123", req.Auth.APIKey)

	require.Equal(t, body, req.Body)
	require.Equal(t, string(llm.APIFormatOpenAIChatCompletion), req.APIFormat)

	// TransformerMetadata propagated for the round-trip.
	require.Equal(t, "bar", req.TransformerMetadata["foo"])
}
