package main

import (
	"context"
	"os"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/conf"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server"
)

func main() {
	server.Run(
		fx.Provide(conf.Load),
		fx.Supply(log.Config{
			Name:        "AxonHub",
			Debug:       true,
			SkipLevel:   0,
			Level:       log.DebugLevel,
			LevelKey:    "",
			TimeKey:     "",
			CallerKey:   "",
			FunctionKey: "",
			NameKey:     "",
			Encoding:    "console_json",
			Includes:    nil,
			Excludes:    nil,
		}),
		fx.Invoke(func(lc fx.Lifecycle, server *server.Server) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go func() {
						err := server.Run()
						if err != nil {
							log.Error(context.Background(), "server run error:", log.Cause(err))
							os.Exit(1)
						}
					}()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					return server.Shutdown(ctx)
				},
			})
		}),
	)
}
