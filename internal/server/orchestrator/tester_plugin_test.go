package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestChannelTestingRejectsPluginsBeforeDependencies(t *testing.T) {
	owner := &ent.User{ID: 1, IsOwner: true}
	parent := contexts.WithUser(authz.NewUserContext(t.Context(), owner.ID), owner)
	ctx, err := authz.DelegatePlugin(parent, 1, scopes.AllScopesAsStrings())
	require.NoError(t, err)
	// Missing dependencies make accidental work before rejection fail immediately.
	tester := &TestChannelOrchestrator{}
	channelID := objects.GUID{Type: ent.TypeChannel, ID: 1}
	for _, callCtx := range []context.Context{ctx, authz.WithScopeDecision(ctx, scopes.ScopeWriteChannels)} {
		for name, call := range map[string]func(*testing.T) error{
			"channel": func(t *testing.T) error {
				result, err := tester.TestChannel(callCtx, channelID, nil, nil)
				require.Nil(t, result)
				return err
			},
			"all-keys": func(t *testing.T) error {
				result, err := tester.TestChannelAPIKeys(callCtx, channelID, nil, nil)
				require.Nil(t, result)
				return err
			},
			"single-key": func(t *testing.T) error {
				result, err := tester.TestSingleAPIKey(callCtx, channelID, "key", nil, nil)
				require.Nil(t, result)
				return err
			},
		} {
			t.Run(name, func(t *testing.T) {
				coded, ok := xerrors.IsCodedError(call(t))
				require.True(t, ok)
				require.Equal(t, xerrors.ErrCodeForbidden, coded.Code)
				require.Equal(t, "channel testing is not available to plugins", coded.Message)
			})
		}
	}
}
