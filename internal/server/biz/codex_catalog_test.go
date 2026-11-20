package biz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func TestCodexCatalog_RawMetadata(t *testing.T) {
	catalog, err := ParseCodexCatalog([]byte(`{"extension":{"v":2},"models":[{"slug":"A","visibility":"hide","nested":{"x":1}},{"slug":"a"},{"slug":"B"}]}`))
	require.NoError(t, err)
	raw, err := catalog.Intersect([]string{"A", "B", "local-only"})
	require.NoError(t, err)
	require.JSONEq(t, `{"extension":{"v":2},"models":[{"slug":"A","visibility":"hide","nested":{"x":1}},{"slug":"B"}]}`, string(raw))
}

func TestCodexCatalog_EmptyArray(t *testing.T) {
	catalog, err := ParseCodexCatalog([]byte(`{"models":[]}`))
	require.NoError(t, err)
	raw, err := catalog.Intersect(nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"models":[]}`, string(raw))
}

func TestCodexCatalog_ErrorMapping(t *testing.T) {
	for _, body := range []string{`not-json`, `{"data":[]}`, `{"models":null}`, `{"models":[{}]}`, `{"models":[{"slug":1}]}`} {
		_, err := ParseCodexCatalog([]byte(body))
		require.Error(t, err)
	}
	for _, status := range []int{401, 404} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte("private-secret"))
			}))
			defer upstream.Close()
			db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			defer db.Close()
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
			source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetBaseURL(upstream.URL).SetCredentials(objects.ChannelCredentials{APIKey: "source-key"}).SaveX(ctx)
			_, err := NewChannelServiceForTest(db).FetchCodexCatalog(ctx, source.ID, "")
			var catalogErr *CodexCatalogError
			require.ErrorAs(t, err, &catalogErr)
			require.Equal(t, status, catalogErr.UpstreamStatus)
			require.NotContains(t, err.Error(), "private-secret")
		})
	}
}

func TestCodexCatalog_ClientVersionPrecedence(t *testing.T) {
	requests := make(chan *http.Request, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/models", r.URL.Path)
		require.Equal(t, "Bearer source-key", r.Header.Get("Authorization"))
		requests <- r
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	const channelUA = "codex_cli_rs/0.171.0-alpha.12 (test)"
	ops := []objects.OverrideOperation{
		{Op: objects.OverrideOpSet, Path: "VERSION", Value: "0.170.0"},
		{Op: objects.OverrideOpSet, Path: "User-Agent", Value: channelUA},
	}
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetStatus(channel.StatusDisabled).
		SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).
		SetBaseURL(upstream.URL + "///?client_version=ignored&client_version=ignored").
		SetSettings(&objects.ChannelSettings{HeaderOverrideOperations: ops}).
		SetCredentials(objects.ChannelCredentials{APIKey: "source-key"}).SaveX(ctx)
	svc := NewChannelServiceForTest(db)
	t.Cleanup(svc.Stop)
	for _, test := range []struct{ input, want string }{
		{"0.160.0", "0.160.0"},
		{"", "0.170.0"},
		{"v +/&? injected=1", "0.170.0"},
	} {
		got, err := svc.FetchCodexCatalog(ctx, source.ID, test.input)
		require.NoError(t, err)
		require.Zero(t, got.Count())
		req := <-requests
		require.Equal(t, []string{test.want}, req.URL.Query()["client_version"])
		require.Equal(t, test.want, req.Header.Get("Version"))
		require.Equal(t, channelUA, req.Header.Get("User-Agent"))
	}
	ops = append(ops, objects.OverrideOperation{Op: objects.OverrideOpDelete, Path: "version"})
	db.Channel.UpdateOneID(source.ID).SetSettings(&objects.ChannelSettings{HeaderOverrideOperations: ops}).SaveX(ctx)
	_, err := svc.FetchCodexCatalog(ctx, source.ID, "")
	require.NoError(t, err)
	req := <-requests
	require.Equal(t, []string{"0.171.0"}, req.URL.Query()["client_version"])
	require.Empty(t, req.Header.Values("Version"))
	require.Equal(t, channelUA, req.Header.Get("User-Agent"))
	require.Equal(t, channel.StatusDisabled, db.Channel.GetX(ctx, source.ID).Status)
}

func TestCodexCatalog_RedirectCredentials(t *testing.T) {
	var leaked bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("Authorization") != ""
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer origin.Close()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetBaseURL(origin.URL).SetCredentials(objects.ChannelCredentials{APIKey: "source-key"}).SaveX(ctx)
	_, err := NewChannelServiceForTest(db).FetchCodexCatalog(ctx, source.ID, "")
	require.Error(t, err)
	require.False(t, leaked)
}
