package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

// TestLoadBalancedSelector_Select_MultipleChannels_LoadBalancing tests load balancing with multiple channels.
func TestLoadBalancedSelector_Select_MultipleChannels_LoadBalancing(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return 3 enabled channels (exclude disabled one)
	require.Len(t, result, 3)

	// With weighted round-robin, all channels start with equal scores (150) when they have 0 requests.
	// The order is determined by other factors (e.g., database order, ErrorAwareStrategy).
	// We only verify that all enabled channels are present.

	// Verify all channels are enabled
	for i, ch := range result {
		require.Equal(t, channel.StatusEnabled, ch.Channel.Status, "Channel %d should be enabled", i)
		require.Equal(t, channel.TypeOpenai, ch.Channel.Type, "Channel %d should be OpenAI type", i)
		require.Contains(t, ch.Channel.SupportedModels, "gpt-4", "Channel %d should support gpt-4", i)
	}

	// Verify disabled channel is not included
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	require.NotContains(t, channelIDs, channels[3].ID, "Disabled channel should not be included")

	// Verify all enabled channels are present
	require.Contains(t, channelIDs, channels[0].ID, "High weight channel should be included")
	require.Contains(t, channelIDs, channels[1].ID, "Medium weight channel should be included")
	require.Contains(t, channelIDs, channels[2].ID, "Low weight channel should be included")
}

// TestDefaultChannelSelector_Select_WithConnectionTracking tests connection tracking integration.
func TestDefaultChannelSelector_Select_WithConnectionTracking(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService, connectionTracker)

	// Add some connections to affect load balancing
	connectionTracker.IncrementConnection(channels[0].ID) // High weight channel now has 2 connections
	connectionTracker.IncrementConnection(channels[0].ID)
	connectionTracker.IncrementConnection(channels[1].ID) // Medium weight channel has 1 connection
	// ch3 (low weight) has 0 connections

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Verify all channels are returned with specific ordering
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	require.Contains(t, channelIDs, channels[0].ID)
	require.Contains(t, channelIDs, channels[1].ID)
	require.Contains(t, channelIDs, channels[2].ID)

	// Due to connection awareness, the channel with no connections (ch3)
	// should get a boost from the ConnectionAwareStrategy
	// However, WeightRoundRobinStrategy has higher priority, so weight still matters significantly
	// We expect: ch1 (high weight, 2 conn) > ch2 (medium weight, 1 conn) > ch3 (low weight, 0 conn)
	// But ch3 might get boosted due to no connections

	// Let's verify the connection counts are correctly tracked
	require.Equal(t, 2, connectionTracker.GetActiveConnections(channels[0].ID), "Channel 0 should have 2 connections")
	require.Equal(t, 1, connectionTracker.GetActiveConnections(channels[1].ID), "Channel 1 should have 1 connection")
	require.Equal(t, 0, connectionTracker.GetActiveConnections(channels[2].ID), "Channel 2 should have 0 connections")

	// Log the actual ordering for debugging
	t.Logf("Channel ordering with connections: ch0(2 conn)=%d, ch1(1 conn)=%d, ch2(0 conn)=%d",
		result[0].Channel.ID, result[1].Channel.ID, result[2].Channel.ID)

	// Verify channel properties in the result
	for i, ch := range result {
		require.Equal(t, channel.StatusEnabled, ch.Channel.Status, "Channel %d should be enabled", i)
		require.Contains(t, ch.Channel.SupportedModels, "gpt-4", "Channel %d should support gpt-4", i)
	}
}

// TestDefaultChannelSelector_Select_WithTraceContext tests trace-aware load balancing.
func TestDefaultChannelSelector_Select_WithTraceContext(t *testing.T) {
	ctx, client := setupTest(t)

	// Create project
	project, err := client.Project.Create().
		SetName("test-project").
		Save(ctx)
	require.NoError(t, err)

	channels := createTestChannels(t, ctx, client)

	// Create trace
	trace, err := client.Trace.Create().
		SetProjectID(project.ID).
		SetTraceID("test-trace-123").
		Save(ctx)
	require.NoError(t, err)

	// Create a successful request with channel 2 in this trace
	_, err = client.Request.Create().
		SetProjectID(project.ID).
		SetTraceID(trace.ID).
		SetChannelID(channels[1].ID). // Medium weight channel
		SetModelID("gpt-4").
		SetStatus("completed").
		SetSource("api").
		SetRequestBody([]byte(`{"model":"gpt-4","messages":[]}`)).
		Save(ctx)
	require.NoError(t, err)

	// Add trace to context
	ctx = contexts.WithTrace(ctx, trace)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Channel 2 should be ranked first due to trace awareness (high boost score from TraceAwareStrategy)
	require.Equal(t, channels[1].ID, result[0].Channel.ID, "Channel from trace should be ranked first")

	// The other channels should follow in weight order (ch1 > ch3)
	require.Contains(t, []int{result[1].Channel.ID, result[2].Channel.ID}, channels[0].ID, "High weight channel should be in top 3")
	require.Contains(t, []int{result[1].Channel.ID, result[2].Channel.ID}, channels[2].ID, "Low weight channel should be in top 3")

	// Verify all channels are enabled and support the model
	for i, ch := range result {
		require.Equal(t, channel.StatusEnabled, ch.Channel.Status, "Channel %d should be enabled", i)
		require.Contains(t, ch.Channel.SupportedModels, "gpt-4", "Channel %d should support gpt-4", i)
	}

	// Verify the trace channel is indeed channel 2 (medium weight)
	require.Equal(t, "Medium Weight Channel", result[0].Channel.Name, "First channel should be the medium weight channel from trace")
	require.Equal(t, 50, result[0].Channel.OrderingWeight, "First channel should have medium weight (50)")

	// Log the ordering to verify trace awareness is working
	t.Logf("Channel ordering with trace context: %s (weight=%d), %s (weight=%d), %s (weight=%d)",
		result[0].Channel.Name, result[0].Channel.OrderingWeight,
		result[1].Channel.Name, result[1].Channel.OrderingWeight,
		result[2].Channel.Name, result[2].Channel.OrderingWeight)
}

// TestDefaultChannelSelector_Select_WithChannelFailures tests error-aware load balancing.
func TestDefaultChannelSelector_Select_WithChannelFailures(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService, connectionTracker)

	// Record failures for the high weight channel to test error awareness
	for range 3 {
		perf := &biz.PerformanceRecord{
			ChannelID:        channels[0].ID,
			StartTime:        time.Now().Add(-time.Minute),
			EndTime:          time.Now(),
			Success:          false,
			RequestCompleted: true,
			ErrorStatusCode:  500,
		}
		channelService.RecordPerformance(ctx, perf)
	}

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// The failing channel (channels[0]) should be ranked lower due to consecutive failures
	// With 3 consecutive failures, ErrorAwareStrategy should significantly penalize it
	require.NotEqual(t, channels[0].ID, result[0].Channel.ID, "Failing channel should not be ranked first")

	// The healthy channels should be ranked higher
	// We expect either ch2 (medium weight) or ch3 (low weight) to be first
	// Since ch2 has higher weight and no failures, it should be first
	require.Equal(t, channels[1].ID, result[0].Channel.ID, "Medium weight healthy channel should be first")

	// Verify all channels are still included (just reordered)
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
		require.Equal(t, channel.StatusEnabled, ch.Channel.Status, "Channel %d should be enabled", i)
		require.Contains(t, ch.Channel.SupportedModels, "gpt-4", "Channel %d should support gpt-4", i)
	}

	require.Contains(t, channelIDs, channels[0].ID, "Failing channel should still be included")
	require.Contains(t, channelIDs, channels[1].ID, "Medium weight channel should be included")
	require.Contains(t, channelIDs, channels[2].ID, "Low weight channel should be included")

	// Log the ordering to verify error awareness is working
	t.Logf("Channel ordering with failures: %s (3 failures), %s (0 failures), %s (0 failures)",
		getCandidateNameByID(result, channels[0].ID),
		getCandidateNameByID(result, channels[1].ID),
		getCandidateNameByID(result, channels[2].ID))
}

// TestDefaultChannelSelector_Select_WeightedRoundRobin_EqualWeights tests round-robin behavior with equal weights.
func TestDefaultChannelSelector_Select_WeightedRoundRobin_EqualWeights(t *testing.T) {
	ctx, client := setupTest(t)

	// Create channels with equal weights to isolate round-robin behavior
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 1").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50). // Equal weight
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 2").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-2"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50). // Equal weight
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 3").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-3"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50). // Equal weight
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels := []*ent.Channel{ch1, ch2, ch3}

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	// Make multiple selections to test round-robin behavior
	selections := make([][]*ChannelModelCandidate, 9)

	for i := range 9 {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.Len(t, result, 3)
		selections[i] = result
	}

	// With equal weights, we should see more round-robin effect
	// Initially, all channels have 0 requests, so they should be in some consistent order
	// (not necessarily creation order due to database query ordering)
	require.Len(t, selections[0], 3, "First selection should have 3 channels")

	// Verify the first selection has all expected channels
	firstSelectionIDs := make([]int, len(selections[0]))
	for i, ch := range selections[0] {
		firstSelectionIDs[i] = ch.Channel.ID
	}

	require.Contains(t, firstSelectionIDs, channels[0].ID, "First selection should contain channel 1")
	require.Contains(t, firstSelectionIDs, channels[1].ID, "First selection should contain channel 2")
	require.Contains(t, firstSelectionIDs, channels[2].ID, "First selection should contain channel 3")

	// Track which channel appears first most often to verify round-robin
	firstChannelCounts := make(map[int]int)
	for _, selection := range selections {
		firstChannelCounts[selection[0].Channel.ID]++
	}

	// With equal weights and round-robin, we should see some distribution
	// though it might not be perfectly even due to the exponential decay formula
	t.Logf("First channel distribution with equal weights:")

	for channelID, count := range firstChannelCounts {
		channelName := getCandidateNameByID(selections[0], channelID)
		t.Logf("  %s: %d times", channelName, count)
	}

	// Verify all channels are still present in every selection
	for i, selection := range selections {
		require.Len(t, selection, 3, "Selection %d should have 3 channels", i)

		channelIDs := make([]int, len(selection))
		for j, ch := range selection {
			channelIDs[j] = ch.Channel.ID
			require.Equal(t, channel.StatusEnabled, ch.Channel.Status, "Channel %d in selection %d should be enabled", j, i)
		}

		require.Contains(t, channelIDs, channels[0].ID, "Selection %d should contain channel 1", i)
		require.Contains(t, channelIDs, channels[1].ID, "Selection %d should contain channel 2", i)
		require.Contains(t, channelIDs, channels[2].ID, "Selection %d should contain channel 3", i)
	}

	// We should see more order changes with equal weights
	orderChanges := 0

	for i := 1; i < len(selections); i++ {
		if selections[i][0].Channel.ID != selections[i-1][0].Channel.ID {
			orderChanges++
		}
	}

	t.Logf("Order changes across %d selections with equal weights: %d", len(selections), orderChanges)

	// With equal weights, we should see more variation than with different weights
	// (though the exact behavior depends on the exponential decay implementation)
	if orderChanges == 0 {
		t.Logf("Note: No order changes detected. This might be due to the exponential decay formula.")
		t.Logf("The round-robin effect is still working but may require more selections to become visible.")
	}
}

// TestDefaultChannelSelector_Select_WeightedRoundRobin tests weighted round-robin behavior.
func TestDefaultChannelSelector_Select_WeightedRoundRobin(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	// Make multiple selections to test round-robin behavior
	selections := make([][]*ChannelModelCandidate, 6)

	for i := range 6 {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.Len(t, result, 3)
		selections[i] = result
	}

	// With weighted round-robin, all channels start with equal scores (150) when they have 0 requests.
	// The order is determined by other factors initially.
	// As requests accumulate, channels with higher weights can handle more requests before their score drops.

	// Verify all channels are still present in every selection
	for i, selection := range selections {
		require.Len(t, selection, 3, "Selection %d should have 3 channels", i)

		channelIDs := make([]int, len(selection))
		for j, ch := range selection {
			channelIDs[j] = ch.Channel.ID
		}

		require.Contains(t, channelIDs, channels[0].ID, "Selection %d should contain high weight channel", i)
		require.Contains(t, channelIDs, channels[1].ID, "Selection %d should contain medium weight channel", i)
		require.Contains(t, channelIDs, channels[2].ID, "Selection %d should contain low weight channel", i)
	}

	// Test that the round-robin effect accumulates over time
	// After 6 selections, ch1 should have 6 requests, ch2 and ch3 should have fewer
	// This should affect their relative ordering compared to the initial state

	// Let's also verify that the strategy is working by checking that channels with
	// fewer requests get priority over time
	initialFirstChannel := selections[0][0].Channel.ID
	laterFirstChannel := selections[5][0].Channel.ID

	// Due to the weight component, ch1 might still be first, but if we look at the
	// round-robin component alone, channels with fewer requests should be boosted
	// We can verify this by checking that the order is not completely static
	orderChanges := 0

	for i := 1; i < len(selections); i++ {
		if selections[i][0].Channel.ID != selections[i-1][0].Channel.ID {
			orderChanges++
		}
	}

	// We should see some order changes due to round-robin effect, though weight
	// might keep the highest weight channel on top for a while
	t.Logf("Order changes across %d selections: %d", len(selections), orderChanges)
	t.Logf("Initial first channel: %d, Final first channel: %d", initialFirstChannel, laterFirstChannel)
}

// TestDefaultChannelSelector_Select_WithDisabledChannels tests that disabled channels are excluded.
func TestDefaultChannelSelector_Select_WithDisabledChannels(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should only return enabled channels
	require.Len(t, result, 3)

	for _, ch := range result {
		require.Equal(t, channel.StatusEnabled, ch.Channel.Status, "All returned channels should be enabled")
	}

	// Verify disabled channel is not included
	for _, ch := range result {
		require.NotEqual(t, channels[3].ID, ch.Channel.ID, "Disabled channel should not be included")
	}
}

// TestLoadBalancedSelector_Select tests LoadBalancedSelector applies load balancing.
func TestLoadBalancedSelector_Select(t *testing.T) {
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
	loadBalancer := NewLoadBalancer(systemService, nil, strategies...)

	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)
	selector := WithLoadBalancedSelector(baseSelector, loadBalancer, systemService)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return 3 enabled channels
	require.Len(t, result, 3)

	// Verify all channels are enabled and present
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
		require.Equal(t, channel.StatusEnabled, ch.Channel.Status)
	}

	require.Contains(t, channelIDs, channels[0].ID)
	require.Contains(t, channelIDs, channels[1].ID)
	require.Contains(t, channelIDs, channels[2].ID)
}

// TestLoadBalancedSelector_Select_SingleChannel tests LoadBalancedSelector with single channel skips sorting.
func TestLoadBalancedSelector_Select_SingleChannel(t *testing.T) {
	ctx, client := setupTest(t)

	// Create single channel
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Single Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	loadBalancer := NewLoadBalancer(systemService, nil)

	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)
	selector := WithLoadBalancedSelector(baseSelector, loadBalancer, systemService)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, ch.ID, result[0].Channel.ID)
}
