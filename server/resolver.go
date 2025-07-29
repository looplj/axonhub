package server

import (
	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/server/biz"
)

var guidTypeToNodeType = map[string]string{
	"APIKey":  "api_keys",
	"User":    "users",
	"Channel": "channels",
	"Job":     "jobs",
	"Request": "requests",
	"Role":    "roles",
}

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver is the resolver root.
type Resolver struct{
	client        *ent.Client
	authService   *biz.AuthService
	systemService *biz.SystemService
}

// NewSchema creates a graphql executable schema.
func NewSchema(client *ent.Client, authService *biz.AuthService, systemService *biz.SystemService) graphql.ExecutableSchema {
	return NewExecutableSchema(Config{
		Resolvers: &Resolver{
			client:        client,
			authService:   authService,
			systemService: systemService,
		},
	})
}
