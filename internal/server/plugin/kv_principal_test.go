package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
)

func TestPluginPrincipalKV(t *testing.T) {
	db := newPluginDB(t)
	manager := newTestManager(t, db)
	installed := installUI(t, manager, uiZip(t, nil, nil))
	for _, u := range []*ent.User{{ID: 1, IsOwner: true}, {ID: 2, Scopes: []string{"write_plugins"}}, {ID: 3}} {
		parent := contexts.WithUser(authz.NewUserContext(context.Background(), u.ID), u)
		delegated, err := authz.DelegatePlugin(parent, installed.ID, nil)
		require.NoError(t, err)
		writable := u.ID != 3
		ctx := WithCall(delegated, Call{ManifestID: installed.PluginID, InstallationID: installed.ID, KVWritable: writable})
		_, err = manager.KVSet(ctx, installed.PluginID, installed.ID, "", "key", "value", 0)
		if writable {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
		_, err = manager.KVGet(ctx, installed.PluginID, installed.ID, "", "key")
		require.NoError(t, err)
		_, err = manager.KVGet(ctx, installed.PluginID, installed.ID+1, "", "key")
		require.Error(t, err)
		_, err = manager.KVSet(ctx, installed.PluginID, installed.ID+1, "", "key", "value", 0)
		require.Error(t, err)
		// A plugin context never gains System authority; generic bypass paths fail closed.
		require.Error(t, manager.KVDeleteInstallation(ctx, installed.PluginID, installed.ID))
		require.Error(t, manager.Enable(ctx, installed.ID, nil))
	}
}
