package gql

import (
	"context"
	"fmt"
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
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelperformance"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/ent/thread"
	"github.com/looplj/axonhub/internal/ent/trace"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/ent/userrole"
	"github.com/looplj/axonhub/internal/server/biz"
)

type Dependencies struct {
	fx.In

	Ent                *ent.Client
	AuthService        *biz.AuthService
	APIKeyService      *biz.APIKeyService
	UserService        *biz.UserService
	SystemService      *biz.SystemService
	ChannelService     *biz.ChannelService
	RequestService     *biz.RequestService
	ProjectService     *biz.ProjectService
	DataStorageService *biz.DataStorageService
	RoleService        *biz.RoleService
	TraceService       *biz.TraceService
	ThreadService      *biz.ThreadService
	UsageLogService    *biz.UsageLogService
}

type GraphqlHandler struct {
	Graphql    http.Handler
	Playground http.Handler
}

func NewGraphqlHandlers(deps Dependencies) *GraphqlHandler {
	gqlSrv := handler.New(
		NewSchema(
			deps.Ent,
			deps.AuthService,
			deps.APIKeyService,
			deps.UserService,
			deps.SystemService,
			deps.ChannelService,
			deps.RequestService,
			deps.ProjectService,
			deps.DataStorageService,
			deps.RoleService,
			deps.TraceService,
			deps.ThreadService,
			deps.UsageLogService,
		),
	)

	gqlSrv.AddTransport(transport.Options{})
	gqlSrv.AddTransport(transport.GET{})
	gqlSrv.AddTransport(transport.POST{})
	gqlSrv.AddTransport(transport.MultipartForm{})

	gqlSrv.SetQueryCache(lru.New[*ast.QueryDocument](1024))

	gqlSrv.Use(extension.Introspection{})
	gqlSrv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](1024),
	})
	gqlSrv.Use(&loggingTracer{})
	gqlSrv.Use(entgql.Transactioner{TxOpener: deps.Ent})

	return &GraphqlHandler{
		Graphql:    gqlSrv,
		Playground: playground.Handler("AxonHub", "/admin/graphql"),
	}
}

var guidTypeToNodeType = map[string]string{
	ent.TypeUser:               user.Table,
	ent.TypeAPIKey:             apikey.Table,
	ent.TypeChannel:            channel.Table,
	ent.TypeChannelPerformance: channelperformance.Table,
	ent.TypeRequest:            request.Table,
	ent.TypeRequestExecution:   requestexecution.Table,
	ent.TypeRole:               role.Table,
	ent.TypeSystem:             system.Table,
	ent.TypeUsageLog:           usagelog.Table,
	ent.TypeProject:            project.Table,
	ent.TypeUserProject:        userproject.Table,
	ent.TypeUserRole:           userrole.Table,
	ent.TypeThread:             thread.Table,
	ent.TypeTrace:              trace.Table,
	ent.TypeDataStorage:        datastorage.Table,
}

const maxPaginationLimit = 1000

// validatePaginationArgs ensures GraphQL list queries receive a bounded window.
func validatePaginationArgs(first, last *int) error {
	provided := false

	if first != nil {
		provided = true

		if *first <= 0 {
			return fmt.Errorf("first must be greater than 0")
		}

		if *first > maxPaginationLimit {
			return fmt.Errorf("first cannot exceed %d", maxPaginationLimit)
		}
	}

	if last != nil {
		provided = true

		if *last <= 0 {
			return fmt.Errorf("last must be greater than 0")
		}

		if *last > maxPaginationLimit {
			return fmt.Errorf("last cannot exceed %d", maxPaginationLimit)
		}
	}

	if !provided {
		return fmt.Errorf("either first or last must be provided")
	}

	return nil
}

func getNilableChannel(ctx context.Context, client *ent.Client, channelID int) (*ent.Channel, error) {
	if channelID == 0 {
		return nil, nil
	}

	ch, err := client.Channel.Query().Where(channel.ID(channelID)).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to load channel: %w", err)
	}

	return ch, nil
}
