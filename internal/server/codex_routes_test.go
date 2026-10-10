package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/api"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/internal/server/static"
)

type codexRouteHarness struct {
	router   *gin.Engine
	db       *ent.Client
	ctx      context.Context
	system   *biz.SystemService
	channels *biz.ChannelService
	key      *ent.APIKey
	keys     *biz.APIKeyService
	source   *ent.Channel
	orch     *orchestrator.ChatCompletionOrchestrator
}

func newCodexRouteHarness(t *testing.T) *codexRouteHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := "file:codex_routes_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&_fk=0"
	db := enttest.NewEntClient(t, "sqlite3", dsn)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	owner := db.User.Create().SetEmail("codex@example.invalid").SetPassword("test").SetIsOwner(true).SaveX(ctx)
	project := db.Project.Create().SetName("test").SaveX(ctx)
	key := db.APIKey.Create().SetName("display name").SetKey("ah-codex-test").SetUserID(owner.ID).SetProjectID(project.ID).SetType(apikey.TypeUser).SaveX(ctx)
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetStatus(channel.StatusDisabled).SetBaseURL("https://example.invalid").SetCredentials(objects.ChannelCredentials{APIKey: "upstream"}).SetDefaultTestModel("test").SetSupportedModels([]string{"A", "B"}).SaveX(ctx)
	cache := xcache.Config{Mode: xcache.ModeMemory}
	system := biz.NewSystemService(biz.SystemServiceParams{Ent: db, CacheConfig: cache})
	channels := biz.NewChannelServiceForTest(db)
	t.Cleanup(channels.Stop)
	projects := biz.NewProjectService(biz.ProjectServiceParams{Ent: db, CacheConfig: cache})
	keys := biz.NewAPIKeyService(biz.APIKeyServiceParams{Ent: db, CacheConfig: cache, ProjectService: projects, KeyPrefix: "ah"})
	t.Cleanup(keys.Stop)
	auth := biz.NewAuthService(biz.AuthServiceParams{Ent: db, SystemService: system, APIKeyService: keys, AllowNoAuth: true})
	models := biz.NewModelService(biz.ModelServiceParams{Ent: db, SystemService: system, ChannelService: channels})
	compatibility := api.NewCodexCompatibilityHandlers(system, channels, models)
	h := &codexRouteHarness{db: db, ctx: ctx, system: system, channels: channels, key: key, keys: keys, source: source}
	openAI, requests := setupCodexResponseHandlers(t, h, models)
	router := gin.New()
	router.Use(middleware.WithEntClient(db))
	router.NoRoute(static.Handler())
	timeout := time.Second
	if os.Getenv("AXONHUB_CODEX_LIVE_QA") == "1" {
		timeout = 90 * time.Second
	}
	registerCodexRoutes(&Server{Engine: router, Config: Config{LLMRequestTimeout: timeout}}, Handlers{CodexCompatibility: compatibility, OpenAI: openAI}, Services{AuthService: auth, SystemService: system, RequestService: requests})
	legacy := router.Group("/v1", middleware.WithAPIKeyConfig(auth, nil), middleware.WithTimeout(timeout))
	legacy.POST("/responses", openAI.CreateResponse)
	legacy.GET("/models", openAI.ListModels)
	router.GET("/v1/responses", middleware.WithAPIKeyConfig(auth, nil), openAI.CreateResponseWebSocket(timeout))
	h.router = router
	h.orch = openAI.ResponseCompletionHandlers.ChatCompletionOrchestrator
	return h
}

func (h *codexRouteHarness) request(method, path, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Chatgpt-Account-Id", "untrusted")
	req.Header.Set("If-None-Match", "caller-etag")
	req.Header.Set("Cookie", "caller-cookie=synthetic")
	result := httptest.NewRecorder()
	h.router.ServeHTTP(result, req)
	return result
}

func TestCodexRoutes_Disabled(t *testing.T) {
	h := newCodexRouteHarness(t)
	for _, route := range []struct{ method, path string }{{"GET", "/codex/models"}, {"GET", "/codex/v1/user-auth-credential/whoami"}, {"POST", "/codex/responses"}, {"GET", "/codex/responses"}} {
		for _, key := range []string{"", "bad", h.key.Key} {
			result := h.request(route.method, route.path, key)
			require.Equal(t, 404, result.Code)
			require.Contains(t, result.Header().Get("Content-Type"), "application/json")
			require.Equal(t, "no-store", result.Header().Get("Cache-Control"))
		}
	}
}

func TestCodexRoutes_StrictAuthentication(t *testing.T) {
	h := newCodexRouteHarness(t)
	require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
	for _, key := range []string{"", "bad", biz.NoAuthAPIKeyValue} {
		require.Equal(t, 401, h.request("GET", "/codex/models", key).Code)
	}
}

func TestCodexRoutes_WhoamiStableID(t *testing.T) {
	h := newCodexRouteHarness(t)
	require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
	result := h.request("GET", "/codex/v1/user-auth-credential/whoami", h.key.Key)
	require.Equal(t, 200, result.Code)
	var first map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(result.Body.Bytes(), &first))
	require.JSONEq(t, `"display name"`, string(first["email"]))
	require.JSONEq(t, `"promax"`, string(first["chatgpt_plan_type"]))
	require.Equal(t, "false", string(first["chatgpt_account_is_fedramp"]))
	require.NotContains(t, result.Body.String(), h.key.Key)
	_, err := h.keys.UpdateAPIKey(h.ctx, h.key.ID, ent.UpdateAPIKeyInput{Name: lo.ToPtr("renamed")})
	require.NoError(t, err)
	var after map[string]json.RawMessage
	require.Eventually(t, func() bool {
		second := h.request(http.MethodGet, "/codex/v1/user-auth-credential/whoami", h.key.Key)
		if second.Code != http.StatusOK || json.Unmarshal(second.Body.Bytes(), &after) != nil {
			return false
		}
		return string(after["email"]) == `"renamed"`
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, first["chatgpt_user_id"], after["chatgpt_user_id"])
	require.Equal(t, first["chatgpt_account_id"], after["chatgpt_account_id"])
}

func TestCodexRoutes_UnknownPaths404(t *testing.T) {
	h := newCodexRouteHarness(t)
	for _, path := range []string{"/codex/alpha/history/v2/test", "/codex/responses/compact", "/codex/missing"} {
		result := h.request("POST", path, h.key.Key)
		require.Equal(t, 404, result.Code)
		require.Contains(t, result.Header().Get("Content-Type"), "application/json")
	}
}
