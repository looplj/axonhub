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
	systemService  *biz.SystemService
	channelService *biz.ChannelService
	requestService *biz.RequestService
	httpClient     *httpclient.HttpClient
}

// NewSchema creates a graphql executable schema.
func NewSchema(
	client *ent.Client,
	authService *biz.AuthService,
	systemService *biz.SystemService,
	channelService *biz.ChannelService,
	requestService *biz.RequestService,
) graphql.ExecutableSchema {
	return NewExecutableSchema(Config{
		Resolvers: &Resolver{
			client:         client,
			authService:    authService,
			systemService:  systemService,
			channelService: channelService,
			requestService: requestService,
			httpClient:     httpclient.NewHttpClient(),
		},
	})
}
