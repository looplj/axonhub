package gql

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// These requests use the real HTTP/JWT/Ent chain, without a GraphQL private-type denylist.
func TestPluginPrincipalPrivateResources(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_principal_private")
	template := fx.db.ChannelOverrideTemplate.Create().SetUserID(fx.readerID).SetName("private-template-secret").SaveX(fx.setupCtx)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, fx.pages.ID, []string{"read_channels", "write_channels"}))
	id := fmt.Sprintf("gid://axonhub/ChannelOverrideTemplate/%d", template.ID)
	queries := []string{
		`{ channelOverrideTemplates(first: 5) { edges { node { id name } } } }`,
		fmt.Sprintf(`{ node(id: %q) { ... on ChannelOverrideTemplate { name } } }`, id),
		fmt.Sprintf(`{ nodes(ids: [%q]) { ... on ChannelOverrideTemplate { name } } }`, id),
		fmt.Sprintf(`query { aliased: node(id: %q) { ... PrivateTemplate } } fragment PrivateTemplate on ChannelOverrideTemplate { name }`, id),
		fmt.Sprintf(`{ node(id: "gid://axonhub/User/%d") { ... on User { channelOverrideTemplates(first: 5) { edges { node { name } } } } } }`, fx.readerID),
	}
	for _, token := range []string{fx.ownerToken, fx.readerToken} {
		for _, query := range queries {
			code, payload, raw := fx.postPlugin(token, pluginPagesID, query)
			require.Equal(t, http.StatusOK, code, raw)
			require.Contains(t, payload, "errors", raw)
			require.NotContains(t, raw, "private-template-secret")
			require.NotContains(t, raw, "Cannot query field", "test query must exercise authorization: %s", raw)
		}
	}
}

func TestPluginPrincipalEmptyGrantAndProfile(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_principal_empty")
	require.NoError(t, fx.manager.Enable(fx.setupCtx, fx.pages.ID, []string{}))
	_, payload, raw := fx.postPlugin(fx.ownerToken, pluginPagesID, `{ channels(first: 1) { edges { node { name } } } }`)
	require.Contains(t, payload, "errors", raw)
	_, payload, raw = fx.postPlugin(fx.ownerToken, pluginPagesID, `{ me { firstName isOwner scopes projects { isOwner } oidcIdentities { email } } }`)
	require.NotContains(t, payload, "errors", raw)
	me := payload["data"].(map[string]any)["me"].(map[string]any)
	require.Equal(t, false, me["isOwner"])
	require.Empty(t, me["scopes"])
	require.Empty(t, me["projects"])
	require.Empty(t, me["oidcIdentities"])
}
