package biz

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
)

type ModelServiceParams struct {
	fx.In

	Ent *ent.Client
}

func NewModelService(params ModelServiceParams) *ModelService {
	return &ModelService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}
}

type ModelService struct {
	*AbstractService
}

// CreateModel creates a new model with the provided input.
func (svc *ModelService) CreateModel(ctx context.Context, input ent.CreateModelInput) (*ent.Model, error) {
	// Check if a model with the same developer and modelId already exists
	existing, err := svc.entFromContext(ctx).Model.Query().
		Where(
			model.Developer(input.Developer),
			model.ModelID(input.ModelID),
		).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check model existence: %w", err)
	}

	if existing != nil {
		return nil, fmt.Errorf("model with developer '%s' and modelId '%s' already exists", input.Developer, input.ModelID)
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

// UpdateModel updates an existing model with the provided input.
func (svc *ModelService) UpdateModel(ctx context.Context, id int, input *ent.UpdateModelInput) (*ent.Model, error) {
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

type ModelChannelConnection struct {
	Channel  *ent.Channel `json:"channel"`
	ModelIds []string     `json:"modelIds"`
}

// QueryModelChannelConnections queries channels and their models based on model associations.
// Results are ordered by the first occurrence of each channel in the associations list,
// and both channels and models are deduplicated.
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

	result := svc.matchAssociations(associations, channels)

	return result, nil
}

func (svc *ModelService) matchAssociations(associations []*objects.ModelAssociation, channels []*ent.Channel) []*ModelChannelConnection {
	// Track channel order and connections
	channelIndex := make(map[int]int)                           // channel ID -> result index
	channelConnections := make(map[int]*ModelChannelConnection) // channel ID -> connection

	// Process associations in order
	for _, assoc := range associations {
		connections := svc.matchAssociation(assoc, channels)
		for _, conn := range connections {
			existing, exists := channelConnections[conn.Channel.ID]
			if !exists {
				// New channel - assign next index and create connection
				channelIndex[conn.Channel.ID] = len(channelIndex)
				channelConnections[conn.Channel.ID] = conn
			} else {
				existing.ModelIds = append(existing.ModelIds, conn.ModelIds...)
			}
		}
	}

	// Build result slice in order and deduplicate models
	result := make([]*ModelChannelConnection, len(channelIndex))

	for chID, conn := range channelConnections {
		conn.ModelIds = lo.Uniq(conn.ModelIds)
		result[channelIndex[chID]] = conn
	}

	return result
}

// matchAssociation matches a single association against all channels and returns model channel connections.
func (svc *ModelService) matchAssociation(assoc *objects.ModelAssociation, channels []*ent.Channel) []*ModelChannelConnection {
	connections := make([]*ModelChannelConnection, 0)

	switch assoc.Type {
	case "channel_model":
		if assoc.ChannelModel != nil {
			ch, found := lo.Find(channels, func(c *ent.Channel) bool {
				return c.ID == assoc.ChannelModel.ChannelID
			})
			if found && lo.Contains(ch.SupportedModels, assoc.ChannelModel.ModelID) {
				connections = append(connections, &ModelChannelConnection{
					Channel:  ch,
					ModelIds: []string{assoc.ChannelModel.ModelID},
				})
			}
		}
	case "channel_regex":
		if assoc.ChannelRegex != nil {
			ch, found := lo.Find(channels, func(c *ent.Channel) bool {
				return c.ID == assoc.ChannelRegex.ChannelID
			})
			if found {
				modelIds := xregexp.FilterByPattern(ch.SupportedModels, assoc.ChannelRegex.Pattern)
				if len(modelIds) > 0 {
					connections = append(connections, &ModelChannelConnection{
						Channel:  ch,
						ModelIds: modelIds,
					})
				}
			}
		}
	case "regex":
		if assoc.Regex != nil {
			for _, ch := range channels {
				modelIds := xregexp.FilterByPattern(ch.SupportedModels, assoc.Regex.Pattern)
				if len(modelIds) > 0 {
					connections = append(connections, &ModelChannelConnection{
						Channel:  ch,
						ModelIds: modelIds,
					})
				}
			}
		}
	case "model":
		if assoc.ModelID != nil {
			modelID := assoc.ModelID.ModelID
			for _, ch := range channels {
				if lo.Contains(ch.SupportedModels, modelID) {
					connections = append(connections, &ModelChannelConnection{
						Channel:  ch,
						ModelIds: []string{modelID},
					})
				}
			}
		}

	}

	return connections
}
