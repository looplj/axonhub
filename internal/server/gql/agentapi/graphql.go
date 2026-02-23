package agentapi

import (
	"net/http"

	"entgo.io/contrib/entgql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/vektah/gqlparser/v2/ast"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/gql/logging"
)

type GraphqlHandler struct {
	Graphql    http.Handler
	Playground http.Handler
}

type Dependencies struct {
	fx.In

	Ent                 *ent.Client
	AgentRuntimeService *biz.AgentRuntimeService
}

func NewGraphqlHandlers(deps Dependencies) *GraphqlHandler {
	gqlSrv := handler.New(NewSchema(deps.AgentRuntimeService))

	gqlSrv.AddTransport(transport.Options{})
	gqlSrv.AddTransport(transport.GET{})
	gqlSrv.AddTransport(transport.POST{})
	gqlSrv.AddTransport(transport.MultipartForm{})

	gqlSrv.SetQueryCache(lru.New[*ast.QueryDocument](1024))

	gqlSrv.Use(extension.Introspection{})
	gqlSrv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](1024),
	})
	gqlSrv.Use(entgql.Transactioner{
		TxOpener: deps.Ent,
	})
	gqlSrv.Use(&logging.LoggingTracer{})

	return &GraphqlHandler{
		Graphql:    gqlSrv,
		Playground: playground.Handler("AxonHub Agent API", "/agent/v1/graphql"),
	}
}
