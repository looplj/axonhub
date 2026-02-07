package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func TestTraceStickyKeyProvider_MultipleKeys_NoTrace(t *testing.T) {
	keys := []string{"key-1", "key-2", "key-3"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: keys,
			},
		},
		cachedEnabledAPIKeys: keys,
	}

	provider := NewTraceStickyKeyProvider(ch)
	ctx := context.Background()

	key := provider.Get(ctx)
	require.Contains(t, keys, key)
}

func TestTraceStickyKeyProvider_MultipleKeys_WithTrace_Sticky(t *testing.T) {
	keys := []string{"key-1", "key-2", "key-3"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: keys,
			},
		},
		cachedEnabledAPIKeys: keys,
	}

	provider := NewTraceStickyKeyProvider(ch)

	trace := &ent.Trace{TraceID: "trace-abc-123"}
	ctx := contexts.WithTrace(context.Background(), trace)

	key1 := provider.Get(ctx)
	key2 := provider.Get(ctx)
	key3 := provider.Get(ctx)

	require.Equal(t, key1, key2, "same trace should select same key")
	require.Equal(t, key2, key3, "same trace should select same key")
	require.Contains(t, keys, key1)
}

func TestTraceStickyKeyProvider_DifferentTraces_MaySelectDifferentKeys(t *testing.T) {
	keys := []string{"key-1", "key-2", "key-3", "key-4", "key-5"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: keys,
			},
		},
		cachedEnabledAPIKeys: keys,
	}

	provider := NewTraceStickyKeyProvider(ch)

	selectedKeys := make(map[string]bool)

	for i := range 100 {
		trace := &ent.Trace{TraceID: "trace-" + string(rune('A'+i%26)) + "-" + string(rune('0'+i%10))}
		ctx := contexts.WithTrace(context.Background(), trace)
		key := provider.Get(ctx)
		selectedKeys[key] = true
	}

	require.Greater(t, len(selectedKeys), 1, "different traces should select different keys")
}

func TestTraceStickyKeyProvider_EmptyEnabledKeys_FallbackToFirst(t *testing.T) {
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: []string{"fallback-key"},
			},
		},
		cachedEnabledAPIKeys: []string{},
	}

	provider := NewTraceStickyKeyProvider(ch)
	ctx := context.Background()

	key := provider.Get(ctx)
	require.Equal(t, "fallback-key", key)
}

func TestTraceStickyKeyProvider_AddKey_MinimalRemapping(t *testing.T) {
	originalKeys := []string{"key-1", "key-2", "key-3"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: originalKeys,
			},
		},
		cachedEnabledAPIKeys: originalKeys,
	}

	provider := NewTraceStickyKeyProvider(ch)

	traces := make([]*ent.Trace, 20)
	for i := range traces {
		traces[i] = &ent.Trace{TraceID: "trace-" + string(rune('A'+i))}
	}

	originalSelections := make(map[string]string)

	for _, trace := range traces {
		ctx := contexts.WithTrace(context.Background(), trace)
		originalSelections[trace.TraceID] = provider.Get(ctx)
	}

	newKeys := []string{"key-1", "key-2", "key-3", "key-4"}
	ch.cachedEnabledAPIKeys = newKeys
	ch.Credentials.APIKeys = newKeys

	remappedCount := 0

	for _, trace := range traces {
		ctx := contexts.WithTrace(context.Background(), trace)

		newSelection := provider.Get(ctx)
		if originalSelections[trace.TraceID] != newSelection {
			remappedCount++
		}
	}

	require.LessOrEqual(t, remappedCount, len(traces)*2/3,
		"adding a key should not remap most of the traces (rendezvous hashing property)")
}

func TestTraceStickyKeyProvider_RemoveKey_MinimalRemapping(t *testing.T) {
	originalKeys := []string{"key-1", "key-2", "key-3", "key-4"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: originalKeys,
			},
		},
		cachedEnabledAPIKeys: originalKeys,
	}

	provider := NewTraceStickyKeyProvider(ch)

	traces := make([]*ent.Trace, 20)
	for i := range traces {
		traces[i] = &ent.Trace{TraceID: "trace-" + string(rune('A'+i))}
	}

	originalSelections := make(map[string]string)

	for _, trace := range traces {
		ctx := contexts.WithTrace(context.Background(), trace)
		originalSelections[trace.TraceID] = provider.Get(ctx)
	}

	newKeys := []string{"key-1", "key-2", "key-4"}
	ch.cachedEnabledAPIKeys = newKeys
	ch.Credentials.APIKeys = newKeys

	unaffectedCount := 0

	for _, trace := range traces {
		ctx := contexts.WithTrace(context.Background(), trace)
		newSelection := provider.Get(ctx)
		oldSelection := originalSelections[trace.TraceID]

		if oldSelection != "key-3" && oldSelection == newSelection {
			unaffectedCount++
		}
	}

	tracesNotUsingRemovedKey := 0

	for _, selection := range originalSelections {
		if selection != "key-3" {
			tracesNotUsingRemovedKey++
		}
	}

	if tracesNotUsingRemovedKey > 0 {
		require.Equal(t, tracesNotUsingRemovedKey, unaffectedCount,
			"traces not using removed key should keep their selection")
	}
}

func TestTraceStickyKeyProvider_RemoveKey_Stable(t *testing.T) {
	originalKeys := []string{"key-1", "key-2", "key-3", "key-4"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: originalKeys,
			},
		},
		cachedEnabledAPIKeys: originalKeys,
	}

	trace := &ent.Trace{TraceID: fmt.Sprintf("trace-%d", time.Now().UnixNano())}
	ctx := contexts.WithTrace(context.Background(), trace)

	provider := NewTraceStickyKeyProvider(ch)

	selectedKey1 := provider.Get(ctx)

	// Removing any key EXCEPT the selected one should not change the selection.
	otherKeys := lo.Filter(originalKeys, func(k string, _ int) bool {
		return k != selectedKey1
	})

	for _, keyToRemove := range otherKeys {
		newKeys := lo.Filter(originalKeys, func(k string, _ int) bool {
			return k != keyToRemove
		})
		ch.cachedEnabledAPIKeys = newKeys
		ch.Credentials.APIKeys = newKeys

		selectedKey2 := provider.Get(ctx)
		require.Equal(t, selectedKey1, selectedKey2, "removing a non-selected key should not change the selection")
	}
}

func TestTraceStickyKeyProvider_AddKey_Stable(t *testing.T) {
	t.Skipf("Add key can not be stable, skip this test")

	originalKeys := []string{"key-1", "key-2", "key-3"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: originalKeys,
			},
		},
		cachedEnabledAPIKeys: originalKeys,
	}

	trace := &ent.Trace{TraceID: fmt.Sprintf("trace-%d", time.Now().UnixNano())}
	ctx := contexts.WithTrace(context.Background(), trace)

	provider := NewTraceStickyKeyProvider(ch)

	selectedKey1 := provider.Get(ctx)

	// Adding a new key should either keep the selection or change it to the NEW key.
	newKey := "key-new"
	newKeys := append(originalKeys, newKey)
	ch.cachedEnabledAPIKeys = newKeys
	ch.Credentials.APIKeys = newKeys

	selectedKey2 := provider.Get(ctx)

	require.Equal(t, selectedKey1, selectedKey2, "removing a non-selected key should not change the selection")
}

func TestTraceStickyKeyProvider_DisableKey_SimulatedByRemoval(t *testing.T) {
	allKeys := []string{"key-1", "key-2", "key-3"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: allKeys,
			},
		},
		cachedEnabledAPIKeys: allKeys,
	}

	provider := NewTraceStickyKeyProvider(ch)

	trace := &ent.Trace{TraceID: "trace-sticky-test"}
	ctx := contexts.WithTrace(context.Background(), trace)

	initialKey := provider.Get(ctx)
	require.Contains(t, allKeys, initialKey)

	ch.cachedEnabledAPIKeys = []string{"key-1", "key-3"}

	keyAfterDisable := provider.Get(ctx)
	require.Contains(t, []string{"key-1", "key-3"}, keyAfterDisable)
	require.NotEqual(t, "key-2", keyAfterDisable, "disabled key should not be selected")

	if initialKey != "key-2" {
		require.Equal(t, initialKey, keyAfterDisable,
			"if original key was not disabled, selection should remain stable")
	}
}

func TestTraceStickyKeyProvider_EnableKey_AfterDisable(t *testing.T) {
	allKeys := []string{"key-1", "key-2", "key-3"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: allKeys,
			},
		},
		cachedEnabledAPIKeys: []string{"key-1", "key-3"},
	}

	provider := NewTraceStickyKeyProvider(ch)

	trace := &ent.Trace{TraceID: "trace-reenable-test"}
	ctx := contexts.WithTrace(context.Background(), trace)

	_ = provider.Get(ctx)

	ch.cachedEnabledAPIKeys = allKeys

	keyAfterEnable := provider.Get(ctx)
	require.Contains(t, allKeys, keyAfterEnable)
}

func TestTraceStickyKeyProvider_AllKeysDisabled_FallbackToFirst(t *testing.T) {
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKeys: []string{"key-1", "key-2"},
			},
		},
		cachedEnabledAPIKeys: []string{},
	}

	provider := NewTraceStickyKeyProvider(ch)
	ctx := context.Background()

	key := provider.Get(ctx)
	require.Equal(t, "key-1", key, "should fallback to first key when all disabled")
}

func TestRendezvousSelect_Deterministic(t *testing.T) {
	keys := []string{"key-a", "key-b", "key-c", "key-d"}
	seed := "test-seed-123"

	result1 := rendezvousSelect(keys, seed)
	result2 := rendezvousSelect(keys, seed)
	result3 := rendezvousSelect(keys, seed)

	require.Equal(t, result1, result2)
	require.Equal(t, result2, result3)
}

func TestRendezvousSelect_DifferentSeeds(t *testing.T) {
	keys := []string{"key-a", "key-b", "key-c", "key-d", "key-e"}

	results := make(map[string]int)

	for i := range 100 {
		seed := "seed-" + string(rune('0'+i%10)) + string(rune('A'+i%26))
		result := rendezvousSelect(keys, seed)
		results[result]++
	}

	require.Greater(t, len(results), 1, "different seeds should produce different results")
}

func TestRendezvousSelect_StableWithKeyAddition(t *testing.T) {
	originalKeys := []string{"key-a", "key-b", "key-c"}
	newKeys := []string{"key-a", "key-b", "key-c", "key-d"}

	seeds := []string{"seed1", "seed2", "seed3", "seed4", "seed5"}

	stableCount := 0

	for _, seed := range seeds {
		original := rendezvousSelect(originalKeys, seed)
		after := rendezvousSelect(newKeys, seed)

		if original == after {
			stableCount++
		}
	}

	require.GreaterOrEqual(t, stableCount, len(seeds)/2,
		"most selections should remain stable when adding a key")
}

func TestRendezvousSelect_OnlyAffectedKeysRemap(t *testing.T) {
	originalKeys := []string{"key-a", "key-b", "key-c"}
	newKeys := []string{"key-a", "key-c"}

	seeds := make([]string, 50)
	for i := range seeds {
		seeds[i] = "seed-" + string(rune('A'+i))
	}

	for _, seed := range seeds {
		original := rendezvousSelect(originalKeys, seed)
		after := rendezvousSelect(newKeys, seed)

		if original != "key-b" {
			require.Equal(t, original, after,
				"selection not using d key should remain stable for seed: "+seed)
		} else {
			require.NotEqual(t, "key-b", after,
				"d key should not be selected")
		}
	}
}

func TestHash64_Deterministic(t *testing.T) {
	input := "test-input-string"

	h1 := hashAPIKey(input)
	h2 := hashAPIKey(input)
	h3 := hashAPIKey(input)

	require.Equal(t, h1, h2)
	require.Equal(t, h2, h3)
}

func TestHash64_DifferentInputs(t *testing.T) {
	h1 := hashAPIKey("input-1")
	h2 := hashAPIKey("input-2")
	h3 := hashAPIKey("input-3")

	require.NotEqual(t, h1, h2)
	require.NotEqual(t, h2, h3)
	require.NotEqual(t, h1, h3)
}

func TestTraceStickyKeyProvider_LegacyAPIKey(t *testing.T) {
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKey: "legacy-key",
			},
		},
		cachedEnabledAPIKeys: []string{"legacy-key"},
	}

	provider := NewTraceStickyKeyProvider(ch)
	ctx := context.Background()

	key := provider.Get(ctx)
	require.Equal(t, "legacy-key", key)
}

func TestTraceStickyKeyProvider_MixedLegacyAndNewKeys(t *testing.T) {
	keys := []string{"legacy-key", "new-key-1", "new-key-2"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{
				APIKey:  "legacy-key",
				APIKeys: []string{"new-key-1", "new-key-2"},
			},
		},
		cachedEnabledAPIKeys: keys,
	}

	provider := NewTraceStickyKeyProvider(ch)

	trace := &ent.Trace{TraceID: "trace-mixed-test"}
	ctx := contexts.WithTrace(context.Background(), trace)

	key := provider.Get(ctx)
	require.Contains(t, keys, key)
}

func TestTraceStickyKeyProvider_KeyOrderIndependence(t *testing.T) {
	keys1 := []string{"key-a", "key-b", "key-c"}
	keys2 := []string{"key-c", "key-a", "key-b"}

	ch1 := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{APIKeys: keys1},
		},
		cachedEnabledAPIKeys: keys1,
	}

	ch2 := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{APIKeys: keys2},
		},
		cachedEnabledAPIKeys: keys2,
	}

	provider1 := NewTraceStickyKeyProvider(ch1)
	provider2 := NewTraceStickyKeyProvider(ch2)

	trace := &ent.Trace{TraceID: "trace-order-test"}
	ctx := contexts.WithTrace(context.Background(), trace)

	key1 := provider1.Get(ctx)
	key2 := provider2.Get(ctx)

	require.Equal(t, key1, key2, "rendezvous hashing should be order-independent")
}

// ==================== DeleteDisabledAPIKeys Tests ====================.
func TestChannelService_DeleteDisabledAPIKeys_SingleKey(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test channel with multiple keys, one disabled
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{
			APIKeys: []string{"key1", "key2", "key3"},
		}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetDisabledAPIKeys([]objects.DisabledAPIKey{
			{Key: "key2", ErrorCode: 401, Reason: "invalid key"},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Delete the disabled key
	result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, []string{"key2"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)
	require.Empty(t, result.Message)

	// Verify key is removed from both disabled list and credentials
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updatedCh.DisabledAPIKeys, 0)
	require.Len(t, updatedCh.Credentials.APIKeys, 2)
	require.NotContains(t, updatedCh.Credentials.APIKeys, "key2")
	require.Contains(t, updatedCh.Credentials.APIKeys, "key1")
	require.Contains(t, updatedCh.Credentials.APIKeys, "key3")
}

func TestChannelService_DeleteDisabledAPIKeys_MultipleKeys(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test channel with multiple keys, some disabled
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{
			APIKeys: []string{"key1", "key2", "key3", "key4"},
		}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetDisabledAPIKeys([]objects.DisabledAPIKey{
			{Key: "key2", ErrorCode: 401, Reason: "invalid key"},
			{Key: "key4", ErrorCode: 429, Reason: "rate limited"},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Delete multiple disabled keys
	result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, []string{"key2", "key4"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)

	// Verify keys are removed
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updatedCh.DisabledAPIKeys, 0)
	require.Len(t, updatedCh.Credentials.APIKeys, 2)
	require.Contains(t, updatedCh.Credentials.APIKeys, "key1")
	require.Contains(t, updatedCh.Credentials.APIKeys, "key3")
	require.NotContains(t, updatedCh.Credentials.APIKeys, "key2")
	require.NotContains(t, updatedCh.Credentials.APIKeys, "key4")
}

func TestChannelService_DeleteDisabledAPIKeys_OAuthChannel(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create an OAuth channel
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("OAuth Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{
				AccessToken: "test-token",
			},
		}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	// Try to delete keys from OAuth channel
	result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, []string{"some-key"})
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "cannot delete API keys for OAuth channels")
}

func TestChannelService_DeleteDisabledAPIKeys_PreserveAtLeastOneKey(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test channel with only one key (disabled)
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{
			APIKeys: []string{"only-key"},
		}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetDisabledAPIKeys([]objects.DisabledAPIKey{
			{Key: "only-key", ErrorCode: 401, Reason: "invalid key"},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Try to delete the only key - should preserve it
	result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, []string{"only-key"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)
	require.Equal(t, "ONE_KEY_PRESERVED", result.Message)

	// Verify the key is preserved
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updatedCh.Credentials.APIKeys, 1)
	require.Contains(t, updatedCh.Credentials.APIKeys, "only-key")
	// The key should be removed from disabled list since it's the only one
	require.Len(t, updatedCh.DisabledAPIKeys, 0)
}

func TestChannelService_DeleteDisabledAPIKeys_WithLegacyAPIKey(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test channel with legacy APIKey field
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{
			APIKey:  "legacy-key",
			APIKeys: []string{"new-key-1", "new-key-2"},
		}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetDisabledAPIKeys([]objects.DisabledAPIKey{
			{Key: "legacy-key", ErrorCode: 401, Reason: "invalid key"},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Delete the legacy key
	result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, []string{"legacy-key"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)

	// Verify legacy key is removed
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Empty(t, updatedCh.Credentials.APIKey)
	require.Len(t, updatedCh.Credentials.APIKeys, 2)
}

func TestChannelService_DeleteDisabledAPIKeys_PartialKeysNotInDisabledList(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test channel with some disabled keys
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{
			APIKeys: []string{"key1", "key2", "key3"},
		}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetDisabledAPIKeys([]objects.DisabledAPIKey{
			{Key: "key2", ErrorCode: 401, Reason: "invalid key"},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Try to delete key2 (disabled) and key3 (not disabled)
	result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, []string{"key2", "key3"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)

	// Verify both keys are removed from credentials
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updatedCh.DisabledAPIKeys, 0)
	require.Len(t, updatedCh.Credentials.APIKeys, 1)
	require.Contains(t, updatedCh.Credentials.APIKeys, "key1")
	require.NotContains(t, updatedCh.Credentials.APIKeys, "key2")
	require.NotContains(t, updatedCh.Credentials.APIKeys, "key3")
}

func TestChannelService_DeleteDisabledAPIKeys_NoDisabledKeys(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test channel with no disabled keys
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{
			APIKeys: []string{"key1", "key2"},
		}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	// Try to delete a key that's not disabled
	result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, []string{"key1"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)

	// Verify key is removed from credentials even if not disabled
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updatedCh.Credentials.APIKeys, 1)
	require.Contains(t, updatedCh.Credentials.APIKeys, "key2")
	require.NotContains(t, updatedCh.Credentials.APIKeys, "key1")
}
