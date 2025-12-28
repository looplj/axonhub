package biz

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func TestModelService_ValidateModelSettings(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	svc := &ModelService{
		AbstractService: &AbstractService{
			db: client,
		},
	}

	t.Run("valid regex patterns", func(t *testing.T) {
		settings := &objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type: "channel_regex",
					ChannelRegex: &objects.ChannelRegexAssociation{
						ChannelID: 1,
						Pattern:   "gpt-.*",
					},
				},
				{
					Type: "regex",
					Regex: &objects.RegexAssociation{
						Pattern: "claude-.*",
						Exclude: []*objects.ExcludeAssociation{
							{
								ChannelNamePattern: ".*backup",
							},
						},
					},
				},
				{
					Type: "model",
					ModelID: &objects.ModelIDAssociation{
						ModelID: "test-model",
						Exclude: []*objects.ExcludeAssociation{
							{
								ChannelTags: []string{"test"},
							},
						},
					},
				},
			},
		}

		err := svc.validateModelSettings(settings)
		require.NoError(t, err)
	})

	t.Run("invalid regex pattern in channel_regex", func(t *testing.T) {
		settings := &objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type: "channel_regex",
					ChannelRegex: &objects.ChannelRegexAssociation{
						ChannelID: 1,
						Pattern:   "[invalid", // invalid regex
					},
				},
			},
		}

		err := svc.validateModelSettings(settings)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid regex pattern in channel_regex association")
	})

	t.Run("invalid regex pattern in regex association", func(t *testing.T) {
		settings := &objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type: "regex",
					Regex: &objects.RegexAssociation{
						Pattern: "(?P<invalid", // invalid regex
					},
				},
			},
		}

		err := svc.validateModelSettings(settings)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid regex pattern in regex association")
	})

	t.Run("invalid regex pattern in exclude rule", func(t *testing.T) {
		settings := &objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type: "regex",
					Regex: &objects.RegexAssociation{
						Pattern: ".*",
						Exclude: []*objects.ExcludeAssociation{
							{
								ChannelNamePattern: "[invalid", // invalid regex
							},
						},
					},
				},
			},
		}

		err := svc.validateModelSettings(settings)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid regex pattern in exclude rule")
	})

	t.Run("nil settings should pass", func(t *testing.T) {
		err := svc.validateModelSettings(nil)
		require.NoError(t, err)
	})

	t.Run("empty associations should pass", func(t *testing.T) {
		settings := &objects.ModelSettings{
			Associations: []*objects.ModelAssociation{},
		}

		err := svc.validateModelSettings(settings)
		require.NoError(t, err)
	})

	t.Run("empty patterns should pass", func(t *testing.T) {
		settings := &objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type: "channel_regex",
					ChannelRegex: &objects.ChannelRegexAssociation{
						ChannelID: 1,
						Pattern:   "", // empty pattern
					},
				},
				{
					Type: "regex",
					Regex: &objects.RegexAssociation{
						Pattern: "", // empty pattern
					},
				},
			},
		}

		err := svc.validateModelSettings(settings)
		require.NoError(t, err)
	})
}

func TestModelService_CreateModel_WithRegexValidation(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	svc := &ModelService{
		AbstractService: &AbstractService{
			db: client,
		},
	}

	t.Run("create model with valid regex patterns", func(t *testing.T) {
		input := ent.CreateModelInput{
			Developer: "test-dev",
			ModelID:   "test-model",
			Type:      lo.ToPtr(model.TypeChat),
			Name:      "Test Model",
			Group:     "test-group",
			Settings: &objects.ModelSettings{
				Associations: []*objects.ModelAssociation{
					{
						Type: "regex",
						Regex: &objects.RegexAssociation{
							Pattern: "gpt-.*",
						},
					},
				},
			},
		}

		model, err := svc.CreateModel(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, model)
		require.Equal(t, "test-model", model.ModelID)
	})

	t.Run("create model with invalid regex patterns should fail", func(t *testing.T) {
		input := ent.CreateModelInput{
			Developer: "test-dev",
			ModelID:   "invalid-model",
			Type:      lo.ToPtr(model.TypeChat),
			Name:      "Invalid Model",
			Group:     "test-group",
			Settings: &objects.ModelSettings{
				Associations: []*objects.ModelAssociation{
					{
						Type: "regex",
						Regex: &objects.RegexAssociation{
							Pattern: "[invalid", // invalid regex
						},
					},
				},
			},
		}

		model, err := svc.CreateModel(ctx, input)
		require.Error(t, err)
		require.Nil(t, model)
		require.Contains(t, err.Error(), "invalid regex pattern")
	})
}

func TestModelService_UpdateModel_WithRegexValidation(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	svc := &ModelService{
		AbstractService: &AbstractService{
			db: client,
		},
	}

	// Create a model first
	model, err := svc.CreateModel(ctx, ent.CreateModelInput{
		Developer: "test-dev",
		ModelID:   "test-model",
		Type:      lo.ToPtr(model.TypeChat),
		Name:      "Test Model",
		Group:     "test-group",
	})
	require.NoError(t, err)

	t.Run("update model with valid regex patterns", func(t *testing.T) {
		input := &ent.UpdateModelInput{
			Settings: &objects.ModelSettings{
				Associations: []*objects.ModelAssociation{
					{
						Type: "regex",
						Regex: &objects.RegexAssociation{
							Pattern: "claude-.*",
						},
					},
				},
			},
		}

		updatedModel, err := svc.UpdateModel(ctx, model.ID, input)
		require.NoError(t, err)
		require.NotNil(t, updatedModel)
		require.NotNil(t, updatedModel.Settings)
		require.Len(t, updatedModel.Settings.Associations, 1)
	})

	t.Run("update model with invalid regex patterns should fail", func(t *testing.T) {
		input := &ent.UpdateModelInput{
			Settings: &objects.ModelSettings{
				Associations: []*objects.ModelAssociation{
					{
						Type: "regex",
						Regex: &objects.RegexAssociation{
							Pattern: "(?P<invalid", // invalid regex
						},
					},
				},
			},
		}

		updatedModel, err := svc.UpdateModel(ctx, model.ID, input)
		require.Error(t, err)
		require.Nil(t, updatedModel)
		require.Contains(t, err.Error(), "invalid regex pattern")
	})
}
