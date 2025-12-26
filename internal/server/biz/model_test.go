package biz

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
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
		SetCredentials(&objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Anthropic Channel").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "key2"}).
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
		SetCredentials(&objects.ChannelCredentials{APIKey: "key3"}).
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
		SetCredentials(&objects.ChannelCredentials{APIKey: "key4"}).
		SetSupportedModels([]string{"gpt-4-disabled"}).
		SetDefaultTestModel("gpt-4-disabled").
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	// Create channel service for testing
	channelSvc := &ChannelService{
		AbstractService: &AbstractService{
			db: client,
		},
	}

	// Load enabled channels
	err = channelSvc.loadChannels(ctx)
	require.NoError(t, err)

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
		result := modelSvc.ListEnabledModels(ctx)

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
		result := modelSvc.ListEnabledModels(ctx)

		for _, model := range result {
			require.NotEmpty(t, model.ID, "Model ID should not be empty")
			require.NotEmpty(t, model.DisplayName, "Model DisplayName should not be empty")
			require.NotEmpty(t, model.OwnedBy, "Model OwnedBy should not be empty")
			require.Equal(t, model.ID, model.DisplayName, "Model ID and DisplayName should match")
		}
	})

	t.Run("verify model owned by channel type", func(t *testing.T) {
		result := modelSvc.ListEnabledModels(ctx)

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
		result := modelSvc.ListEnabledModels(ctx)

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
			SetCredentials(&objects.ChannelCredentials{APIKey: "key5"}).
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

		// Reload channels
		err = channelSvc.loadChannels(ctx)
		require.NoError(t, err)

		result := modelSvc.ListEnabledModels(ctx)

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
			SetCredentials(&objects.ChannelCredentials{APIKey: "key6"}).
			SetSupportedModels([]string{"provider/gpt-4", "provider/gpt-3.5-turbo"}).
			SetDefaultTestModel("provider/gpt-4").
			SetStatus(channel.StatusEnabled).
			SetSettings(&objects.ChannelSettings{
				AutoTrimedModelPrefixes: []string{"provider"},
			}).
			Save(ctx)
		require.NoError(t, err)

		// Reload channels
		err = channelSvc.loadChannels(ctx)
		require.NoError(t, err)

		result := modelSvc.ListEnabledModels(ctx)

		resultMap := make(map[string]bool)
		for _, model := range result {
			resultMap[model.ID] = true
		}

		require.True(t, resultMap["gpt-4"], "Auto-trimmed model should be included")
		require.True(t, resultMap["gpt-3.5-turbo"], "Auto-trimmed model should be included")
	})
}
