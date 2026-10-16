package antigravity

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"io"
	"strings"
)

func TestReasoningCatalogBaseModels(t *testing.T) {
	catalog := ModelCatalog{}
	for _, id := range []string{"gemini-4-argon-low", "gemini-4-argon-medium", "gemini-4-argon-high", "gemini-3.5-flash-extra-low", "gemini-3.5-flash-low", "gpt-oss-120b-medium", "gemini-3-pro-image", "gemini-3.8-flash-tiered", "claude-sonnet-thinking"} {
		catalog[id] = struct{}{}
	}
	require.Equal(t, []string{"claude-sonnet-thinking", "gemini-3-pro-image", "gemini-3.5-flash", "gemini-3.8-flash-tiered", "gemini-4-argon", "gpt-oss-120b"}, catalog.BaseModels())
	require.Len(t, catalog, 9, "normalization must not mutate the wire catalog")
}

func TestReasoningCatalogResolve(t *testing.T) {
	catalog := ModelCatalog{}
	for _, id := range []string{"gemini-4-argon-low", "gemini-4-argon-medium", "gemini-4-argon-high", "gpt-oss-120b-medium", "gemini-2.5-flash", "claude-sonnet-5-5-low", "claude-sonnet-5-5-high"} {
		catalog[id] = struct{}{}
	}
	for _, tc := range []struct{ model, effort, want string }{
		{"gemini-4-argon", "low", "gemini-4-argon-low"},
		{"gemini-4-argon", "high", "gemini-4-argon-high"},
		{"gemini-4-argon", "xhigh", "gemini-4-argon-high"},
		{"gemini-4-argon", "", "gemini-4-argon-medium"},
		{"gemini-4-argon-low", "", "gemini-4-argon-low"},
		{"gemini-4-argon-low", "high", "gemini-4-argon-high"},
		{"antigravity-gemini-4-argon:antigravity", "low", "gemini-4-argon-low"},
		{"gpt-oss-120b", "", "gpt-oss-120b-medium"},
		{"claude-sonnet-5-5", "high", "claude-sonnet-5-5-high"},
		{"claude-sonnet-5-5", "", "claude-sonnet-5-5-low"},
		{"gemini-2.5-flash", "high", "gemini-2.5-flash"},
	} {
		t.Run(tc.model+"/"+tc.effort, func(t *testing.T) {
			got, err := catalog.Resolve(tc.model, tc.effort)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
	_, err := catalog.Resolve("gpt-oss-120b", "high")
	require.ErrorContains(t, err, "does not offer")
}

func TestTransformerReasoningCatalogCacheAndCredentialIsolation(t *testing.T) {
	calls := 0
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: &mockRoundTripper{roundTrip: func(r *http.Request) (*http.Response, error) {
		body := `{"access_token":"access","token_type":"Bearer","expires_in":3600}`
		if strings.HasSuffix(r.URL.Path, ":fetchAvailableModels") {
			calls++
			body = `{"models":{"gemini-4-argon-low":{},"gemini-4-argon-high":{}}}`
			if calls > 1 {
				body = `{"models":{"gemini-4-argon-medium":{}}}`
			}
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	}}})
	tr, err := NewTransformer(Config{BaseURL: EndpointDaily, APIKey: "first-refresh|project"}, WithHTTPClient(client))
	require.NoError(t, err)
	for _, effort := range []string{"low", "high"} {
		req := &llm.Request{Model: "gemini-4-argon", ReasoningEffort: effort, Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}}}}
		raw, err := tr.TransformRequest(t.Context(), req)
		require.NoError(t, err)
		var envelope AntigravityEnvelope
		require.NoError(t, json.Unmarshal(raw.Body, &envelope))
		require.Equal(t, "gemini-4-argon-"+effort, envelope.Model)
		require.Equal(t, "gemini-4-argon", req.Model)
	}
	require.Equal(t, 1, calls, "catalog should be reused within the same credential")
	tr.SetAPIKey("second-refresh|project")
	_, err = tr.TransformRequest(t.Context(), &llm.Request{Model: "gemini-4-argon", ReasoningEffort: "high", Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}}}})
	require.ErrorContains(t, err, "does not offer")
	require.Equal(t, 2, calls, "credentials must not share the previous account's catalog")
}
