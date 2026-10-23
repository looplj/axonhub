package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/api"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

type codexImageRequest struct {
	path string
	body []byte
}

func setupCodexImageRoute(t *testing.T, format llm.APIFormat, response string) (*codexRouteHarness, <-chan codexImageRequest) {
	t.Helper()
	requests := make(chan codexImageRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		requests <- codexImageRequest{path: r.URL.Path, body: body}
		assert.Equal(t, "Bearer upstream", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, err = io.WriteString(w, response)
		assert.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)

	h := newCodexRouteHarness(t)
	_, err := h.source.Update().
		SetBaseURL(upstream.URL).
		SetDefaultTestModel("gpt-image-2").
		SetSupportedModels([]string{"gpt-image-2"}).
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: format.String()}}).
		Save(h.ctx)
	require.NoError(t, err)
	built, err := h.channels.GetChannel(h.ctx, h.source.ID)
	require.NoError(t, err)
	h.channels.SetEnabledChannelsForTest([]*biz.Channel{built})
	selector := orchestrator.NewDefaultSelector(h.channels, h.openAI.ModelService, h.system)
	h.openAI.ImageGenerationHandlers = api.NewChatCompletionHandlers(orchestrator.NewChatCompletionOrchestrator(h.channels, selector, h.requests, httpclient.NewHttpClient(), openai.NewImageGenerationInboundTransformer(), h.system, h.usage, h.prompts, nil, h.protect, biz.NewLiveStreamRegistry(), orchestrator.NewChannelLimiterManager(), nil))
	h.openAI.ImageEditHandlers = api.NewChatCompletionHandlers(orchestrator.NewChatCompletionOrchestrator(h.channels, selector, h.requests, httpclient.NewHttpClient(), openai.NewImageEditInboundTransformer(), h.system, h.usage, h.prompts, nil, h.protect, biz.NewLiveStreamRegistry(), orchestrator.NewChannelLimiterManager(), nil))
	require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
	return h, requests
}

func TestCodexRoutes_ImageGenerationUsesNativeUpstream(t *testing.T) {
	h, requests := setupCodexImageRoute(t, llm.APIFormatOpenAIImageGeneration, `{"created":1700000000,"data":[{"b64_json":"synthetic-generated"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/codex/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"synthetic generation"}`))
	req.Header.Set("Authorization", "Bearer "+h.key.Key)
	req.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()

	// When: an authenticated Codex image generation request reaches the route.
	h.router.ServeHTTP(result, req)

	// Then: the real channel pipeline uses the native Images endpoint and response.
	observed := <-requests
	require.Equal(t, http.StatusOK, result.Code, result.Body.String())
	require.Equal(t, "/v1/images/generations", observed.path)
	require.Equal(t, "gpt-image-2", gjson.GetBytes(observed.body, "model").String())
	require.Contains(t, string(observed.body), "synthetic generation")
	require.Equal(t, "synthetic-generated", gjson.GetBytes(result.Body.Bytes(), "data.0.b64_json").String())
}

func TestCodexRoutes_ImageEditUsesNativeUpstream(t *testing.T) {
	h, requests := setupCodexImageRoute(t, llm.APIFormatOpenAIImageEdit, `{"created":1700000001,"data":[{"b64_json":"synthetic-edited"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/codex/images/edits", strings.NewReader(`{"model":"gpt-image-2","image":"data:image/png;base64,c3ludGhldGljLWltYWdl","prompt":"synthetic edit"}`))
	req.Header.Set("Authorization", "Bearer "+h.key.Key)
	req.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()

	// When: an authenticated Codex image edit request reaches the route.
	h.router.ServeHTTP(result, req)

	// Then: the real channel pipeline uses the native edit endpoint and response.
	observed := <-requests
	require.Equal(t, http.StatusOK, result.Code, result.Body.String())
	require.Equal(t, "/v1/images/edits", observed.path)
	require.Equal(t, "gpt-image-2", gjson.GetBytes(observed.body, "model").String())
	require.Contains(t, string(observed.body), "synthetic edit")
	require.Contains(t, string(observed.body), "data:image/png;base64,c3ludGhldGljLWltYWdl")
	require.Equal(t, "synthetic-edited", gjson.GetBytes(result.Body.Bytes(), "data.0.b64_json").String())
}
