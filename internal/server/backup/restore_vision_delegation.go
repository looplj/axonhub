package backup

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
)

func repairRestoredVisionDelegations(ctx context.Context, db *ent.Client) error {
	models, err := db.Model.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query restored models for vision delegation repair: %w", err)
	}

	byModelID := make(map[string]*ent.Model, len(models))
	for _, candidate := range models {
		byModelID[candidate.ModelID] = candidate
	}

	for _, source := range models {
		if source.Settings == nil || !source.Settings.VisionDelegation.Enabled {
			continue
		}

		nextSettings := *source.Settings
		targetModelID := nextSettings.VisionDelegation.TargetModelID
		if targetModelID == nil || *targetModelID == "" {
			nextSettings.VisionDelegation.Enabled = false
			nextSettings.VisionDelegation.TargetModelID = nil
		} else if target := byModelID[*targetModelID]; target == nil {
			nextSettings.VisionDelegation.Enabled = false
			nextSettings.VisionDelegation.TargetModelID = nil
		} else if target.ModelID == source.ModelID ||
			target.Status != model.StatusEnabled ||
			target.Type != model.TypeChat ||
			target.ModelCard == nil ||
			!target.ModelCard.SupportsVision() ||
			(target.Settings != nil && target.Settings.VisionDelegation.Enabled) {
			nextSettings.VisionDelegation.Enabled = false
		} else {
			continue
		}

		if _, err := db.Model.UpdateOneID(source.ID).SetSettings(&nextSettings).Save(ctx); err != nil {
			return fmt.Errorf("failed to repair restored vision delegation for model %q: %w", source.ModelID, err)
		}
	}

	return nil
}
