package gql

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/stretchr/testify/require"
)

func TestCodexGraphQLSurface(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	setup := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte("private-secret"))
	}))
	defer upstream.Close()
	ids := []int{}
	for _, status := range []channel.Status{channel.StatusEnabled, channel.StatusDisabled, channel.StatusArchived} {
		source := db.Channel.Create().SetName(string(status)).SetType(channel.TypeCodex).SetStatus(status).SetBaseURL(upstream.URL).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{APIKey: "private-secret"}).SaveX(setup)
		ids = append(ids, source.ID)
	}
	channels := biz.NewChannelServiceForTest(db)
	defer channels.Stop()
	resolver := &Resolver{client: db, channelService: channels, systemService: biz.NewSystemService(biz.SystemServiceParams{Ent: db})}
	graphql := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolver}))
	surface := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := authz.NewUserContext(ent.NewContext(r.Context(), db), 123)
		ctx = contexts.WithUser(ctx, &ent.User{ID: 123, Scopes: []string{"read_settings", "write_settings", "read_channels"}})
		graphql.ServeHTTP(w, r.WithContext(ctx))
	}))
	defer surface.Close()
	for _, tc := range []struct {
		query    string
		expected string
	}{
		{`query {codexCompatibilitySettings {enabled channelID}}`, `{"data":{"codexCompatibilitySettings":{"enabled":false,"channelID":null}}}`},
		{fmt.Sprintf(`mutation {updateCodexCompatibilitySettings(input:{enabled:true,channelID:%d})}`, ids[1]), `{"data":{"updateCodexCompatibilitySettings":true}}`},
		{`query {codexCompatibilitySettings {enabled channelID}}`, fmt.Sprintf(`{"data":{"codexCompatibilitySettings":{"enabled":true,"channelID":%d}}}`, ids[1])},
		{fmt.Sprintf(`mutation {testCodexCatalog(channelID:%d){success modelCount upstreamStatus error}}`, ids[2]), `{"data":{"testCodexCatalog":{"success":false,"modelCount":0,"upstreamStatus":404,"error":"Codex catalog upstream HTTP 404"}}}`},
	} {
		payload, err := json.Marshal(map[string]string{"query": tc.query})
		require.NoError(t, err)
		response, err := surface.Client().Post(surface.URL, "application/json", strings.NewReader(string(payload)))
		require.NoError(t, err)
		raw, err := io.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())
		require.NoError(t, err)
		require.Equal(t, 200, response.StatusCode)
		require.JSONEq(t, tc.expected, string(raw))
		require.NotContains(t, string(raw), "private-secret")
	}
	options, err := resolver.Query().CodexCatalogChannels(setup)
	require.NoError(t, err)
	require.Len(t, options, 3)
	for _, scopes := range [][]string{{"read_settings"}, {"read_channels"}, nil} {
		ctx := authz.NewUserContext(ent.NewContext(t.Context(), db), 123)
		ctx = contexts.WithUser(ctx, &ent.User{ID: 123, Scopes: scopes})
		_, err := resolver.Query().CodexCatalogChannels(ctx)
		require.Error(t, err)
	}
}
