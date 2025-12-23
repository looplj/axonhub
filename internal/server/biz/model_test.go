package biz

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func TestModelService_QueryModelChannelConnections(t *testing.T) {
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

	// Create test channels
	channel1, err := client.Channel.Create().
		SetType("openai").
		SetName("OpenAI Channel").
		SetStatus("enabled").
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	channel2, err := client.Channel.Create().
		SetType("anthropic").
		SetName("Anthropic Channel").
		SetStatus("enabled").
		SetSupportedModels([]string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}).
		SetDefaultTestModel("claude-3-opus").
		Save(ctx)
	require.NoError(t, err)

	channel3, err := client.Channel.Create().
		SetType("gemini").
		SetName("Gemini Channel").
		SetStatus("enabled").
		SetSupportedModels([]string{"gemini-pro", "gemini-1.5-pro", "gemini-1.5-flash"}).
		SetDefaultTestModel("gemini-pro").
		Save(ctx)
	require.NoError(t, err)

	t.Run("empty associations", func(t *testing.T) {
		result, err := svc.QueryModelChannelConnections(ctx, []*objects.ModelAssociation{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("channel_model association", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: 1,
					ModelID:   "gpt-4",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, channel1.ID, result[0].Channel.ID)
		assert.Equal(t, []string{"gpt-4"}, result[0].ModelIds)
	})

	t.Run("channel_model association with non-existent model", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: 1,
					ModelID:   "non-existent-model",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("channel_regex association", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_regex",
				ChannelRegex: &objects.ChannelRegexAssociation{
					ChannelID: 1,
					Pattern:   "^gpt-4.*",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, channel1.ID, result[0].Channel.ID)
		assert.ElementsMatch(t, []string{"gpt-4", "gpt-4-turbo"}, result[0].ModelIds)
	})

	t.Run("regex association matches all channels", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "regex",
				Regex: &objects.RegexAssociation{
					Pattern: ".*pro$",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 1)

		// Only channel3 has models matching the pattern
		assert.Equal(t, channel3.ID, result[0].Channel.ID)
		assert.ElementsMatch(t, []string{"gemini-pro", "gemini-1.5-pro"}, result[0].ModelIds)
	})

	t.Run("multiple associations preserves order", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channel1.ID,
					ModelID:   "gpt-4",
				},
			},
			{
				Type: "channel_regex",
				ChannelRegex: &objects.ChannelRegexAssociation{
					ChannelID: channel2.ID,
					Pattern:   "^claude-3-.*",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 2)

		// Verify order matches associations order
		assert.Equal(t, channel1.ID, result[0].Channel.ID)
		assert.Equal(t, []string{"gpt-4"}, result[0].ModelIds)

		assert.Equal(t, channel2.ID, result[1].Channel.ID)
		assert.ElementsMatch(t, []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}, result[1].ModelIds)
	})

	t.Run("multiple associations reverse order", func(t *testing.T) {
		// Test with reversed order to verify order preservation
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_regex",
				ChannelRegex: &objects.ChannelRegexAssociation{
					ChannelID: channel2.ID,
					Pattern:   "^claude-3-.*",
				},
			},
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channel1.ID,
					ModelID:   "gpt-4",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 2)

		// Verify order matches associations order (channel2 first, then channel1)
		assert.Equal(t, channel2.ID, result[0].Channel.ID)
		assert.ElementsMatch(t, []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}, result[0].ModelIds)

		assert.Equal(t, channel1.ID, result[1].Channel.ID)
		assert.Equal(t, []string{"gpt-4"}, result[1].ModelIds)
	})

	t.Run("invalid regex pattern", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_regex",
				ChannelRegex: &objects.ChannelRegexAssociation{
					ChannelID: 1,
					Pattern:   "[invalid",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("disabled channel is included", func(t *testing.T) {
		// Create a disabled channel
		disabledChannel, err := client.Channel.Create().
			SetType("openai").
			SetName("Disabled Channel").
			SetSupportedModels([]string{"gpt-4-disabled"}).
			SetDefaultTestModel("gpt-4-disabled").
			SetStatus("disabled").
			Save(ctx)
		require.NoError(t, err)

		associations := []*objects.ModelAssociation{
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: disabledChannel.ID,
					ModelID:   "gpt-4-disabled",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, disabledChannel.ID, result[0].Channel.ID)
		assert.Equal(t, []string{"gpt-4-disabled"}, result[0].ModelIds)
	})

	t.Run("regex matches models across multiple channels", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "regex",
				Regex: &objects.RegexAssociation{
					Pattern: ".*-3-.*",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 1)

		// Only channel2 (anthropic) has models matching the pattern
		assert.Equal(t, channel2.ID, result[0].Channel.ID)
		assert.ElementsMatch(t, []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}, result[0].ModelIds)
	})

	t.Run("channel_regex with specific channel", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_regex",
				ChannelRegex: &objects.ChannelRegexAssociation{
					ChannelID: 3,
					Pattern:   "gemini-1\\.5-.*",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, channel3.ID, result[0].Channel.ID)
		assert.ElementsMatch(t, []string{"gemini-1.5-pro", "gemini-1.5-flash"}, result[0].ModelIds)
	})

	t.Run("mixed associations produce separate connections", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channel1.ID,
					ModelID:   "gpt-4",
				},
			},
			{
				Type: "channel_regex",
				ChannelRegex: &objects.ChannelRegexAssociation{
					ChannelID: channel1.ID,
					Pattern:   "^gpt-4$",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, channel1.ID, result[0].Channel.ID)
		assert.Equal(t, []string{"gpt-4"}, result[0].ModelIds)
		assert.Equal(t, channel1.ID, result[1].Channel.ID)
		assert.Equal(t, []string{"gpt-4"}, result[1].ModelIds)
	})

	t.Run("duplicate channel associations preserve order", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channel1.ID,
					ModelID:   "gpt-4",
				},
			},
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channel2.ID,
					ModelID:   "claude-3-opus",
				},
			},
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channel1.ID,
					ModelID:   "gpt-3.5-turbo",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 3)

		// Channel order follows association order
		assert.Equal(t, channel1.ID, result[0].Channel.ID)
		assert.Equal(t, []string{"gpt-4"}, result[0].ModelIds)

		assert.Equal(t, channel2.ID, result[1].Channel.ID)
		assert.Equal(t, []string{"claude-3-opus"}, result[1].ModelIds)

		assert.Equal(t, channel1.ID, result[2].Channel.ID)
		assert.Equal(t, []string{"gpt-3.5-turbo"}, result[2].ModelIds)
	})

	t.Run("model associations produce separate connections in order", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channel1.ID,
					ModelID:   "gpt-3.5-turbo",
				},
			},
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channel1.ID,
					ModelID:   "gpt-4-turbo",
				},
			},
			{
				Type: "channel_model",
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: channel1.ID,
					ModelID:   "gpt-4",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 3)
		assert.Equal(t, channel1.ID, result[0].Channel.ID)
		// Model connections follow association order
		assert.Equal(t, []string{"gpt-3.5-turbo"}, result[0].ModelIds)
		assert.Equal(t, channel1.ID, result[1].Channel.ID)
		assert.Equal(t, []string{"gpt-4-turbo"}, result[1].ModelIds)
		assert.Equal(t, channel1.ID, result[2].Channel.ID)
		assert.Equal(t, []string{"gpt-4"}, result[2].ModelIds)
	})

	t.Run("model association finds all channels supporting model", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "model",
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, channel1.ID, result[0].Channel.ID)
		assert.Equal(t, []string{"gpt-4"}, result[0].ModelIds)
	})

	t.Run("model association with non-existent model", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "model",
				ModelID: &objects.ModelIDAssociation{
					ModelID: "non-existent-model",
				},
			},
		}

		result, err := svc.QueryModelChannelConnections(ctx, associations)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}
