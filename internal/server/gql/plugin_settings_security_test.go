package gql

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestPluginSystemEntityProtectsSigningKey(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_system_secret")
	installed, err := fx.manager.Install(fx.setupCtx, pluginEndpointZip(t, "acme.settings", []string{"read_settings"}, true), nil)
	require.NoError(t, err)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, []string{"read_settings"}))
	secret := fx.db.System.Query().Where(system.KeyEQ(biz.SystemKeySecretKey)).OnlyX(fx.setupCtx)
	fx.db.System.Create().SetKey(biz.SystemKeyBrandName).SetValue("visible-plugin-brand").SaveX(fx.setupCtx)
	code, payload, raw := fx.postPlugin(fx.ownerToken, "acme.settings", `{ systems(first: 100) { edges { node { key value } } } }`)
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	require.Contains(t, raw, "visible-plugin-brand")
	require.NotContains(t, raw, secret.Value)
	require.NotContains(t, raw, biz.SystemKeySecretKey)
	id := fmt.Sprintf("gid://axonhub/System/%d", secret.ID)
	for _, query := range []string{
		fmt.Sprintf(`{ node(id: %q) { ... on System { key value } } }`, id),
		fmt.Sprintf(`{ nodes(ids: [%q]) { ... on System { key value } } }`, id),
		fmt.Sprintf(`query { alias: node(id: %q) { ... Secret } } fragment Secret on System { key value }`, id),
	} {
		code, payload, raw = fx.postPlugin(fx.ownerToken, "acme.settings", query)
		require.Equal(t, http.StatusOK, code, raw)
		require.Contains(t, payload, "errors", raw)
		require.NotContains(t, raw, secret.Value)
	}
	// The ordinary direct owner path keeps its existing visibility.
	_, payload, raw = fx.postAdmin(fx.ownerToken, fmt.Sprintf(`{ node(id: %q) { ... on System { key value } } }`, id))
	require.NotContains(t, payload, "errors", raw)
	require.Contains(t, raw, secret.Value)
}

// Settings reads authorize at the resolver: a warm cache must not change the decision.
func TestSettingsReadAuthorizationIgnoresCacheState(t *testing.T) {
	fx := newPluginEndpointFixture(t, "settings_read_authz")
	fx.db.System.Create().SetKey(biz.SystemKeySecuritySettings).SetValue(`{"blocked_ips":["203.0.113.77"]}`).SaveX(fx.setupCtx)
	installed, err := fx.manager.Install(fx.setupCtx, pluginEndpointZip(t, "acme.settings", []string{"read_settings", "write_channels"}, true), nil)
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

	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, []string{"write_channels"}))
	denied(fx.postPlugin(fx.ownerToken, "acme.settings", security))
	allowed(fx.postPlugin(fx.ownerToken, "acme.settings", retry))

	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, []string{"read_settings"}))
	code, payload, raw = fx.postPlugin(fx.ownerToken, "acme.settings", security)
	allowed(code, payload, raw)
	require.Contains(t, raw, "203.0.113.77")
}
