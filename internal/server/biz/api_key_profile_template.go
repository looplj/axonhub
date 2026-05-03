package biz

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikeyprofiletemplate"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
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

		templateProfile, err := deepCopyProfile(template.Profile)
		if err != nil {
			return fmt.Errorf("failed to copy template profile: %w", err)
		}
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

func (s *APIKeyProfileTemplateService) SaveAsTemplate(ctx context.Context, apiKeyID int, profileName, templateName, description string) (*ent.APIKeyProfileTemplate, error) {
	var template *ent.APIKeyProfileTemplate
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		apiKey, getErr := client.APIKey.Get(ctx, apiKeyID)
		if getErr != nil {
			return fmt.Errorf("failed to get API key: %w", getErr)
		}

		if apiKey.Profiles == nil {
			return fmt.Errorf("API key has no profiles")
		}

		var sourceProfile *objects.APIKeyProfile
		for i := range apiKey.Profiles.Profiles {
			if apiKey.Profiles.Profiles[i].Name == profileName {
				sourceProfile = &apiKey.Profiles.Profiles[i]
				break
			}
		}

		if sourceProfile == nil {
			return fmt.Errorf("profile '%s' not found in API key", profileName)
		}

		copiedProfile, getErr := deepCopyProfile(sourceProfile)
		if getErr != nil {
			log.Error(ctx, "Failed to copy API key profile for template", log.Cause(getErr), log.String("profile_name", profileName))
			return fmt.Errorf("failed to copy profile: %w", getErr)
		}

		var saveErr error
		template, saveErr = client.APIKeyProfileTemplate.Create().
			SetName(templateName).
			SetDescription(description).
			SetProjectID(apiKey.ProjectID).
			SetProfile(copiedProfile).
			Save(ctx)
		if saveErr != nil {
			return fmt.Errorf("failed to create template: %w", saveErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return template, nil
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

func deepCopyProfile(profile *objects.APIKeyProfile) (*objects.APIKeyProfile, error) {
	if profile == nil {
		return nil, nil
	}

	data, err := json.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal profile for deep copy: %w", err)
	}

	var copy objects.APIKeyProfile
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile for deep copy: %w", err)
	}

	return &copy, nil
}
