package server

import (
	"net/http"

	"go.uber.org/fx"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/looplj/axonhub/ent"
)

type Dependencies struct {
	fx.In

	Client *ent.Client
}

type GraphqlHandler struct {
	Graphql    http.Handler
	Playground http.Handler
}

func NewGraphqlHandlers(deps Dependencies) *GraphqlHandler {
	gqlSrv := handler.New(NewSchema(deps.Client))

	gqlSrv.AddTransport(transport.Options{})
	gqlSrv.AddTransport(transport.GET{})
	gqlSrv.AddTransport(transport.POST{})
	gqlSrv.AddTransport(transport.MultipartForm{})

	gqlSrv.SetQueryCache(lru.New[*ast.QueryDocument](1024))

	gqlSrv.Use(extension.Introspection{})
	gqlSrv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](1024),
	})
	return &GraphqlHandler{
		Graphql:    gqlSrv,
		Playground: playground.Handler("AxonHub", "/graphql"),
	}
}
