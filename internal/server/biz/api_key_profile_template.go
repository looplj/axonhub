package biz

import (
	"context"
	"fmt"
	"reflect"

	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
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
		profile.TemplateID = nil
		profile.TemplateName = ""
		profile.Name = input.Name
		if err := normalizeAndValidateProfileRoutingPolicy(profile); err != nil {
			return nil, err
		}
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
		existing, getErr := client.APIKeyProfileTemplate.Get(ctx, id)
		if getErr != nil {
			return fmt.Errorf("failed to get template: %w", getErr)
		}

		update := client.APIKeyProfileTemplate.UpdateOneID(id).
			SetInput(input)

		publishedProfile := profile
		if publishedProfile == nil && input.Name != nil {
			publishedProfile = existing.Profile.Clone()
		}

		if publishedProfile != nil {
			publishedProfile.TemplateID = nil
			publishedProfile.TemplateName = ""
			if err := normalizeAndValidateProfileRoutingPolicy(publishedProfile); err != nil {
				return err
			}

			if input.Name != nil {
				publishedProfile.Name = *input.Name
			} else {
				publishedProfile.Name = existing.Name
			}
			update.SetProfile(publishedProfile)
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

		if publishedProfile != nil {
			if syncErr := s.syncLinkedProfiles(ctx, existing, template, publishedProfile); syncErr != nil {
				return syncErr
			}
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

		if detachErr := s.detachLinkedProfiles(ctx, template); detachErr != nil {
			return detachErr
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
		if err := normalizeAndValidateProfileRoutingPolicy(templateProfile); err != nil {
			return err
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
		templateProfile.TemplateID = lo.ToPtr(template.ID)
		templateProfile.TemplateName = template.Name

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

// CountLinkedProfiles returns how many API key profiles currently follow a
// template. API key profiles are embedded JSON, so the count is intentionally
// computed from the project's keys instead of introducing a second source of
// truth.
func (s *APIKeyProfileTemplateService) CountLinkedProfiles(ctx context.Context, template *ent.APIKeyProfileTemplate) (int, error) {
	client := s.entFromContext(ctx)
	keys, err := client.APIKey.Query().
		Where(apikey.ProjectIDEQ(template.ProjectID)).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list API keys linked to template: %w", err)
	}

	count := 0
	for _, key := range keys {
		if key.Profiles == nil {
			continue
		}
		for i := range key.Profiles.Profiles {
			profile := &key.Profiles.Profiles[i]
			if (profile.TemplateID != nil && *profile.TemplateID == template.ID) || isLegacyTemplateMatch(profile, template) {
				count++
			}
		}
	}

	return count, nil
}

func (s *APIKeyProfileTemplateService) syncLinkedProfiles(ctx context.Context, previousTemplate, template *ent.APIKeyProfileTemplate, publishedProfile *objects.APIKeyProfile) error {
	client := s.entFromContext(ctx)
	keys, err := client.APIKey.Query().
		Where(apikey.ProjectIDEQ(template.ProjectID)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to list API keys linked to template: %w", err)
	}

	for _, key := range keys {
		if key.Profiles == nil {
			continue
		}

		changed := false
		for i := range key.Profiles.Profiles {
			current := &key.Profiles.Profiles[i]
			isLinked := current.TemplateID != nil && *current.TemplateID == template.ID
			if !isLinked && !isLegacyTemplateMatch(current, previousTemplate) {
				continue
			}

			// The API key profile name is an alias local to that key. Preserve it
			// during publishing so renamed templates and conflict suffixes never
			// invalidate activeProfile or collide with neighboring profiles.
			profileName := current.Name
			next := publishedProfile.Clone()
			next.Name = profileName
			next.TemplateID = lo.ToPtr(template.ID)
			next.TemplateName = template.Name
			key.Profiles.Profiles[i] = *next
			changed = true
		}

		if changed {
			if _, err := client.APIKey.UpdateOneID(key.ID).SetProfiles(key.Profiles).Save(ctx); err != nil {
				return fmt.Errorf("failed to publish template to API key %d: %w", key.ID, err)
			}
		}
	}

	return nil
}

// isLegacyTemplateMatch recognizes profiles created before explicit template
// linkage existed. Only an unchanged profile with the template's original name
// is adopted, so previously customized profiles remain independent.
func isLegacyTemplateMatch(profile *objects.APIKeyProfile, template *ent.APIKeyProfileTemplate) bool {
	if profile == nil || profile.TemplateID != nil || template == nil || template.Profile == nil {
		return false
	}
	if profile.Name != template.Name && profile.Name != template.Profile.Name {
		return false
	}

	left := normalizeProfileForComparison(profile)
	right := normalizeProfileForComparison(template.Profile)
	left.Name = ""
	right.Name = ""
	left.TemplateID = nil
	right.TemplateID = nil
	left.TemplateName = ""
	right.TemplateName = ""

	return reflect.DeepEqual(left, right)
}

func (s *APIKeyProfileTemplateService) detachLinkedProfiles(ctx context.Context, template *ent.APIKeyProfileTemplate) error {
	client := s.entFromContext(ctx)
	keys, err := client.APIKey.Query().
		Where(apikey.ProjectIDEQ(template.ProjectID)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to list API keys linked to template: %w", err)
	}

	for _, key := range keys {
		if key.Profiles == nil {
			continue
		}

		changed := false
		for i := range key.Profiles.Profiles {
			profile := &key.Profiles.Profiles[i]
			if profile.TemplateID != nil && *profile.TemplateID == template.ID {
				profile.TemplateID = nil
				profile.TemplateName = ""
				changed = true
			}
		}

		if changed {
			if _, err := client.APIKey.UpdateOneID(key.ID).SetProfiles(key.Profiles).Save(ctx); err != nil {
				return fmt.Errorf("failed to detach template from API key %d: %w", key.ID, err)
			}
		}
	}

	return nil
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
