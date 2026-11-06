package gql

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPluginFetchEndpoint(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_fetch")
	upstream := &fetchUpstream{}
	server := httptest.NewTLSServer(upstream)
	t.Cleanup(server.Close)
	fx.manager.SetOutboundTransport(server.Client().Transport)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	exact := server.URL + "/v1/check"
	regex := "^https://" + regexp.QuoteMeta(parsed.Host) + `/v1/items/[0-9]+$`
	installed, err := fx.manager.Install(fx.setupCtx, pluginFetchZip(t, "acme.fetch", []any{
		exact,
		map[string]string{"regex": regex},
	}), nil, nil)
	require.NoError(t, err)
	require.Empty(t, installed.Plugin.GrantedNetwork)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, nil, []string{exact, regex}))

	code, payload, raw := fx.postFetch("", "acme.fetch", map[string]any{"url": exact}, "")
	require.Equal(t, http.StatusUnauthorized, code, raw)

	code, _, raw = fx.postFetch(fx.serviceAccount, "acme.fetch", map[string]any{"url": exact}, "")
	require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, code, raw)

	code, payload, raw = fx.postFetch(fx.writerToken, "acme.missing", map[string]any{"url": exact}, "")
	require.Equal(t, http.StatusNotFound, code, raw)
	require.Equal(t, "NOT_FOUND", payload["code"], raw)
	code, payload, raw = fx.postFetch(fx.writerToken, pluginDisabledID, map[string]any{"url": exact}, "")
	require.Equal(t, http.StatusNotFound, code, raw)
	require.Equal(t, "NOT_FOUND", payload["code"], raw)
	code, payload, raw = fx.postFetch(fx.writerToken, pluginPagelessID, map[string]any{"url": exact}, "")
	require.Equal(t, http.StatusNotFound, code, raw)
	require.Equal(t, "NOT_FOUND", payload["code"], raw)

	code, payload, raw = fx.postFetch(fx.writerToken, "acme.fetch", map[string]any{"url": "https://other.example/v1/check"}, "")
	require.Equal(t, http.StatusForbidden, code, raw)
	require.Equal(t, "FORBIDDEN", payload["code"], raw)
	require.Zero(t, upstream.hits("/v1/check"))

	code, payload, raw = fx.postFetch(fx.ownerToken, "acme.fetch", map[string]any{
		"url":     exact + "?x=1",
		"headers": map[string]string{"Authorization": "plugin-secret"},
	}, "session=user-jwt")
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, float64(http.StatusOK), payload["status"], raw)
	require.Equal(t, "ok", payload["body"], raw)
	require.Equal(t, "text/plain", payload["headers"].(map[string]any)["content-type"])
	require.Equal(t, "plugin-secret", upstream.header("/v1/check").Get("Authorization"))
	require.NotContains(t, upstream.header("/v1/check").Get("Authorization"), fx.ownerToken)
	require.Empty(t, upstream.header("/v1/check").Get("Cookie"))
	require.Empty(t, upstream.header("/v1/check").Get("X-Trace"))
	require.Equal(t, 1, upstream.hits("/v1/check"))

	code, _, raw = fx.postFetch(fx.ownerToken, "acme.fetch", map[string]any{"url": exact + "/child"}, "")
	require.Equal(t, http.StatusForbidden, code, raw)
	require.Equal(t, 1, upstream.hits("/v1/check"))
	require.Zero(t, upstream.hits("/v1/check/child"))

	item := server.URL + "/v1/items/42"
	code, payload, raw = fx.postFetch(fx.ownerToken, "acme.fetch", map[string]any{"url": item + "?q=1"}, "")
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, "item", payload["body"], raw)
	code, payload, raw = fx.postFetch(fx.ownerToken, "acme.fetch", map[string]any{"url": item + "/extra"}, "")
	require.Equal(t, http.StatusForbidden, code, raw)
	require.Equal(t, "FORBIDDEN", payload["code"], raw)
	require.Equal(t, 1, upstream.hits("/v1/items/42"))

	code, payload, raw = fx.postFetch(fx.ownerToken, "acme.fetch", map[string]any{"url": exact + "?redirect=1"}, "")
	require.Equal(t, http.StatusConflict, code, raw)
	require.Equal(t, "INVALID_STATE", payload["code"], raw)
	require.NotContains(t, raw, "secret")
	require.NotContains(t, raw, "Location")
	require.Equal(t, 2, upstream.hits("/v1/check"))
	require.Zero(t, upstream.hits("/secret"))

	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, nil, []string{}))
	require.NoError(t, fx.manager.Refresh(fx.setupCtx))
	code, payload, raw = fx.postFetch(fx.ownerToken, "acme.fetch", map[string]any{"url": exact}, "")
	require.Equal(t, http.StatusForbidden, code, raw)
	require.Equal(t, "FORBIDDEN", payload["code"], raw)
	require.Equal(t, 2, upstream.hits("/v1/check"))
}

func TestPluginFetchBodyLimits(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_fetch_body_limits")
	received := make(chan string, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		received <- string(body)
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(server.Close)
	fx.manager.SetOutboundTransport(server.Client().Transport)
	exact := server.URL + "/body"
	installed, err := fx.manager.Install(fx.setupCtx, pluginFetchZip(t, "acme.fetch-body", []any{exact}), nil, nil)
	require.NoError(t, err)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, nil, []string{exact}))

	for _, body := range []string{
		strings.Repeat(`"`, 600*1024),
		strings.Repeat("\x00", 1<<20), // Six-byte JSON escaping at the decoded limit.
	} {
		code, payload, raw := fx.postFetch(fx.ownerToken, "acme.fetch-body", map[string]any{
			"url": exact, "method": "POST", "body": body,
		}, "")
		require.Equal(t, http.StatusOK, code, raw)
		require.Equal(t, "ok", payload["body"], raw)
		require.Equal(t, body, <-received)
	}

	for _, body := range []string{
		strings.Repeat("a", (1<<20)+1), // Decoded body exceeds its limit.
		strings.Repeat("\x00", 2<<20),  // Encoded envelope exceeds its limit.
	} {
		code, payload, raw := fx.postFetch(fx.ownerToken, "acme.fetch-body", map[string]any{
			"url": exact, "method": "POST", "body": body,
		}, "")
		require.Equal(t, http.StatusRequestEntityTooLarge, code, raw)
		require.Equal(t, "LIMIT_EXCEEDED", payload["code"], raw)
		require.Empty(t, received, "oversized requests must not reach upstream")
	}
}

func (fx *pluginEndpointFixture) postFetch(token, pluginID string, body any, cookie string) (int, map[string]any, string) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err.Error()
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/plugins/"+pluginID+"/fetch", bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trace", "1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	recorder := httptest.NewRecorder()
	fx.engine.ServeHTTP(recorder, req)
	raw := recorder.Body.String()
	var payload map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &payload)
	return recorder.Code, payload, raw
}

func pluginFetchZip(t *testing.T, id string, network []any) []byte {
	t.Helper()
	encoded, err := json.Marshal(network)
	require.NoError(t, err)
	manifest := fmt.Sprintf(`{
  "schemaVersion": 1,
  "id": %q,
  "name": "Fetch Plugin",
  "version": "1.0.0",
  "description": "outbound",
  "axonhubVersion": "^1.0.0",
  "pluginApiVersion": 1,
  "permissions": [],
  "network": %s,
  "ui": {"pages": [{"key": "main", "entry": "ui/page.html", "title": "Fetch"}]}
}`, id, encoded)
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	write := func(name, content string) {
		t.Helper()
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, content)
		require.NoError(t, err)
	}
	write("manifest.json", manifest)
	write("ui/page.html", "<html>fetch</html>")
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

type fetchUpstream struct {
	mu      sync.Mutex
	counts  map[string]int
	headers map[string]http.Header
}

func (u *fetchUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.mu.Lock()
	if u.counts == nil {
		u.counts = map[string]int{}
		u.headers = map[string]http.Header{}
	}
	u.counts[r.URL.Path]++
	u.headers[r.URL.Path] = r.Header.Clone()
	u.mu.Unlock()

	if r.URL.Query().Get("redirect") == "1" {
		w.Header().Set("Location", "https://example.com/secret")
		w.WriteHeader(http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	if r.URL.Path == "/v1/items/42" {
		_, _ = io.WriteString(w, "item")
		return
	}
	_, _ = io.WriteString(w, "ok")
}

func (u *fetchUpstream) hits(path string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.counts[path]
}

func (u *fetchUpstream) header(path string) http.Header {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.headers[path]
}
