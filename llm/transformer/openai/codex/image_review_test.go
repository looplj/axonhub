package codex

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCodexGenericImagesReturnsConfigurationError(t *testing.T) {
	// Given
	out, err := NewOutboundTransformer(Params{TokenProvider: failingTokenGetter{}})
	require.NoError(t, err)
	// When
	_, err = out.TransformRequest(t.Context(), &llm.Request{RequestType: llm.RequestTypeImage, Image: &llm.ImageRequest{Prompt: "draw"}})
	// Then
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Contains(t, err.Error(), "endpoint")
}

func TestCodexNonstreamingTokenFailureSendsZeroHTTP(t *testing.T) {
	// Given: a real HTTP executor and a failing token getter for nonstreaming Images.
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	out, err := NewImageOutboundTransformer(ImageParams{TokenProvider: failingTokenGetter{}, BaseURL: server.URL})
	require.NoError(t, err)
	pipe := pipeline.NewFactory(httpclient.NewHttpClient()).Pipeline(openai.NewImageGenerationInboundTransformer(), out)
	// When
	_, err = pipe.Process(t.Context(), &httpclient.Request{Method: http.MethodPost, Headers: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"model":"gpt-image-2","prompt":"draw","stream":false}`)})
	// Then
	require.ErrorContains(t, err, "refresh failed")
	require.Zero(t, calls.Load())
}

func TestCodexGenericExecutorRejectsImagesBeforeHTTP(t *testing.T) {
	// Given
	inner := &mockCodexExecutor{}
	executor := (&OutboundTransformer{}).CustomizeExecutor(inner)
	// When
	_, err := executor.Do(t.Context(), &httpclient.Request{RequestType: llm.RequestTypeImage.String()})
	// Then
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Zero(t, inner.doCalls.Load())
	require.Zero(t, inner.doStreamCalls.Load())
}

func TestCodexNativeResponsesImageToolPreserved(t *testing.T) {
	// Given
	body := []byte(`{"model":"gpt-5.5","input":"synthetic","tools":[{"type":"image_generation","quality":"low","size":"1024x1024","output_format":"png"}],"tool_choice":{"type":"image_generation"}}`)
	req, err := responses.NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Headers: http.Header{"Content-Type": {"application/json"}}, Body: body})
	require.NoError(t, err)
	out, err := NewOutboundTransformer(Params{BaseURL: "https://relay.example/v1", TokenProvider: oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{AccessToken: "test-token"})})
	require.NoError(t, err)
	// When
	encoded, err := out.TransformRequest(t.Context(), req)
	// Then
	require.NoError(t, err)
	require.JSONEq(t, gjson.GetBytes(body, "tools").Raw, gjson.GetBytes(encoded.Body, "tools").Raw)
	require.JSONEq(t, gjson.GetBytes(body, "tool_choice").Raw, gjson.GetBytes(encoded.Body, "tool_choice").Raw)
	require.Equal(t, "https://relay.example/v1/responses", encoded.URL)
}
