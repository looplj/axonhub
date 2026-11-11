package gql

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

func TestPluginBypassErrorPresentation(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	u := &ent.User{ID: 1, IsOwner: true}
	parent := contexts.WithUser(authz.NewUserContext(t.Context(), u.ID), u)
	ctx, err := authz.DelegatePlugin(parent, 1, nil)
	require.NoError(t, err)
	_, err = db.User.Query().All(authz.WithSystemBypass(ctx, "test"))
	require.ErrorIs(t, err, privacy.Deny)
	require.ErrorIs(t, err, authz.ErrPluginSystemBypass)
	called := false
	closureErr := authz.RunWithSystemBypassVoid(ctx, "test", func(context.Context) error { called = true; return nil })
	require.ErrorIs(t, closureErr, authz.ErrPluginSystemBypass)
	require.False(t, called)
	require.Equal(t, xerrors.ErrCodeForbidden, presentGraphqlError(ctx, closureErr).Extensions["code"])
	presented := presentGraphqlError(ctx, fmt.Errorf("outer: %w", err))
	require.Equal(t, xerrors.ErrCodeForbidden, presented.Extensions["code"])
	require.Contains(t, presented.Message, "plugin")
}

func TestPluginPersonalOperationsUseBusinessAuthorization(t *testing.T) {
	fx := newPluginEndpointFixture(t, "personal_business_auth")
	require.NoError(t, fx.manager.Enable(fx.setupCtx, fx.pages.ID, []string{"read_channels", "write_channels"}, nil))
	queries := []string{
		`mutation { alias: updateMe(input: { firstName: "denied" }) { id } }`,
		`mutation { ...Fields } fragment Fields on Mutation { updateMyPassword(input: { newPassword: "denied" }) }`,
		`mutation { ... on Mutation { unlinkOIDCIdentity(id: "gid://axonhub/OIDCIdentity/1") } }`,
	}
	for _, query := range queries {
		_, payload, raw := fx.postPlugin(fx.ownerToken, pluginPagesID, query)
		require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
	}
	// The direct-user endpoint retains self-service access.
	_, payload, raw := fx.postAdmin(fx.readerToken, `mutation { updateMe(input: { firstName: "allowed" }) { firstName } }`)
	require.NotContains(t, payload, "errors", raw)
	require.Contains(t, raw, "allowed")
}

func TestPluginChannelTestsReturnForbidden(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_channel_tests_denied")
	require.NoError(t, fx.manager.Enable(fx.setupCtx, fx.pages.ID, []string{"read_channels", "write_channels"}, nil))
	id := fmt.Sprintf("gid://axonhub/Channel/%d", fx.channelID)
	for _, query := range []string{
		fmt.Sprintf(`mutation { alias: testChannel(input: {channelID: %q}) { success } }`, id),
		fmt.Sprintf(`mutation { testChannelAPIKeys(channelID: %q) { total } }`, id),
		fmt.Sprintf(`mutation { ...Test } fragment Test on Mutation { testChannelAPIKey(channelID: %q, key: "key") { success } }`, id),
	} {
		_, payload, raw := fx.postPlugin(fx.ownerToken, pluginPagesID, query)
		require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
		require.Equal(t, "channel testing is not available to plugins", graphqlErrorMessage(t, payload), raw)
	}
}
