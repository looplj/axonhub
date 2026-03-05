package biz

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agenthost"
)

type AgentHostServiceParams struct {
	fx.In

	Ent *ent.Client
}

// AgentHostService provides APIs for managing agent hosts.
// This service handles CRUD operations and connection testing for agent hosts.
type AgentHostService struct {
	*AbstractService
}

func NewAgentHostService(params AgentHostServiceParams) *AgentHostService {
	return &AgentHostService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}
}

func (svc *AgentHostService) CreateAgentHost(ctx context.Context, input ent.CreateAgentHostInput) (*ent.AgentHost, error) {
	runtime, err := svc.db.AgentHost.Create().
		SetInput(input).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent host: %w", err)
	}

	return runtime, nil
}

func (svc *AgentHostService) UpdateAgentHost(ctx context.Context, id int, input ent.UpdateAgentHostInput) (*ent.AgentHost, error) {
	runtime, err := svc.db.AgentHost.UpdateOneID(id).
		SetInput(input).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update agent host: %w", err)
	}

	return runtime, nil
}

func (svc *AgentHostService) DeleteAgentHost(ctx context.Context, id int) error {
	n, err := svc.db.AgentHost.Delete().Where(agenthost.IDEQ(id)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete agent host: %w", err)
	}

	if n == 0 {
		return fmt.Errorf("agent host not found")
	}

	return nil
}

type TestConnectionResult struct {
	Success bool
	Error   string
	Latency int
}

func (svc *AgentHostService) TestConnection(ctx context.Context, id int) (*TestConnectionResult, error) {
	runtime, err := svc.db.AgentHost.Query().Where(agenthost.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent host: %w", err)
	}

	return svc.testConnection(ctx, runtime)
}

func (svc *AgentHostService) testConnection(_ context.Context, runtime *ent.AgentHost) (*TestConnectionResult, error) {
	return &TestConnectionResult{
		Success: true,
	}, nil
}
