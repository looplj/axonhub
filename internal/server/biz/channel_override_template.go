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
	input ent.CreateChannelOverrideTemplateInput,
) (*ent.ChannelOverrideTemplate, error) {
	if input.HeaderOverrideOperations != nil {
		if err := ValidateOverrideHeaders(input.HeaderOverrideOperations); err != nil {
			return nil, fmt.Errorf("invalid header override operations: %w", err)
		}
	}

	if input.BodyOverrideOperations != nil {
		if err := ValidateBodyOverrideOperations(input.BodyOverrideOperations); err != nil {
			return nil, fmt.Errorf("invalid body override operations: %w", err)
		}
	}

	template, err := svc.entFromContext(ctx).ChannelOverrideTemplate.Create().
		SetUserID(userID).
		SetName(input.Name).
		SetNillableDescription(input.Description).
		SetHeaderOverrideOperations(input.HeaderOverrideOperations).
		SetBodyOverrideOperations(input.BodyOverrideOperations).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel override template: %w", err)
	}

	return template, nil
}

// UpdateTemplate updates fields of an existing template using the input structure.
func (svc *ChannelOverrideTemplateService) UpdateTemplate(
	ctx context.Context,
	id int,
	input ent.UpdateChannelOverrideTemplateInput,
) (*ent.ChannelOverrideTemplate, error) {
	mut := svc.entFromContext(ctx).ChannelOverrideTemplate.UpdateOneID(id)

	if input.Name != nil {
		mut.SetName(*input.Name)
	}

	if input.ClearDescription {
		mut.ClearDescription()
	} else {
		mut.SetNillableDescription(input.Description)
	}

	if input.HeaderOverrideOperations != nil {
		if err := ValidateOverrideHeaders(input.HeaderOverrideOperations); err != nil {
			return nil, fmt.Errorf("invalid header override operations: %w", err)
		}

		mut.SetHeaderOverrideOperations(input.HeaderOverrideOperations)
	}

	if input.AppendHeaderOverrideOperations != nil {
		if err := ValidateOverrideHeaders(input.AppendHeaderOverrideOperations); err != nil {
			return nil, fmt.Errorf("invalid header override operations to append: %w", err)
		}

		mut.AppendHeaderOverrideOperations(input.AppendHeaderOverrideOperations)
	}

	if input.BodyOverrideOperations != nil {
		if err := ValidateBodyOverrideOperations(input.BodyOverrideOperations); err != nil {
			return nil, fmt.Errorf("invalid body override operations: %w", err)
		}

		mut.SetBodyOverrideOperations(input.BodyOverrideOperations)
	}

	if input.AppendBodyOverrideOperations != nil {
		if err := ValidateBodyOverrideOperations(input.AppendBodyOverrideOperations); err != nil {
			return nil, fmt.Errorf("invalid body override operations to append: %w", err)
		}

		mut.AppendBodyOverrideOperations(input.AppendBodyOverrideOperations)
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

	updated = make([]*ent.Channel, 0, len(channels))

	err = svc.RunInTransaction(ctx, func(ctx context.Context) error {
		db := svc.entFromContext(ctx)

		for _, ch := range channels {
			// Copy existing settings to avoid mutating shared pointers.
			settings := objects.ChannelSettings{}
			if ch.Settings != nil {
				settings = *ch.Settings
			}

			// Get existing header operations from channel settings
			existingHeaderOps := getHeaderOverrideOperations(&settings)

			// Merge template header operations with existing channel header operations
			mergedHeaderOps := MergeOverrideHeaders(existingHeaderOps, template.HeaderOverrideOperations)
			settings.HeaderOverrideOperations = mergedHeaderOps

			// Clear legacy header field
			settings.OverrideHeaders = nil

			// Get existing body operations from channel settings
			existingBodyOps := getBodyOverrideOperations(&settings)

			// Merge template body operations with existing channel body operations
			mergedBodyOps := MergeOverrideOperations(existingBodyOps, template.BodyOverrideOperations)
			settings.BodyOverrideOperations = mergedBodyOps

			// Clear legacy body parameters field
			settings.OverrideParameters = ""

			updatedChannel, err := db.Channel.UpdateOneID(ch.ID).
				SetSettings(&settings).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to update channel %s: %w", ch.Name, err)
			}

			updated = append(updated, updatedChannel)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if svc.channelService != nil {
		svc.channelService.asyncReloadChannels()
	}

	return updated, nil
}

// getHeaderOverrideOperations returns the header override operations from channel settings.
// It prioritizes the new HeaderOverrideOperations field over the legacy OverrideHeaders field.
func getHeaderOverrideOperations(settings *objects.ChannelSettings) []objects.OverrideOperation {
	if settings.HeaderOverrideOperations != nil {
		return settings.HeaderOverrideOperations
	}

	if len(settings.OverrideHeaders) > 0 {
		return objects.HeaderEntriesToOverrideOperations(settings.OverrideHeaders)
	}

	return nil
}

// getBodyOverrideOperations returns the body override operations from channel settings.
// It prioritizes the new BodyOverrideOperations field over the legacy OverrideParameters field.
func getBodyOverrideOperations(settings *objects.ChannelSettings) []objects.OverrideOperation {
	if settings.BodyOverrideOperations != nil {
		return settings.BodyOverrideOperations
	}

	if settings.OverrideParameters != "" {
		ops, err := objects.ParseOverrideOperations(settings.OverrideParameters)
		if err != nil {
			return nil
		}

		return ops
	}

	return nil
}

// MergeOverrideOperations merges existing body operations with template operations.
// - For set/delete ops, matching is by Path. Template overrides existing.
// - For rename/copy ops, they are always appended.
// - Existing ops not mentioned in the template are preserved.
func MergeOverrideOperations(existing, template []objects.OverrideOperation) []objects.OverrideOperation {
	result := make([]objects.OverrideOperation, 0, len(existing)+len(template))
	result = append(result, existing...)

	for _, op := range template {
		if op.Op == objects.OverrideOpRename || op.Op == objects.OverrideOpCopy {
			result = append(result, op)
			continue
		}

		found := false

		for i := range result {
			if (result[i].Op == objects.OverrideOpSet || result[i].Op == objects.OverrideOpDelete) &&
				result[i].Path == op.Path {
				result[i] = op
				found = true

				break
			}
		}

		if !found {
			result = append(result, op)
		}
	}

	return result
}

// QueryChannelOverrideTemplatesInput represents the input for querying templates.
type QueryChannelOverrideTemplatesInput struct {
	After  *entgql.Cursor[int]
	First  *int
	Before *entgql.Cursor[int]
	Last   *int
	Search *string
}

// QueryTemplates queries channel override templates with filtering and pagination.
func (svc *ChannelOverrideTemplateService) QueryTemplates(
	ctx context.Context,
	input QueryChannelOverrideTemplatesInput,
) (*ent.ChannelOverrideTemplateConnection, error) {
	query := svc.entFromContext(ctx).ChannelOverrideTemplate.Query()

	if input.Search != nil && *input.Search != "" {
		query = query.Where(channeloverridetemplate.NameContains(*input.Search))
	}

	return query.Paginate(ctx, input.After, input.First, input.Before, input.Last)
}
