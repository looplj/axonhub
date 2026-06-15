package biz

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikeyprofiletemplate"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

type APIKeyProfileTemplateServiceParams struct {
	fx.In

	Ent *ent.Client
}

type APIKeyProfileTemplateService struct {
	*AbstractService
}

func NewAPIKeyProfileTemplateService(params APIKeyProfileTemplateServiceParams) *APIKeyProfileTemplateService {
	return &APIKeyProfileTemplateService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}
}

func (s *APIKeyProfileTemplateService) CreateTemplate(ctx context.Context, input ent.CreateAPIKeyProfileTemplateInput, profile *objects.APIKeyProfile) (*ent.APIKeyProfileTemplate, error) {
	client := s.entFromContext(ctx)

	if profile != nil {
		profile.Name = input.Name
	}

	create := client.APIKeyProfileTemplate.Create().
		SetInput(input).
		SetProfile(profile)

	template, err := create.Save(ctx)
	if err != nil {
		// Name uniqueness is enforced by the (project_id, name, deleted_at) unique
		// index; surface a friendly error instead of a raw constraint violation.
		if ent.IsConstraintError(err) {
			return nil, xerrors.DuplicateNameError("Template", input.Name)
		}

		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	return template, nil
}

func (s *APIKeyProfileTemplateService) GetTemplate(ctx context.Context, id int) (*ent.APIKeyProfileTemplate, error) {
	client := s.entFromContext(ctx)

	template, err := client.APIKeyProfileTemplate.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return template, nil
}

// GetForRead loads a template by id or name for read-only access. Exactly one
// of id or name must be non-nil.
//
// Like APIKeyService.GetForRead, it goes through the context-bound ent client
// so the APIKeyProfileTemplate privacy policy runs: an API key principal must
// hold read_api_keys and is filtered to templates inside its own project, where
// names are unique (DB index on project_id+name) — so a name identifies at most
// one template and foreign templates surface as NotFound.
func (s *APIKeyProfileTemplateService) GetForRead(ctx context.Context, id *int, name *string) (*ent.APIKeyProfileTemplate, error) {
	if (id == nil) == (name == nil) {
		return nil, fmt.Errorf("exactly one of template id or name must be provided")
	}

	client := s.entFromContext(ctx)
	q := client.APIKeyProfileTemplate.Query()

	switch {
	case id != nil:
		q = q.Where(apikeyprofiletemplate.IDEQ(*id))
	case name != nil:
		q = q.Where(apikeyprofiletemplate.NameEQ(*name))
	}

	template, err := q.Only(ctx)
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (s *APIKeyProfileTemplateService) ListTemplates(ctx context.Context, projectID int) ([]*ent.APIKeyProfileTemplate, error) {
	client := s.entFromContext(ctx)

	templates, err := client.APIKeyProfileTemplate.Query().
		Where(apikeyprofiletemplate.ProjectIDEQ(projectID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	return templates, nil
}

func (s *APIKeyProfileTemplateService) UpdateTemplate(ctx context.Context, id int, input ent.UpdateAPIKeyProfileTemplateInput, profile *objects.APIKeyProfile) (*ent.APIKeyProfileTemplate, error) {
	var template *ent.APIKeyProfileTemplate
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		update := client.APIKeyProfileTemplate.UpdateOneID(id).
			SetInput(input)

		if profile != nil {
			existing, getErr := client.APIKeyProfileTemplate.Get(ctx, id)
			if getErr != nil {
				return fmt.Errorf("failed to get template: %w", getErr)
			}
			if input.Name != nil {
				profile.Name = *input.Name
			} else {
				profile.Name = existing.Name
			}
			update.SetProfile(profile)
		}

		var saveErr error
		template, saveErr = update.Save(ctx)
		if saveErr != nil {
			// The unique index on (project_id, name, deleted_at) is the source of
			// truth for name uniqueness; map it to a friendly error.
			if ent.IsConstraintError(saveErr) {
				return xerrors.DuplicateNameError("Template", lo.FromPtr(input.Name))
			}

			return fmt.Errorf("failed to update template: %w", saveErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (s *APIKeyProfileTemplateService) DeleteTemplate(ctx context.Context, id int) (*ent.APIKeyProfileTemplate, error) {
	var template *ent.APIKeyProfileTemplate
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		var getErr error
		template, getErr = client.APIKeyProfileTemplate.Get(ctx, id)
		if getErr != nil {
			return fmt.Errorf("failed to get template for deletion: %w", getErr)
		}

		getErr = client.APIKeyProfileTemplate.DeleteOneID(id).Exec(ctx)
		if getErr != nil {
			return fmt.Errorf("failed to delete template: %w", getErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (s *APIKeyProfileTemplateService) LoadTemplate(ctx context.Context, templateID, apiKeyID int) (*ent.APIKey, error) {
	var updatedKey *ent.APIKey
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		template, err := client.APIKeyProfileTemplate.Get(ctx, templateID)
		if err != nil {
			return fmt.Errorf("failed to get template: %w", err)
		}

		apiKey, getErr := client.APIKey.Get(ctx, apiKeyID)
		if getErr != nil {
			return fmt.Errorf("failed to get API key: %w", getErr)
		}

		if template.ProjectID != apiKey.ProjectID {
			return fmt.Errorf("template and API key must belong to the same project")
		}

		templateProfile := template.Profile.Clone()
		if templateProfile == nil {
			return fmt.Errorf("template has no profile")
		}

		existingProfiles := apiKey.Profiles
		if existingProfiles == nil {
			existingProfiles = &objects.APIKeyProfiles{}
		}

		profileName := templateProfile.Name
		if profileName == "" {
			profileName = template.Name
		}
		resolvedName := resolveProfileNameConflict(existingProfiles.Profiles, profileName)
		templateProfile.Name = resolvedName

		existingProfiles.Profiles = append(existingProfiles.Profiles, *templateProfile)

		updatedKey, err = client.APIKey.UpdateOneID(apiKeyID).
			SetProfiles(existingProfiles).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update API key profiles: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return updatedKey, nil
}

func resolveProfileNameConflict(existingProfiles []objects.APIKeyProfile, newName string) string {
	nameSet := make(map[string]bool, len(existingProfiles))
	for _, p := range existingProfiles {
		nameSet[p.Name] = true
	}

	if !nameSet[newName] {
		return newName
	}

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s (%d)", newName, i)
		if !nameSet[candidate] {
			return candidate
		}
	}
}
