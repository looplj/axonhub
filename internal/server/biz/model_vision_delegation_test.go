package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

func TestModelServiceVisionDelegationCandidatesAndValidation(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	ch := createVisionDelegationTestChannel(t, ctx, client, "vision-model", "chained-model")
	svc := &ModelService{AbstractService: &AbstractService{db: client}}

	visionTarget := createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "vision-model", model.StatusEnabled,
		&objects.ModelCard{Vision: true, Modalities: objects.ModelCardModalities{Input: []string{"text", "image"}}},
		&objects.ModelSettings{},
	)
	textSource := createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "text-model", model.StatusEnabled,
		&objects.ModelCard{Modalities: objects.ModelCardModalities{Input: []string{"text"}}},
		visionDelegationTestSettings(ch.ID, "text-model", visionTarget.ModelID, true),
	)
	createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "disabled-vision", model.StatusDisabled,
		&objects.ModelCard{Vision: true}, &objects.ModelSettings{},
	)
	createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "unroutable-vision", model.StatusEnabled,
		&objects.ModelCard{Vision: true}, &objects.ModelSettings{},
	)
	createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "chained-model", model.StatusEnabled,
		&objects.ModelCard{Vision: true},
		visionDelegationTestSettings(ch.ID, "chained-model", "vision-model", true),
	)

	candidates, err := svc.ListVisionDelegationCandidates(ctx, "source-model")
	require.NoError(t, err)
	require.Equal(t, []string{"vision-model"}, lo.Map(candidates, func(candidate *ent.Model, _ int) string {
		return candidate.ModelID
	}))

	settings := visionDelegationTestSettings(ch.ID, "source-model", visionTarget.ModelID, true)
	require.NoError(t, svc.validateVisionDelegation(ctx, "source-model", model.TypeChat, &objects.ModelCard{}, settings))
	require.Equal(t, "vision-model", lo.FromPtr(settings.VisionDelegation.TargetModelID))

	require.ErrorContains(t, svc.validateVisionDelegation(
		ctx, "vision-model", model.TypeChat, &objects.ModelCard{},
		visionDelegationTestSettings(ch.ID, "vision-model", "vision-model", true),
	), "must differ")
	require.ErrorContains(t, svc.validateVisionDelegation(
		ctx, "source-model", model.TypeEmbedding, &objects.ModelCard{}, settings,
	), "only available for chat")
	require.ErrorContains(t, svc.validateVisionDelegation(
		ctx, "source-model", model.TypeChat, &objects.ModelCard{Vision: true}, settings,
	), "already supports image")
	require.ErrorContains(t, svc.validateVisionDelegation(
		ctx, "source-model", model.TypeChat, &objects.ModelCard{},
		visionDelegationTestSettings(ch.ID, "source-model", "chained-model", true),
	), "delegates vision itself")

	effective := svc.EffectiveModelCard(ctx, textSource)
	require.NotNil(t, effective)
	require.True(t, effective.SupportsVision())
	require.Equal(t, []string{"text", "image"}, effective.Modalities.Input)
	require.False(t, textSource.ModelCard.SupportsVision())
	require.Equal(t, []string{"text"}, textSource.ModelCard.Modalities.Input)

	_, err = client.Model.UpdateOneID(visionTarget.ID).SetStatus(model.StatusDisabled).Save(ctx)
	require.NoError(t, err)
	effective = svc.EffectiveModelCard(ctx, textSource)
	require.NotNil(t, effective)
	require.False(t, effective.SupportsVision())
	require.Equal(t, []string{"text"}, effective.Modalities.Input)
}

func TestModelServiceVisionDelegationLifecycle(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	ch := createVisionDelegationTestChannel(t, ctx, client, "vision-model", "vision-model-2")
	svc := &ModelService{AbstractService: &AbstractService{db: client}}
	target := createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "vision-model", model.StatusEnabled,
		&objects.ModelCard{Vision: true}, &objects.ModelSettings{},
	)
	sources := []*ent.Model{
		createVisionDelegationTestModel(t, ctx, client, ch.ID, "source-a", model.StatusEnabled, &objects.ModelCard{}, &objects.ModelSettings{}),
		createVisionDelegationTestModel(t, ctx, client, ch.ID, "source-b", model.StatusEnabled, &objects.ModelCard{}, &objects.ModelSettings{}),
	}

	for _, source := range sources {
		_, err := svc.UpdateModel(ctx, source.ID, &ent.UpdateModelInput{
			Settings: visionDelegationTestSettings(ch.ID, source.ModelID, target.ModelID, true),
		})
		require.NoError(t, err)
	}

	updatedTarget, err := svc.UpdateModel(ctx, target.ID, &ent.UpdateModelInput{ModelID: lo.ToPtr("vision-model-2")})
	require.NoError(t, err)
	require.Equal(t, "vision-model-2", updatedTarget.ModelID)
	requireVisionDelegationReferences(t, ctx, client, sources, true, lo.ToPtr("vision-model-2"))

	_, err = svc.UpdateModelStatus(ctx, target.ID, model.StatusDisabled)
	require.NoError(t, err)
	requireVisionDelegationReferences(t, ctx, client, sources, false, lo.ToPtr("vision-model-2"))

	_, err = svc.UpdateModelStatus(ctx, target.ID, model.StatusEnabled)
	require.NoError(t, err)
	requireVisionDelegationReferences(t, ctx, client, sources, false, lo.ToPtr("vision-model-2"))

	for _, source := range sources {
		settings := visionDelegationTestSettings(ch.ID, source.ModelID, "vision-model-2", true)
		_, err := client.Model.UpdateOneID(source.ID).SetSettings(settings).Save(ctx)
		require.NoError(t, err)
	}
	require.NoError(t, svc.DeleteModel(ctx, target.ID))
	requireVisionDelegationReferences(t, ctx, client, sources, false, nil)
}

func TestModelServiceUpdateModelDisablesDelegationWhenSourceBecomesNativeVision(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	ch := createVisionDelegationTestChannel(t, ctx, client, "vision-model", "source-model")
	svc := &ModelService{AbstractService: &AbstractService{db: client}}
	target := createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "vision-model", model.StatusEnabled,
		&objects.ModelCard{Vision: true}, &objects.ModelSettings{},
	)
	source := createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "source-model", model.StatusEnabled,
		&objects.ModelCard{Modalities: objects.ModelCardModalities{Input: []string{"text"}}},
		visionDelegationTestSettings(ch.ID, "source-model", target.ModelID, true),
	)
	nativeVisionCard := &objects.ModelCard{
		Vision:     true,
		Modalities: objects.ModelCardModalities{Input: []string{"text", "image"}},
	}

	updated, err := svc.UpdateModel(ctx, source.ID, &ent.UpdateModelInput{ModelCard: nativeVisionCard})
	require.NoError(t, err)
	require.True(t, updated.ModelCard.SupportsVision())
	require.False(t, updated.Settings.VisionDelegation.Enabled)
	require.Equal(t, target.ModelID, lo.FromPtr(updated.Settings.VisionDelegation.TargetModelID))

	_, err = svc.UpdateModel(ctx, source.ID, &ent.UpdateModelInput{
		ModelCard: nativeVisionCard,
		Settings:  visionDelegationTestSettings(ch.ID, source.ModelID, target.ModelID, true),
	})
	require.ErrorContains(t, err, "already supports image input natively")
}

func TestModelServiceVisionDelegationBulkLifecycle(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	ch := createVisionDelegationTestChannel(t, ctx, client, "vision-model")
	svc := &ModelService{AbstractService: &AbstractService{db: client}}
	target := createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "vision-model", model.StatusEnabled,
		&objects.ModelCard{Vision: true}, &objects.ModelSettings{},
	)
	source := createVisionDelegationTestModel(
		t, ctx, client, ch.ID, "source", model.StatusEnabled, &objects.ModelCard{},
		visionDelegationTestSettings(ch.ID, "source", target.ModelID, true),
	)

	require.NoError(t, svc.BulkArchiveModels(ctx, []int{target.ID}))
	requireVisionDelegationReferences(t, ctx, client, []*ent.Model{source}, false, lo.ToPtr(target.ModelID))

	_, err := client.Model.UpdateOneID(source.ID).
		SetSettings(visionDelegationTestSettings(ch.ID, source.ModelID, target.ModelID, true)).
		Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.BulkDeleteModels(ctx, []int{target.ID}))
	requireVisionDelegationReferences(t, ctx, client, []*ent.Model{source}, false, nil)
}

func createVisionDelegationTestChannel(t *testing.T, ctx context.Context, client *ent.Client, modelIDs ...string) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Vision Delegation Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels(modelIDs).
		SetDefaultTestModel(modelIDs[0]).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

func createVisionDelegationTestModel(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	channelID int,
	modelID string,
	status model.Status,
	card *objects.ModelCard,
	settings *objects.ModelSettings,
) *ent.Model {
	t.Helper()

	if settings != nil && len(settings.Associations) == 0 && modelID != "unroutable-vision" && status == model.StatusEnabled {
		settings.Associations = visionDelegationTestSettings(channelID, modelID, "", false).Associations
	}
	created, err := client.Model.Create().
		SetDeveloper("test").
		SetModelID(modelID).
		SetType(model.TypeChat).
		SetName(modelID).
		SetIcon("test").
		SetGroup("test").
		SetModelCard(card).
		SetSettings(settings).
		SetStatus(status).
		Save(ctx)
	require.NoError(t, err)

	return created
}

func visionDelegationTestSettings(channelID int, modelID, targetModelID string, enabled bool) *objects.ModelSettings {
	settings := &objects.ModelSettings{
		Associations: []*objects.ModelAssociation{
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channelID,
					ModelID:   modelID,
				},
			},
		},
		VisionDelegation: objects.VisionDelegation{Enabled: enabled},
	}
	if targetModelID != "" {
		settings.VisionDelegation.TargetModelID = lo.ToPtr(targetModelID)
	}

	return settings
}

func requireVisionDelegationReferences(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	sources []*ent.Model,
	enabled bool,
	targetModelID *string,
) {
	t.Helper()

	for _, source := range sources {
		refreshed, err := client.Model.Get(ctx, source.ID)
		require.NoError(t, err)
		require.Equal(t, enabled, refreshed.Settings.VisionDelegation.Enabled)
		require.Equal(t, targetModelID, refreshed.Settings.VisionDelegation.TargetModelID)
	}
}
