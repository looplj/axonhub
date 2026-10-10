package server

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCodexRoutes_LegacyModelsFormatAndNoAuth(t *testing.T) {
	h := newCodexRouteHarness(t)
	result := h.request("GET", "/v1/models", "")
	require.Equal(t, 200, result.Code, result.Body.String())
	require.JSONEq(t, `{"object":"list","data":[]}`, result.Body.String())
	require.Equal(t, 404, h.request("GET", "/codex/models", "").Code)
	require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
	require.Equal(t, 401, h.request("GET", "/codex/models", "").Code)
	require.Equal(t, 200, h.request("GET", "/v1/models", "").Code)
}

func TestCodexRoutes_KeyIPRestriction(t *testing.T) {
	h := newCodexRouteHarness(t)
	require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
	restricted := h.db.APIKey.Create().SetName("restricted").SetKey("ah-restricted").SetType(apikey.TypeUser).SetUserID(h.key.UserID).SetProjectID(h.key.ProjectID).SetAllowedIps([]string{"10.0.0.1"}).SaveX(h.ctx)
	require.Equal(t, 403, h.request("GET", "/codex/v1/user-auth-credential/whoami", restricted.Key).Code)
}

func TestCodexRoutes_CallerPermissionDenied(t *testing.T) {
	h := newCodexRouteHarness(t)
	var catalogCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		catalogCalls.Add(1)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()
	h.db.Channel.UpdateOneID(h.source.ID).SetBaseURL(upstream.URL).SaveX(h.ctx)
	require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
	restricted := h.db.APIKey.Create().SetName("restricted").SetKey("ah-restricted").SetType(apikey.TypeUser).SetUserID(h.key.UserID).SetProjectID(h.key.ProjectID).SetScopes([]string{"write_requests"}).SaveX(h.ctx)
	require.Equal(t, 403, h.request("GET", "/codex/models", restricted.Key).Code)
	require.Zero(t, catalogCalls.Load())
}

func TestCodexRoutes_IPBlocklistPrecedesSwitch(t *testing.T) {
	h := newCodexRouteHarness(t)
	require.NoError(t, h.system.SetSecuritySettings(h.ctx, biz.SecuritySettings{BlockedIPs: []string{"192.0.2.1"}}))
	require.Equal(t, 403, h.request("GET", "/codex/models", "").Code)
}
