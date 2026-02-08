package biz

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
	"github.com/looplj/axonhub/internal/scopes"
)

type ModelServiceParams struct {
	fx.In

	ChannelService *ChannelService
	SystemService  *SystemService
	Ent            *ent.Client
}

func NewModelService(params ModelServiceParams) *ModelService {
	return &ModelService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		channelService: params.ChannelService,
		systemService:  params.SystemService,
	}
}

type ModelService struct {
	*AbstractService

	channelService *ChannelService
	systemService  *SystemService
}

// validateModelSettings validates regex patterns in model settings.
func (svc *ModelService) validateModelSettings(settings *objects.ModelSettings) error {
	if settings == nil || len(settings.Associations) == 0 {
		return nil
	}

	for _, assoc := range settings.Associations {
		// Validate ChannelRegex pattern
		if assoc.ChannelRegex != nil && assoc.ChannelRegex.Pattern != "" {
			if err := xregexp.ValidateRegex(assoc.ChannelRegex.Pattern); err != nil {
				return fmt.Errorf("invalid regex pattern in channel_regex association: %w", err)
			}
		}

		// Validate ChannelTagsRegex pattern
		if assoc.ChannelTagsRegex != nil && assoc.ChannelTagsRegex.Pattern != "" {
			if err := xregexp.ValidateRegex(assoc.ChannelTagsRegex.Pattern); err != nil {
				return fmt.Errorf("invalid regex pattern in channel_tags_regex association: %w", err)
			}
		}

		// Validate Regex pattern
		if assoc.Regex != nil && assoc.Regex.Pattern != "" {
			if err := xregexp.ValidateRegex(assoc.Regex.Pattern); err != nil {
				return fmt.Errorf("invalid regex pattern in regex association: %w", err)
			}
		}

		// Validate Exclude patterns
		if assoc.Regex != nil && len(assoc.Regex.Exclude) > 0 {
			for _, exclude := range assoc.Regex.Exclude {
				if exclude.ChannelNamePattern != "" {
					if err := xregexp.ValidateRegex(exclude.ChannelNamePattern); err != nil {
						return fmt.Errorf("invalid regex pattern in exclude rule: %w", err)
					}
				}
			}
		}

		if assoc.ModelID != nil && len(assoc.ModelID.Exclude) > 0 {
			for _, exclude := range assoc.ModelID.Exclude {
				if exclude.ChannelNamePattern != "" {
					if err := xregexp.ValidateRegex(exclude.ChannelNamePattern); err != nil {
						return fmt.Errorf("invalid regex pattern in exclude rule: %w", err)
					}
				}
			}
		}
	}

	return nil
}

// CreateModel creates a new model with the provided input.
func (svc *ModelService) CreateModel(ctx context.Context, input ent.CreateModelInput) (*ent.Model, error) {
	// Validate regex patterns in settings if provided
	if input.Settings != nil {
		if err := svc.validateModelSettings(input.Settings); err != nil {
			return nil, err
		}
	}

	// Check if a model with the same developer and modelId already exists
	existing, err := svc.entFromContext(ctx).Model.Query().
		Where(model.ModelID(input.ModelID)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check model existence: %w", err)
	}

	if existing != nil {
		return nil, fmt.Errorf("model '%s' already exists", input.ModelID)
	}

	createBuilder := svc.entFromContext(ctx).Model.Create().
		SetDeveloper(input.Developer).
		SetModelID(input.ModelID).
		SetIcon(input.Icon).
		SetType(*input.Type).
		SetName(input.Name).
		SetGroup(input.Group).
		SetModelCard(input.ModelCard).
		SetSettings(input.Settings)

	if input.Remark != nil {
		createBuilder.SetRemark(*input.Remark)
	}

	model, err := createBuilder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	return model, nil
}

// BulkCreateModels creates multiple models with the provided inputs.
func (svc *ModelService) BulkCreateModels(ctx context.Context, inputs []*ent.CreateModelInput) ([]*ent.Model, error) {
	// Check for duplicates in the input
	inputMap := make(map[string]bool)

	for _, input := range inputs {
		key := fmt.Sprintf("%s:%s", input.Developer, input.ModelID)
		if inputMap[key] {
			return nil, fmt.Errorf("duplicate model in input: developer '%s' and modelId '%s'", input.Developer, input.ModelID)
		}

		inputMap[key] = true
	}

	// Check if any models already exist
	existingModels, err := svc.entFromContext(ctx).Model.Query().
		Where(func(s *sql.Selector) {
			var predicates []*sql.Predicate
			for _, input := range inputs {
				predicates = append(predicates, sql.And(
					sql.EQ(model.FieldDeveloper, input.Developer),
					sql.EQ(model.FieldModelID, input.ModelID),
				))
			}

			s.Where(sql.Or(predicates...))
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing models: %w", err)
	}

	if len(existingModels) > 0 {
		existingKeys := lo.Map(existingModels, func(m *ent.Model, _ int) string {
			return fmt.Sprintf("%s:%s", m.Developer, m.ModelID)
		})

		return nil, fmt.Errorf("models already exist: %v", existingKeys)
	}

	// Create all models in a transaction
	bulk := make([]*ent.ModelCreate, len(inputs))
	for i, input := range inputs {
		createBuilder := svc.entFromContext(ctx).Model.Create().
			SetDeveloper(input.Developer).
			SetModelID(input.ModelID).
			SetIcon(input.Icon).
			SetType(*input.Type).
			SetName(input.Name).
			SetGroup(input.Group).
			SetModelCard(input.ModelCard).
			SetSettings(input.Settings)

		if input.Remark != nil {
			createBuilder.SetRemark(*input.Remark)
		}

		bulk[i] = createBuilder
	}

	models, err := svc.entFromContext(ctx).Model.CreateBulk(bulk...).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk create models: %w", err)
	}

	return models, nil
}

// UpdateModel updates an existing model with the provided input.
func (svc *ModelService) UpdateModel(ctx context.Context, id int, input *ent.UpdateModelInput) (*ent.Model, error) {
	// Validate regex patterns in settings if provided
	if input.Settings != nil {
		if err := svc.validateModelSettings(input.Settings); err != nil {
			return nil, err
		}
	}

	mut := svc.entFromContext(ctx).Model.UpdateOneID(id).
		SetNillableName(input.Name).
		SetNillableGroup(input.Group).
		SetNillableStatus(input.Status).
		SetNillableIcon(input.Icon)

	if input.ModelCard != nil {
		mut.SetModelCard(input.ModelCard)
	}

	if input.Settings != nil {
		mut.SetSettings(input.Settings)
	}

	if input.Remark != nil {
		mut.SetRemark(*input.Remark)
	}

	if input.ClearRemark {
		mut.ClearRemark()
	}

	model, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update model: %w", err)
	}

	return model, nil
}

// UpdateModelStatus updates the status of a model.
func (svc *ModelService) UpdateModelStatus(ctx context.Context, id int, status model.Status) (*ent.Model, error) {
	model, err := svc.entFromContext(ctx).Model.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update model status: %w", err)
	}

	return model, nil
}

// DeleteModel deletes a model by ID.
func (svc *ModelService) DeleteModel(ctx context.Context, id int) error {
	if err := svc.entFromContext(ctx).Model.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	return nil
}

// BulkArchiveModels archives multiple models by their IDs.
func (svc *ModelService) BulkArchiveModels(ctx context.Context, ids []int) error {
	_, err := svc.entFromContext(ctx).Model.Update().
		Where(model.IDIn(ids...)).
		SetStatus(model.StatusArchived).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk archive models: %w", err)
	}

	return nil
}

// BulkDisableModels disables multiple models by their IDs.
func (svc *ModelService) BulkDisableModels(ctx context.Context, ids []int) error {
	_, err := svc.entFromContext(ctx).Model.Update().
		Where(model.IDIn(ids...)).
		SetStatus(model.StatusDisabled).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk disable models: %w", err)
	}

	return nil
}

// BulkEnableModels enables multiple models by their IDs.
func (svc *ModelService) BulkEnableModels(ctx context.Context, ids []int) error {
	_, err := svc.entFromContext(ctx).Model.Update().
		Where(model.IDIn(ids...)).
		SetStatus(model.StatusEnabled).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk enable models: %w", err)
	}

	return nil
}

// BulkDeleteModels deletes multiple models by their IDs.
func (svc *ModelService) BulkDeleteModels(ctx context.Context, ids []int) error {
	_, err := svc.entFromContext(ctx).Model.Delete().
		Where(model.IDIn(ids...)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk delete models: %w", err)
	}

	return nil
}

// QueryModelChannelConnections queries channels and their models based on model associations.
// Results are ordered by the matching order of associations.
func (svc *ModelService) QueryModelChannelConnections(ctx context.Context, associations []*objects.ModelAssociation) ([]*ModelChannelConnection, error) {
	if len(associations) == 0 {
		return []*ModelChannelConnection{}, nil
	}

	// Query all enabled/disabled channels
	channels, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.StatusIn(channel.StatusEnabled, channel.StatusDisabled)).
		Order(channel.ByOrderingWeight(sql.OrderDesc())).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}

	if len(channels) == 0 {
		return []*ModelChannelConnection{}, nil
	}

	// Use the shared MatchAssociations function
	return MatchAssociations(associations, lo.Map(channels, func(ch *ent.Channel, _ int) *Channel {
		return &Channel{Channel: ch}
	})), nil
}

// GetModelByModelID retrieves a model by its modelId and status.
func (svc *ModelService) GetModelByModelID(ctx context.Context, modelID string, status model.Status) (*ent.Model, error) {
	return svc.entFromContext(ctx).Model.Query().
		Where(
			model.ModelID(modelID),
			model.StatusEQ(status),
		).
		First(ctx)
}

// ListModels retrieves all models that have explicit Model entity configuration.
// Returns models with their status.
func (svc *ModelService) ListModels(ctx context.Context, statusIn []model.Status) ([]*ModelIdentityWithStatus, error) {
	query := svc.entFromContext(ctx).Model.Query()

	// Apply status filter if provided
	if len(statusIn) > 0 {
		query = query.Where(model.StatusIn(statusIn...))
	} else {
		// Default to enabled models only
		query = query.Where(model.StatusEQ(model.StatusEnabled))
	}

	models, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query configured models: %w", err)
	}

	// Convert to ModelIdentityWithStatus
	result := make([]*ModelIdentityWithStatus, 0, len(models))
	for _, m := range models {
		result = append(result, &ModelIdentityWithStatus{
			ID:     m.ModelID,
			Status: channel.Status(m.Status.String()),
		})
	}

	return result, nil
}

// ListEnabledModels returns all unique models across all enabled channels,
// considering model mappings, prefixes, and auto-trimmed models.
// It uses GetModelEntries to reduce code duplication.
// When QueryAllChannelModels in system settings is false, it returns configured models instead.
// If an API key is present in context and has an active profile with modelIDs configured,
// only those models will be returned.
func (svc *ModelService) ListEnabledModels(ctx context.Context) ([]ModelFacade, error) {
	var (
		channels = svc.channelService.GetEnabledChannels()
		profile  *objects.APIKeyProfile
	)

	ctx = authz.WithScopeDecision(ctx, scopes.ScopeReadChannels)

	if apiKey, ok := contexts.GetAPIKey(ctx); ok && apiKey != nil {
		profile = apiKey.GetActiveProfile()

		if profile != nil && len(profile.ChannelIDs) > 0 {
			channels = lo.Filter(channels, func(ch *Channel, _ int) bool {
				return lo.Contains(profile.ChannelIDs, ch.ID)
			})
		}

		if profile != nil && len(profile.ChannelTags) > 0 {
			channels = lo.Filter(channels, func(ch *Channel, _ int) bool {
				return len(lo.Intersect(profile.ChannelTags, ch.Tags)) > 0
			})
		}
	}

	settings := svc.systemService.ModelSettingsOrDefault(ctx)
	if !settings.QueryAllChannelModels {
		query := svc.entFromContext(ctx).
			Model.
			Query().
			Where(model.StatusEQ(model.StatusEnabled))
		if profile != nil && len(profile.ModelIDs) > 0 {
			query = query.Where(model.ModelIDIn(profile.ModelIDs...))
		}

		enabledModels, err := query.All(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list configured models: %w", err)
		}

		var models []ModelFacade

		for _, model := range enabledModels {
			if model.Settings == nil {
				continue
			}

			associations := MatchAssociations(model.Settings.Associations, channels)
			if len(associations) > 0 {
				models = append(models, ModelFacade{
					ID:          model.ModelID,
					DisplayName: model.ModelID,
					CreatedAt:   model.CreatedAt,
					Created:     model.CreatedAt.Unix(),
					OwnedBy:     "configured",
				})
			}
		}

		return models, nil
	}

	var (
		models   []ModelFacade
		modelSet = make(map[string]bool)
	)

	for _, ch := range channels {
		entries := ch.GetModelEntries()

		for requestModel := range entries {
			if _, ok := modelSet[requestModel]; ok {
				continue
			}

			modelSet[requestModel] = true

			models = append(models, ModelFacade{
				ID:          requestModel,
				DisplayName: requestModel,
				CreatedAt:   ch.CreatedAt,
				Created:     ch.CreatedAt.Unix(),
				OwnedBy:     ch.Channel.Type.String(),
			})
		}
	}

	if profile != nil && len(profile.ModelIDs) > 0 {
		models = lo.Filter(models, func(m ModelFacade, _ int) bool {
			return lo.Contains(profile.ModelIDs, m.ID)
		})
	}

	return models, nil
}

// CountAssociatedChannels counts the number of unique channels associated with the given model associations.
func (svc *ModelService) CountAssociatedChannels(ctx context.Context, associations []*objects.ModelAssociation) (int, error) {
	if len(associations) == 0 {
		return 0, nil
	}

	// Query all enabled/disabled channels
	channels, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.StatusIn(channel.StatusEnabled, channel.StatusDisabled)).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to query channels: %w", err)
	}

	if len(channels) == 0 {
		return 0, nil
	}

	// Use the shared MatchAssociations function
	connections := MatchAssociations(associations, lo.Map(channels, func(ch *ent.Channel, _ int) *Channel {
		return &Channel{Channel: ch}
	}))

	// Remove duplicate channels
	connections = lo.UniqBy(connections, func(conn *ModelChannelConnection) int {
		return conn.Channel.ID
	})

	return len(connections), nil
}

func (svc *ModelService) QueryUnassociatedChannels(ctx context.Context) ([]*UnassociatedChannel, error) {
	channels, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.StatusIn(channel.StatusEnabled, channel.StatusDisabled)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}

	if len(channels) == 0 {
		return []*UnassociatedChannel{}, nil
	}

	models, err := svc.entFromContext(ctx).Model.Query().
		Where(model.StatusIn(model.StatusEnabled, model.StatusDisabled)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}

	allAssociations := make([]*objects.ModelAssociation, 0)

	for _, m := range models {
		if m.Settings != nil && len(m.Settings.Associations) > 0 {
			allAssociations = append(allAssociations, m.Settings.Associations...)
		}
	}

	return findUnassociatedChannels(channels, allAssociations), nil
}

func findUnassociatedChannels(channels []*ent.Channel, associations []*objects.ModelAssociation) []*UnassociatedChannel {
	if len(channels) == 0 {
		return []*UnassociatedChannel{}
	}

	// Wrap channels
	channelWrappers := make([]*Channel, 0, len(channels))
	for _, ch := range channels {
		channelWrappers = append(channelWrappers, &Channel{Channel: ch})
	}

	// Use MatchAssociations to get all associated models
	connections := MatchAssociations(associations, channelWrappers)

	// Build a map of associated (channelID, modelID) combinations
	associatedMap := make(map[ChannelModelKey]bool)

	for _, conn := range connections {
		for _, entry := range conn.Models {
			key := ChannelModelKey{
				ChannelID: conn.Channel.ID,
				ModelID:   entry.RequestModel,
			}
			associatedMap[key] = true
		}
	}

	// Check each channel for unassociated models
	result := make([]*UnassociatedChannel, 0)

	for _, ch := range channels {
		channelWrapper := &Channel{Channel: ch}
		entries := channelWrapper.GetModelEntries()

		unassociatedModels := make([]string, 0)

		for modelID := range entries {
			key := ChannelModelKey{
				ChannelID: ch.ID,
				ModelID:   modelID,
			}
			if !associatedMap[key] {
				unassociatedModels = append(unassociatedModels, modelID)
			}
		}

		if len(unassociatedModels) > 0 {
			result = append(result, &UnassociatedChannel{
				Channel: ch,
				Models:  unassociatedModels,
			})
		}
	}

	return result
}

type ModelIdentify struct {
	ID string `json:"id"`
}

type UnassociatedChannel struct {
	Channel *ent.Channel `json:"channel"`
	Models  []string     `json:"models"`
}

type ModelFacade struct {
	ID string `json:"id"`
	// Display name, for user-friendly display from anthropic API.
	DisplayName string `json:"display_name"`
	// Created time in seconds.
	Created int64 `json:"created"`
	// Created time in time.Time.
	CreatedAt time.Time `json:"created_at"`
	// Owned by
	OwnedBy string `json:"owned_by"`
}
