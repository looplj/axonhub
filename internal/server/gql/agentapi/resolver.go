package agentapi

import (
	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

type Resolver struct {
	agentBootstrapService *biz.AgentBootstrapService
	agentDeployService    *biz.AgentDeployService
	entClient             *ent.Client
}

func NewSchema(agentHostService *biz.AgentBootstrapService, agentDeployService *biz.AgentDeployService, entClient *ent.Client) graphql.ExecutableSchema {
	return NewExecutableSchema(Config{
		Resolvers: &Resolver{
			agentBootstrapService: agentHostService,
			agentDeployService:    agentDeployService,
			entClient:             entClient,
		},
	})
}
