package gql

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/looplj/axonhub/internal/server/plugin"
)

const (
	pluginPagesID    = "acme.pages"
	pluginOtherID    = "acme.other"
	pluginPagelessID = "acme.pageless"
	pluginDisabledID = "acme.disabled"
	pluginKVSecret   = "page-secret"
)

func TestPluginGraphql(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_endpoint")
	ctx := fx.setupCtx

	require.NoError(t, fx.manager.Enable(ctx, fx.pages.ID, []string{"read_channels", "webhook"}, nil))

	code, _, raw := fx.postPlugin("", pluginPagesID, `{ __typename }`)
	require.Equal(t, http.StatusUnauthorized, code, raw)

	code, _, raw = fx.postPlugin(fx.serviceAccount, pluginPagesID, `{ __typename }`)
	require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, code, raw)

	code, _, raw = fx.postPlugin(fx.writerToken, "acme.missing", `{ __typename }`)
	require.Equal(t, http.StatusNotFound, code, raw)
	code, _, raw = fx.postPlugin(fx.writerToken, pluginDisabledID, `{ __typename }`)
	require.Equal(t, http.StatusNotFound, code, raw)
	code, _, raw = fx.postPlugin(fx.writerToken, pluginPagelessID, `{ __typename }`)
	require.Equal(t, http.StatusNotFound, code, raw)

	code, payload, raw := fx.postPlugin(fx.writerToken, pluginPagesID, `query {
		channels(first: 5) { edges { node { id name credentials { apiKey } } } }
	}`)
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	require.NotContains(t, raw, pluginKVSecret)
	node := payload["data"].(map[string]any)["channels"].(map[string]any)["edges"].([]any)[0].(map[string]any)["node"].(map[string]any)
	require.Equal(t, "seeded channel", node["name"])
	require.Nil(t, node["credentials"])

	deleteQuery := fmt.Sprintf(`mutation { deleteChannel(id: "gid://axonhub/Channel/%d") }`, fx.channelID)
	code, payload, raw = fx.postPlugin(fx.writerToken, pluginPagesID, deleteQuery)
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
	require.True(t, fx.channelExists(t))

	require.NoError(t, fx.manager.Enable(ctx, fx.pages.ID, []string{"read_channels", "write_channels"}, nil))
	code, payload, raw = fx.postPlugin(fx.readerToken, pluginPagesID, deleteQuery)
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
	require.True(t, fx.channelExists(t))

	code, payload, raw = fx.postPlugin(fx.writerToken, pluginPagesID, deleteQuery)
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	require.Equal(t, true, payload["data"].(map[string]any)["deleteChannel"])
	require.False(t, fx.channelExists(t))

	require.NoError(t, fx.manager.Enable(ctx, fx.pages.ID, []string{"read_requests"}, nil))
	code, payload, raw = fx.postPlugin(fx.writerToken, pluginPagesID, `query { requests(first: 1) { edges { node { id } } } }`)
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)

	code, payload, raw = fx.postPlugin(fx.ownerToken, pluginPagesID, `mutation { updateAutoBackupSettings(input: {enabled: false}) }`)
	require.Equal(t, http.StatusOK, code, raw)
	require.Contains(t, graphqlErrorMessage(t, payload), "owner", raw)

	before, err := fx.db.User.Get(ctx, fx.readerID)
	require.NoError(t, err)
	selfService := []string{
		`mutation { updateMe(input: {firstName: "Hacked"}) { id } }`,
		`mutation { updateMyPassword(input: {newPassword: "another-password"}) }`,
		`mutation { unlinkOIDCIdentity(id: "gid://axonhub/OIDCIdentity/1") }`,
		`mutation { createChannelOverrideTemplate(input: {name: "from-plugin"}) { id } }`,
		`mutation { updateChannelOverrideTemplate(id: "gid://axonhub/ChannelOverrideTemplate/1", input: {}) { id } }`,
		`mutation { deleteChannelOverrideTemplate(id: "gid://axonhub/ChannelOverrideTemplate/1") }`,
	}
	for _, query := range selfService {
		code, payload, raw = fx.postPlugin(fx.readerToken, pluginPagesID, query)
		require.Equal(t, http.StatusOK, code, raw)
		require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
	}
	after, err := fx.db.User.Get(ctx, fx.readerID)
	require.NoError(t, err)
	require.Equal(t, before.FirstName, after.FirstName)
	require.Equal(t, before.Password, after.Password)

	got := mutationRootNames(fx.handler.executable)
	require.Equal(t, pluginMutationFieldSnapshot, got)

	var complexity strings.Builder
	complexity.WriteString("query { ")
	for i := range pluginGraphqlComplexityLimit + 1 {
		fmt.Fprintf(&complexity, "f%d: pluginCountersArePerInstance ", i)
	}
	complexity.WriteString("}")
	code, _, raw = fx.postPlugin(fx.writerToken, pluginPagesID, complexity.String())
	require.Equal(t, http.StatusOK, code, raw)
	require.Contains(t, raw, "exceeds the limit of 1000")
	require.Contains(t, raw, `"data":null`)

	// Requests beyond the former burst limit must continue to succeed.
	for range 20 {
		code, _, raw = fx.postPlugin(fx.writerToken, pluginPagesID, `{ __typename }`)
		require.Equal(t, http.StatusOK, code, raw)
	}

	require.Equal(t, 2<<20, pluginGraphqlMaxResponseBytes)
	recorder := httptest.NewRecorder()
	writer := newCappedResponseWriter(recorder, pluginGraphqlMaxResponseBytes)
	_, err = writer.Write(bytes.Repeat([]byte("a"), pluginGraphqlMaxResponseBytes+1))
	require.ErrorIs(t, err, errPluginResponseTooLarge)
	writer.finish()
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "aaa")
	require.Less(t, recorder.Body.Len(), 1024)
}

func TestPluginKVGraphqlPagination(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_kv_pagination")
	ctx := fx.setupCtx
	require.NoError(t, fx.manager.Enable(ctx, fx.pages.ID, []string{"read_channels"}, nil))
	for _, key := range []string{"note.a", "note.b", "other"} {
		_, err := fx.manager.KVSet(ctx, pluginPagesID, fx.pages.ID, "", key, key, 0)
		require.NoError(t, err)
	}
	query := `query($after: String, $first: Int) {
  pluginKVList(prefix: "note.", first: $first, after: $after) {
   entries { key value }
   nextCursor
  }
 }`
	code, payload, raw := fx.postPluginVars(fx.ownerToken, pluginPagesID, query, map[string]any{"first": 1})
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	page := payload["data"].(map[string]any)["pluginKVList"].(map[string]any)
	require.Len(t, page["entries"], 1)
	require.Equal(t, "note.a", page["nextCursor"])
	code, payload, raw = fx.postPluginVars(fx.ownerToken, pluginPagesID, query, map[string]any{"first": 1, "after": page["nextCursor"]})
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	page = payload["data"].(map[string]any)["pluginKVList"].(map[string]any)
	require.Nil(t, page["nextCursor"])
	require.Len(t, page["entries"], 1)
	require.Equal(t, "note.b", page["entries"].([]any)[0].(map[string]any)["key"])
	// Omitting pagination arguments uses the default page size.
	_, payload, raw = fx.postPlugin(fx.ownerToken, pluginPagesID, `query { pluginKVList(prefix: "note.") { entries { key } nextCursor } }`)
	require.NotContains(t, payload, "errors", raw)
	page = payload["data"].(map[string]any)["pluginKVList"].(map[string]any)
	require.Len(t, page["entries"], 2)
	require.Nil(t, page["nextCursor"])
	_, payload, raw = fx.postPluginVars(fx.ownerToken, pluginPagesID, query, map[string]any{"first": 101})
	require.Contains(t, payload, "errors", raw)
}

func TestPluginKVGraphql(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_kv_endpoint")
	ctx := fx.setupCtx
	require.NoError(t, fx.manager.Enable(ctx, fx.pages.ID, []string{"read_channels"}, nil))
	require.NoError(t, fx.manager.Enable(ctx, fx.other.ID, []string{"read_channels"}, nil))

	setQuery := `mutation($key: String!, $value: String!) {
		setPluginKV(key: $key, value: $value) { namespace key value expiresAt }
	}`
	code, payload, raw := fx.postPluginVars(fx.ownerToken, pluginPagesID, setQuery, map[string]any{
		"key":   "shared",
		"value": "from-pages",
	})
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	entry := payload["data"].(map[string]any)["setPluginKV"].(map[string]any)
	require.Equal(t, "default", entry["namespace"])
	require.Equal(t, "shared", entry["key"])
	require.Equal(t, "from-pages", entry["value"])
	require.Nil(t, entry["expiresAt"])

	stored, err := fx.manager.KVGet(ctx, pluginPagesID, fx.pages.ID, "", "shared")
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, "from-pages", stored.Value)
	require.Equal(t, "default", stored.Namespace)

	code, payload, raw = fx.postPlugin(fx.ownerToken, pluginOtherID, `query { pluginKV(key: "shared") { value } }`)
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	require.Nil(t, payload["data"].(map[string]any)["pluginKV"])

	code, payload, raw = fx.postPluginVars(fx.ownerToken, pluginOtherID, setQuery, map[string]any{
		"key":   "shared",
		"value": "from-other",
	})
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	stored, err = fx.manager.KVGet(ctx, pluginPagesID, fx.pages.ID, "", "shared")
	require.NoError(t, err)
	require.Equal(t, "from-pages", stored.Value)
	other, err := fx.manager.KVGet(ctx, pluginOtherID, fx.other.ID, "", "shared")
	require.NoError(t, err)
	require.Equal(t, "from-other", other.Value)

	adminSet := `mutation { setPluginKV(key: "shared", value: "from-admin") { value } }`
	code, payload, raw = fx.postAdmin(fx.ownerToken, adminSet)
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
	code, payload, raw = fx.postAdmin(fx.ownerToken, `query { pluginKV(key: "shared") { value } }`)
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
	stored, err = fx.manager.KVGet(ctx, pluginPagesID, fx.pages.ID, "", "shared")
	require.NoError(t, err)
	require.Equal(t, "from-pages", stored.Value)

	code, payload, raw = fx.postPlugin(fx.readerToken, pluginPagesID, `query { pluginKV(key: "shared") { value } }`)
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	require.Equal(t, "from-pages", payload["data"].(map[string]any)["pluginKV"].(map[string]any)["value"])

	code, payload, raw = fx.postPluginVars(fx.readerToken, pluginPagesID, setQuery, map[string]any{
		"key":   "shared",
		"value": "from-reader",
	})
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
	stored, err = fx.manager.KVGet(ctx, pluginPagesID, fx.pages.ID, "", "shared")
	require.NoError(t, err)
	require.Equal(t, "from-pages", stored.Value)

	code, payload, raw = fx.postPluginVars(fx.ownerToken, pluginPagesID, setQuery, map[string]any{
		"key":   "owner-only",
		"value": "owner-write",
	})
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	stored, err = fx.manager.KVGet(ctx, pluginPagesID, fx.pages.ID, "", "owner-only")
	require.NoError(t, err)
	require.Equal(t, "owner-write", stored.Value)

	code, payload, raw = fx.postPlugin(fx.ownerToken, pluginPagesID, `mutation { setPluginKV(key: "bad key", value: "x") { value } }`)
	require.Equal(t, http.StatusOK, code, raw)
	require.Contains(t, graphqlErrorMessage(t, payload), "plugin kv", raw)
}

type pluginEndpointFixture struct {
	db             *ent.Client
	manager        *plugin.Manager
	handler        *GraphqlHandler
	engine         http.Handler
	setupCtx       context.Context
	ownerToken     string
	writerToken    string
	readerToken    string
	serviceAccount string
	readerID       int
	channelID      int
	pages          *ent.Plugin
	other          *ent.Plugin
}

func newPluginEndpointFixture(t *testing.T, name string) *pluginEndpointFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { _ = db.Close() })
	db.Driver().(*entsql.Driver).DB().SetMaxOpenConns(4)

	setupCtx := ent.NewContext(context.Background(), db)
	setupCtx = authz.WithTestBypass(setupCtx)

	manager, err := plugin.NewManager(plugin.ManagerParams{
		Client: db,
		Cache:  xcache.Config{Mode: xcache.ModeMemory},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, manager.Stop(context.Background()))
	})

	hashed, err := biz.HashPassword("test-password")
	require.NoError(t, err)
	makeUser := func(email, first string, owner bool, scopes []string) *ent.User {
		t.Helper()
		create := db.User.Create().
			SetEmail(email).
			SetPassword(hashed).
			SetFirstName(first).
			SetLastName("User").
			SetStatus(user.StatusActivated).
			SetIsOwner(owner)
		if scopes != nil {
			create.SetScopes(scopes)
		}
		return create.SaveX(setupCtx)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	owner := makeUser("owner-"+suffix+"@example.com", "Owner", true, nil)
	writer := makeUser("writer-"+suffix+"@example.com", "Writer", false, []string{"read_channels", "write_channels"})
	reader := makeUser("reader-"+suffix+"@example.com", "Reader", false, []string{"read_channels"})

	proj := db.Project.Create().
		SetName("plugin-endpoint-" + suffix).
		SetStatus(project.StatusActive).
		SaveX(setupCtx)
	serviceKey, err := biz.GenerateAPIKey("ah")
	require.NoError(t, err)
	db.APIKey.Create().
		SetName("service-" + suffix).
		SetKey(serviceKey).
		SetUserID(owner.ID).
		SetProjectID(proj.ID).
		SetType(apikey.TypeServiceAccount).
		SetStatus(apikey.StatusEnabled).
		SetScopes([]string{"read_channels"}).
		SaveX(setupCtx)

	ch := db.Channel.Create().
		SetName("seeded channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: pluginKVSecret}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SaveX(setupCtx)

	cacheCfg := xcache.Config{Mode: xcache.ModeMemory}
	projectSvc := &biz.ProjectService{
		ProjectCache: xcache.NewFromConfig[xcache.Entry[ent.Project]](cacheCfg),
	}
	apiKeySvc := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig: cacheCfg, Ent: db, ProjectService: projectSvc, KeyPrefix: "ah",
	})
	t.Cleanup(apiKeySvc.Stop)
	systemSvc := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cacheCfg, Ent: db})
	require.NoError(t, systemSvc.SetSecretKey(setupCtx, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
	userSvc := biz.NewUserService(biz.UserServiceParams{CacheConfig: cacheCfg, Ent: db, APIKeyService: apiKeySvc})
	authSvc := biz.NewAuthService(biz.AuthServiceParams{
		SystemService: systemSvc, APIKeyService: apiKeySvc, UserService: userSvc, Ent: db,
	})
	channelSvc := biz.NewChannelServiceForTest(db)
	t.Cleanup(channelSvc.Stop)

	handler := NewGraphqlHandlers(Dependencies{
		Ent:                            db,
		AuthService:                    authSvc,
		UserService:                    userSvc,
		SystemService:                  systemSvc,
		ChannelService:                 channelSvc,
		ChannelOverrideTemplateService: biz.NewChannelOverrideTemplateService(biz.ChannelOverrideTemplateServiceParams{Client: db, ChannelService: channelSvc}),
		PluginManager:                  manager,
		QuotaService:                   biz.NewQuotaService(db, systemSvc),
	})

	engine := gin.New()
	engine.Use(middleware.WithEntClient(db))
	pluginGroup := engine.Group("/admin/plugins", middleware.WithJWTAuth(authSvc), middleware.WithProjectID())
	pluginGroup.POST("/:pluginId/graphql", middleware.WithTimeout(PluginGraphqlTimeout), func(c *gin.Context) {
		handler.PluginGraphql.ServeHTTP(c.Writer, c.Request)
	})
	pluginGroup.POST("/:pluginId/fetch", middleware.WithTimeout(PluginFetchTimeout), func(c *gin.Context) {
		handler.PluginFetch.ServeHTTP(c.Writer, c.Request)
	})
	adminGroup := engine.Group("/admin", middleware.WithAdminGraphqlAuth(authSvc), middleware.WithProjectID())
	adminGroup.POST("/graphql", func(c *gin.Context) {
		handler.Graphql.ServeHTTP(c.Writer, c.Request)
	})

	token := func(row *ent.User) string {
		t.Helper()
		signed, err := authSvc.GenerateJWTToken(setupCtx, row)
		require.NoError(t, err)
		return signed
	}
	install := func(id string, permissions []string, withPage bool) *ent.Plugin {
		t.Helper()
		result, err := manager.Install(setupCtx, pluginEndpointZip(t, id, permissions, withPage), nil, nil)
		require.NoError(t, err)
		return result.Plugin
	}

	pages := install(pluginPagesID, []string{"read_channels", "write_channels", "read_requests", "webhook"}, true)
	other := install(pluginOtherID, []string{"read_channels"}, true)
	pageless := install(pluginPagelessID, []string{"read_channels"}, false)
	_ = install(pluginDisabledID, []string{"read_channels"}, true)
	require.NoError(t, manager.Enable(setupCtx, pageless.ID, []string{"read_channels"}, nil))

	return &pluginEndpointFixture{
		db:             db,
		manager:        manager,
		handler:        handler,
		engine:         engine,
		setupCtx:       setupCtx,
		ownerToken:     token(owner),
		writerToken:    token(writer),
		readerToken:    token(reader),
		serviceAccount: serviceKey,
		readerID:       reader.ID,
		channelID:      ch.ID,
		pages:          pages,
		other:          other,
	}
}

func (fx *pluginEndpointFixture) postPlugin(token, pluginID, query string) (int, map[string]any, string) {
	return fx.postPluginVars(token, pluginID, query, nil)
}

func (fx *pluginEndpointFixture) postPluginVars(token, pluginID, query string, variables map[string]any) (int, map[string]any, string) {
	return fx.post("/admin/plugins/"+pluginID+"/graphql", token, query, variables)
}

func (fx *pluginEndpointFixture) postAdmin(token, query string) (int, map[string]any, string) {
	return fx.post("/admin/graphql", token, query, nil)
}

func (fx *pluginEndpointFixture) post(path, token, query string, variables map[string]any) (int, map[string]any, string) {
	body := map[string]any{"query": query}
	if variables != nil {
		body["variables"] = variables
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err.Error()
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	fx.engine.ServeHTTP(recorder, req)
	raw := recorder.Body.String()
	var payload map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &payload)
	return recorder.Code, payload, raw
}

func (fx *pluginEndpointFixture) channelExists(t *testing.T) bool {
	t.Helper()
	ok, err := fx.db.Channel.Query().Where(channel.ID(fx.channelID)).Exist(fx.setupCtx)
	require.NoError(t, err)
	return ok
}

func graphqlErrorCode(t *testing.T, payload map[string]any) string {
	t.Helper()
	first := graphqlError(t, payload)
	extensions, _ := first["extensions"].(map[string]any)
	code, _ := extensions["code"].(string)
	return code
}

func graphqlErrorMessage(t *testing.T, payload map[string]any) string {
	t.Helper()
	message, _ := graphqlError(t, payload)["message"].(string)
	return message
}

func graphqlError(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	items, _ := payload["errors"].([]any)
	require.NotEmpty(t, items)
	first, _ := items[0].(map[string]any)
	require.NotNil(t, first)
	return first
}

func mutationRootNames(schema interface{ Schema() *ast.Schema }) []string {
	fields := schema.Schema().Mutation.Fields
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	slices.Sort(names)
	return names
}

func pluginEndpointZip(t *testing.T, id string, permissions []string, withPage bool) []byte {
	t.Helper()
	encoded, err := json.Marshal(permissions)
	require.NoError(t, err)
	ui := ""
	if withPage {
		ui = `,
  "ui": {"pages": [{"key": "main", "entry": "ui/page.html", "title": "Overview"}]}`
	}
	manifest := fmt.Sprintf(`{
  "schemaVersion": 1,
  "id": %q,
  "name": "Page Plugin",
  "version": "1.0.0",
  "description": "UI only",
  "axonhubVersion": "^1.0.0",
  "pluginApiVersion": 1,
  "permissions": %s%s
}`, id, encoded, ui)

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	write := func(name, content string) {
		t.Helper()
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, content)
		require.NoError(t, err)
	}
	write("manifest.json", manifest)
	if withPage {
		write("ui/page.html", "<html>ui</html>")
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

// pluginMutationFieldSnapshot is every Mutation root field. A new mutation
// fails TestPluginGraphql until this list is updated and the field is either
// covered by scope, direct-user, or resource ownership checks.
var pluginMutationFieldSnapshot = []string{
	"addUserToProject",
	"applyChannelOverrideTemplate",
	"archiveThread",
	"archiveTrace",
	"backup",
	"bulkAddChannelTags",
	"bulkArchiveAPIKeys",
	"bulkArchiveChannels",
	"bulkArchiveModels",
	"bulkCreateChannels",
	"bulkCreateModels",
	"bulkDeleteChannels",
	"bulkDeleteModels",
	"bulkDeletePromptProtectionRules",
	"bulkDeletePrompts",
	"bulkDeleteRoles",
	"bulkDisableAPIKeys",
	"bulkDisableChannels",
	"bulkDisableModels",
	"bulkDisablePromptProtectionRules",
	"bulkDisablePrompts",
	"bulkEnableAPIKeys",
	"bulkEnableChannels",
	"bulkEnableModels",
	"bulkEnablePromptProtectionRules",
	"bulkEnablePrompts",
	"bulkImportChannels",
	"bulkManageChannelTags",
	"bulkRecoverChannels",
	"bulkRemoveChannelTags",
	"bulkUpdateChannelAutoDisable",
	"bulkUpdateChannelOrdering",
	"checkProviderQuotas",
	"clearCache",
	"clearChannelOverrideTemplates",
	"completeAutoDisableChannelOnboarding",
	"completeOnboarding",
	"completeSystemModelSettingOnboarding",
	"createAPIKey",
	"createApiKeyProfileTemplate",
	"createChannel",
	"createChannelOverrideTemplate",
	"createDataStorage",
	"createModel",
	"createPluginUISession",
	"createProject",
	"createPrompt",
	"createPromptProtectionRule",
	"createRole",
	"createUser",
	"deleteApiKeyProfileTemplate",
	"deleteChannel",
	"deleteChannelOverrideTemplate",
	"deleteDisabledChannelAPIKeys",
	"deleteModel",
	"deletePlugin",
	"deletePluginKV",
	"deleteProject",
	"deletePrompt",
	"deletePromptProtectionRule",
	"deleteProxyPreset",
	"deleteRole",
	"deleteUser",
	"detectChannelEndpoints",
	"disableChannelAPIKey",
	"disablePlugin",
	"duplicateChannel",
	"enableAllChannelAPIKeys",
	"enableChannelAPIKey",
	"enablePlugin",
	"enableSelectedChannelAPIKeys",
	"installPlugin",
	"installPluginFromMarket",
	"loadApiKeyProfileTemplate",
	"previewPromptProtectionRule",
	"removeUserFromProject",
	"resetChannelQuotaNow",
	"resetClaudeCodeSafeguardsSettings",
	"restore",
	"retainThread",
	"retainTrace",
	"rotateAPIKey",
	"saveChannelEndpoints",
	"saveChannelModelPrices",
	"saveProxyPreset",
	"setPluginKV",
	"syncChannelModels",
	"testChannel",
	"testChannelAPIKey",
	"testChannelAPIKeys",
	"testCodexCatalog",
	"triggerAutoBackup",
	"triggerGcCleanup",
	"unarchiveThread",
	"unarchiveTrace",
	"unlinkOIDCIdentity",
	"unretainThread",
	"unretainTrace",
	"updateAPIKey",
	"updateAPIKeyProfiles",
	"updateAPIKeyStatus",
	"updateApiKeyProfileTemplate",
	"updateAutoBackupSettings",
	"updateBrandSettings",
	"updateCatalogSettings",
	"updateChannel",
	"updateChannelOverrideTemplate",
	"updateChannelSettings",
	"updateChannelStatus",
	"updateClaudeCodeSafeguardsSettings",
	"updateCodexCompatibilitySettings",
	"updateDataStorage",
	"updateDefaultDataStorage",
	"updateMe",
	"updateModel",
	"updateModelStatus",
	"updateMyPassword",
	"updatePassThroughSettings",
	"updatePluginSettings",
	"updateProject",
	"updateProjectProfiles",
	"updateProjectStatus",
	"updateProjectUser",
	"updatePrompt",
	"updatePromptProtectionRule",
	"updatePromptProtectionRuleStatus",
	"updatePromptStatus",
	"updateProviderQuotaCollectionSettings",
	"updateQuotaRoutingSettings",
	"updateRetryPolicy",
	"updateRole",
	"updateSecuritySettings",
	"updateStoragePolicy",
	"updateSystemChannelSettings",
	"updateSystemGeneralSettings",
	"updateSystemModelSettings",
	"updateUsageCostInjectionSettings",
	"updateUser",
	"updateUserAgentPassThroughSettings",
	"updateUserStatus",
	"updateVideoStorageSettings",
	"updateWebhookNotifierConfig",
}
