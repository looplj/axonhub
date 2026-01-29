package openapi

import (
	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/internal/server/biz"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

type Resolver struct {
	apiKeyService *biz.APIKeyService
}

func NewSchema(apiKeyService *biz.APIKeyService) graphql.ExecutableSchema {
	return NewExecutableSchema(Config{
		Resolvers: &Resolver{
			apiKeyService: apiKeyService,
		},
	})
}
