package gql

import (
	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver is the resolver root.
type Resolver struct {
	client                         *ent.Client
	authService                    *biz.AuthService
	apiKeyService                  *biz.APIKeyService
	userService                    *biz.UserService
	systemService                  *biz.SystemService
	channelService                 *biz.ChannelService
	requestService                 *biz.RequestService
	projectService                 *biz.ProjectService
	dataStorageService             *biz.DataStorageService
	roleService                    *biz.RoleService
	traceService                   *biz.TraceService
	threadService                  *biz.ThreadService
	channelOverrideTemplateService *biz.ChannelOverrideTemplateService
	modelService                   *biz.ModelService
	httpClient                     *httpclient.HttpClient
	modelFetcher                   *biz.ModelFetcher
	TestChannelOrchestrator        *orchestrator.TestChannelOrchestrator
}

// NewSchema creates a graphql executable schema.
func NewSchema(
	client *ent.Client,
	authService *biz.AuthService,
	apiKeyService *biz.APIKeyService,
	userService *biz.UserService,
	systemService *biz.SystemService,
	channelService *biz.ChannelService,
	requestService *biz.RequestService,
	projectService *biz.ProjectService,
	dataStorageService *biz.DataStorageService,
	roleService *biz.RoleService,
	traceService *biz.TraceService,
	threadService *biz.ThreadService,
	usageLogService *biz.UsageLogService,
	channelOverrideTemplateService *biz.ChannelOverrideTemplateService,
	modelService *biz.ModelService,
) graphql.ExecutableSchema {
	httpClient := httpclient.NewHttpClient()
	modelFetcher := biz.NewModelFetcher(httpClient, channelService)

	return NewExecutableSchema(Config{
		Resolvers: &Resolver{
			client:                         client,
			authService:                    authService,
			apiKeyService:                  apiKeyService,
			userService:                    userService,
			systemService:                  systemService,
			channelService:                 channelService,
			requestService:                 requestService,
			projectService:                 projectService,
			dataStorageService:             dataStorageService,
			roleService:                    roleService,
			traceService:                   traceService,
			threadService:                  threadService,
			channelOverrideTemplateService: channelOverrideTemplateService,
			modelService:                   modelService,
			httpClient:                     httpClient,
			modelFetcher:                   modelFetcher,
			TestChannelOrchestrator:        orchestrator.NewTestChannelOrchestrator(channelService, requestService, systemService, usageLogService, httpClient),
		},
	})
}
