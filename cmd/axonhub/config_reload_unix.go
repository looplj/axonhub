//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/conf"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/middleware"
)

// registerConfigReload wires the explicitly supported runtime reloads to
// SIGHUP. Editing the file alone never changes a running process.
func registerConfigReload(lc fx.Lifecycle, loader *conf.Loader, ipAccessControl *middleware.IPAccessControlConfig) {
	var cancel context.CancelFunc
	done := make(chan struct{})
	signals := make(chan os.Signal, 1)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if loader.ConfigFile() == "" {
				log.Warn(ctx, "SIGHUP config reload disabled because no config file was loaded at startup")
				return nil
			}

			// A one-element buffer coalesces bursts of HUP signals while a
			// reload is in progress. The reload loop is deliberately serial.
			runCtx, runCancel := context.WithCancel(context.Background()) //nolint:gosec // cancel is stored in the outer closure and called via OnStop
			cancel = runCancel
			signal.Notify(signals, syscall.SIGHUP)
			go runConfigReloadLoop(runCtx, signals, loader, ipAccessControl, done)

			log.Info(ctx, "SIGHUP config reload enabled", log.String("config_file", loader.ConfigFile()))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if cancel == nil {
				return nil
			}

			// Stop accepting process signals before canceling the loop so
			// shutdown cannot begin another reload.
			signal.Stop(signals)
			cancel()

			select {
			case <-done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}

func runConfigReloadLoop(
	ctx context.Context,
	signals <-chan os.Signal,
	loader *conf.Loader,
	ipAccessControl *middleware.IPAccessControlConfig,
	done chan<- struct{},
) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error(context.Background(), "config reload signal worker panicked", log.Any("panic", recovered))
		}
		close(done)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-signals:
			if err := reloadIPAccessControl(ctx, loader, ipAccessControl); err != nil {
				// Reload failures must not affect the current runtime snapshot.
				log.Warn(ctx, "SIGHUP config reload rejected", log.Cause(err))
			}
		}
	}
}

func reloadIPAccessControl(ctx context.Context, loader *conf.Loader, ipAccessControl *middleware.IPAccessControlConfig) error {
	cfg, err := loader.Reload()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := ipAccessControl.Apply(
		cfg.APIServer.IPAccessControl.Enabled,
		cfg.APIServer.IPAccessControl.AllowedIPs,
		cfg.APIServer.IPAccessControl.RedirectURL,
	); err != nil {
		return fmt.Errorf("apply IP access control configuration: %w", err)
	}

	log.Info(ctx, "SIGHUP config reload applied", log.String("section", "server.ip_access_control"))
	return nil
}
