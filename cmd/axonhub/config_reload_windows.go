//go:build windows

package main

import (
	"context"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/conf"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/middleware"
)

// registerConfigReload is a no-op on Windows because Windows does not support
// the Unix SIGHUP contract used for explicit configuration reloads.
func registerConfigReload(lc fx.Lifecycle, _ *conf.Loader, _ *middleware.IPAccessControlConfig) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Warn(ctx, "SIGHUP config reload is unavailable on Windows")
			return nil
		},
	})
}
