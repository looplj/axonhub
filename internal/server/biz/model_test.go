package biz

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
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
		require.Empty(t, result)
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
		require.Equal(t, channel1.ID, result[0].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"}}, result[0].Models)
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
		require.Empty(t, result)
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
		require.Equal(t, channel1.ID, result[0].Channel.ID)
		require.ElementsMatch(t, []ChannelModelEntry{
			{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"},
			{RequestModel: "gpt-4-turbo", ActualModel: "gpt-4-turbo", Source: "direct"},
		}, result[0].Models)
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
		require.Equal(t, channel3.ID, result[0].Channel.ID)
		require.ElementsMatch(t, []ChannelModelEntry{
			{RequestModel: "gemini-pro", ActualModel: "gemini-pro", Source: "direct"},
			{RequestModel: "gemini-1.5-pro", ActualModel: "gemini-1.5-pro", Source: "direct"},
		}, result[0].Models)
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
		require.Equal(t, channel1.ID, result[0].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"}}, result[0].Models)

		require.Equal(t, channel2.ID, result[1].Channel.ID)
		require.ElementsMatch(t, []ChannelModelEntry{
			{RequestModel: "claude-3-opus", ActualModel: "claude-3-opus", Source: "direct"},
			{RequestModel: "claude-3-sonnet", ActualModel: "claude-3-sonnet", Source: "direct"},
			{RequestModel: "claude-3-haiku", ActualModel: "claude-3-haiku", Source: "direct"},
		}, result[1].Models)
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
		require.Equal(t, channel2.ID, result[0].Channel.ID)
		require.ElementsMatch(t, []ChannelModelEntry{
			{RequestModel: "claude-3-opus", ActualModel: "claude-3-opus", Source: "direct"},
			{RequestModel: "claude-3-sonnet", ActualModel: "claude-3-sonnet", Source: "direct"},
			{RequestModel: "claude-3-haiku", ActualModel: "claude-3-haiku", Source: "direct"},
		}, result[0].Models)

		require.Equal(t, channel1.ID, result[1].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"}}, result[1].Models)
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
		require.Empty(t, result)
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
		require.Equal(t, disabledChannel.ID, result[0].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-4-disabled", ActualModel: "gpt-4-disabled", Source: "direct"}}, result[0].Models)
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
		require.Equal(t, channel2.ID, result[0].Channel.ID)
		require.ElementsMatch(t, []ChannelModelEntry{
			{RequestModel: "claude-3-opus", ActualModel: "claude-3-opus", Source: "direct"},
			{RequestModel: "claude-3-sonnet", ActualModel: "claude-3-sonnet", Source: "direct"},
			{RequestModel: "claude-3-haiku", ActualModel: "claude-3-haiku", Source: "direct"},
		}, result[0].Models)
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
		require.Equal(t, channel3.ID, result[0].Channel.ID)
		require.ElementsMatch(t, []ChannelModelEntry{
			{RequestModel: "gemini-1.5-pro", ActualModel: "gemini-1.5-pro", Source: "direct"},
			{RequestModel: "gemini-1.5-flash", ActualModel: "gemini-1.5-flash", Source: "direct"},
		}, result[0].Models)
	})

	t.Run("mixed associations with global deduplication", func(t *testing.T) {
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
		// Global deduplication: same (channel, model) only appears once
		require.Len(t, result, 1)
		require.Equal(t, channel1.ID, result[0].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"}}, result[0].Models)
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
		require.Equal(t, channel1.ID, result[0].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"}}, result[0].Models)

		require.Equal(t, channel2.ID, result[1].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "claude-3-opus", ActualModel: "claude-3-opus", Source: "direct"}}, result[1].Models)

		require.Equal(t, channel1.ID, result[2].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-3.5-turbo", ActualModel: "gpt-3.5-turbo", Source: "direct"}}, result[2].Models)
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
		require.Equal(t, channel1.ID, result[0].Channel.ID)
		// Model connections follow association order
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-3.5-turbo", ActualModel: "gpt-3.5-turbo", Source: "direct"}}, result[0].Models)
		require.Equal(t, channel1.ID, result[1].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-4-turbo", ActualModel: "gpt-4-turbo", Source: "direct"}}, result[1].Models)
		require.Equal(t, channel1.ID, result[2].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"}}, result[2].Models)
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
		require.Equal(t, channel1.ID, result[0].Channel.ID)
		require.Equal(t, []ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"}}, result[0].Models)
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
		require.Empty(t, result)
	})
}

func TestModelService_ListEnabledModels(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create channels with different configurations
	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("OpenAI Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Anthropic Channel").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"claude-3-opus-20240229"}).
		SetDefaultTestModel("claude-3-opus-20240229").
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{
			ModelMappings: []objects.ModelMapping{
				{From: "claude-3-opus", To: "claude-3-opus-20240229"},
				{From: "claude-opus", To: "claude-3-opus-20240229"},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Prefix Channel").
		SetBaseURL("https://api.deepseek.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key3"}).
		SetSupportedModels([]string{"deepseek-chat", "deepseek-reasoner"}).
		SetDefaultTestModel("deepseek-chat").
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{
			ExtraModelPrefix: "deepseek",
		}).
		Save(ctx)
	require.NoError(t, err)

	// Create disabled channel (should not be included)
	_, err = client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Disabled Channel").
		SetBaseURL("https://api.disabled.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key4"}).
		SetSupportedModels([]string{"gpt-4-disabled"}).
		SetDefaultTestModel("gpt-4-disabled").
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	// Create channel service for testing
	channelSvc := NewChannelServiceForTest(client)
	enabledEntities, err := client.Channel.Query().
		Where(channel.StatusEQ(channel.StatusEnabled)).
		All(ctx)
	require.NoError(t, err)

	enabledChannels := make([]*Channel, 0, len(enabledEntities))
	for _, e := range enabledEntities {
		built, buildErr := channelSvc.buildChannel(e)
		require.NoError(t, buildErr)

		enabledChannels = append(enabledChannels, built)
	}

	channelSvc.SetEnabledChannelsForTest(enabledChannels)

	// Create model service with channel service dependency
	// SystemService with default settings (QueryAllChannelModels: true)
	systemSvc := &SystemService{
		AbstractService: &AbstractService{
			db: client,
		},
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	modelSvc := &ModelService{
		AbstractService: &AbstractService{
			db: client,
		},
		channelService: channelSvc,
		systemService:  systemSvc,
	}

	t.Run("list all enabled models from channels", func(t *testing.T) {
		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Convert to map for easier comparison (order doesn't matter)
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		// Should include models from enabled channels
		expectedModels := []string{
			"gpt-4", "gpt-3.5-turbo",
			"claude-3-opus-20240229", "claude-3-opus", "claude-opus",
			"deepseek-chat", "deepseek-reasoner",
			"deepseek/deepseek-chat", "deepseek/deepseek-reasoner",
		}

		expectedMap := make(map[string]bool)
		for _, model := range expectedModels {
			expectedMap[model] = true
		}

		require.Equal(t, expectedMap, resultMap, "Model lists should match")
		require.Len(t, result, len(expectedModels), "Should have same number of models")
	})

	t.Run("verify model properties", func(t *testing.T) {
		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		for _, model := range result {
			require.NotEmpty(t, model.ID, "Model ID should not be empty")
			require.NotEmpty(t, model.DisplayName, "Model DisplayName should not be empty")
			require.NotEmpty(t, model.OwnedBy, "Model OwnedBy should not be empty")
			require.Equal(t, model.ID, model.DisplayName, "Model ID and DisplayName should match")
		}
	})

	t.Run("verify model owned by channel type", func(t *testing.T) {
		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		for _, model := range result {
			switch model.ID {
			case "gpt-4", "gpt-3.5-turbo", "deepseek-chat", "deepseek-reasoner",
				"deepseek/deepseek-chat", "deepseek/deepseek-reasoner":
				require.Equal(t, "openai", model.OwnedBy, "Model %s should be owned by openai", model.ID)
			case "claude-3-opus-20240229", "claude-3-opus", "claude-opus":
				require.Equal(t, "anthropic", model.OwnedBy, "Model %s should be owned by anthropic", model.ID)
			}
		}
	})

	t.Run("disabled channel models not included", func(t *testing.T) {
		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.False(t, resultMap["gpt-4-disabled"], "Disabled channel model should not be included")
	})

	t.Run("mapping to unsupported model should be ignored", func(t *testing.T) {
		// Create channel with invalid mapping
		_, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName("Invalid Mapping Channel").
			SetBaseURL("https://api.example.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "key5"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetStatus(channel.StatusEnabled).
			SetSettings(&objects.ChannelSettings{
				ModelMappings: []objects.ModelMapping{
					{From: "gpt-4-latest", To: "gpt-4"},
					{From: "invalid-mapping", To: "unsupported-model"},
				},
			}).
			Save(ctx)
		require.NoError(t, err)

		enabledEntities, err := client.Channel.Query().
			Where(channel.StatusEQ(channel.StatusEnabled)).
			All(ctx)
		require.NoError(t, err)

		enabledChannels := make([]*Channel, 0, len(enabledEntities))
		for _, e := range enabledEntities {
			built, buildErr := channelSvc.buildChannel(e)
			require.NoError(t, buildErr)

			enabledChannels = append(enabledChannels, built)
		}

		channelSvc.SetEnabledChannelsForTest(enabledChannels)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "Valid model should be included")
		require.True(t, resultMap["gpt-4-latest"], "Valid mapping should be included")
		require.False(t, resultMap["invalid-mapping"], "Invalid mapping should not be included")
	})

	t.Run("auto-trimmed models", func(t *testing.T) {
		// Create channel with auto-trim prefix
		_, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName("Auto Trim Channel").
			SetBaseURL("https://api.example.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "key6"}).
			SetSupportedModels([]string{"provider/gpt-4", "provider/gpt-3.5-turbo"}).
			SetDefaultTestModel("provider/gpt-4").
			SetStatus(channel.StatusEnabled).
			SetSettings(&objects.ChannelSettings{
				AutoTrimedModelPrefixes: []string{"provider"},
			}).
			Save(ctx)
		require.NoError(t, err)

		enabledEntities, err := client.Channel.Query().
			Where(channel.StatusEQ(channel.StatusEnabled)).
			All(ctx)
		require.NoError(t, err)

		enabledChannels := make([]*Channel, 0, len(enabledEntities))
		for _, e := range enabledEntities {
			built, buildErr := channelSvc.buildChannel(e)
			require.NoError(t, buildErr)

			enabledChannels = append(enabledChannels, built)
		}

		channelSvc.SetEnabledChannelsForTest(enabledChannels)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "Auto-trimmed model should be included")
		require.True(t, resultMap["gpt-3.5-turbo"], "Auto-trimmed model should be included")
	})

	t.Run("API key with active profile modelIDs returns only specified models", func(t *testing.T) {
		// Create API key with active profile that restricts models
		apiKey := &ent.APIKey{
			ID:   1,
			Name: "test-api-key",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "production",
				Profiles: []objects.APIKeyProfile{
					{
						Name:     "production",
						ModelIDs: []string{"gpt-4", "claude-3-opus-20240229"},
					},
				},
			},
		}

		// Add API key to context
		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should only return models specified in the profile
		require.Len(t, result, 2, "Should only return 2 models from profile")

		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "gpt-4 should be in result")
		require.True(t, resultMap["claude-3-opus-20240229"], "claude-3-opus-20240229 should be in result")
		require.False(t, resultMap["gpt-3.5-turbo"], "gpt-3.5-turbo should not be in result")
		require.False(t, resultMap["deepseek-chat"], "deepseek-chat should not be in result")

		// Verify owned by is channel type (openai for gpt-4, anthropic for claude)
		for _, model := range result {
			if model.ID == "gpt-4" {
				require.Equal(t, "openai", model.OwnedBy)
			} else if model.ID == "claude-3-opus-20240229" {
				require.Equal(t, "anthropic", model.OwnedBy)
			}
		}
	})

	t.Run("API key with active profile but empty modelIDs returns all models", func(t *testing.T) {
		// Create API key with active profile but no modelIDs
		apiKey := &ent.APIKey{
			ID:   2,
			Name: "test-api-key-2",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "development",
				Profiles: []objects.APIKeyProfile{
					{
						Name:          "development",
						ModelIDs:      []string{},
						ModelMappings: []objects.ModelMapping{{From: "gpt-4", To: "gpt-4"}},
					},
				},
			},
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should return all models (not restricted)
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "gpt-4 should be in result")
		require.True(t, resultMap["gpt-3.5-turbo"], "gpt-3.5-turbo should be in result")
		require.True(t, resultMap["claude-3-opus-20240229"], "claude-3-opus-20240229 should be in result")
	})

	t.Run("API key without profiles returns all models", func(t *testing.T) {
		// Create API key without profiles
		apiKey := &ent.APIKey{
			ID:   3,
			Name: "test-api-key-3",
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should return all models
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "gpt-4 should be in result")
		require.True(t, resultMap["gpt-3.5-turbo"], "gpt-3.5-turbo should be in result")
	})

	t.Run("API key with nil profiles returns all models", func(t *testing.T) {
		// Create API key with nil profiles
		apiKey := &ent.APIKey{
			ID:       4,
			Name:     "test-api-key-4",
			Profiles: nil,
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should return all models
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "gpt-4 should be in result")
		require.True(t, resultMap["gpt-3.5-turbo"], "gpt-3.5-turbo should be in result")
	})

	t.Run("API key with empty active profile returns all models", func(t *testing.T) {
		// Create API key with empty active profile
		apiKey := &ent.APIKey{
			ID:   5,
			Name: "test-api-key-5",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "",
				Profiles: []objects.APIKeyProfile{
					{
						Name:     "production",
						ModelIDs: []string{"gpt-4"},
					},
				},
			},
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should return all models when active profile is empty
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "gpt-4 should be in result")
		require.True(t, resultMap["gpt-3.5-turbo"], "gpt-3.5-turbo should be in result")
	})

	t.Run("API key with non-existent active profile returns all models", func(t *testing.T) {
		// Create API key with active profile that doesn't exist
		apiKey := &ent.APIKey{
			ID:   6,
			Name: "test-api-key-6",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "non-existent",
				Profiles: []objects.APIKeyProfile{
					{
						Name:     "production",
						ModelIDs: []string{"gpt-4"},
					},
				},
			},
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should return all models when active profile doesn't exist
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "gpt-4 should be in result")
		require.True(t, resultMap["gpt-3.5-turbo"], "gpt-3.5-turbo should be in result")
	})

	t.Run("no API key in context returns all models", func(t *testing.T) {
		// Context without API key
		ctxNoAPIKey := context.Background()
		ctxNoAPIKey = ent.NewContext(ctxNoAPIKey, client)
		ctxNoAPIKey = privacy.DecisionContext(ctxNoAPIKey, privacy.Allow)

		result, err := modelSvc.ListEnabledModels(ctxNoAPIKey)
		require.NoError(t, err)

		// Should return all models
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "gpt-4 should be in result")
		require.True(t, resultMap["gpt-3.5-turbo"], "gpt-3.5-turbo should be in result")
		require.True(t, resultMap["claude-3-opus-20240229"], "claude-3-opus-20240229 should be in result")
	})

	t.Run("QueryAllChannelModels=false returns configured models only", func(t *testing.T) {
		// Create system setting with QueryAllChannelModels=false
		modelSettings := SystemModelSettings{
			QueryAllChannelModels: false,
		}
		err = systemSvc.SetModelSettings(ctx, modelSettings)
		require.NoError(t, err)

		// Create configured models with associations
		_, err = client.Model.Create().
			SetDeveloper("openai").
			SetModelID("gpt-4").
			SetName("GPT-4").
			SetType(model.TypeChat).
			SetGroup("gpt").
			SetIcon("icon").
			SetModelCard(&objects.ModelCard{}).
			SetSettings(&objects.ModelSettings{
				Associations: []*objects.ModelAssociation{
					{
						Type: "model",
						ModelID: &objects.ModelIDAssociation{
							ModelID: "gpt-4",
						},
					},
				},
			}).
			SetStatus(model.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)

		_, err = client.Model.Create().
			SetDeveloper("anthropic").
			SetModelID("claude-3-opus").
			SetName("Claude 3 Opus").
			SetType(model.TypeChat).
			SetGroup("claude").
			SetIcon("icon").
			SetModelCard(&objects.ModelCard{}).
			SetSettings(&objects.ModelSettings{
				Associations: []*objects.ModelAssociation{
					{
						Type: "model",
						ModelID: &objects.ModelIDAssociation{
							ModelID: "claude-3-opus-20240229",
						},
					},
				},
			}).
			SetStatus(model.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)

		// Create a model with nil settings (should be skipped)
		_, err = client.Model.Create().
			SetDeveloper("openai").
			SetModelID("gpt-3.5-turbo").
			SetName("GPT-3.5 Turbo").
			SetType(model.TypeChat).
			SetGroup("gpt").
			SetIcon("icon").
			SetModelCard(&objects.ModelCard{}).
			SetSettings(&objects.ModelSettings{}).
			SetStatus(model.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)

		// Create a model with empty associations (should be skipped)
		_, err = client.Model.Create().
			SetDeveloper("openai").
			SetModelID("gpt-4-turbo").
			SetName("GPT-4 Turbo").
			SetType(model.TypeChat).
			SetGroup("gpt").
			SetIcon("icon").
			SetModelCard(&objects.ModelCard{}).
			SetSettings(&objects.ModelSettings{
				Associations: []*objects.ModelAssociation{},
			}).
			SetStatus(model.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should only return models with valid associations
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "gpt-4 should be in result")
		require.True(t, resultMap["claude-3-opus"], "claude-3-opus should be in result")
		require.False(t, resultMap["gpt-3.5-turbo"], "gpt-3.5-turbo with nil settings should not be in result")
		require.False(t, resultMap["gpt-4-turbo"], "gpt-4-turbo with empty associations should not be in result")
		require.Equal(t, "configured", result[0].OwnedBy, "OwnedBy should be 'configured'")
	})

	t.Run("QueryAllChannelModels=false with profile modelIDs", func(t *testing.T) {
		// Create system setting with QueryAllChannelModels=false
		modelSettings := SystemModelSettings{
			QueryAllChannelModels: false,
		}
		err = systemSvc.SetModelSettings(ctx, modelSettings)
		require.NoError(t, err)

		// Get the first channel ID
		channels := channelSvc.GetEnabledChannels()
		require.Greater(t, len(channels), 0, "Should have at least one channel")

		firstChannelID := channels[0].ID

		// Create configured models with channel_model associations
		_, err = client.Model.Create().
			SetDeveloper("openai").
			SetModelID("gpt-4-configured").
			SetName("GPT-4 Configured").
			SetType(model.TypeChat).
			SetGroup("gpt").
			SetIcon("icon").
			SetModelCard(&objects.ModelCard{}).
			SetSettings(&objects.ModelSettings{
				Associations: []*objects.ModelAssociation{
					{
						Type: "channel_model",
						ChannelModel: &objects.ChannelModelAssociation{
							ChannelID: firstChannelID,
							ModelID:   "gpt-4",
						},
					},
				},
			}).
			SetStatus(model.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)

		_, err = client.Model.Create().
			SetDeveloper("openai").
			SetModelID("gpt-3.5-turbo-configured").
			SetName("GPT-3.5 Turbo Configured").
			SetType(model.TypeChat).
			SetGroup("gpt").
			SetIcon("icon").
			SetModelCard(&objects.ModelCard{}).
			SetSettings(&objects.ModelSettings{
				Associations: []*objects.ModelAssociation{
					{
						Type: "channel_model",
						ChannelModel: &objects.ChannelModelAssociation{
							ChannelID: firstChannelID,
							ModelID:   "gpt-3.5-turbo",
						},
					},
				},
			}).
			SetStatus(model.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)

		// Create API key with profile that restricts models
		apiKey := &ent.APIKey{
			ID:   10,
			Name: "test-api-key-10",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "production",
				Profiles: []objects.APIKeyProfile{
					{
						Name:     "production",
						ModelIDs: []string{"gpt-4-configured"},
					},
				},
			},
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should only return gpt-4-configured (filtered by profile)
		require.Len(t, result, 1, "Should only return 1 model")

		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4-configured"], "gpt-4-configured should be in result")
		require.False(t, resultMap["gpt-3.5-turbo-configured"], "gpt-3.5-turbo-configured should not be in result")
	})

	t.Run("API key with ChannelIDs filters channels", func(t *testing.T) {
		// Ensure QueryAllChannelModels is true (default)
		modelSettings := SystemModelSettings{
			QueryAllChannelModels: true,
		}
		err = systemSvc.SetModelSettings(ctx, modelSettings)
		require.NoError(t, err)

		// Get the first channel ID
		channels := channelSvc.GetEnabledChannels()
		require.Greater(t, len(channels), 0, "Should have at least one channel")

		firstChannelID := channels[0].ID

		// Create API key with profile that restricts channels
		apiKey := &ent.APIKey{
			ID:   11,
			Name: "test-api-key-11",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "production",
				Profiles: []objects.APIKeyProfile{
					{
						Name:       "production",
						ChannelIDs: []int{firstChannelID},
					},
				},
			},
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should only return models from the specified channel
		require.NotEmpty(t, result, "Should have models from the specified channel")

		// Verify all models are from the first channel
		for _, model := range result {
			require.Equal(t, channels[0].Channel.Type.String(), model.OwnedBy,
				"Model %s should be from the first channel", model.ID)
		}
	})

	t.Run("API key with ChannelTags filters channels", func(t *testing.T) {
		// Ensure QueryAllChannelModels is true (default)
		modelSettings := SystemModelSettings{
			QueryAllChannelModels: true,
		}
		err = systemSvc.SetModelSettings(ctx, modelSettings)
		require.NoError(t, err)

		// Create a channel with tags
		_, err = client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName("Tagged Channel").
			SetBaseURL("https://api.tagged.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "key-tagged"}).
			SetSupportedModels([]string{"tagged-model-1", "tagged-model-2"}).
			SetDefaultTestModel("tagged-model-1").
			SetStatus(channel.StatusEnabled).
			SetTags([]string{"production", "team-a"}).
			Save(ctx)
		require.NoError(t, err)

		enabledEntities, err := client.Channel.Query().
			Where(channel.StatusEQ(channel.StatusEnabled)).
			All(ctx)
		require.NoError(t, err)

		enabledChannels := make([]*Channel, 0, len(enabledEntities))
		for _, e := range enabledEntities {
			built, buildErr := channelSvc.buildChannel(e)
			require.NoError(t, buildErr)

			enabledChannels = append(enabledChannels, built)
		}

		channelSvc.SetEnabledChannelsForTest(enabledChannels)

		// Create API key with profile that filters by channel tags
		apiKey := &ent.APIKey{
			ID:   12,
			Name: "test-api-key-12",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "production",
				Profiles: []objects.APIKeyProfile{
					{
						Name:        "production",
						ChannelTags: []string{"production"},
					},
				},
			},
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should include models from tagged channels
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["tagged-model-1"], "tagged-model-1 should be in result")
		require.True(t, resultMap["tagged-model-2"], "tagged-model-2 should be in result")
	})

	t.Run("API key with both ChannelIDs and ChannelTags", func(t *testing.T) {
		// Get the first channel ID
		channels := channelSvc.GetEnabledChannels()
		require.Greater(t, len(channels), 0, "Should have at least one channel")

		firstChannelID := channels[0].ID

		// Create API key with both ChannelIDs and ChannelTags
		apiKey := &ent.APIKey{
			ID:   13,
			Name: "test-api-key-13",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "production",
				Profiles: []objects.APIKeyProfile{
					{
						Name:        "production",
						ChannelIDs:  []int{firstChannelID},
						ChannelTags: []string{"non-existent-tag"},
					},
				},
			},
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should return models from the channel with matching ID
		// (ChannelIDs filter is applied first, then ChannelTags filter)
		if len(result) > 0 {
			for _, model := range result {
				require.Equal(t, channels[0].Channel.Type.String(), model.OwnedBy,
					"Model %s should be from the first channel", model.ID)
			}
		}
	})

	t.Run("empty channels returns empty models", func(t *testing.T) {
		// Create a new model service with empty channel service
		emptyChannelSvc := NewChannelServiceForTest(client)

		emptyModelSvc := &ModelService{
			AbstractService: &AbstractService{
				db: client,
			},
			channelService: emptyChannelSvc,
			systemService:  systemSvc,
		}

		result, err := emptyModelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)
		require.Empty(t, result, "Should return empty models when no channels")
	})

	t.Run("QueryAllChannelModels=true with profile modelIDs filters models", func(t *testing.T) {
		// Ensure QueryAllChannelModels is true (default)
		modelSettings := SystemModelSettings{
			QueryAllChannelModels: true,
		}
		err = systemSvc.SetModelSettings(ctx, modelSettings)
		require.NoError(t, err)

		// Create API key with profile that restricts models
		apiKey := &ent.APIKey{
			ID:   14,
			Name: "test-api-key-14",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "production",
				Profiles: []objects.APIKeyProfile{
					{
						Name:     "production",
						ModelIDs: []string{"gpt-4", "claude-3-opus"},
					},
				},
			},
		}

		ctx := contexts.WithAPIKey(ctx, apiKey)

		result, err := modelSvc.ListEnabledModels(ctx)
		require.NoError(t, err)

		// Should only return models specified in profile
		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "gpt-4 should be in result")
		require.True(t, resultMap["claude-3-opus"], "claude-3-opus should be in result")
		require.False(t, resultMap["gpt-3.5-turbo"], "gpt-3.5-turbo should not be in result")
	})
}

func TestFindUnassociatedChannels(t *testing.T) {
	// Create test channels
	channel1 := &ent.Channel{
		ID:              1,
		Type:            "openai",
		Name:            "OpenAI Channel",
		Status:          "enabled",
		SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
	}

	channel2 := &ent.Channel{
		ID:              2,
		Type:            "anthropic",
		Name:            "Anthropic Channel",
		Status:          "enabled",
		SupportedModels: []string{"claude-3-opus", "claude-3-sonnet"},
	}

	channel3 := &ent.Channel{
		ID:              3,
		Type:            "gemini",
		Name:            "Gemini Channel",
		Status:          "disabled",
		SupportedModels: []string{"gemini-pro", "gemini-1.5-pro"},
	}

	channels := []*ent.Channel{channel1, channel2, channel3}

	t.Run("no associations - all channels unassociated", func(t *testing.T) {
		result := findUnassociatedChannels(channels, []*objects.ModelAssociation{})
		require.Len(t, result, 3)

		// Verify all channels have unassociated models
		for _, info := range result {
			require.NotEmpty(t, info.Models)
		}
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

		result := findUnassociatedChannels(channels, associations)

		// Find channel1 in results
		var channel1Info *UnassociatedChannel

		for _, info := range result {
			if info.Channel.ID == 1 {
				channel1Info = info
				break
			}
		}

		require.NotNil(t, channel1Info)
		// gpt-4 should be associated, so only gpt-3.5-turbo should be unassociated
		require.Contains(t, channel1Info.Models, "gpt-3.5-turbo")
		require.NotContains(t, channel1Info.Models, "gpt-4")
	})

	t.Run("regex association", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "regex",
				Regex: &objects.RegexAssociation{
					Pattern: "^claude-3-.*",
				},
			},
		}

		result := findUnassociatedChannels(channels, associations)

		// Find channel2 in results
		var channel2Info *UnassociatedChannel

		for _, info := range result {
			if info.Channel.ID == 2 {
				channel2Info = info
				break
			}
		}

		// claude-3-opus and claude-3-sonnet should be associated by regex
		if channel2Info != nil {
			require.NotContains(t, channel2Info.Models, "claude-3-opus")
			require.NotContains(t, channel2Info.Models, "claude-3-sonnet")
		}
	})

	t.Run("model association with exclude", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "model",
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gemini-pro",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelIds: []int{3},
						},
					},
				},
			},
		}

		result := findUnassociatedChannels(channels, associations)

		// Find channel3 in results
		var channel3Info *UnassociatedChannel

		for _, info := range result {
			if info.Channel.ID == 3 {
				channel3Info = info
				break
			}
		}

		require.NotNil(t, channel3Info)
		// gemini-pro should be unassociated in channel3 due to exclude
		require.Contains(t, channel3Info.Models, "gemini-pro")
	})

	t.Run("channel_regex association", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "channel_regex",
				ChannelRegex: &objects.ChannelRegexAssociation{
					ChannelID: 1,
					Pattern:   "^gpt-.*",
				},
			},
		}

		result := findUnassociatedChannels(channels, associations)

		// Find channel1 in results
		var channel1Info *UnassociatedChannel

		for _, info := range result {
			if info.Channel.ID == 1 {
				channel1Info = info
				break
			}
		}

		// Both gpt-4 and gpt-3.5-turbo should be associated by regex
		if channel1Info != nil {
			require.NotContains(t, channel1Info.Models, "gpt-4")
			require.NotContains(t, channel1Info.Models, "gpt-3.5-turbo")
		}
	})

	t.Run("multiple associations", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type: "model",
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
				},
			},
			{
				Type: "model",
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-3.5-turbo",
				},
			},
			{
				Type: "regex",
				Regex: &objects.RegexAssociation{
					Pattern: "^claude-3-.*",
				},
			},
			{
				Type: "model",
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gemini-pro",
				},
			},
			{
				Type: "model",
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gemini-1.5-pro",
				},
			},
		}

		result := findUnassociatedChannels(channels, associations)

		// All models should be associated
		require.Empty(t, result)
	})

	t.Run("no channels", func(t *testing.T) {
		result := findUnassociatedChannels([]*ent.Channel{}, []*objects.ModelAssociation{})
		require.Empty(t, result)
	})
}
