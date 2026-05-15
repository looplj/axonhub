package biz

import (
	"fmt"
	"sort"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func normalizeSystemModelSettings(settings *SystemModelSettings) {
	if settings == nil {
		return
	}

	if settings.DeveloperSettings == nil {
		settings.DeveloperSettings = []*DeveloperModelSettings{}
	}

	for _, developer := range settings.DeveloperSettings {
		if developer == nil {
			continue
		}

		developer.Developer = strings.TrimSpace(developer.Developer)
		if developer.Associations == nil {
			developer.Associations = []*objects.ModelAssociation{}
		}
		normalizeDeveloperAssociations(developer.Associations)
	}
}

func validateSystemModelSettings(settings *SystemModelSettings) error {
	if settings == nil {
		return nil
	}

	seenDevelopers := make(map[string]struct{}, len(settings.DeveloperSettings))
	for _, developerSettings := range settings.DeveloperSettings {
		if developerSettings == nil {
			continue
		}

		developer := strings.TrimSpace(developerSettings.Developer)
		if developer == "" {
			return fmt.Errorf("model developer is required")
		}

		if _, ok := seenDevelopers[developer]; ok {
			return fmt.Errorf("duplicate model developer %q", developer)
		}
		seenDevelopers[developer] = struct{}{}

		if err := validateDeveloperAssociations(developerSettings.Associations); err != nil {
			return fmt.Errorf("invalid developer settings for %q: %w", developer, err)
		}
	}

	return nil
}

func normalizeDeveloperAssociations(associations []*objects.ModelAssociation) {
	for _, assoc := range associations {
		if assoc == nil {
			continue
		}

		switch assoc.Type {
		case "channel_model":
			if assoc.ChannelModel != nil {
				assoc.ChannelModel.ModelID = ""
			}
		case "channel_tags_model":
			if assoc.ChannelTagsModel != nil {
				assoc.ChannelTagsModel.ModelID = ""
			}
		}
	}
}

func validateDeveloperAssociations(associations []*objects.ModelAssociation) error {
	for _, assoc := range associations {
		if assoc == nil {
			continue
		}

		switch assoc.Type {
		case "channel_model":
			if assoc.ChannelModel == nil || assoc.ChannelModel.ChannelID == 0 {
				return fmt.Errorf("developer channel association requires channel")
			}
		case "channel_tags_model":
			if assoc.ChannelTagsModel == nil || len(assoc.ChannelTagsModel.ChannelTags) == 0 {
				return fmt.Errorf("developer channel tags association requires channel tags")
			}
		default:
			return fmt.Errorf("developer association type %q is not supported", assoc.Type)
		}
	}

	if err := validateModelSettings(&objects.ModelSettings{Associations: associations}); err != nil {
		return err
	}

	return nil
}

func developerAssociationsForDeveloper(settings *SystemModelSettings, developer string) []*objects.ModelAssociation {
	if settings == nil || developer == "" {
		return nil
	}

	for _, developerSettings := range settings.DeveloperSettings {
		if developerSettings == nil || developerSettings.Developer != developer {
			continue
		}

		return developerSettings.Associations
	}

	return nil
}

// EffectiveModelAssociations returns the associations that should actually be
// used for one model. A matching developer setting is inherited by default;
// model settings add extra rules on top of it unless inheritance is explicitly
// disabled on the model.
func EffectiveModelAssociations(systemSettings *SystemModelSettings, model *ent.Model) []*objects.ModelAssociation {
	if model == nil {
		return nil
	}

	var modelAssociations []*objects.ModelAssociation
	if model.Settings != nil {
		modelAssociations = model.Settings.Associations
		if model.Settings.DisableDeveloperSettingsInheritance {
			return mergeInheritedModelAssociations(nil, modelAssociations)
		}
	}

	return mergeInheritedModelAssociations(
		inheritDeveloperAssociationsForModel(developerAssociationsForDeveloper(systemSettings, model.Developer), model.ModelID),
		modelAssociations,
	)
}

func inheritDeveloperAssociationsForModel(developerAssociations []*objects.ModelAssociation, modelID string) []*objects.ModelAssociation {
	if modelID == "" {
		return nil
	}

	associations := make([]*objects.ModelAssociation, 0, len(developerAssociations))
	for _, assoc := range developerAssociations {
		inherited := inheritDeveloperAssociationForModel(assoc, modelID)
		if inherited != nil {
			associations = append(associations, inherited)
		}
	}

	return associations
}

func inheritDeveloperAssociationForModel(assoc *objects.ModelAssociation, modelID string) *objects.ModelAssociation {
	if assoc == nil {
		return nil
	}

	// Developer rules only choose channels or channel tags. The concrete model
	// ID is filled in at inheritance time so sibling models do not share one
	// fixed actual model.
	inherited := cloneModelAssociation(assoc)
	switch inherited.Type {
	case "channel_model":
		if inherited.ChannelModel == nil {
			return nil
		}
		inherited.ChannelModel.ModelID = modelID
	case "channel_tags_model":
		if inherited.ChannelTagsModel == nil {
			return nil
		}
		inherited.ChannelTagsModel.ModelID = modelID
	default:
		return nil
	}

	return inherited
}

func cloneModelAssociation(assoc *objects.ModelAssociation) *objects.ModelAssociation {
	if assoc == nil {
		return nil
	}

	clone := *assoc
	clone.When = cloneModelAssociationWhen(assoc.When)
	if assoc.ChannelModel != nil {
		channelModel := *assoc.ChannelModel
		clone.ChannelModel = &channelModel
	}
	if assoc.ChannelRegex != nil {
		channelRegex := *assoc.ChannelRegex
		clone.ChannelRegex = &channelRegex
	}
	if assoc.Regex != nil {
		regex := *assoc.Regex
		regex.Exclude = cloneExcludeAssociations(assoc.Regex.Exclude)
		clone.Regex = &regex
	}
	if assoc.ModelID != nil {
		modelID := *assoc.ModelID
		modelID.Exclude = cloneExcludeAssociations(assoc.ModelID.Exclude)
		clone.ModelID = &modelID
	}
	if assoc.ChannelTagsModel != nil {
		channelTagsModel := *assoc.ChannelTagsModel
		channelTagsModel.ChannelTags = append([]string(nil), assoc.ChannelTagsModel.ChannelTags...)
		clone.ChannelTagsModel = &channelTagsModel
	}
	if assoc.ChannelTagsRegex != nil {
		channelTagsRegex := *assoc.ChannelTagsRegex
		channelTagsRegex.ChannelTags = append([]string(nil), assoc.ChannelTagsRegex.ChannelTags...)
		clone.ChannelTagsRegex = &channelTagsRegex
	}

	return &clone
}

func cloneModelAssociationWhen(when *objects.ModelAssociationWhen) *objects.ModelAssociationWhen {
	if when == nil {
		return nil
	}

	clone := *when
	if when.Condition != nil {
		condition := cloneCondition(*when.Condition)
		clone.Condition = &condition
	}

	return &clone
}

func cloneCondition(condition objects.Condition) objects.Condition {
	clone := condition
	if len(condition.Conditions) == 0 {
		return clone
	}

	clone.Conditions = make([]objects.Condition, len(condition.Conditions))
	for i := range condition.Conditions {
		clone.Conditions[i] = cloneCondition(condition.Conditions[i])
	}

	return clone
}

func cloneExcludeAssociations(excludes []*objects.ExcludeAssociation) []*objects.ExcludeAssociation {
	if len(excludes) == 0 {
		return nil
	}

	clones := make([]*objects.ExcludeAssociation, 0, len(excludes))
	for _, exclude := range excludes {
		if exclude == nil {
			continue
		}

		clone := *exclude
		clone.ChannelIds = append([]int(nil), exclude.ChannelIds...)
		clone.ChannelTags = append([]string(nil), exclude.ChannelTags...)
		clones = append(clones, &clone)
	}

	return clones
}

func mergeInheritedModelAssociations(
	developerAssociations []*objects.ModelAssociation,
	modelAssociations []*objects.ModelAssociation,
) []*objects.ModelAssociation {
	// Lower priority values run first. When priorities are equal, model-level
	// rules are placed before inherited developer rules so local configuration
	// can refine the shared defaults.
	type associationWithSource struct {
		association *objects.ModelAssociation
		sourceRank  int
		order       int
	}

	items := make([]associationWithSource, 0, len(developerAssociations)+len(modelAssociations))
	for i, assoc := range modelAssociations {
		if assoc == nil {
			continue
		}

		items = append(items, associationWithSource{
			association: assoc,
			sourceRank:  0,
			order:       i,
		})
	}

	for i, assoc := range developerAssociations {
		if assoc == nil {
			continue
		}

		items = append(items, associationWithSource{
			association: assoc,
			sourceRank:  1,
			order:       i,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.association.Priority != right.association.Priority {
			return left.association.Priority < right.association.Priority
		}

		if left.sourceRank != right.sourceRank {
			return left.sourceRank < right.sourceRank
		}

		return left.order < right.order
	})

	associations := make([]*objects.ModelAssociation, 0, len(items))
	for _, item := range items {
		associations = append(associations, item.association)
	}

	return associations
}
