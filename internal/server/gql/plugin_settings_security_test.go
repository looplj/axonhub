package gql

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestAdminGraphQLProtectsCredentialsAndRoleAssignments(t *testing.T) {
	fx := newPluginEndpointFixture(t, "admin_credential_and_role_read")
	projectKey := fx.db.APIKey.Query().Where(apikey.KeyEQ(fx.serviceAccount)).OnlyX(fx.setupCtx)
	readerKey := fx.db.APIKey.Create().SetName("Metadata reader").SetKey("test-reader-credential").
		SetUserID(fx.readerID).SetProjectID(projectKey.ProjectID).SetType(apikey.TypeServiceAccount).
		SetScopes([]string{"read_api_keys"}).SaveX(fx.setupCtx)
	keyID := fmt.Sprintf("gid://axonhub/APIKey/%d", projectKey.ID)
	query := fmt.Sprintf(`{ node(id: %q) { ... on APIKey { name key } } }`, keyID)

	_, payload, raw := fx.postAdmin(readerKey.Key, query)
	require.NotContains(t, payload, "errors", raw)
	require.Contains(t, raw, projectKey.Name)
	require.NotContains(t, raw, projectKey.Key)
	_, payload, raw = fx.postAdmin(fx.ownerToken, query)
	require.NotContains(t, payload, "errors", raw)
	require.Contains(t, raw, projectKey.Key)

	systemRole := fx.db.Role.Create().SetName("System role").SetLevel(role.LevelSystem).SetProjectID(0).SaveX(fx.setupCtx)
	assignment := fx.db.UserRole.Create().SetUserID(projectKey.UserID).SetRoleID(systemRole.ID).SaveX(fx.setupCtx)
	roleID := fmt.Sprintf("gid://axonhub/UserRole/%d", assignment.ID)
	query = fmt.Sprintf(`{ node(id: %q) { ... on UserRole { userID roleID } } }`, roleID)
	_, payload, raw = fx.postAdmin(fx.readerToken, query)
	require.Contains(t, payload, "errors", raw)
	_, payload, raw = fx.postAdmin(fx.ownerToken, query)
	require.NotContains(t, payload, "errors", raw)
	require.Contains(t, raw, "roleID")
}

func TestSystemSettingsAreNotExposedAsGraphQLEntities(t *testing.T) {
	fx := newPluginEndpointFixture(t, "internal_system_settings")
	installed, err := fx.manager.Install(fx.setupCtx, pluginEndpointZip(t, "acme.settings", []string{"read_settings"}, true), nil, nil)
	require.NoError(t, err)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, []string{"read_settings"}, nil))
	secret := fx.db.System.Query().Where(system.KeyEQ(biz.SystemKeySecretKey)).OnlyX(fx.setupCtx)
	brand := fx.db.System.Create().SetKey(biz.SystemKeyBrandName).SetValue("visible-brand").SaveX(fx.setupCtx)
	fx.db.User.UpdateOneID(fx.readerID).SetScopes([]string{"read_settings"}).SaveX(fx.setupCtx)

	for _, endpoint := range []struct {
		name string
		post func(string) (int, map[string]any, string)
	}{
		{"admin owner", func(query string) (int, map[string]any, string) { return fx.postAdmin(fx.ownerToken, query) }},
		{"admin settings reader", func(query string) (int, map[string]any, string) { return fx.postAdmin(fx.readerToken, query) }},
		{"plugin", func(query string) (int, map[string]any, string) {
			return fx.postPlugin(fx.ownerToken, "acme.settings", query)
		}},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			_, payload, raw := endpoint.post(`{ systems { totalCount } }`)
			require.Contains(t, graphqlError(t, payload)["message"], `Cannot query field "systems"`, raw)

			id := fmt.Sprintf("gid://axonhub/System/%d", secret.ID)
			_, payload, raw = endpoint.post(fmt.Sprintf(`{ node(id: %q) { ... on System { value } } }`, id))
			require.Contains(t, graphqlError(t, payload)["message"], `Unknown type "System"`)
			require.NotContains(t, raw, secret.Value)

			id = fmt.Sprintf("gid://axonhub/System/%d", brand.ID)
			_, payload, _ = endpoint.post(fmt.Sprintf(`{ node(id: %q) { id } }`, id))
			require.Contains(t, graphqlError(t, payload)["message"], "unknown node type: System")

			code, payload, raw := endpoint.post(`{ brandSettings { brandName } }`)
			require.Equal(t, http.StatusOK, code, raw)
			require.NotContains(t, payload, "errors", raw)
			require.Contains(t, raw, brand.Value)
		})
	}
}

// Settings reads authorize at the resolver: a warm cache must not change the decision.
func TestSettingsReadAuthorizationIgnoresCacheState(t *testing.T) {
	fx := newPluginEndpointFixture(t, "settings_read_authz")
	fx.db.System.Create().SetKey(biz.SystemKeySecuritySettings).SetValue(`{"blocked_ips":["203.0.113.77"]}`).SaveX(fx.setupCtx)
	installed, err := fx.manager.Install(fx.setupCtx, pluginEndpointZip(t, "acme.settings", []string{"read_settings", "write_channels"}, true), nil, nil)
	require.NoError(t, err)

	const security = `{ securitySettings { blockedIPs } }`
	const retry = `{ retryPolicy { maxChannelRetries } }`
	allowed := func(code int, payload map[string]any, raw string) {
		t.Helper()
		require.Equal(t, http.StatusOK, code, raw)
		require.NotContains(t, payload, "errors", raw)
	}
	denied := func(code int, payload map[string]any, raw string) {
		t.Helper()
		require.Equal(t, http.StatusOK, code, raw)
		require.Contains(t, payload, "errors", raw)
		require.NotContains(t, raw, "203.0.113.77")
	}

	// Warm the cache as the owner first.
	code, payload, raw := fx.postAdmin(fx.ownerToken, security)
	allowed(code, payload, raw)
	require.Contains(t, raw, "203.0.113.77")

	denied(fx.postAdmin(fx.readerToken, security))
	denied(fx.postAdmin(fx.writerToken, security))
	denied(fx.postAdmin(fx.readerToken, retry))
	allowed(fx.postAdmin(fx.writerToken, retry))
	allowed(fx.postAdmin(fx.readerToken, `{ brandSettings { brandName } systemStatus { isInitialized } }`))

	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, []string{"write_channels"}, nil))
	denied(fx.postPlugin(fx.ownerToken, "acme.settings", security))
	allowed(fx.postPlugin(fx.ownerToken, "acme.settings", retry))

	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, []string{"read_settings"}, nil))
	code, payload, raw = fx.postPlugin(fx.ownerToken, "acme.settings", security)
	allowed(code, payload, raw)
	require.Contains(t, raw, "203.0.113.77")
}
