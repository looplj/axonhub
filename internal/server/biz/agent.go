package biz

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/prompt"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type AgentService struct {
	*AbstractService
}

type AgentServiceParams struct {
	fx.In

	Ent *ent.Client
}

func NewAgentService(params AgentServiceParams) *AgentService {
	return &AgentService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}
}

type CreateAgentInput struct {
	Name         string
	Description  *string
	Status       *agent.Status
	Model        *string
	SystemPrompt string

	BuiltinTools []objects.AgentBuiltinTool
	SkillsPolicy *objects.AgentSkillsPolicy
}

type CreateAgentResult struct {
	Agent  *ent.Agent
	APIKey *ent.APIKey
}

func (svc *AgentService) CreateAgent(ctx context.Context, input CreateAgentInput) (CreateAgentResult, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return CreateAgentResult{}, fmt.Errorf("project id not found in context")
	}

	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return CreateAgentResult{}, fmt.Errorf("user not found in context")
	}

	return RunInTransaction(ctx, svc.AbstractService, func(txCtx context.Context) (CreateAgentResult, error) {
		client := svc.entFromContext(txCtx)

		promptName := fmt.Sprintf("agent:%s", input.Name)
		promptDescription := "Agent system prompt"
		promptRole := "system"

		systemPrompt, err := authz.RunWithSystemBypass(txCtx, "create-agent-prompt", func(bypassCtx context.Context) (*ent.Prompt, error) {
			return client.Prompt.Create().
				SetProjectID(projectID).
				SetType(prompt.TypeAgent).
				SetName(promptName).
				SetDescription(promptDescription).
				SetRole(promptRole).
				SetContent(input.SystemPrompt).
				SetStatus(prompt.StatusDisabled).
				SetOrder(0).
				SetSettings(objects.PromptSettings{
					Action: objects.PromptAction{Type: objects.PromptActionTypeNoop},
				}).
				Save(bypassCtx)
		})
		if err != nil {
			return CreateAgentResult{}, fmt.Errorf("failed to create prompt: %w", err)
		}

		apiKeyName := fmt.Sprintf("agent:%s", input.Name)
		svcKey, err := authz.RunWithSystemBypass(txCtx, "create-agent-api-key", func(bypassCtx context.Context) (*ent.APIKey, error) {
			generatedKey, err := GenerateAPIKey()
			if err != nil {
				return nil, fmt.Errorf("failed to generate api key: %w", err)
			}

			return client.APIKey.Create().
				SetName(apiKeyName).
				SetKey(generatedKey).
				SetUserID(user.ID).
				SetProjectID(projectID).
				SetType(apikey.TypeAgent).
				SetScopes([]string{
					string(scopes.ScopeReadAgents),
					string(scopes.ScopeWriteAgents),
				}).
				Save(bypassCtx)
		})
		if err != nil {
			return CreateAgentResult{}, fmt.Errorf("failed to create service account api key: %w", err)
		}

		create := client.Agent.Create().
			SetProjectID(projectID).
			SetCreatedByUserID(user.ID).
			SetName(input.Name).
			SetNillableDescription(input.Description).
			SetPromptID(systemPrompt.ID).
			SetNillableModel(input.Model).
			SetNillableStatus(input.Status).
			SetNillableSkillsPolicy(input.SkillsPolicy).
			SetAgentBuiltinTools(input.BuiltinTools).
			SetAPIKeyID(svcKey.ID)

		createdAgent, err := create.Save(txCtx)
		if err != nil {
			return CreateAgentResult{}, fmt.Errorf("failed to create agent: %w", err)
		}

		return CreateAgentResult{
			Agent:  createdAgent,
			APIKey: svcKey,
		}, nil
	})
}

type UpdateAgentInput struct {
	Name         *string
	Description  *string
	Status       *agent.Status
	Model        *string
	SystemPrompt *string

	BuiltinTools []objects.AgentBuiltinTool
	SkillsPolicy *objects.AgentSkillsPolicy
}

func (svc *AgentService) UpdateAgent(ctx context.Context, id int, input UpdateAgentInput) (*ent.Agent, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	var returned *ent.Agent

	if err := svc.RunInTransaction(ctx, func(txCtx context.Context) error {
		client := svc.entFromContext(txCtx)

		entity, err := client.Agent.Query().
			Where(
				agent.IDEQ(id),
				agent.ProjectIDEQ(projectID),
			).
			Only(txCtx)
		if err != nil {
			return fmt.Errorf("failed to load agent: %w", err)
		}

		if input.SystemPrompt != nil {
			if _, err := authz.RunWithSystemBypass(txCtx, "update-agent-prompt", func(bypassCtx context.Context) (*ent.Prompt, error) {
				return client.Prompt.UpdateOneID(entity.PromptID).
					Where(prompt.ProjectIDEQ(projectID)).
					SetContent(*input.SystemPrompt).
					Save(bypassCtx)
			}); err != nil {
				return fmt.Errorf("failed to update agent system prompt: %w", err)
			}
		}

		update := client.Agent.UpdateOneID(entity.ID).
			Where(agent.ProjectIDEQ(projectID)).
			SetNillableName(input.Name).
			SetNillableDescription(input.Description).
			SetNillableStatus(input.Status).
			SetNillableModel(input.Model)

		if input.BuiltinTools != nil {
			update.SetAgentBuiltinTools(input.BuiltinTools)
		}

		if input.SkillsPolicy != nil && input.SkillsPolicy.Add != "" {
			update.SetSkillsPolicy(*input.SkillsPolicy)
		}

		updated, err := update.Save(txCtx)
		if err != nil {
			return fmt.Errorf("failed to update agent: %w", err)
		}

		returned = updated
		return nil
	}); err != nil {
		return nil, err
	}

	return returned, nil
}

func (svc *AgentService) DeleteAgent(ctx context.Context, id int) error {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return fmt.Errorf("project id not found in context")
	}

	n, err := svc.entFromContext(ctx).Agent.Delete().
		Where(
			agent.IDEQ(id),
			agent.ProjectIDEQ(projectID),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	if n == 0 {
		return fmt.Errorf("agent not found or not in project")
	}

	return nil
}
