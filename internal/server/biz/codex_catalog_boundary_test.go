package biz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestCodexCatalog_TransportTimeout(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetBaseURL(upstream.URL).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{APIKey: "source-key"}).SaveX(ctx)
	svc := NewChannelServiceForTest(db)
	defer svc.Stop()
	deadline, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	_, err := svc.FetchCodexCatalog(deadline, source.ID, "")
	close(release)
	var catalogErr *CodexCatalogError
	require.ErrorAs(t, err, &catalogErr)
	require.Equal(t, 502, catalogErr.Status)
	require.Equal(t, "Codex catalog request was cancelled", catalogErr.Message)
}

func TestCodexCatalog_UsesRuntimeProviderOnCacheHit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer runtime-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetBaseURL(upstream.URL).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "runtime-token", ExpiresAt: time.Now().Add(time.Hour)}}).SaveX(ctx)
	svc := NewChannelServiceForTest(db)
	defer svc.Stop()
	cached, err := svc.GetChannel(ctx, source.ID)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{cached})
	db.Channel.UpdateOneID(source.ID).SetStatus(channel.StatusDisabled).SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "db-token", ExpiresAt: time.Now().Add(time.Hour)}}).SaveX(ctx)
	_, err = svc.FetchCodexCatalog(ctx, source.ID, "")
	require.NoError(t, err)
}

func TestCodexCatalog_ReusesOAuthProviderForDisabledChannel(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetBaseURL("https://example.com/codex").SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetStatus(channel.StatusDisabled).SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(-time.Hour),
	}}).SaveX(ctx)
	svc := NewChannelServiceForTest(db)
	defer svc.Stop()

	first, _, err := svc.codexCatalogOutbound(ctx, source)
	require.NoError(t, err)
	second, _, err := svc.codexCatalogOutbound(ctx, source)
	require.NoError(t, err)
	require.Same(t, first, second)
}

func TestCodexCatalog_DefaultClientRedirectGap(t *testing.T) {
	var propagated bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		propagated = r.Header.Get("Authorization") != ""
		_, _ = w.Write([]byte(`{}`))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer origin.Close()
	_, err := httpclient.NewHttpClient().Do(t.Context(), &httpclient.Request{Method: "GET", URL: origin.URL, Headers: http.Header{"Authorization": []string{"Bearer synthetic"}}})
	require.NoError(t, err)
	require.True(t, propagated, "existing client forwards authorization across same-host different-port origins")
}
