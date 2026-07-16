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


func TestBuildChatCompletionHTTPRequest_DiagnosesUnsupportedNativeTools(t *testing.T) {
	ctx := context.Background()
	provider := auth.NewStaticKeyProvider("test-key")
	// Body is pre-built by channels via openai.RequestFromLLM which already
	// omits image/web_search/google_*. The builder must still surface Lossy.
	body := []byte(`{"model":"gpt-4","tools":[{"type":"function","function":{"name":"calculator"}}]}`)
	llmReq := &llm.Request{
		Model:     "gpt-4",
		APIFormat: llm.APIFormatOpenAIResponse,
		Tools: []llm.Tool{
			{Type: llm.ToolTypeImageGeneration},
			{Type: llm.ToolTypeWebSearch},
			{Type: llm.ToolTypeGoogleSearch},
			{Type: llm.ToolTypeGoogleCodeExecution},
			{Type: llm.ToolTypeGoogleUrlContext},
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "calculator"}},
		},
	}

	req := BuildChatCompletionHTTPRequest(ctx, provider, "https://api.example.com/v1", body, llmReq)
	require.NotNil(t, req)
	require.NotContains(t, string(req.Body), "image_generation")
	require.NotContains(t, string(req.Body), "web_search")
	require.NotContains(t, string(req.Body), "google_search")

	fields := map[string]bool{}
	for _, d := range llm.LossyDowngrades(llmReq) {
		fields[d.SourceField] = true
		require.Equal(t, llm.APIFormatOpenAIChatCompletion, d.TargetProtocol)
		require.Equal(t, llm.LossyDowngradeReasonNoEquivalentSemantics, d.Reason)
	}
	for _, field := range []string{
		"tools[].type=image_generation",
		"tools[].type=web_search",
		"tools[].type=google_search",
		"tools[].type=google_code_execution",
		"tools[].type=google_url_context",
	} {
		require.True(t, fields[field], "missing Lossy for %s: %#v", field, llm.LossyDowngrades(llmReq))
	}
}

func TestRecordOpenAIChatUnsupportedNativeToolLossyDowngrades_NoopWithoutNativeTools(t *testing.T) {
	llmReq := &llm.Request{
		Model: "gpt-4",
		Tools: []llm.Tool{{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "calculator"}}},
	}
	RecordOpenAIChatUnsupportedNativeToolLossyDowngrades(llmReq)
	require.Empty(t, llm.LossyDowngrades(llmReq))
}
