package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/api"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/dependencies"
	"github.com/looplj/axonhub/internal/server/gql"
	"go.uber.org/fx"
)

func New(config Config) *Server {
	engine := gin.New()
	engine.Use(gin.Recovery())

	if !config.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	return &Server{
		config: config,
		Engine: engine,
	}
}

type Server struct {
	*gin.Engine

	config Config
	server *http.Server
	addr   string
}

func (srv *Server) Run() error {
	log.Info(
		context.Background(),
		"run server",
		log.String("name", srv.config.Name),
		log.Int("port", srv.config.Port),
	)
	addr := fmt.Sprintf("0.0.0.0:%d", srv.config.Port)
	srv.server = &http.Server{
		Addr:         addr,
		Handler:      srv.Engine,
		ReadTimeout:  srv.config.ReadTimeout,
		WriteTimeout: srv.config.WriteTimeout,
	}
	srv.addr = addr

	err := srv.server.ListenAndServe()
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	}

	return nil
}

func (srv *Server) Shutdown(ctx context.Context) error {
	return srv.server.Shutdown(ctx)
}

func Run(opts ...fx.Option) {
	var constructors []any

	constructors = append(constructors, gql.NewGraphqlHandlers, New)
	app := fx.New(
		append([]fx.Option{
			fx.NopLogger,
			fx.Provide(constructors...),
			dependencies.Module,
			biz.Module,
			api.Module,
			fx.Invoke(func(cfg log.Config) {
				log.SetGlobalConfig(cfg)
				slog.SetDefault(log.GetGlobalLogger().AsSlog())
			}),
			fx.Invoke(SetupRoutes),
		}, opts...)...,
	)
	app.Run()
}
