package gql

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/plugin"
)

func (r *Resolver) pluginManagerOrErr() (*plugin.Manager, error) {
	if r == nil || r.pluginManager == nil {
		return nil, errors.New("plugin manager is not configured")
	}
	return r.pluginManager, nil
}

func requirePluginRead(ctx context.Context) error {
	if user, ok := contexts.GetUser(ctx); ok && user != nil && scopes.IsDirectOwner(ctx) {
		return nil
	}
	if scopes.UserHasScope(ctx, scopes.ScopeReadPlugins) {
		return nil
	}
	return xerrors.ForbiddenError("permission denied: requires read_plugins scope")
}

func requirePluginWrite(ctx context.Context) error {
	if user, ok := contexts.GetUser(ctx); ok && user != nil && scopes.IsDirectOwner(ctx) {
		return nil
	}
	if scopes.UserHasScope(ctx, scopes.ScopeWritePlugins) {
		return nil
	}
	return xerrors.ForbiddenError("permission denied: requires write_plugins scope")
}

func requirePluginOwner(ctx context.Context) error {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil || !scopes.IsDirectOwner(ctx) {
		return ErrNotOwner
	}
	return nil
}

func pluginRowID(id objects.GUID) (int, error) {
	if id.Type != "Plugin" || id.ID <= 0 {
		return 0, fmt.Errorf("plugin %d not found", id.ID)
	}
	return id.ID, nil
}

func readPluginUpload(file graphql.Upload) ([]byte, error) {
	if file.File == nil {
		return nil, errors.New("plugin package is empty")
	}
	defer func() {
		if closer, ok := file.File.(io.Closer); ok {
			_ = closer.Close()
		}
	}()
	data, err := io.ReadAll(file.File)
	if err != nil {
		return nil, fmt.Errorf("read plugin package: %w", err)
	}
	return data, nil
}
