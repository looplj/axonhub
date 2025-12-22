package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
)

// GetModelByModelID retrieves a model by its modelId and status.
func (svc *ModelService) GetModelByModelID(ctx context.Context, modelID string, status model.Status) (*ent.Model, error) {
	return svc.entFromContext(ctx).Model.Query().
		Where(
			model.ModelID(modelID),
			model.StatusEQ(status),
		).
		First(ctx)
}
