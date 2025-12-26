package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestDeduplicationTracker(t *testing.T) {
	tracker := make(deduplicationTracker)

	// First add should return true
	assert.True(t, tracker.add(1, "model-a"))
	assert.True(t, tracker.add(1, "model-b"))
	assert.True(t, tracker.add(2, "model-a"))

	// Duplicate adds should return false
	assert.False(t, tracker.add(1, "model-a"))
	assert.False(t, tracker.add(1, "model-b"))
	assert.False(t, tracker.add(2, "model-a"))

	// Verify key format
	assert.Equal(t, "1:model-a", tracker.makeKey(1, "model-a"))
	assert.Equal(t, "2:model-b", tracker.makeKey(2, "model-b"))
}

func TestMatchAssociations_Deduplication(t *testing.T) {
	ctx := context.Background()

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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)
		assert.Len(t, result, 1, "should only have one connection")
		assert.Equal(t, 1, result[0].Channel.ID)
		assert.Len(t, result[0].Models, 1)
		assert.Equal(t, "gpt-4", result[0].Models[0].RequestModel)
		assert.Equal(t, 1, result[0].Priority, "should use first association's priority")
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)
		assert.Len(t, result, 2, "should have two connections for two channels")

		// Verify each channel has gpt-4 only once
		for _, conn := range result {
			assert.Len(t, conn.Models, 1)
			assert.Equal(t, "gpt-4", conn.Models[0].RequestModel)
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)
		assert.Len(t, result, 1, "should have one connection")
		assert.Equal(t, 1, result[0].Channel.ID)

		// Count gpt-4 occurrences
		gpt4Count := 0

		for _, model := range result[0].Models {
			if model.RequestModel == "gpt-4" {
				gpt4Count++
			}
		}

		assert.Equal(t, 1, gpt4Count, "gpt-4 should appear only once")
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)

		// Verify no duplicates within each channel
		for _, conn := range result {
			modelSet := make(map[string]bool)
			for _, model := range conn.Models {
				assert.False(t, modelSet[model.RequestModel], "model %s should not duplicate in channel %d", model.RequestModel, conn.Channel.ID)
				modelSet[model.RequestModel] = true
			}
		}
	})
}

func TestMatchAssociations_EmptyConnectionFiltering(t *testing.T) {
	ctx := context.Background()

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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)
		assert.Len(t, result, 1, "should have one connection")
		assert.Len(t, result[0].Models, 1, "should have one model")
		assert.Equal(t, "gpt-4", result[0].Models[0].RequestModel)
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)
		assert.Len(t, result, 0, "should have no connections")
	})
}

func TestMatchAssociations_ComplexScenario(t *testing.T) {
	ctx := context.Background()

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

	result, err := MatchAssociations(ctx, associations, channels)
	assert.NoError(t, err)

	// Verify no duplicates within each connection
	for _, conn := range result {
		modelSet := make(map[string]bool)

		for _, model := range conn.Models {
			key := model.RequestModel
			assert.False(t, modelSet[key], "model %s should not duplicate in channel %d", key, conn.Channel.ID)
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
	assert.Equal(t, 1, channel1Models["gpt-4"], "gpt-4 should appear only once in channel 1")
	assert.Equal(t, 1, channel1Models["gpt-3.5-turbo"], "gpt-3.5-turbo should appear only once in channel 1")
	assert.Equal(t, 1, channel1Models["gpt-4-turbo"], "gpt-4-turbo should appear only once in channel 1")
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
	ctx := context.Background()

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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)

		// Should only match openai-primary, not openai-backup
		assert.Len(t, result, 1)
		assert.Equal(t, 1, result[0].Channel.ID)
		assert.Equal(t, "openai-primary", result[0].Channel.Name)
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)

		// Should only match channel 1, not channel 2
		assert.Len(t, result, 1)
		assert.Equal(t, 1, result[0].Channel.ID)
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)

		// Should only match openai-primary
		assert.Len(t, result, 1)
		assert.Equal(t, 1, result[0].Channel.ID)
		assert.Equal(t, "gpt-4", result[0].Models[0].RequestModel)
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)

		// Should exclude both openai channels, no results
		assert.Len(t, result, 0)
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)

		// Should only match openai-primary (channel 1)
		// Excludes: openai-backup (by pattern), anthropic-primary (by ID)
		assert.Len(t, result, 1)
		assert.Equal(t, 1, result[0].Channel.ID)
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)

		// Should exclude all channels: 1 by pattern, 2 by ID
		assert.Len(t, result, 0)
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)

		// Should match both openai channels
		assert.Len(t, result, 2)
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

		result, err := MatchAssociations(ctx, associations, channels)
		assert.NoError(t, err)

		// Should match both openai channels
		assert.Len(t, result, 2)
	})
}
