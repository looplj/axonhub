package biz

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// normalizeVisionDelegationSettings trims the configured target model ID in place.
// Callers own the settings being persisted (create/update inputs), so mutating
// them here is safe; read paths must not go through this function.
func (svc *ModelService) normalizeVisionDelegationSettings(settings *objects.ModelSettings) {
	if settings == nil {
		return
	}

	if targetModelID := settings.VisionDelegation.TargetModelID; targetModelID != nil {
		settings.VisionDelegation.TargetModelID = lo.ToPtr(strings.TrimSpace(*targetModelID))
	}
}

func (svc *ModelService) validateVisionDelegation(
	ctx context.Context,
	sourceModelID string,
	sourceType model.Type,
	sourceCard *objects.ModelCard,
	settings *objects.ModelSettings,
) (*ent.Model, error) {
	if settings == nil || !settings.VisionDelegation.Enabled {
		return nil, nil
	}

	if sourceType != model.TypeChat {
		return nil, fmt.Errorf("vision delegation is only available for chat models")
	}
	if sourceCard != nil && sourceCard.SupportsVision() {
		return nil, fmt.Errorf("model %q already supports image input natively", sourceModelID)
	}

	targetModelID := ""
	if settings.VisionDelegation.TargetModelID != nil {
		targetModelID = strings.TrimSpace(*settings.VisionDelegation.TargetModelID)
	}
	if targetModelID == "" {
		return nil, fmt.Errorf("vision delegation target model is required")
	}

	target, err := svc.entFromContext(ctx).Model.Query().
		Where(model.ModelIDEQ(targetModelID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("vision delegation target model %q does not exist", targetModelID)
		}
		return nil, fmt.Errorf("failed to query vision delegation target model: %w", err)
	}

	if err := svc.validateVisionDelegationTarget(ctx, sourceModelID, target); err != nil {
		return nil, err
	}

	return target, nil
}

func (svc *ModelService) validateVisionDelegationTarget(ctx context.Context, sourceModelID string, target *ent.Model) error {
	if target == nil {
		return fmt.Errorf("vision delegation target model is unavailable")
	}
	if target.ModelID == sourceModelID {
		return fmt.Errorf("vision delegation target must differ from the source model")
	}
	if target.Status != model.StatusEnabled {
		return fmt.Errorf("vision delegation target model %q is not enabled", target.ModelID)
	}
	if target.Type != model.TypeChat {
		return fmt.Errorf("vision delegation target model %q is not a chat model", target.ModelID)
	}
	if target.ModelCard == nil || !target.ModelCard.SupportsVision() {
		return fmt.Errorf("vision delegation target model %q does not support image input natively", target.ModelID)
	}
	if target.Settings != nil && target.Settings.VisionDelegation.Enabled {
		return fmt.Errorf("vision delegation target model %q delegates vision itself", target.ModelID)
	}

	routable, err := svc.isModelRoutableForVisionDelegation(ctx, target)
	if err != nil {
		return err
	}
	if !routable {
		return fmt.Errorf("vision delegation target model %q has no active route", target.ModelID)
	}

	return nil
}

func (svc *ModelService) isModelRoutableForVisionDelegation(ctx context.Context, target *ent.Model) (bool, error) {
	var channels []*Channel
	if svc.channelService != nil {
		channels = svc.channelService.GetEnabledChannels()
	} else {
		entities, err := svc.entFromContext(ctx).Channel.Query().
			Where(channel.StatusEQ(channel.StatusEnabled)).
			All(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to query enabled channels: %w", err)
		}
		channels = lo.Map(entities, func(ch *ent.Channel, _ int) *Channel {
			return &Channel{Channel: ch}
		})
	}

	connections := MatchConnections(
		EffectiveModelAssociations(svc.modelSettingsOrDefault(ctx), target),
		channels,
	)
	if len(connections) == 0 {
		return false, nil
	}

	return hasCapableEndpointForModel(target, connections) &&
		len(llm.CapableAPIFormats(llm.RequestTypeChat)) > 0, nil
}

// GetVisionDelegationTarget re-validates the delegation config against live
// state (target enabled, still vision-capable, still routable) on every image
// request and fails closed: a stale config surfaces to the client as an error
// instead of silently sending image parts to a text-only upstream.
func (svc *ModelService) GetVisionDelegationTarget(ctx context.Context, source *ent.Model) (*ent.Model, error) {
	if source == nil || source.Settings == nil || !source.Settings.VisionDelegation.Enabled {
		return nil, fmt.Errorf("vision delegation is not enabled")
	}

	return svc.validateVisionDelegation(ctx, source.ModelID, source.Type, source.ModelCard, source.Settings)
}

func (svc *ModelService) EffectiveModelCard(ctx context.Context, source *ent.Model) *objects.ModelCard {
	if source == nil || source.ModelCard == nil {
		return nil
	}

	effective := *source.ModelCard
	if source.Settings != nil && source.Settings.VisionDelegation.Enabled {
		if _, err := svc.GetVisionDelegationTarget(ctx, source); err == nil {
			effective = effective.WithDelegatedVision(source.Settings)
		}
	}

	return &effective
}

func (svc *ModelService) ListVisionDelegationCandidates(ctx context.Context, sourceModelID string) ([]*ent.Model, error) {
	models, err := svc.entFromContext(ctx).Model.Query().
		Where(
			model.StatusEQ(model.StatusEnabled),
			model.TypeEQ(model.TypeChat),
			model.ModelIDNEQ(sourceModelID),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query vision delegation candidates: %w", err)
	}

	candidates := make([]*ent.Model, 0, len(models))
	for _, candidate := range models {
		if err := svc.validateVisionDelegationTarget(ctx, sourceModelID, candidate); err == nil {
			candidates = append(candidates, candidate)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})

	return candidates, nil
}

func (svc *ModelService) renameVisionDelegationReferences(ctx context.Context, oldModelID, newModelID string) error {
	if oldModelID == "" || oldModelID == newModelID {
		return nil
	}

	return svc.mutateVisionDelegationReferences(ctx, map[string]string{oldModelID: newModelID}, false, false)
}

func (svc *ModelService) disableVisionDelegationReferences(ctx context.Context, targetModelIDs []string, clearTarget bool) error {
	replacements := make(map[string]string, len(targetModelIDs))
	for _, modelID := range targetModelIDs {
		replacements[modelID] = modelID
	}

	return svc.mutateVisionDelegationReferences(ctx, replacements, true, clearTarget)
}

func (svc *ModelService) mutateVisionDelegationReferences(
	ctx context.Context,
	replacements map[string]string,
	disable bool,
	clearTarget bool,
) error {
	if len(replacements) == 0 {
		return nil
	}

	models, err := svc.entFromContext(ctx).Model.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query vision delegation references: %w", err)
	}

	for _, source := range models {
		if source.Settings == nil || source.Settings.VisionDelegation.TargetModelID == nil {
			continue
		}

		currentTarget := *source.Settings.VisionDelegation.TargetModelID
		replacement, ok := replacements[currentTarget]
		if !ok {
			continue
		}

		nextSettings := *source.Settings
		if disable {
			nextSettings.VisionDelegation.Enabled = false
		}
		if clearTarget {
			nextSettings.VisionDelegation.TargetModelID = nil
		} else {
			nextSettings.VisionDelegation.TargetModelID = lo.ToPtr(replacement)
		}

		if _, err := svc.entFromContext(ctx).Model.UpdateOneID(source.ID).
			SetSettings(&nextSettings).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update vision delegation reference for model %q: %w", source.ModelID, err)
		}
	}

	return nil
}

func modelCanServeAsVisionDelegationTarget(m *ent.Model) bool {
	return m != nil &&
		m.Status == model.StatusEnabled &&
		m.Type == model.TypeChat &&
		m.ModelCard != nil &&
		m.ModelCard.SupportsVision() &&
		(m.Settings == nil || !m.Settings.VisionDelegation.Enabled)
}
