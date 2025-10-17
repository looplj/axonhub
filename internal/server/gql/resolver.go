package gql

import (
	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver is the resolver root.
type Resolver struct {
	client         *ent.Client
	authService    *biz.AuthService
	apiKeyService  *biz.APIKeyService
	userService    *biz.UserService
	systemService  *biz.SystemService
	channelService *biz.ChannelService
	requestService *biz.RequestService
	projectService *biz.ProjectService
	roleService    *biz.RoleService
	httpClient     *httpclient.HttpClient
	modelFetcher   *biz.ModelFetcher
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
	roleService *biz.RoleService,
) graphql.ExecutableSchema {
	httpClient := httpclient.NewHttpClient()
	modelFetcher := biz.NewModelFetcher(httpClient, channelService)

	return NewExecutableSchema(Config{
		Resolvers: &Resolver{
			client:         client,
			authService:    authService,
			apiKeyService:  apiKeyService,
			userService:    userService,
			systemService:  systemService,
			channelService: channelService,
			requestService: requestService,
			projectService: projectService,
			roleService:    roleService,
			httpClient:     httpClient,
			modelFetcher:   modelFetcher,
		},
	})
}
