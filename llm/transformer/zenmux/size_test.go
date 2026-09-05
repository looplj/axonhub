package zenmux

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/transformer"
)

func TestZenMuxNativeVideo_mapsOpenAISizeToNativeFields(t *testing.T) {
	// Given: a native ZenMux video request using the public OpenAI square size.
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	outbound, err := NewOutboundTransformer(server.URL, "zenmux-test-key")
	require.NoError(t, err)
	request := nativeVideoRequest("5")
	request.Video.Size = "256x256"
	request.Video.Ratio = ""
	request.Video.Resolution = ""

	// When: the request is transformed for the native ZenMux video API.
	providerRequest, err := outbound.TransformRequest(context.Background(), request)

	// Then: the public size becomes native content, ratio, and minimum resolution fields.
	require.NoError(t, err)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(providerRequest.Body, &body))
	var content []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body["content"], &content))
	require.Len(t, content, 4)
	var ratio, resolution string
	require.NoError(t, json.Unmarshal(body["ratio"], &ratio))
	require.NoError(t, json.Unmarshal(body["resolution"], &resolution))
	require.Equal(t, "1:1", ratio)
	require.Equal(t, "480p", resolution)
	require.NotContains(t, body, "size")
}

func TestZenMuxNativeVideo_preservesExplicitNativeFields(t *testing.T) {
	// Given: a request with explicit native fields and a public size.
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	outbound, err := NewOutboundTransformer(server.URL, "zenmux-test-key")
	require.NoError(t, err)
	request := nativeVideoRequest("5")
	request.Video.Size = "1920x1080"

	// When: the request is transformed for the native ZenMux video API.
	providerRequest, err := outbound.TransformRequest(context.Background(), request)

	// Then: explicit native ratio and resolution are retained instead of inferred values.
	require.NoError(t, err)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(providerRequest.Body, &body))
	var ratio, resolution string
	require.NoError(t, json.Unmarshal(body["ratio"], &ratio))
	require.NoError(t, json.Unmarshal(body["resolution"], &resolution))
	require.Equal(t, "16:9", ratio)
	require.Equal(t, "720p", resolution)
	require.NotContains(t, body, "size")
}

func TestZenMuxNativeVideo_rejectsUnsupportedOpenAISize(t *testing.T) {
	// Given: a request whose size has no documented native mapping.
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	outbound, err := NewOutboundTransformer(server.URL, "zenmux-test-key")
	require.NoError(t, err)
	request := nativeVideoRequest("5")
	request.Video.Size = "257x257"
	request.Video.Ratio = ""
	request.Video.Resolution = ""

	// When: the unsupported public size is transformed without native overrides.
	_, err = outbound.TransformRequest(context.Background(), request)

	// Then: only the unsupported size is rejected as an invalid request.
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
}
