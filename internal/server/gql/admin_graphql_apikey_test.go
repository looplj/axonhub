package gql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
)

// TestAdminGraphqlServiceAccount reads the management data with a service
// account API key and checks that the same key cannot mutate it.
func TestAdminGraphqlServiceAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = db.Close() })

	setupCtx := ent.NewContext(context.Background(), db)
	setupCtx = authz.WithTestBypass(setupCtx)

	hashed, err := biz.HashPassword("test-password")
	require.NoError(t, err)

	owner := db.User.Create().
		SetEmail(fmt.Sprintf("owner-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashed).
		SetFirstName("Owner").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		SaveX(setupCtx)

	proj := db.Project.Create().
		SetName(fmt.Sprintf("project-%d", time.Now().UnixNano())).
		SetStatus(project.StatusActive).
		SaveX(setupCtx)

	mustKey := func(name string, keyScopes []string) string {
		value, err := biz.GenerateAPIKey("ah")
		require.NoError(t, err)

		db.APIKey.Create().
			SetName(name).
			SetKey(value).
			SetUserID(owner.ID).
			SetProjectID(proj.ID).
			SetType(apikey.TypeServiceAccount).
			SetStatus(apikey.StatusEnabled).
			SetScopes(keyScopes).
			SaveX(setupCtx)

		return value
	}

	readKey := mustKey("read-only", []string{string(scopes.ScopeReadChannels)})
	noScopeKey := mustKey("no-scope", []string{})

	ch := db.Channel.Create().
		SetName("seeded channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SaveX(setupCtx)

	db.Model.Create().
		SetDeveloper("openai").
		SetModelID("gpt-4").
		SetName("GPT-4").
		SetIcon("openai").
		SetGroup("openai").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{}).
		SetStatus(model.StatusEnabled).
		SaveX(setupCtx)

	channelSvc := biz.NewChannelServiceForTest(db)
	t.Cleanup(channelSvc.Stop)

	handler := NewGraphqlHandlers(Dependencies{Ent: db, ChannelService: channelSvc})

	cacheCfg := xcache.Config{Mode: xcache.ModeMemory}
	projectSvc := &biz.ProjectService{
		ProjectCache: xcache.NewFromConfig[xcache.Entry[ent.Project]](cacheCfg),
	}
	apiKeySvc := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig: cacheCfg, Ent: db, ProjectService: projectSvc, KeyPrefix: "ah",
	})
	t.Cleanup(apiKeySvc.Stop)
	systemSvc := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cacheCfg, Ent: db})
	authSvc := biz.NewAuthService(biz.AuthServiceParams{
		SystemService: systemSvc, APIKeyService: apiKeySvc, Ent: db,
	})

	engine := gin.New()
	engine.Use(middleware.WithEntClient(db))
	engine.POST("/admin/graphql", middleware.WithAdminGraphqlAuth(authSvc), func(c *gin.Context) {
		handler.Graphql.ServeHTTP(c.Writer, c.Request)
	})

	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	t.Run("reads channels and models", func(t *testing.T) {
		code, body := adminGraphqlPost(t, server.URL, readKey,
			`query { channels(first: 5) { edges { node { id name } } } models(first: 5) { edges { node { id name } } } }`)
		require.Equal(t, http.StatusOK, code)
		require.NotContains(t, string(body), `"errors"`, "body: %s", body)
		require.Contains(t, string(body), "seeded channel")
		require.Contains(t, string(body), "GPT-4")
	})

	t.Run("mutation is refused", func(t *testing.T) {
		code, body := adminGraphqlPost(t, server.URL, readKey,
			fmt.Sprintf(`mutation { deleteChannel(id: "gid://axonhub/Channel/%d") }`, ch.ID))
		require.Equal(t, http.StatusOK, code)
		require.Contains(t, string(body), "read-only", "body: %s", body)
	})

	t.Run("missing scope is denied", func(t *testing.T) {
		code, body := adminGraphqlPost(t, server.URL, noScopeKey,
			`query { channels(first: 5) { edges { node { id } } } }`)
		require.Equal(t, http.StatusOK, code)
		require.Contains(t, string(body), `"errors"`, "body: %s", body)
	})
}

func adminGraphqlPost(t *testing.T, baseURL, bearer, query string) (int, []byte) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"query": query})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/admin/graphql", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, body
}
