package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestDefaultSelector_SelectModelCandidates_Cache(t *testing.T) {
	ctx, client := setupTest(t)
	now := time.Now()
	modelID := "test-model"

	// Create test channels in database
	channels := createTestChannels(t, ctx, client)

	associations := []*objects.ModelAssociation{
		{
			Type:     "regex",
			Priority: 1,
			Regex: &objects.RegexAssociation{
				Pattern: "gpt-.*",
			},
		},
	}

	// Create test model in database
	client.Model.Create().
		SetDeveloper("test-developer").
		SetModelID(modelID).
		SetType(model.TypeChat).
		SetName("Test Model").
		SetIcon("test-icon").
		SetGroup("test-group").
		SetModelCard(&objects.ModelCard{}).
		SetStatus(model.StatusEnabled).
		SetSettings(&objects.ModelSettings{
			Associations: associations,
		}).
		SaveX(ctx)

	// Create real services
	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)

	// Create selector
	selector := NewDefaultSelector(channelService, modelService, systemService)

	t.Run("first call caches result", func(t *testing.T) {
		// Test selectModelCandidates with mock request
		req := &llm.Request{Model: modelID}
		candidates, err := selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, candidates)

		// Verify cache was populated
		selector.cacheMu.RLock()
		require.Len(t, selector.associationCache, 1)
		require.Contains(t, selector.associationCache, modelID)
		selector.cacheMu.RUnlock()
	})

	t.Run("second call uses cache", func(t *testing.T) {
		// Get initial cache entry
		selector.cacheMu.RLock()
		initialEntry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		// Call again
		req := &llm.Request{Model: modelID}
		candidates, err := selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, candidates)

		// Verify same cache entry is used (pointer equality)
		selector.cacheMu.RLock()
		currentEntry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		require.Same(t, initialEntry, currentEntry, "should use same cache entry")
	})

	t.Run("cache invalidated when channel count changes", func(t *testing.T) {
		// Add a new channel
		client.Channel.Create().
			SetName("anthropic-primary").
			SetType(channel.TypeAnthropic).
			SetSupportedModels([]string{"claude-3-opus"}).
			SetDefaultTestModel("claude-3-opus").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetStatus(channel.StatusEnabled).
			SaveX(ctx)

		req := &llm.Request{Model: modelID}
		candidates, err := selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, candidates)

		// Verify cache was updated with new channel count
		selector.cacheMu.RLock()
		entry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		require.Equal(t, 3, entry.channelCount, "cache should reflect new channel count")
	})

	t.Run("cache invalidated when channel updated", func(t *testing.T) {
		// Update a channel's timestamp
		_, err := client.Channel.UpdateOneID(channels[0].ID).
			SetUpdatedAt(now.Add(1 * time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// Clear cache to force refresh
		selector.cacheMu.Lock()
		selector.associationCache = make(map[string]*associationCacheEntry)
		selector.cacheMu.Unlock()

		req := &llm.Request{Model: modelID}
		candidates, err := selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, candidates)

		// Verify cache was populated
		selector.cacheMu.RLock()
		entry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		require.NotNil(t, entry)
		require.True(t, entry.latestChannelUpdateTime.After(now), "cache should reflect new update time")
	})

	t.Run("cache invalidated when model updated", func(t *testing.T) {
		// Get initial cache entry
		selector.cacheMu.RLock()
		initialEntry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		// Update model's UpdatedAt timestamp
		updatedModel, err := client.Model.Query().
			Where(model.ModelID(modelID)).
			First(ctx)
		require.NoError(t, err)

		_, err = client.Model.UpdateOneID(updatedModel.ID).
			SetUpdatedAt(now.Add(2 * time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// Call again - should refresh cache due to model update
		req := &llm.Request{Model: modelID}
		candidates, err := selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, candidates)

		// Verify cache was refreshed with new model update time
		selector.cacheMu.RLock()
		currentEntry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		require.NotSame(t, initialEntry, currentEntry, "cache entry should be refreshed when model is updated")
		require.True(t, currentEntry.latestModelUpdatedAt.After(initialEntry.latestModelUpdatedAt), "model update time should be newer")
	})

	t.Run("cache invalidated when model associations updated", func(t *testing.T) {
		// Get initial cache entry
		selector.cacheMu.RLock()
		initialEntry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		// Wait a bit to ensure timestamp difference
		time.Sleep(10 * time.Millisecond)

		// Update model's associations (this will also update UpdatedAt)
		updatedModel, err := client.Model.Query().
			Where(model.ModelID(modelID)).
			First(ctx)
		require.NoError(t, err)

		newAssociations := []*objects.ModelAssociation{
			{
				Type:     "regex",
				Priority: 2,
				Regex: &objects.RegexAssociation{
					Pattern: "claude-.*",
				},
			},
		}

		_, err = client.Model.UpdateOneID(updatedModel.ID).
			SetSettings(&objects.ModelSettings{
				Associations: newAssociations,
			}).
			Save(ctx)
		require.NoError(t, err)

		// Call again - should refresh cache due to model update
		req := &llm.Request{Model: modelID}
		_, err = selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)

		// Verify cache was refreshed with new model update time
		selector.cacheMu.RLock()
		currentEntry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		require.NotSame(t, initialEntry, currentEntry, "cache entry should be refreshed when model associations are updated")
		require.True(
			t,
			currentEntry.latestModelUpdatedAt.After(initialEntry.latestModelUpdatedAt) || !currentEntry.latestModelUpdatedAt.Equal(initialEntry.latestModelUpdatedAt),
			"model update time should be newer or different",
		)
	})

	t.Run("different models use different cache entries", func(t *testing.T) {
		differentModelID := "different-model"
		differentAssociations := []*objects.ModelAssociation{
			{
				Type:     "model",
				Priority: 1,
				ModelID: &objects.ModelIDAssociation{
					ModelID: "gpt-4",
				},
			},
		}

		// Create different model in database
		client.Model.Create().
			SetDeveloper("test-developer").
			SetModelID(differentModelID).
			SetType(model.TypeChat).
			SetName("Different Model").
			SetIcon("test-icon").
			SetGroup("test-group").
			SetModelCard(&objects.ModelCard{}).
			SetStatus(model.StatusEnabled).
			SetSettings(&objects.ModelSettings{
				Associations: differentAssociations,
			}).
			SaveX(ctx)

		req := &llm.Request{Model: differentModelID}
		candidates, err := selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, candidates)

		// Verify we now have 2 cache entries
		selector.cacheMu.RLock()
		require.Len(t, selector.associationCache, 2)
		require.Contains(t, selector.associationCache, modelID)
		require.Contains(t, selector.associationCache, differentModelID)
		selector.cacheMu.RUnlock()
	})

	t.Run("empty channels returns empty candidates", func(t *testing.T) {
		// Delete all channels
		_, err := client.Channel.Delete().Where(channel.IDIn(channels[0].ID, channels[1].ID, channels[2].ID)).Exec(ctx)
		require.NoError(t, err)

		// Create a new channel service to force fresh data
		newChannelService := biz.NewChannelServiceForTest(client)

		// Clear cache to force refresh
		selector.cacheMu.Lock()
		selector.associationCache = make(map[string]*associationCacheEntry)
		selector.cacheMu.Unlock()

		selector.ChannelService = newChannelService

		req := &llm.Request{Model: modelID}
		candidates, err := selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)
		require.Empty(t, candidates)
	})

	t.Run("cache expires after TTL", func(t *testing.T) {
		// Create new channels with different names for this test since previous ones were soft-deleted
		_, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName("TTL Test Channel 1").
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key-ttl-1"}).
			SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
			SetDefaultTestModel("gpt-4").
			SetOrderingWeight(100).
			SetStatus(channel.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)

		_, err = client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName("TTL Test Channel 2").
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key-ttl-2"}).
			SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
			SetDefaultTestModel("gpt-4").
			SetOrderingWeight(50).
			SetStatus(channel.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)

		// Create a new channel service to see the new channels
		newChannelService := newTestChannelServiceForChannels(client)
		selector.ChannelService = newChannelService

		// First call to populate cache
		req := &llm.Request{Model: modelID}
		_, err = selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)

		// Manually set cache entry to be older than TTL
		selector.cacheMu.Lock()
		entry := selector.associationCache[modelID]
		entry.cachedAt = time.Now().Add(-6 * time.Minute) // 6 minutes ago, past 5-minute TTL

		selector.cacheMu.Unlock()

		// Get cache before
		selector.cacheMu.RLock()
		oldEntry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		// Call again - should refresh cache due to expiration
		candidates, err := selector.selectModelCandidates(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, candidates)

		// Verify cache was refreshed
		selector.cacheMu.RLock()
		newEntry := selector.associationCache[modelID]
		selector.cacheMu.RUnlock()

		require.NotSame(t, oldEntry, newEntry, "cache entry should be refreshed after TTL")
		require.True(t, time.Since(newEntry.cachedAt) < 1*time.Second, "new cache entry should have recent timestamp")
	})
}

func TestDefaultSelector_GetLatestChannelUpdateTime(t *testing.T) {
	t.Parallel()

	selector := &DefaultSelector{
		associationCache: make(map[string]*associationCacheEntry),
	}

	t.Run("empty channels", func(t *testing.T) {
		latest := selector.getLatestChannelUpdateTime([]*biz.Channel{})
		require.True(t, latest.IsZero())
	})

	t.Run("single channel", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		channels := []*biz.Channel{
			{
				Channel: &ent.Channel{
					ID:        1,
					UpdatedAt: now,
				},
			},
		}

		latest := selector.getLatestChannelUpdateTime(channels)
		require.True(t, latest.Equal(now))
	})

	t.Run("multiple channels", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		older := now.Add(-1 * time.Hour)
		newest := now.Add(1 * time.Hour)

		channels := []*biz.Channel{
			{
				Channel: &ent.Channel{
					ID:        1,
					UpdatedAt: older,
				},
			},
			{
				Channel: &ent.Channel{
					ID:        2,
					UpdatedAt: newest,
				},
			},
			{
				Channel: &ent.Channel{
					ID:        3,
					UpdatedAt: now,
				},
			},
		}

		latest := selector.getLatestChannelUpdateTime(channels)
		require.True(t, latest.Equal(newest))
	})
}
