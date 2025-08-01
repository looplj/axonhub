package api

import (
	"net/http"

	"go.uber.org/fx"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/looplj/axonhub/internal/ent"
)

type Dependencies struct {
	fx.In

	Client *ent.Client
}

type GraphqlHandler struct {
	Graphql    http.Handler
	Playground http.Handler
}

func NewGraphqlHandlers(schema graphql.ExecutableSchema) *GraphqlHandler {
	return &GraphqlHandler{
		Graphql:    NewGraphHandler(schema),
		Playground: playground.Handler("AxonHub", "/graphql"),
	}
}

func NewGraphHandler(es graphql.ExecutableSchema) *handler.Server {
	srv := handler.New(es)

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1024))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](1024),
	})
	return srv
}
