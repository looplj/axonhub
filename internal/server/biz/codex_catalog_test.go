package biz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/stretchr/testify/require"
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

func TestCodexCatalog_ClientVersionEncoding(t *testing.T) {
	version := "v +/&? injected=1"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/models", r.URL.Path)
		require.Equal(t, version, r.URL.Query().Get("client_version"))
		require.Len(t, r.URL.Query(), 1)
		require.Equal(t, "Bearer source-key", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetStatus(channel.StatusDisabled).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetBaseURL(upstream.URL + "///").SetCredentials(objects.ChannelCredentials{APIKey: "source-key"}).SaveX(ctx)
	got, err := NewChannelServiceForTest(db).FetchCodexCatalog(ctx, source.ID, version)
	require.NoError(t, err)
	require.Zero(t, got.Count())
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
