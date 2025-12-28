package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestDuplicateKeyTracker(t *testing.T) {
	tracker := NewDuplicateKeyTracker()

	// First add should return true
	require.True(t, tracker.Add(1, "model-a"))
	require.True(t, tracker.Add(1, "model-b"))
	require.True(t, tracker.Add(2, "model-a"))

	// Duplicate adds should return false
	require.False(t, tracker.Add(1, "model-a"))
	require.False(t, tracker.Add(1, "model-b"))
	require.False(t, tracker.Add(2, "model-a"))

	// Verify key format
	require.Equal(t, "1:model-a", ChannelModelKey{ChannelID: 1, ModelID: "model-a"}.String())
	require.Equal(t, "2:model-b", ChannelModelKey{ChannelID: 2, ModelID: "model-b"}.String())
}

func TestMatchAssociations_Deduplication(t *testing.T) {
	// Create test channels
	channels := []*Channel{
		{
			Channel: &ent.Channel{
				ID:              1,
				Name:            "channel-1",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			},
		},
		{
			Channel: &ent.Channel{
				ID:              2,
				Name:            "channel-2",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4", "claude-3"},
			},
		},
	}

	t.Run("same channel same model should not duplicate", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "channel_model",
				Priority: 1,
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: 1,
					ModelID:   "gpt-4",
				},
			},
			{
				Type:     "channel_model",
				Priority: 2,
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: 1,
					ModelID:   "gpt-4",
				},
			},
		}

		result := MatchAssociations(associations, channels)
		require.Len(t, result, 1, "should only have one connection")
		require.Equal(t, 1, result[0].Channel.ID)
		require.Len(t, result[0].Models, 1)
		require.Equal(t, "gpt-4", result[0].Models[0].RequestModel)
		require.Equal(t, 1, result[0].Priority, "should use first association's priority")
	})

	t.Run("different channels same model should not duplicate", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "model",
				Priority: 1,
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
				},
			},
		}

		result := MatchAssociations(associations, channels)
		require.Len(t, result, 2, "should have two connections for two channels")

		// Verify each channel has gpt-4 only once
		for _, conn := range result {
			require.Len(t, conn.Models, 1)
			require.Equal(t, "gpt-4", conn.Models[0].RequestModel)
		}
	})

	t.Run("regex deduplication within same channel", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "channel_regex",
				Priority: 1,
				ChannelRegex: &objects.ChannelRegexAssociation{
					ChannelID: 1,
					Pattern:   "gpt-.*",
				},
			},
			{
				Type:     "channel_model",
				Priority: 2,
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: 1,
					ModelID:   "gpt-4",
				},
			},
		}

		result := MatchAssociations(associations, channels)

		require.Len(t, result, 1, "should have one connection")
		require.Equal(t, 1, result[0].Channel.ID)

		// Count gpt-4 occurrences
		gpt4Count := 0

		for _, model := range result[0].Models {
			if model.RequestModel == "gpt-4" {
				gpt4Count++
			}
		}

		require.Equal(t, 1, gpt4Count, "gpt-4 should appear only once")
	})

	t.Run("multiple regex patterns deduplication", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: "gpt-.*",
				},
			},
			{
				Type:     "regex",
				Priority: 2,
				Regex: &objects.RegexAssociation{
					Pattern: ".*-4",
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Verify no duplicates within each channel
		for _, conn := range result {
			modelSet := make(map[string]bool)
			for _, model := range conn.Models {
				require.False(t, modelSet[model.RequestModel], "model %s should not duplicate in channel %d", model.RequestModel, conn.Channel.ID)
				modelSet[model.RequestModel] = true
			}
		}
	})
}

func TestMatchAssociations_EmptyConnectionFiltering(t *testing.T) {
	channels := []*Channel{
		{
			Channel: &ent.Channel{
				ID:              1,
				Name:            "channel-1",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4"},
			},
		},
	}

	t.Run("filter empty connection after deduplication", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "channel_model",
				Priority: 1,
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: 1,
					ModelID:   "gpt-4",
				},
			},
			{
				Type:     "channel_regex",
				Priority: 2,
				ChannelRegex: &objects.ChannelRegexAssociation{
					ChannelID: 1,
					Pattern:   "gpt-.*",
				},
			},
		}

		result := MatchAssociations(associations, channels)
		require.Len(t, result, 1, "should have one connection")
		require.Len(t, result[0].Models, 1, "should have one model")
		require.Equal(t, "gpt-4", result[0].Models[0].RequestModel)
	})

	t.Run("no empty connections in result", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "channel_model",
				Priority: 1,
				ChannelModel: &objects.ChannelModelAssociation{
					ChannelID: 999,
					ModelID:   "non-existent",
				},
			},
		}

		result := MatchAssociations(associations, channels)
		require.Len(t, result, 0, "should have no connections")
	})
}

func TestMatchAssociations_ComplexScenario(t *testing.T) {
	channels := []*Channel{
		{
			Channel: &ent.Channel{
				ID:              1,
				Name:            "openai",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo"},
			},
		},
		{
			Channel: &ent.Channel{
				ID:              2,
				Name:            "anthropic",
				Type:            channel.TypeAnthropic,
				SupportedModels: []string{"claude-3-opus", "claude-3-sonnet"},
			},
		},
		{
			Channel: &ent.Channel{
				ID:              3,
				Name:            "openai-backup",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			},
		},
	}

	associations := []*objects.ModelAssociation{
		{
			Type:     "channel_model",
			Priority: 1,
			ChannelModel: &objects.ChannelModelAssociation{
				ChannelID: 1,
				ModelID:   "gpt-4",
			},
		},
		{
			Type:     "regex",
			Priority: 2,
			Regex: &objects.RegexAssociation{
				Pattern: "gpt-.*",
			},
		},
		{
			Type:     "model",
			Priority: 3,
			ModelID: &objects.ModelIDAssociation{
				ModelID: "claude-3-opus",
			},
		},
		{
			Type:     "channel_regex",
			Priority: 4,
			ChannelRegex: &objects.ChannelRegexAssociation{
				ChannelID: 1,
				Pattern:   ".*turbo",
			},
		},
	}

	result := MatchAssociations(associations, channels)

	// Verify no duplicates within each connection
	for _, conn := range result {
		modelSet := make(map[string]bool)

		for _, model := range conn.Models {
			key := model.RequestModel
			require.False(t, modelSet[key], "model %s should not duplicate in channel %d", key, conn.Channel.ID)
			modelSet[key] = true
		}
	}

	// Aggregate all models for channel 1 across all connections
	channel1Models := make(map[string]int)

	for _, conn := range result {
		if conn.Channel.ID == 1 {
			for _, model := range conn.Models {
				channel1Models[model.RequestModel]++
			}
		}
	}

	// Verify each model appears only once across all connections for channel 1
	require.Equal(t, 1, channel1Models["gpt-4"], "gpt-4 should appear only once in channel 1")
	require.Equal(t, 1, channel1Models["gpt-3.5-turbo"], "gpt-3.5-turbo should appear only once in channel 1")
	require.Equal(t, 1, channel1Models["gpt-4-turbo"], "gpt-4-turbo should appear only once in channel 1")
}

func findConnection(connections []*ModelChannelConnection, channelID int) *ModelChannelConnection {
	for _, conn := range connections {
		if conn.Channel.ID == channelID {
			return conn
		}
	}

	return nil
}

func countModel(models []ChannelModelEntry, modelID string) int {
	count := 0

	for _, model := range models {
		if model.RequestModel == modelID {
			count++
		}
	}

	return count
}

func TestMatchAssociations_ExcludeChannels(t *testing.T) {
	channels := []*Channel{
		{
			Channel: &ent.Channel{
				ID:              1,
				Name:            "openai-primary",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			},
		},
		{
			Channel: &ent.Channel{
				ID:              2,
				Name:            "openai-backup",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			},
		},
		{
			Channel: &ent.Channel{
				ID:              3,
				Name:            "anthropic-primary",
				Type:            channel.TypeAnthropic,
				SupportedModels: []string{"claude-3-opus"},
			},
		},
	}

	t.Run("regex exclude by channel name pattern", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: "gpt-.*",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelNamePattern: ".*backup",
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should only match openai-primary, not openai-backup
		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].Channel.ID)
		require.Equal(t, "openai-primary", result[0].Channel.Name)
	})

	t.Run("regex exclude by channel IDs", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: "gpt-.*",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelIds: []int{2},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should only match channel 1, not channel 2
		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].Channel.ID)
	})

	t.Run("model exclude by channel name pattern", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "model",
				Priority: 1,
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelNamePattern: "openai-backup",
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should only match openai-primary
		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].Channel.ID)
		require.Equal(t, "gpt-4", result[0].Models[0].RequestModel)
	})

	t.Run("model exclude by channel IDs", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "model",
				Priority: 1,
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelIds: []int{1, 2},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should exclude both openai channels, no results
		require.Len(t, result, 0)
	})

	t.Run("exclude with both pattern and IDs", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: ".*",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelNamePattern: ".*backup",
							ChannelIds:         []int{3},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should only match openai-primary (channel 1)
		// Excludes: openai-backup (by pattern), anthropic-primary (by ID)
		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].Channel.ID)
	})

	t.Run("multiple exclude rules", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "model",
				Priority: 1,
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelNamePattern: ".*primary",
						},
						{
							ChannelIds: []int{2},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should exclude all channels: 1 by pattern, 2 by ID
		require.Len(t, result, 0)
	})

	t.Run("no exclude when list is empty", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: "gpt-.*",
					Exclude: []*objects.ExcludeAssociation{},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should match both openai channels
		require.Len(t, result, 2)
	})

	t.Run("no exclude when nil", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "model",
				Priority: 1,
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
					Exclude: nil,
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should match both openai channels
		require.Len(t, result, 2)
	})
}

func TestMatchAssociations_ExcludeChannelsByTags(t *testing.T) {
	channels := []*Channel{
		{
			Channel: &ent.Channel{
				ID:              1,
				Name:            "openai-primary",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
				Tags:            []string{"production", "openai"},
			},
		},
		{
			Channel: &ent.Channel{
				ID:              2,
				Name:            "openai-backup",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
				Tags:            []string{"backup", "openai"},
			},
		},
		{
			Channel: &ent.Channel{
				ID:              3,
				Name:            "anthropic-primary",
				Type:            channel.TypeAnthropic,
				SupportedModels: []string{"claude-3-opus"},
				Tags:            []string{"production", "anthropic"},
			},
		},
		{
			Channel: &ent.Channel{
				ID:              4,
				Name:            "development-channel",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4"},
				Tags:            []string{"development", "test"},
			},
		},
	}

	t.Run("regex exclude by single channel tag", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: "gpt-.*",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelTags: []string{"backup"},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should exclude openai-backup (tag: backup), match others
		require.Len(t, result, 2)

		channelIDs := make([]int, 0, len(result))
		for _, conn := range result {
			channelIDs = append(channelIDs, conn.Channel.ID)
		}

		require.Contains(t, channelIDs, 1)    // openai-primary
		require.Contains(t, channelIDs, 4)    // development-channel
		require.NotContains(t, channelIDs, 2) // openai-backup excluded
	})

	t.Run("regex exclude by multiple channel tags", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: ".*",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelTags: []string{"backup", "development"},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should exclude channels with backup or development tags
		require.Len(t, result, 2)

		channelIDs := make([]int, 0, len(result))
		for _, conn := range result {
			channelIDs = append(channelIDs, conn.Channel.ID)
		}

		require.Contains(t, channelIDs, 1)    // openai-primary
		require.Contains(t, channelIDs, 3)    // anthropic-primary
		require.NotContains(t, channelIDs, 2) // openai-backup excluded
		require.NotContains(t, channelIDs, 4) // development-channel excluded
	})

	t.Run("model exclude by channel tag", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "model",
				Priority: 1,
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelTags: []string{"production"},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should exclude channels with production tag (1 and 3), match backup and development
		require.Len(t, result, 2)

		channelIDs := make([]int, 0, len(result))
		for _, conn := range result {
			channelIDs = append(channelIDs, conn.Channel.ID)
		}

		require.Contains(t, channelIDs, 2)    // openai-backup
		require.Contains(t, channelIDs, 4)    // development-channel
		require.NotContains(t, channelIDs, 1) // openai-primary excluded
		require.NotContains(t, channelIDs, 3) // anthropic-primary excluded
	})

	t.Run("exclude with tags, pattern, and IDs combined", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: ".*",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelNamePattern: ".*primary",
							ChannelIds:         []int{4},
							ChannelTags:        []string{"backup"},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should exclude:
		// - Channel 1 and 3 by pattern (.*primary)
		// - Channel 4 by ID
		// - Channel 2 by tag (backup)
		// All channels excluded, no results
		require.Len(t, result, 0)
	})

	t.Run("multiple exclude rules with tags", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "model",
				Priority: 1,
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelTags: []string{"production"},
						},
						{
							ChannelTags: []string{"development"},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should exclude channels with production or development tags (1, 3, 4)
		// Only channel 2 (backup) should remain
		require.Len(t, result, 1)
		require.Equal(t, 2, result[0].Channel.ID)
		require.Equal(t, "openai-backup", result[0].Channel.Name)
	})

	t.Run("exclude with non-existent tag", func(t *testing.T) {
		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: "gpt-.*",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelTags: []string{"non-existent"},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channels)

		// Should match all channels with gpt models since no channel has the non-existent tag
		require.Len(t, result, 3) // channels 1, 2, 4 have gpt models
	})

	t.Run("channel with no tags", func(t *testing.T) {
		// Add a channel with no tags
		channelsWithNoTags := append(channels, &Channel{
			Channel: &ent.Channel{
				ID:              5,
				Name:            "no-tags-channel",
				Type:            channel.TypeOpenai,
				SupportedModels: []string{"gpt-4"},
				Tags:            []string{},
			},
		})

		associations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 1,
				Regex: &objects.RegexAssociation{
					Pattern: "gpt-.*",
					Exclude: []*objects.ExcludeAssociation{
						{
							ChannelTags: []string{"production"},
						},
					},
				},
			},
		}

		result := MatchAssociations(associations, channelsWithNoTags)

		// Should exclude production channels (1), but include others including no-tags channel
		require.Len(t, result, 3) // channels 2, 4, 5

		channelIDs := make([]int, 0, len(result))
		for _, conn := range result {
			channelIDs = append(channelIDs, conn.Channel.ID)
		}

		require.Contains(t, channelIDs, 2)    // openai-backup
		require.Contains(t, channelIDs, 4)    // development-channel
		require.Contains(t, channelIDs, 5)    // no-tags-channel
		require.NotContains(t, channelIDs, 1) // openai-primary excluded
	})
}
