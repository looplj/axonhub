package agentapi

import (
	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/internal/server/biz"
)

type Resolver struct {
	agentBootstrapService *biz.AgentBootstrapService
	agentDeployService    *biz.AgentDeployService
}

func NewSchema(agentHostService *biz.AgentBootstrapService, agentDeployService *biz.AgentDeployService) graphql.ExecutableSchema {
	return NewExecutableSchema(Config{
		Resolvers: &Resolver{
			agentBootstrapService: agentHostService,
			agentDeployService:    agentDeployService,
		},
	})
}
