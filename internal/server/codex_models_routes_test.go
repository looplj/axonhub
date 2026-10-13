package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCodexRoutes_CallerIntersection(t *testing.T) {
	h := newCodexRouteHarness(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer upstream", r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("If-None-Match"))
		require.Empty(t, r.Header.Get("Cookie"))
		w.Header().Set("ETag", "upstream-etag")
		w.Header().Set("Set-Cookie", "private-cookie")
		_, _ = w.Write([]byte(`{"extension":true,"models":[{"slug":"B","visibility":"hide","details":{"future":3}},{"slug":"A"},{"slug":"a"},{"slug":"remote-only"}]}`))
	}))
	defer upstream.Close()
	h.db.Channel.UpdateOneID(h.source.ID).SetBaseURL(upstream.URL).SaveX(h.ctx)
	inference := h.db.Channel.Create().SetName("inference").SetType(channel.TypeOpenai).SetBaseURL("https://example.invalid").SetDefaultTestModel("A").SetSupportedModels([]string{"A", "B", "local-only"}).SetCredentials(objects.ChannelCredentials{APIKey: "infer-key"}).SaveX(h.ctx)
	built, err := h.channels.GetChannel(h.ctx, inference.ID)
	require.NoError(t, err)
	h.channels.SetEnabledChannelsForTest([]*biz.Channel{built})
	require.NoError(t, h.system.SetModelSettings(h.ctx, biz.SystemModelSettings{QueryAllChannelModels: true}))
	require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
	for _, tc := range []struct{ key, model, expected string }{
		{"ah-first", "A", `{"extension":true,"models":[{"slug":"A"}]}`},
		{"ah-second", "B", `{"extension":true,"models":[{"slug":"B","visibility":"hide","details":{"future":3}}]}`},
		{"ah-empty", "local-only", `{"extension":true,"models":[]}`},
	} {
		h.db.APIKey.Create().SetName(tc.key).SetKey(tc.key).SetType(apikey.TypeUser).SetUserID(h.key.UserID).SetProjectID(h.key.ProjectID).SetProfiles(&objects.APIKeyProfiles{ActiveProfile: "only", Profiles: []objects.APIKeyProfile{{Name: "only", ModelIDs: []string{tc.model}}}}).SaveX(h.ctx)
		result := h.request("GET", "/codex/models?client_version=0.162.1", tc.key)
		require.Equal(t, 200, result.Code, result.Body.String())
		require.JSONEq(t, tc.expected, result.Body.String())
		require.Equal(t, "no-store", result.Header().Get("Cache-Control"))
		require.Empty(t, result.Header().Get("ETag"))
		require.Empty(t, result.Header().Get("Set-Cookie"))
	}
	require.Equal(t, channel.StatusDisabled, h.db.Channel.GetX(h.ctx, h.source.ID).Status)
}

func TestCodexRoutes_CatalogErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{{"upstream auth", 401, "private"}, {"upstream404", 404, "private"}, {"bad json", 200, "private"}, {"wrong structure", 200, `{"data":[]}`}} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCodexRouteHarness(t)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer upstream.Close()
			h.db.Channel.UpdateOneID(h.source.ID).SetBaseURL(upstream.URL).SaveX(h.ctx)
			require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
			result := h.request("GET", "/codex/models", h.key.Key)
			require.Equal(t, 502, result.Code)
			require.NotContains(t, result.Body.String(), "private")
		})
	}
}

func TestCodexRoutes_InvalidSource(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		h := newCodexRouteHarness(t)
		require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
		if deleted {
			require.NoError(t, h.db.Channel.DeleteOneID(h.source.ID).Exec(h.ctx))
		} else {
			h.db.Channel.UpdateOneID(h.source.ID).SetType(channel.TypeOpenai).SaveX(h.ctx)
		}
		require.Equal(t, 503, h.request("GET", "/codex/models", h.key.Key).Code)
	}
}

func TestCodexRoutes_SettingReadFailure(t *testing.T) {
	h := newCodexRouteHarness(t)
	h.db.System.Create().SetKey(biz.SystemKeyCodexCompatibility).SetValue("bad JSON").SaveX(h.ctx)
	require.Equal(t, 500, h.request("GET", "/codex/models", "").Code)
}
