package identity

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestPluginDelegationIdentityAndSnapshot(t *testing.T) {
	parent, err := WithPrincipal(context.Background(), Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(7)})
	require.NoError(t, err)
	grants := []string{"read_channels"}
	ctx, err := Delegate(parent, 7, 12, grants, "view")
	require.NoError(t, err)
	grants[0] = "write_channels"
	require.True(t, PluginGranted(ctx, 7, "read_channels"))
	require.False(t, PluginGranted(ctx, 7, "write_channels"))
	require.False(t, ValidPlugin(ctx, 8))
	p, _ := GetPrincipal(ctx)
	same, err := WithPrincipal(ctx, p)
	require.NoError(t, err)
	require.Equal(t, ctx, same)
	p.PluginInstallationID = 13
	_, err = WithPrincipal(ctx, p)
	require.Error(t, err)
	p, _ = GetPrincipal(ctx)
	*p.UserID = 8
	require.True(t, ValidPlugin(ctx, 7))
	for _, typ := range []PrincipalType{PrincipalTypeUser, PrincipalTypeAPIKey, PrincipalTypeSystem, PrincipalTypeTest} {
		_, err := Replace(ctx, Principal{Type: typ})
		require.Error(t, err)
	}
	_, err = Delegate(ctx, 7, 12, nil, "view")
	require.Error(t, err)
	_, err = Delegate(parent, 7, 0, nil, "view")
	require.Error(t, err)
	_, err = Delegate(parent, 7, 12, nil, nil)
	require.Error(t, err)
	view, ok := DelegatedUserView(ctx)
	require.True(t, ok)
	require.Equal(t, "view", view)
	_, ok = DelegatedUserView(parent)
	require.False(t, ok)
	require.False(t, IsPlugin(parent))
}
