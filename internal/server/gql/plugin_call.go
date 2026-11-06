package gql

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/plugin"
)

const (
	// PluginGraphqlTimeout is the budget for POST /admin/plugins/:pluginId/graphql.
	PluginGraphqlTimeout = 10 * time.Second
	// PluginFetchTimeout is the budget for POST /admin/plugins/:pluginId/fetch.
	PluginFetchTimeout = 15 * time.Second

	pluginGraphqlComplexityLimit  = 1000
	pluginGraphqlMaxResponseBytes = 2 << 20
)

var errPluginResponseTooLarge = errors.New("plugin graphql response exceeds 2 MiB")

type pluginGraphqlHTTPHandler struct {
	handler http.Handler
	manager *plugin.Manager
}

func (h *pluginGraphqlHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	manifestID, ok := pluginManifestID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	user, ok := contexts.GetUser(r.Context())
	if !ok || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	row := activePluginPage(h.manager, manifestID)
	if row == nil {
		http.NotFound(w, r)
		return
	}

	granted := append([]string(nil), row.GrantedPermissions...)
	call := plugin.Call{
		ManifestID:     row.PluginID,
		InstallationID: row.ID,
		KVWritable:     scopes.HasSystemScope(user, scopes.ScopeWritePlugins),
	}
	ctx, err := authz.DelegatePlugin(r.Context(), row.ID, pluginCallCeiling(granted))
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ctx = plugin.WithCall(ctx, call)

	capped := newCappedResponseWriter(w, pluginGraphqlMaxResponseBytes)
	defer capped.finish()
	h.handler.ServeHTTP(capped, r.WithContext(ctx))
}

func pluginManifestID(r *http.Request) (string, bool) {
	if r == nil || r.URL == nil {
		return "", false
	}
	const suffix = "/graphql"
	path := r.URL.Path
	if !strings.HasSuffix(path, suffix) {
		return "", false
	}
	trimmed := strings.TrimSuffix(path, suffix)
	const marker = "/plugins/"
	_, after, ok := strings.CutLast(trimmed, marker)
	if !ok {
		return "", false
	}
	id := after
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	decoded, err := url.PathUnescape(id)
	if err != nil || decoded == "" || strings.Contains(decoded, "/") {
		return "", false
	}
	return decoded, true
}

func activePluginPage(manager *plugin.Manager, manifestID string) *plugin.LoadedPlugin {
	if manager == nil || manifestID == "" {
		return nil
	}
	current := manager.Current()
	if current == nil {
		return nil
	}
	row := current.ByPluginID(manifestID)
	if row == nil || !row.Enabled || row.Status != plugin.RuntimeStatusActive {
		return nil
	}
	if row.Manifest.UI == nil || len(row.Manifest.UI.Pages) == 0 {
		return nil
	}
	return row
}

func pluginCallCeiling(granted []string) []string {
	blocked := map[string]struct{}{
		"webhook":                        {},
		string(scopes.ScopeReadPlugins):  {},
		string(scopes.ScopeWritePlugins): {},
	}
	seen := make(map[string]struct{}, len(granted))
	out := make([]string, 0, len(granted))
	for _, item := range granted {
		if _, skip := blocked[item]; skip {
			continue
		}
		if _, dup := seen[item]; dup {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

type cappedResponseWriter struct {
	dst         http.ResponseWriter
	limit       int
	header      http.Header
	buf         bytes.Buffer
	status      int
	wroteHeader bool
	overflow    bool
	finished    bool
}

func newCappedResponseWriter(dst http.ResponseWriter, limit int) *cappedResponseWriter {
	return &cappedResponseWriter{
		dst:    dst,
		limit:  limit,
		header: make(http.Header),
		status: http.StatusOK,
	}
}

func (w *cappedResponseWriter) Header() http.Header {
	return w.header
}

func (w *cappedResponseWriter) WriteHeader(status int) {
	if w.wroteHeader || w.overflow {
		return
	}
	w.status = status
	w.wroteHeader = true
}

func (w *cappedResponseWriter) Write(p []byte) (int, error) {
	if w.overflow {
		return 0, errPluginResponseTooLarge
	}
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	if w.buf.Len()+len(p) > w.limit {
		w.overflow = true
		w.buf.Reset()
		return 0, errPluginResponseTooLarge
	}
	return w.buf.Write(p)
}

func (w *cappedResponseWriter) finish() {
	if w == nil || w.finished {
		return
	}
	w.finished = true
	if w.overflow {
		http.Error(w.dst, "plugin graphql response exceeds 2 MiB", http.StatusInternalServerError)
		return
	}
	dstHeader := w.dst.Header()
	for key, values := range w.header {
		dstHeader[key] = append([]string(nil), values...)
	}
	w.dst.WriteHeader(w.status)
	if w.buf.Len() == 0 {
		return
	}
	_, _ = w.dst.Write(w.buf.Bytes())
}

func pluginCallFromContext(ctx context.Context) (plugin.Call, error) {
	call, ok := plugin.CallFromContext(ctx)
	if !ok || call.InstallationID <= 0 {
		return plugin.Call{}, xerrors.ForbiddenError("permission denied")
	}
	return call, nil
}

func requirePluginKVWritable(call plugin.Call) error {
	if call.KVWritable {
		return nil
	}
	return xerrors.ForbiddenError("permission denied")
}

func pluginKVTTL(ttlSeconds *int) time.Duration {
	return time.Duration(lo.FromPtr(ttlSeconds)) * time.Second
}
