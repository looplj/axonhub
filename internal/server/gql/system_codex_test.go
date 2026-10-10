package gql

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCodexCompatibilitySettingsAuthorization(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	setup := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetBaseURL("https://example.invalid").SetCredentials(objects.ChannelCredentials{APIKey: "private"}).SaveX(setup)
	r := &Resolver{client: db, systemService: biz.NewSystemService(biz.SystemServiceParams{Ent: db})}
	for _, tc := range []struct {
		name    string
		scopes  []string
		allowed bool
	}{
		{"none", nil, false},
		{"settings only", []string{"write_settings"}, false},
		{"channel only", []string{"read_channels"}, false},
		{"both non-owner", []string{"write_settings", "read_channels"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := authz.NewUserContext(ent.NewContext(t.Context(), db), 123)
			ctx = contexts.WithUser(ctx, &ent.User{ID: 123, Scopes: tc.scopes})
			ok, err := r.Mutation().UpdateCodexCompatibilitySettings(ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &source.ID})
			if tc.allowed {
				require.NoError(t, err)
				require.True(t, ok)
			} else {
				require.Error(t, err)
				require.False(t, ok)
			}
		})
	}
}

func TestCodexCatalogTestAuthorization(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("private-secret"))
	}))
	defer upstream.Close()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	setup := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetStatus(channel.StatusDisabled).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetBaseURL(upstream.URL).SetCredentials(objects.ChannelCredentials{APIKey: "private"}).SaveX(setup)
	svc := biz.NewChannelServiceForTest(db)
	defer svc.Stop()
	r := &Resolver{client: db, channelService: svc}
	for _, tc := range []struct {
		name    string
		scopes  []string
		allowed bool
	}{
		{"read-only", []string{"read_settings", "read_channels"}, false},
		{"no channel scope", []string{"write_settings"}, false},
		{"non-owner", []string{"write_settings", "read_channels"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := authz.NewUserContext(ent.NewContext(t.Context(), db), 123)
			ctx = contexts.WithUser(ctx, &ent.User{ID: 123, Scopes: tc.scopes})
			result, err := r.Mutation().TestCodexCatalog(ctx, source.ID)
			if tc.allowed {
				require.NoError(t, err)
				require.False(t, result.Success)
				require.Equal(t, 404, *result.UpstreamStatus)
				require.NotContains(t, *result.Error, "private")
			} else {
				require.Error(t, err)
				require.Nil(t, result)
			}
		})
	}
}
