package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

// TestDecoratorChain_FullStack tests the complete decorator chain: Default -> SelectedChannels -> LoadBalanced.
func TestDecoratorChain_FullStack(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)
	connectionTracker := NewDefaultConnectionTracker(10)

	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(requestService),
		NewErrorAwareStrategy(channelService),
		NewWeightRoundRobinStrategy(channelService),
		NewConnectionAwareStrategy(channelService, connectionTracker),
	}
	loadBalancer := NewLoadBalancer(systemService, strategies...)

	// Build decorator chain: Default -> SelectedChannels -> LoadBalanced
	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService)
	filteredSelector := WithSelectedChannelsSelector(baseSelector, []int{channels[0].ID, channels[1].ID})
	selector := WithLoadBalancedSelector(filteredSelector, loadBalancer)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return only 2 allowed channels
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID)
	assert.Contains(t, channelIDs, channels[1].ID)
	assert.NotContains(t, channelIDs, channels[2].ID, "Filtered channel should not be included")
}

// TestSelectedChannelsSelector_WithAllowedChannels tests filtering with allowed channel IDs.
func TestSelectedChannelsSelector_WithAllowedChannels(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)
	connectionTracker := NewDefaultConnectionTracker(10)

	baseSelector := newTestLoadBalancedSelector(channelService, client, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	// Test without allowed channels - should return all 3 enabled channels
	result, err := baseSelector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Test with allowed channels - should return only 2 channels
	filteredSelector := WithSelectedChannelsSelector(baseSelector, []int{channels[0].ID, channels[2].ID})
	result, err = filteredSelector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID)
	assert.Contains(t, channelIDs, channels[2].ID)
	assert.NotContains(t, channelIDs, channels[1].ID)
}

// TestSelectedChannelsSelector_WithEmptyFilter tests that empty filter returns all channels.
func TestSelectedChannelsSelector_WithEmptyFilter(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService)

	req := &llm.Request{
		Model: "gpt-4",
	}

	// Empty slice should return all channels from wrapped selector
	filteredSelector := WithSelectedChannelsSelector(baseSelector, []int{})
	result, err := filteredSelector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3) // All 3 enabled channels

	// Nil slice should also return all channels
	filteredSelector = WithSelectedChannelsSelector(baseSelector, nil)
	result, err = filteredSelector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Verify all enabled channels are present
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID)
	assert.Contains(t, channelIDs, channels[1].ID)
	assert.Contains(t, channelIDs, channels[2].ID)
}
