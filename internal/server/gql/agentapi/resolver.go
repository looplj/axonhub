package agentapi

import (
	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/internal/server/biz"
)

// This file will not be regenerated automatically.
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	agentBootstrapService *biz.AgentBootstrapService
}

func NewSchema(agentHostService *biz.AgentBootstrapService) graphql.ExecutableSchema {
	return NewExecutableSchema(Config{
		Resolvers: &Resolver{
			agentBootstrapService: agentHostService,
		},
	})
}
