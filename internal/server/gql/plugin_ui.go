package gql

import (
	"context"
	"errors"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/plugin"
)

func (r *Resolver) pluginUIEntries(ctx context.Context) (*plugin.UIEntries, error) {
	if _, err := loggedInPluginUser(ctx); err != nil {
		return nil, err
	}
	manager, err := r.pluginManagerOrErr()
	if err != nil {
		return nil, err
	}
	return manager.UIEntries(), nil
}

func (r *Resolver) createPluginUISession(ctx context.Context, pluginID, entry string) (*plugin.UISession, error) {
	user, err := loggedInPluginUser(ctx)
	if err != nil {
		return nil, err
	}
	// Owner or write_plugins, from the caller before the plugin endpoint's downgrade.
	canWriteKV := callerCanWritePluginKV(ctx)
	manager, err := r.pluginManagerOrErr()
	if err != nil {
		return nil, err
	}
	if r.systemService == nil {
		return nil, errors.New("plugin ui secret is unavailable")
	}
	secret, err := authz.RunWithSystemBypass(ctx, "plugin-ui-secret", func(ctx context.Context) (string, error) {
		return r.systemService.SecretKey(ctx)
	})
	if err != nil || secret == "" {
		return nil, errors.New("plugin ui secret is unavailable")
	}
	token, err := manager.IssueUIToken(secret, pluginID, entry, user.ID, time.Now())
	if err != nil {
		return nil, err
	}
	return &plugin.UISession{
		Token:      token,
		Config:     manager.MaskedUIConfig(pluginID),
		CanWriteKV: canWriteKV,
	}, nil
}

func loggedInPluginUser(ctx context.Context) (*ent.User, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil || user.ID <= 0 {
		return nil, xerrors.ForbiddenError("authentication required")
	}
	return user, nil
}

// callerCanWritePluginKV reports whether this caller may write plugin KV.
// On the plugin GraphQL endpoint the flag was stored before BuildPluginScopedUser.
func callerCanWritePluginKV(ctx context.Context) bool {
	if call, ok := plugin.CallFromContext(ctx); ok {
		return call.KVWritable
	}
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return false
	}
	return scopes.HasSystemScope(user, scopes.ScopeWritePlugins)
}
