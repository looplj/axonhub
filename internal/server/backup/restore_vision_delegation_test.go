package backup

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

func TestRepairRestoredVisionDelegations(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	createRestoredVisionModel(t, ctx, client, "valid-target", model.StatusEnabled, model.TypeChat, true, &objects.ModelSettings{})
	createRestoredVisionModel(t, ctx, client, "disabled-target", model.StatusDisabled, model.TypeChat, true, &objects.ModelSettings{})
	createRestoredVisionModel(t, ctx, client, "text-target", model.StatusEnabled, model.TypeChat, false, &objects.ModelSettings{})
	createRestoredVisionModel(t, ctx, client, "chained-target", model.StatusEnabled, model.TypeChat, true, restoredVisionDelegation("valid-target"))

	createRestoredVisionModel(t, ctx, client, "valid-source", model.StatusEnabled, model.TypeChat, false, restoredVisionDelegation("valid-target"))
	createRestoredVisionModel(t, ctx, client, "missing-source", model.StatusEnabled, model.TypeChat, false, restoredVisionDelegation("missing-target"))
	createRestoredVisionModel(t, ctx, client, "disabled-source", model.StatusEnabled, model.TypeChat, false, restoredVisionDelegation("disabled-target"))
	createRestoredVisionModel(t, ctx, client, "text-source", model.StatusEnabled, model.TypeChat, false, restoredVisionDelegation("text-target"))
	createRestoredVisionModel(t, ctx, client, "chained-source", model.StatusEnabled, model.TypeChat, false, restoredVisionDelegation("chained-target"))

	require.NoError(t, repairRestoredVisionDelegations(ctx, client))

	requireRestoredVisionDelegation(t, ctx, client, "valid-source", true, lo.ToPtr("valid-target"))
	requireRestoredVisionDelegation(t, ctx, client, "missing-source", false, nil)
	requireRestoredVisionDelegation(t, ctx, client, "disabled-source", false, lo.ToPtr("disabled-target"))
	requireRestoredVisionDelegation(t, ctx, client, "text-source", false, lo.ToPtr("text-target"))
	requireRestoredVisionDelegation(t, ctx, client, "chained-source", false, lo.ToPtr("chained-target"))
}

func createRestoredVisionModel(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	modelID string,
	status model.Status,
	modelType model.Type,
	vision bool,
	settings *objects.ModelSettings,
) {
	t.Helper()

	_, err := client.Model.Create().
		SetDeveloper("test").
		SetModelID(modelID).
		SetType(modelType).
		SetName(modelID).
		SetIcon("test").
		SetGroup("test").
		SetStatus(status).
		SetModelCard(&objects.ModelCard{Vision: vision}).
		SetSettings(settings).
		Save(ctx)
	require.NoError(t, err)
}

func restoredVisionDelegation(targetModelID string) *objects.ModelSettings {
	return &objects.ModelSettings{VisionDelegation: objects.VisionDelegation{
		Enabled:       true,
		TargetModelID: lo.ToPtr(targetModelID),
	}}
}

func requireRestoredVisionDelegation(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	modelID string,
	enabled bool,
	targetModelID *string,
) {
	t.Helper()

	restored, err := client.Model.Query().Where(model.ModelID(modelID)).Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, restored.Settings)
	require.Equal(t, enabled, restored.Settings.VisionDelegation.Enabled)
	require.Equal(t, targetModelID, restored.Settings.VisionDelegation.TargetModelID)
}
