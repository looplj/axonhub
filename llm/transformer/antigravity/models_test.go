package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
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

func TestModelCatalog_ResolveUnknownOrNoneEffort(t *testing.T) {
	catalog := ModelCatalog{
		"gemini-4-argon-low":  struct{}{},
		"gemini-4-argon-high": struct{}{},
		"gemini-3-pro-low":    struct{}{},
	}
	for _, effort := range []string{"none", "unsupported-effort", "medium"} {
		model, err := catalog.Resolve("gemini-4-argon", effort)
		require.ErrorContains(t, err, "does not offer reasoning effort")
		require.Empty(t, model, "must not return a nonexistent bare wire ID")
	}
	// Legacy behavior remains available when it resolves to a discovered model.
	model, err := catalog.Resolve("gemini-3-pro", "none")
	require.NoError(t, err)
	require.Equal(t, "gemini-3-pro-low", model)
}

func TestReasoningCatalogFailureCooldownPreservesEffort(t *testing.T) {
	for _, firstEffort := range []string{"", "high"} {
		t.Run("first effort="+firstEffort, func(t *testing.T) {
			var calls atomic.Int32
			tr := &Transformer{httpClient: catalogTestClient(func(r *http.Request) (*http.Response, error) {
				calls.Add(1)
				return catalogTestResponse(http.StatusServiceUnavailable, `{}`), nil
			})}
			_, err := tr.resolveReasoningModel(t.Context(), "gemini-3-pro", firstEffort, "token", 0)
			if firstEffort != "" {
				require.ErrorContains(t, err, "resolve Antigravity reasoning effort")
			} else {
				require.NoError(t, err)
			}
			for range 2 {
				model, err := tr.resolveReasoningModel(t.Context(), "gemini-3-pro", "high", "token", 0)
				require.ErrorContains(t, err, "resolve Antigravity reasoning effort")
				require.Empty(t, model)
			}
			require.EqualValues(t, 1, calls.Load(), "failure is retained during cooldown")
		})
	}
}

func TestReasoningCatalogRefreshKeepsStaleCatalog(t *testing.T) {
	tr := &Transformer{
		modelCatalog: ModelCatalog{"gemini-4-argon-high": {}},
		httpClient: catalogTestClient(func(r *http.Request) (*http.Response, error) {
			return catalogTestResponse(http.StatusServiceUnavailable, `{}`), nil
		}),
	}
	for range 2 {
		model, err := tr.resolveReasoningModel(t.Context(), "gemini-4-argon", "high", "token", 0)
		require.NoError(t, err)
		require.Equal(t, "gemini-4-argon-high", model)
	}
}

func catalogTestClient(fn func(*http.Request) (*http.Response, error)) *httpclient.HttpClient {
	return httpclient.NewHttpClientWithClient(&http.Client{Transport: &mockRoundTripper{roundTrip: fn}})
}

func catalogTestResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func resolveCatalogAsync(tr *Transformer, ctx context.Context, token string, version uint64) <-chan error {
	result := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				result <- fmt.Errorf("catalog resolver panicked: %v", recovered)
			}
		}()
		model, err := tr.resolveReasoningModel(ctx, "gemini-4-argon", "high", token, version)
		if err == nil && model != "gemini-4-argon-high" {
			err = fmt.Errorf("expected account-specific high variant, got %q", model)
		}
		result <- err
	}()
	return result
}

func TestReasoningCatalogRefreshSharedAndCancellable(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(finish)
	tr := &Transformer{httpClient: catalogTestClient(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-release:
			return catalogTestResponse(http.StatusOK, `{"models":{"gemini-4-argon-high":{}}}`), nil
		}
	})}
	firstCtx, cancelFirst := context.WithCancel(t.Context())
	first := resolveCatalogAsync(tr, firstCtx, "token", 0)
	<-started
	cancelFirst()
	select {
	case err := <-first:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("the refreshing caller must be able to cancel")
	}
	// The cancelled initiator must not cancel the refresh or spawn another one.
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err := tr.resolveReasoningModel(ctx, "gemini-4-argon", "high", "token", 0)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.EqualValues(t, 1, calls.Load())
	finish()
	model, err := tr.resolveReasoningModel(t.Context(), "gemini-4-argon", "high", "token", 0)
	require.NoError(t, err)
	require.Equal(t, "gemini-4-argon-high", model)
	require.EqualValues(t, 1, calls.Load(), "the shared refresh must be reusable after cancellation")
}

func TestReasoningCatalogRefreshDoesNotCrossCredentialGenerations(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(finish)
	tr := &Transformer{httpClient: catalogTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") == "Bearer old" {
			close(started)
			<-release
			return catalogTestResponse(http.StatusOK, `{"models":{"gemini-4-argon-high":{}}}`), nil
		}
		return catalogTestResponse(http.StatusOK, `{"models":{"gemini-4-argon-low":{}}}`), nil
	})}
	old := resolveCatalogAsync(tr, t.Context(), "old", 0)
	<-started
	tr.SetAPIKey("new-refresh|project")
	_, err := tr.resolveReasoningModel(t.Context(), "gemini-4-argon", "high", "new", 1)
	require.ErrorContains(t, err, "does not offer reasoning effort")
	finish()
	require.NoError(t, <-old, "old request must use its own catalog, not the new account's")
	tr.modelCatalogMu.Lock()
	defer tr.modelCatalogMu.Unlock()
	require.Equal(t, ModelCatalog{"gemini-4-argon-low": {}}, tr.modelCatalog)
}

func TestReasoningCatalogPreservesNativeEffortTiers(t *testing.T) {
	for _, effort := range []string{"minimal", "xhigh"} {
		model := "gemini-future-" + effort
		catalog := ModelCatalog{model: {}}
		resolved, err := catalog.Resolve("gemini-future", effort)
		require.NoError(t, err)
		require.Equal(t, model, resolved)
	}
}
