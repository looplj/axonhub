package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/prompt"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xmap"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
)

type PromptServiceParams struct {
	fx.In

	Executor executors.ScheduledExecutor
	Ent      *ent.Client
}

func NewPromptService(params PromptServiceParams) *PromptService {
	svc := &PromptService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		cachedEnabledPrompts:   xmap.New[int, []*ent.Prompt](),
		latestCachedUpdateTime: xmap.New[int, time.Time](),
	}

	_, _ = params.Executor.ScheduleFuncAtCronRate(
		svc.loadPromptsPeriodic,
		executors.CRONRule{Expr: "*/1 * * * *"},
	)

	return svc
}

func (svc *PromptService) Initialize(ctx context.Context) error {
	ctx = authz.WithSystemBypass(ctx, "prompt-initialize")

	projects, err := svc.entFromContext(ctx).Project.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query projects for initial prompt loading: %w", err)
	}

	for _, project := range projects {
		if err := svc.loadPrompts(ctx, project.ID); err != nil {
			log.Error(ctx, "failed to load prompts for project", log.Int("project_id", project.ID), log.Cause(err))
		}
	}

	log.Info(ctx, "initial prompt loading completed", log.Int("project_count", len(projects)))

	return nil
}

type PromptService struct {
	*AbstractService

	// cachedEnabledPrompts 记录已启用的 prompt，key 为 projectID
	cachedEnabledPrompts *xmap.Map[int, []*ent.Prompt]

	// latestUpdate 记录最新的 prompt 更新时间，用于优化定时加载
	latestCachedUpdateTime *xmap.Map[int, time.Time]
}

func (svc *PromptService) loadPromptsPeriodic(ctx context.Context) {
	svc.latestCachedUpdateTime.Range(func(projectID int, _ time.Time) bool {
		if err := svc.loadPrompts(ctx, projectID); err != nil {
			log.Error(ctx, "failed to reload prompts periodically", log.Int("project_id", projectID), log.Cause(err))
		}

		return true
	})
}

func (svc *PromptService) ValidatePromptSettings(settings objects.PromptSettings) error {
	if len(settings.Conditions) == 0 {
		return nil
	}

	for _, composite := range settings.Conditions {
		for _, condition := range composite.Conditions {
			if condition.Type == objects.PromptActivationConditionTypeModelPattern {
				if condition.ModelPattern == nil || *condition.ModelPattern == "" {
					return fmt.Errorf("model_pattern is required when type is model_pattern")
				}

				if err := xregexp.ValidateRegex(*condition.ModelPattern); err != nil {
					return fmt.Errorf("invalid regex pattern in model_pattern: %w", err)
				}
			}

			if condition.Type == objects.PromptActivationConditionTypeModelID {
				if condition.ModelID == nil || *condition.ModelID == "" {
					return fmt.Errorf("model_id is required when type is model_id")
				}
			}
		}
	}

	return nil
}

func (svc *PromptService) CreatePrompt(ctx context.Context, input ent.CreatePromptInput) (*ent.Prompt, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	if err := svc.ValidatePromptSettings(input.Settings); err != nil {
		return nil, err
	}

	createBuilder := svc.entFromContext(ctx).Prompt.Create().
		SetProjectID(projectID).
		SetName(input.Name).
		SetRole(input.Role).
		SetContent(input.Content).
		SetSettings(input.Settings)

	if input.Description != nil {
		createBuilder.SetDescription(*input.Description)
	}

	if input.Status != nil {
		createBuilder.SetStatus(*input.Status)
	}

	prompt, err := createBuilder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt: %w", err)
	}

	svc.asyncReloadPrompts(projectID)

	return prompt, nil
}

func (svc *PromptService) UpdatePrompt(ctx context.Context, id int, input *ent.UpdatePromptInput) (*ent.Prompt, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	if input.Settings != nil {
		if err := svc.ValidatePromptSettings(*input.Settings); err != nil {
			return nil, err
		}
	}

	updateBuilder := svc.entFromContext(ctx).Prompt.Update().
		Where(
			prompt.IDEQ(id),
			prompt.ProjectIDEQ(projectID),
		).
		SetNillableName(input.Name).
		SetNillableDescription(input.Description).
		SetNillableRole(input.Role).
		SetNillableContent(input.Content).
		SetNillableStatus(input.Status)

	if input.Settings != nil {
		updateBuilder.SetSettings(*input.Settings)
	}

	n, err := updateBuilder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update prompt: %w", err)
	}

	if n == 0 {
		return nil, fmt.Errorf("prompt not found or not in project")
	}

	p, err := svc.entFromContext(ctx).Prompt.Query().
		Where(
			prompt.IDEQ(id),
			prompt.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query updated prompt: %w", err)
	}

	svc.asyncReloadPrompts(projectID)

	return p, nil
}

func (svc *PromptService) DeletePrompt(ctx context.Context, id int) error {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return fmt.Errorf("project id not found in context")
	}

	n, err := svc.entFromContext(ctx).Prompt.Delete().
		Where(
			prompt.IDEQ(id),
			prompt.ProjectIDEQ(projectID),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete prompt: %w", err)
	}

	if n == 0 {
		return fmt.Errorf("prompt not found or not in project")
	}

	svc.asyncReloadPrompts(projectID)

	return nil
}

func (svc *PromptService) UpdatePromptStatus(ctx context.Context, id int, status prompt.Status) (*ent.Prompt, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	n, err := svc.entFromContext(ctx).Prompt.Update().
		Where(
			prompt.IDEQ(id),
			prompt.ProjectIDEQ(projectID),
		).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update prompt status: %w", err)
	}

	if n == 0 {
		return nil, fmt.Errorf("prompt not found or not in project")
	}

	p, err := svc.entFromContext(ctx).Prompt.Query().
		Where(
			prompt.IDEQ(id),
			prompt.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query updated prompt: %w", err)
	}

	svc.asyncReloadPrompts(projectID)

	return p, nil
}

func (svc *PromptService) BulkDeletePrompts(ctx context.Context, ids []int) error {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return fmt.Errorf("project id not found in context")
	}

	_, err := svc.entFromContext(ctx).Prompt.Delete().
		Where(
			prompt.IDIn(ids...),
			prompt.ProjectID(projectID),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk delete prompts: %w", err)
	}

	svc.asyncReloadPrompts(projectID)

	return nil
}

func (svc *PromptService) BulkEnablePrompts(ctx context.Context, ids []int) error {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return fmt.Errorf("project id not found in context")
	}

	_, err := svc.entFromContext(ctx).Prompt.Update().
		Where(
			prompt.IDIn(ids...),
			prompt.ProjectID(projectID),
		).
		SetStatus(prompt.StatusEnabled).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk enable prompts: %w", err)
	}

	svc.asyncReloadPrompts(projectID)

	return nil
}

func (svc *PromptService) BulkDisablePrompts(ctx context.Context, ids []int) error {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return fmt.Errorf("project id not found in context")
	}

	_, err := svc.entFromContext(ctx).Prompt.Update().
		Where(
			prompt.IDIn(ids...),
			prompt.ProjectID(projectID),
		).
		SetStatus(prompt.StatusDisabled).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk disable prompts: %w", err)
	}

	svc.asyncReloadPrompts(projectID)

	return nil
}

func (svc *PromptService) loadPrompts(ctx context.Context, projectID int) error {
	ctx = authz.WithSystemBypass(ctx, "prompt-load-cache")
	// Check if there are updates for this project
	latestUpdatedPrompt, err := svc.entFromContext(ctx).Prompt.Query().
		Where(prompt.ProjectID(projectID)).
		Order(ent.Desc(prompt.FieldUpdatedAt)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}

	lastUpdate, _ := svc.latestCachedUpdateTime.Load(projectID)
	if latestUpdatedPrompt != nil {
		if !latestUpdatedPrompt.UpdatedAt.After(lastUpdate) {
			return nil
		}

		svc.latestCachedUpdateTime.Store(projectID, latestUpdatedPrompt.UpdatedAt)
	} else {
		svc.latestCachedUpdateTime.Store(projectID, time.Time{})
	}

	entities, err := svc.entFromContext(ctx).Prompt.Query().
		Where(
			prompt.ProjectID(projectID),
			prompt.StatusEQ(prompt.StatusEnabled),
		).
		All(ctx)
	if err != nil {
		return err
	}

	svc.cachedEnabledPrompts.Store(projectID, entities)

	return nil
}

func (svc *PromptService) GetEnabledPrompts(ctx context.Context, projectID int) ([]*ent.Prompt, error) {
	if prompts, ok := svc.cachedEnabledPrompts.Load(projectID); ok {
		return prompts, nil
	}

	if err := svc.loadPrompts(ctx, projectID); err != nil {
		return nil, err
	}

	prompts, _ := svc.cachedEnabledPrompts.Load(projectID)

	return prompts, nil
}

func (svc *PromptService) asyncReloadPrompts(projectID int) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error(context.Background(), "panic in async reload prompts", log.Any("panic", r))
			}
		}()

		reloadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Force reload by resetting latestUpdate timestamp
		svc.latestCachedUpdateTime.Store(projectID, time.Time{})

		if reloadErr := svc.loadPrompts(reloadCtx, projectID); reloadErr != nil {
			log.Error(reloadCtx, "failed to reload prompts", log.Int("project_id", projectID), log.Cause(reloadErr))
		}
	}()
}
