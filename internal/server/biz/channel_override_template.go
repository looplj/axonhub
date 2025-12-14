package biz

import (
	"context"
	"fmt"

	"entgo.io/contrib/entgql"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channeloverridetemplate"
	"github.com/looplj/axonhub/internal/objects"
)

// ChannelOverrideTemplateService handles CRUD and application of channel override templates.
type ChannelOverrideTemplateService struct {
	*AbstractService

	channelService *ChannelService
}

// ChannelOverrideTemplateServiceParams defines constructor dependencies.
type ChannelOverrideTemplateServiceParams struct {
	fx.In

	Client         *ent.Client
	ChannelService *ChannelService
}

// NewChannelOverrideTemplateService constructs the service.
func NewChannelOverrideTemplateService(params ChannelOverrideTemplateServiceParams) *ChannelOverrideTemplateService {
	return &ChannelOverrideTemplateService{
		AbstractService: &AbstractService{db: params.Client},
		channelService:  params.ChannelService,
	}
}

// CreateTemplate creates a new override template for the given user.
func (svc *ChannelOverrideTemplateService) CreateTemplate(
	ctx context.Context,
	userID int,
	name, description, channelType, overrideParameters string,
	overrideHeaders []objects.HeaderEntry,
) (*ent.ChannelOverrideTemplate, error) {
	// Normalize empty parameters to "{}"
	overrideParameters = NormalizeOverrideParameters(overrideParameters)

	if err := ValidateOverrideParameters(overrideParameters); err != nil {
		return nil, fmt.Errorf("invalid override parameters: %w", err)
	}

	if overrideHeaders != nil {
		if err := ValidateOverrideHeaders(overrideHeaders); err != nil {
			return nil, fmt.Errorf("invalid override headers: %w", err)
		}
	}

	template, err := svc.entFromContext(ctx).ChannelOverrideTemplate.Create().
		SetUserID(userID).
		SetName(name).
		SetDescription(description).
		SetChannelType(channeloverridetemplate.ChannelType(channelType)).
		SetOverrideParameters(overrideParameters).
		SetOverrideHeaders(overrideHeaders).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel override template: %w", err)
	}

	return template, nil
}

// UpdateTemplate updates fields of an existing template. Nil pointers mean "skip".
func (svc *ChannelOverrideTemplateService) UpdateTemplate(
	ctx context.Context,
	id int,
	name, description *string,
	overrideParameters *string,
	overrideHeaders []objects.HeaderEntry,
	appendOverrideHeaders []objects.HeaderEntry,
) (*ent.ChannelOverrideTemplate, error) {
	mut := svc.entFromContext(ctx).ChannelOverrideTemplate.UpdateOneID(id)

	if name != nil {
		mut.SetName(*name)
	}

	if description != nil {
		mut.SetDescription(*description)
	}

	if overrideParameters != nil {
		// Normalize empty parameters to "{}"
		normalized := NormalizeOverrideParameters(*overrideParameters)
		if err := ValidateOverrideParameters(normalized); err != nil {
			return nil, fmt.Errorf("invalid override parameters: %w", err)
		}

		mut.SetOverrideParameters(normalized)
	}

	if overrideHeaders != nil {
		if err := ValidateOverrideHeaders(overrideHeaders); err != nil {
			return nil, fmt.Errorf("invalid override headers: %w", err)
		}

		mut.SetOverrideHeaders(overrideHeaders)
	}

	if appendOverrideHeaders != nil {
		if err := ValidateOverrideHeaders(appendOverrideHeaders); err != nil {
			return nil, fmt.Errorf("invalid override headers to append: %w", err)
		}

		mut.AppendOverrideHeaders(appendOverrideHeaders)
	}

	template, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update channel override template: %w", err)
	}

	return template, nil
}

// DeleteTemplate performs a soft delete (handled by Ent mixin).
func (svc *ChannelOverrideTemplateService) DeleteTemplate(ctx context.Context, id int) error {
	if err := svc.entFromContext(ctx).ChannelOverrideTemplate.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete channel override template: %w", err)
	}

	return nil
}

// GetTemplate fetches a template by ID.
func (svc *ChannelOverrideTemplateService) GetTemplate(ctx context.Context, id int) (*ent.ChannelOverrideTemplate, error) {
	template, err := svc.entFromContext(ctx).ChannelOverrideTemplate.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel override template: %w", err)
	}

	return template, nil
}

// ApplyTemplate applies the template to the given channels atomically.
func (svc *ChannelOverrideTemplateService) ApplyTemplate(
	ctx context.Context,
	templateID int,
	channelIDs []int,
) (updated []*ent.Channel, err error) {
	db := svc.entFromContext(ctx)

	template, err := db.ChannelOverrideTemplate.Get(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch template: %w", err)
	}

	channels, err := db.Channel.Query().
		Where(channel.IDIn(channelIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}

	if len(channels) != len(channelIDs) {
		return nil, fmt.Errorf("some channels not found for provided IDs")
	}

	for _, ch := range channels {
		if ch.Type != channel.Type(template.ChannelType) {
			return nil, fmt.Errorf("channel %d type %s does not match template type %s", ch.ID, ch.Type, template.ChannelType)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	txCtx := ent.NewContext(ctx, tx.Client())
	updated = make([]*ent.Channel, 0, len(channels))

	for _, ch := range channels {
		// Copy existing settings to avoid mutating shared pointers.
		settings := objects.ChannelSettings{}
		if ch.Settings != nil {
			settings = *ch.Settings
		}

		settings.OverrideHeaders = MergeOverrideHeaders(settings.OverrideHeaders, template.OverrideHeaders)

		mergedParams, mergeErr := MergeOverrideParameters(settings.OverrideParameters, template.OverrideParameters)
		if mergeErr != nil {
			err = fmt.Errorf("failed to merge override parameters for channel %d: %w", ch.ID, mergeErr)
			return nil, err
		}

		settings.OverrideParameters = mergedParams

		updatedChannel, saveErr := tx.Channel.UpdateOneID(ch.ID).
			SetSettings(&settings).
			Save(txCtx)
		if saveErr != nil {
			err = fmt.Errorf("failed to update channel %d: %w", ch.ID, saveErr)
			return nil, err
		}

		updated = append(updated, updatedChannel)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit apply template transaction: %w", err)
	}

	if svc.channelService != nil {
		svc.channelService.asyncReloadChannels()
	}

	return updated, nil
}

// QueryChannelOverrideTemplatesInput represents the input for querying templates.
type QueryChannelOverrideTemplatesInput struct {
	After       *entgql.Cursor[int]
	First       *int
	Before      *entgql.Cursor[int]
	Last        *int
	ChannelType *channel.Type
	Search      *string
}

// QueryTemplates queries channel override templates with filtering and pagination.
func (svc *ChannelOverrideTemplateService) QueryTemplates(
	ctx context.Context,
	input QueryChannelOverrideTemplatesInput,
) (*ent.ChannelOverrideTemplateConnection, error) {
	query := svc.entFromContext(ctx).ChannelOverrideTemplate.Query()

	if input.ChannelType != nil {
		query = query.Where(channeloverridetemplate.ChannelTypeEQ(channeloverridetemplate.ChannelType(*input.ChannelType)))
	}

	if input.Search != nil && *input.Search != "" {
		query = query.Where(channeloverridetemplate.NameContains(*input.Search))
	}

	return query.Paginate(ctx, input.After, input.First, input.Before, input.Last)
}
