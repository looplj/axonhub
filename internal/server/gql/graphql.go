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
	"github.com/looplj/axonhub/internal/ent/channeloverridetemplate"
	"github.com/looplj/axonhub/internal/ent/channelperformance"
	"github.com/looplj/axonhub/internal/ent/channelprobe"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/prompt"
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
	"github.com/looplj/axonhub/internal/server/backup"
	"github.com/looplj/axonhub/internal/server/biz"
)

type Dependencies struct {
	fx.In

	Ent                            *ent.Client
	AuthService                    *biz.AuthService
	APIKeyService                  *biz.APIKeyService
	UserService                    *biz.UserService
	SystemService                  *biz.SystemService
	ChannelService                 *biz.ChannelService
	RequestService                 *biz.RequestService
	ProjectService                 *biz.ProjectService
	DataStorageService             *biz.DataStorageService
	RoleService                    *biz.RoleService
	TraceService                   *biz.TraceService
	ThreadService                  *biz.ThreadService
	UsageLogService                *biz.UsageLogService
	ChannelOverrideTemplateService *biz.ChannelOverrideTemplateService
	ModelService                   *biz.ModelService
	BackupService                  *backup.BackupService
	ChannelProbeService            *biz.ChannelProbeService
	PromptService                  *biz.PromptService
	ProviderQuotaService           *biz.ProviderQuotaService
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
			deps.ChannelOverrideTemplateService,
			deps.ModelService,
			deps.BackupService,
			deps.ChannelProbeService,
			deps.PromptService,
			deps.ProviderQuotaService,
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
	gqlSrv.Use(entgql.Transactioner{
		TxOpener: deps.Ent,
		// Skip transaction for TestChannel mutation to avoid transaction conflicts
		// when multiple test requests are sent in parallel from the frontend.
		// TestChannel performs LLM API calls which can be long-running, and the
		// database operations within don't require transactional consistency.
		SkipTxFunc: entgql.SkipOperations("TestChannel"),
	})

	return &GraphqlHandler{
		Graphql:    gqlSrv,
		Playground: playground.Handler("AxonHub", "/admin/graphql"),
	}
}

var guidTypeToNodeType = map[string]string{
	ent.TypeUser:                    user.Table,
	ent.TypeAPIKey:                  apikey.Table,
	ent.TypeModel:                   model.Table,
	ent.TypeChannel:                 channel.Table,
	ent.TypeChannelPerformance:      channelperformance.Table,
	ent.TypeChannelProbe:            channelprobe.Table,
	ent.TypeChannelOverrideTemplate: channeloverridetemplate.Table,
	ent.TypeRequest:                 request.Table,
	ent.TypeRequestExecution:        requestexecution.Table,
	ent.TypeRole:                    role.Table,
	ent.TypeSystem:                  system.Table,
	ent.TypeUsageLog:                usagelog.Table,
	ent.TypeProject:                 project.Table,
	ent.TypeUserProject:             userproject.Table,
	ent.TypeUserRole:                userrole.Table,
	ent.TypeThread:                  thread.Table,
	ent.TypeTrace:                   trace.Table,
	ent.TypeDataStorage:             datastorage.Table,
	ent.TypePrompt:                  prompt.Table,
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
