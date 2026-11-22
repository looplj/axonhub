// 职责: 验证 DSH 插件通过网关鉴权与追踪中间件复用持久化会话。
// 关联文件:
//   internal/server/middleware/server_plugin.go
//   internal/server/middleware/trace.go
//   internal/server/middleware/auth.go
//   plugins/examples/dsh-session-trace/plugin.js

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/plugin"
	"github.com/looplj/axonhub/internal/tracing"
	"github.com/looplj/axonhub/llm/transformer/opencode"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestServerMiddlewareDSHTrace(t *testing.T) {
	router, client, traceService := setupTestTraceMiddleware(t)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	owner := client.User.Create().SetEmail("dsh@example.invalid").SetPassword("test").SetIsOwner(true).SaveX(ctx)
	testProject := client.Project.Create().SetName("dsh project").SetStatus(project.StatusActive).SaveX(ctx)
	key := client.APIKey.Create().SetName("dsh key").SetKey("ah-dsh-middleware-test").SetUserID(owner.ID).SetProjectID(testProject.ID).SetType(apikey.TypeUser).SaveX(ctx)
	cache := xcache.Config{Mode: xcache.ModeMemory}
	system := biz.NewSystemService(biz.SystemServiceParams{Ent: client, CacheConfig: cache})
	projects := biz.NewProjectService(biz.ProjectServiceParams{Ent: client, CacheConfig: cache})
	keys := biz.NewAPIKeyService(biz.APIKeyServiceParams{Ent: client, CacheConfig: cache, ProjectService: projects, KeyPrefix: "ah"})
	t.Cleanup(keys.Stop)
	auth := biz.NewAuthService(biz.AuthServiceParams{Ent: client, SystemService: system, APIKeyService: keys})
	plugins, err := plugin.NewManager(plugin.ManagerParams{Client: client, Cache: cache})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, plugins.Stop(context.Background())) })

	root := filepath.Join("..", "..", "..", "plugins", "examples", "dsh-session-trace")
	archive, _, err := plugin.PackDirectory(root)
	require.NoError(t, err)
	installed, err := plugins.Install(ctx, archive, nil, nil)
	require.NoError(t, err)
	require.NoError(t, plugins.Enable(ctx, installed.Plugin.ID, nil, nil))

	config := tracing.Config{OpenCodeTraceEnabled: true, ExtraTraceBodyFields: []string{"trace_id"}}
	router.Use(WithEntClient(client), WithStrictAPIKeyConfig(auth, nil), WithServerPluginMiddleware(plugins),
		WithThread(config, biz.NewThreadService(client, traceService)), WithTrace(config, traceService))
	var trace *ent.Trace
	handler := func(c *gin.Context) {
		var ok bool
		trace, ok = contexts.GetTrace(c.Request.Context())
		if ok {
			session, hasSession := shared.GetSessionID(c.Request.Context())
			require.True(t, hasSession)
			require.Equal(t, trace.TraceID, session)
		}
		c.Status(http.StatusNoContent)
	}
	router.POST("/v1/chat/completions", handler)
	router.POST("/v1/embeddings", handler)

	send := func(path, apiKey, explicit, openCode, body string) int {
		trace = nil
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-Deepseek-Harness-Session-Id", "  dsh-session  ")
		req.Header.Set("Ah-Trace-Id", explicit)
		req.Header.Set(opencode.SessionHeader, openCode)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	chat := "/v1/chat/completions"
	require.Equal(t, http.StatusUnauthorized, send(chat, "wrong-key", "", "", "{}"))
	require.Nil(t, trace)
	require.Equal(t, http.StatusNoContent, send(chat, key.Key, "", "", `{"trace_id":"body"}`))
	require.NotNil(t, trace)
	require.Equal(t, "dsh-session", trace.TraceID)
	require.Equal(t, testProject.ID, trace.ProjectID)
	id := trace.ID
	require.Equal(t, http.StatusNoContent, send(chat, key.Key, "", "", "{}"))
	require.NotNil(t, trace)
	require.Equal(t, id, trace.ID)
	require.Equal(t, http.StatusNoContent, send(chat, key.Key, "explicit", "", "{}"))
	require.NotNil(t, trace)
	require.Equal(t, "explicit", trace.TraceID)
	require.Equal(t, http.StatusNoContent, send(chat, key.Key, "", "opencode", "{}"))
	require.NotNil(t, trace)
	require.Equal(t, "opencode", trace.TraceID)
	require.Equal(t, http.StatusNoContent, send("/v1/embeddings", key.Key, "", "", "{}"))
	require.Nil(t, trace)

	require.NoError(t, plugins.Disable(ctx, installed.Plugin.ID))
	require.Equal(t, http.StatusNoContent, send(chat, key.Key, "", "", "{}"))
	require.Nil(t, trace)
	require.Equal(t, http.StatusNoContent, send(chat, key.Key, "", "", `{"trace_id":"body"}`))
	require.NotNil(t, trace)
	require.Equal(t, "body", trace.TraceID)
}
