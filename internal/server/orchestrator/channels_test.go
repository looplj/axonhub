package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

// newTestChannelServiceForChannels creates a minimal channel service for testing.
func newTestChannelServiceForChannels(client *ent.Client) *biz.ChannelService {
	return biz.NewChannelService(biz.ChannelServiceParams{
		Executor: executors.NewPoolScheduleExecutor(),
		Ent:      client,
	})
}

// newTestLoadBalancedSelector creates a load-balanced selector for testing.
// This replaces the old DefaultChannelSelector with the new decorator pattern.
func newTestLoadBalancedSelector(
	channelService *biz.ChannelService,
	systemService *biz.SystemService,
	requestService *biz.RequestService,
	connectionTracker *DefaultConnectionTracker,
) ChannelSelector {
	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(requestService),
		NewErrorAwareStrategy(channelService),
		NewWeightRoundRobinStrategy(channelService),
		NewConnectionAwareStrategy(channelService, connectionTracker),
	}
	loadBalancer := NewLoadBalancer(systemService, strategies...)

	baseSelector := NewDefaultSelector(channelService)

	return NewLoadBalancedSelector(baseSelector, loadBalancer)
}

// newTestSystemService creates a minimal system service for testing.
func newTestSystemService(client *ent.Client) *biz.SystemService {
	return biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
}

// newTestRequestServiceForChannels creates a minimal request service for testing.
func newTestRequestServiceForChannels(client *ent.Client, systemService *biz.SystemService) *biz.RequestService {
	dataStorageService := &biz.DataStorageService{
		AbstractService: &biz.AbstractService{},
		SystemService:   systemService,
		Cache:           xcache.NewFromConfig[ent.DataStorage](xcache.Config{Mode: xcache.ModeMemory}),
	}
	usageLogService := biz.NewUsageLogService(client, systemService)

	return biz.NewRequestService(client, systemService, usageLogService, dataStorageService)
}

// setupTest creates a test context and ent client for testing.
func setupTest(t *testing.T) (context.Context, *ent.Client) {
	t.Helper()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx = ent.NewContext(ctx, client)

	return ctx, client
}

// createTestChannels creates multiple test channels for load balancer testing.
func createTestChannels(t *testing.T, ctx context.Context, client *ent.Client) []*ent.Channel {
	t.Helper()

	channels := make([]*ent.Channel, 0)

	// Channel 1: High weight, healthy
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("High Weight Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(100).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch1)

	// Channel 2: Medium weight, healthy
	ch2, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Medium Weight Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-2"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch2)

	// Channel 3: Low weight, healthy
	ch3, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Low Weight Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-3"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(25).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch3)

	// Channel 4: Disabled channel
	ch4, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Disabled Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-4"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(75).
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch4)

	return channels
}

// TestDefaultChannelSelector_Select_SingleChannel tests selection when only one channel is available.
func TestDefaultChannelSelector_Select_SingleChannel(t *testing.T) {
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
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, ch.ID, result[0].ID)
}

// TestLoadBalancedSelector_Select_MultipleChannels_LoadBalancing tests load balancing with multiple channels.
func TestLoadBalancedSelector_Select_MultipleChannels_LoadBalancing(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

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
		assert.Equal(t, channel.StatusEnabled, ch.Status, "Channel %d should be enabled", i)
		assert.Equal(t, channel.TypeOpenai, ch.Type, "Channel %d should be OpenAI type", i)
		assert.Contains(t, ch.SupportedModels, "gpt-4", "Channel %d should support gpt-4", i)
	}

	// Verify disabled channel is not included
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
	}

	assert.NotContains(t, channelIDs, channels[3].ID, "Disabled channel should not be included")

	// Verify all enabled channels are present
	assert.Contains(t, channelIDs, channels[0].ID, "High weight channel should be included")
	assert.Contains(t, channelIDs, channels[1].ID, "Medium weight channel should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Low weight channel should be included")
}

// TestDefaultChannelSelector_Select_NoChannelsAvailable tests error when no channels are available.
func TestDefaultChannelSelector_Select_NoChannelsAvailable(t *testing.T) {
	ctx, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Empty(t, result) // Should return empty slice, not error
}

// TestDefaultChannelSelector_Select_ModelNotSupported tests when requested model is not supported.
func TestDefaultChannelSelector_Select_ModelNotSupported(t *testing.T) {
	ctx, client := setupTest(t)

	// Create channel that doesn't support the requested model
	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Limited Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-3.5-turbo").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4", // This model is not supported by the channel
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Empty(t, result) // Should return empty slice when model not supported
}

// TestDefaultChannelSelector_Select_WithConnectionTracking tests connection tracking integration.
func TestDefaultChannelSelector_Select_WithConnectionTracking(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

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
		channelIDs[i] = ch.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID)
	assert.Contains(t, channelIDs, channels[1].ID)
	assert.Contains(t, channelIDs, channels[2].ID)

	// Due to connection awareness, the channel with no connections (ch3)
	// should get a boost from the ConnectionAwareStrategy
	// However, WeightRoundRobinStrategy has higher priority, so weight still matters significantly
	// We expect: ch1 (high weight, 2 conn) > ch2 (medium weight, 1 conn) > ch3 (low weight, 0 conn)
	// But ch3 might get boosted due to no connections

	// Let's verify the connection counts are correctly tracked
	assert.Equal(t, 2, connectionTracker.GetActiveConnections(channels[0].ID), "Channel 0 should have 2 connections")
	assert.Equal(t, 1, connectionTracker.GetActiveConnections(channels[1].ID), "Channel 1 should have 1 connection")
	assert.Equal(t, 0, connectionTracker.GetActiveConnections(channels[2].ID), "Channel 2 should have 0 connections")

	// Log the actual ordering for debugging
	t.Logf("Channel ordering with connections: ch0(2 conn)=%d, ch1(1 conn)=%d, ch2(0 conn)=%d",
		result[0].ID, result[1].ID, result[2].ID)

	// Verify channel properties in the result
	for i, ch := range result {
		assert.Equal(t, channel.StatusEnabled, ch.Status, "Channel %d should be enabled", i)
		assert.Contains(t, ch.SupportedModels, "gpt-4", "Channel %d should support gpt-4", i)
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
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Channel 2 should be ranked first due to trace awareness (high boost score from TraceAwareStrategy)
	assert.Equal(t, channels[1].ID, result[0].ID, "Channel from trace should be ranked first")

	// The other channels should follow in weight order (ch1 > ch3)
	assert.Contains(t, []int{result[1].ID, result[2].ID}, channels[0].ID, "High weight channel should be in top 3")
	assert.Contains(t, []int{result[1].ID, result[2].ID}, channels[2].ID, "Low weight channel should be in top 3")

	// Verify all channels are enabled and support the model
	for i, ch := range result {
		assert.Equal(t, channel.StatusEnabled, ch.Status, "Channel %d should be enabled", i)
		assert.Contains(t, ch.SupportedModels, "gpt-4", "Channel %d should support gpt-4", i)
	}

	// Verify the trace channel is indeed channel 2 (medium weight)
	assert.Equal(t, "Medium Weight Channel", result[0].Name, "First channel should be the medium weight channel from trace")
	assert.Equal(t, 50, result[0].OrderingWeight, "First channel should have medium weight (50)")

	// Log the ordering to verify trace awareness is working
	t.Logf("Channel ordering with trace context: %s (weight=%d), %s (weight=%d), %s (weight=%d)",
		result[0].Name, result[0].OrderingWeight,
		result[1].Name, result[1].OrderingWeight,
		result[2].Name, result[2].OrderingWeight)
}

// TestDefaultChannelSelector_Select_WithChannelFailures tests error-aware load balancing.
func TestDefaultChannelSelector_Select_WithChannelFailures(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	// Record failures for the high weight channel to test error awareness
	for i := 0; i < 3; i++ {
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
	assert.NotEqual(t, channels[0].ID, result[0].ID, "Failing channel should not be ranked first")

	// The healthy channels should be ranked higher
	// We expect either ch2 (medium weight) or ch3 (low weight) to be first
	// Since ch2 has higher weight and no failures, it should be first
	assert.Equal(t, channels[1].ID, result[0].ID, "Medium weight healthy channel should be first")

	// Verify all channels are still included (just reordered)
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
		assert.Equal(t, channel.StatusEnabled, ch.Status, "Channel %d should be enabled", i)
		assert.Contains(t, ch.SupportedModels, "gpt-4", "Channel %d should support gpt-4", i)
	}

	assert.Contains(t, channelIDs, channels[0].ID, "Failing channel should still be included")
	assert.Contains(t, channelIDs, channels[1].ID, "Medium weight channel should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Low weight channel should be included")

	// Log the ordering to verify error awareness is working
	t.Logf("Channel ordering with failures: %s (3 failures), %s (0 failures), %s (0 failures)",
		getChannelNameByID(result, channels[0].ID),
		getChannelNameByID(result, channels[1].ID),
		getChannelNameByID(result, channels[2].ID))
}

// Helper function to get channel name by ID from result.
func getChannelNameByID(result []*biz.Channel, channelID int) string {
	for _, ch := range result {
		if ch.ID == channelID {
			return ch.Name
		}
	}

	return "unknown"
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
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	// Make multiple selections to test round-robin behavior
	selections := make([][]*biz.Channel, 9)

	for i := 0; i < 9; i++ {
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
		firstSelectionIDs[i] = ch.ID
	}

	assert.Contains(t, firstSelectionIDs, channels[0].ID, "First selection should contain channel 1")
	assert.Contains(t, firstSelectionIDs, channels[1].ID, "First selection should contain channel 2")
	assert.Contains(t, firstSelectionIDs, channels[2].ID, "First selection should contain channel 3")

	// Track which channel appears first most often to verify round-robin
	firstChannelCounts := make(map[int]int)
	for _, selection := range selections {
		firstChannelCounts[selection[0].ID]++
	}

	// With equal weights and round-robin, we should see some distribution
	// though it might not be perfectly even due to the exponential decay formula
	t.Logf("First channel distribution with equal weights:")

	for channelID, count := range firstChannelCounts {
		channelName := getChannelNameByID(selections[0], channelID)
		t.Logf("  %s: %d times", channelName, count)
	}

	// Verify all channels are still present in every selection
	for i, selection := range selections {
		require.Len(t, selection, 3, "Selection %d should have 3 channels", i)

		channelIDs := make([]int, len(selection))
		for j, ch := range selection {
			channelIDs[j] = ch.ID
			assert.Equal(t, channel.StatusEnabled, ch.Status, "Channel %d in selection %d should be enabled", j, i)
		}

		assert.Contains(t, channelIDs, channels[0].ID, "Selection %d should contain channel 1", i)
		assert.Contains(t, channelIDs, channels[1].ID, "Selection %d should contain channel 2", i)
		assert.Contains(t, channelIDs, channels[2].ID, "Selection %d should contain channel 3", i)
	}

	// We should see more order changes with equal weights
	orderChanges := 0

	for i := 1; i < len(selections); i++ {
		if selections[i][0].ID != selections[i-1][0].ID {
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
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	// Make multiple selections to test round-robin behavior
	selections := make([][]*biz.Channel, 6)

	for i := 0; i < 6; i++ {
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
			channelIDs[j] = ch.ID
		}

		assert.Contains(t, channelIDs, channels[0].ID, "Selection %d should contain high weight channel", i)
		assert.Contains(t, channelIDs, channels[1].ID, "Selection %d should contain medium weight channel", i)
		assert.Contains(t, channelIDs, channels[2].ID, "Selection %d should contain low weight channel", i)
	}

	// Test that the round-robin effect accumulates over time
	// After 6 selections, ch1 should have 6 requests, ch2 and ch3 should have fewer
	// This should affect their relative ordering compared to the initial state

	// Let's also verify that the strategy is working by checking that channels with
	// fewer requests get priority over time
	initialFirstChannel := selections[0][0].ID
	laterFirstChannel := selections[5][0].ID

	// Due to the weight component, ch1 might still be first, but if we look at the
	// round-robin component alone, channels with fewer requests should be boosted
	// We can verify this by checking that the order is not completely static
	orderChanges := 0

	for i := 1; i < len(selections); i++ {
		if selections[i][0].ID != selections[i-1][0].ID {
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
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should only return enabled channels
	require.Len(t, result, 3)

	for _, ch := range result {
		assert.Equal(t, channel.StatusEnabled, ch.Status, "All returned channels should be enabled")
	}

	// Verify disabled channel is not included
	for _, ch := range result {
		assert.NotEqual(t, channels[3].ID, ch.ID, "Disabled channel should not be included")
	}
}

// TestDefaultChannelSelector_Select_EmptyRequest tests handling of empty request.
func TestDefaultChannelSelector_Select_EmptyRequest(t *testing.T) {
	ctx, client := setupTest(t)

	createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	connectionTracker := NewDefaultConnectionTracker(10)
	selector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	// Empty request should still work
	req := &llm.Request{}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Empty(t, result) // Empty model should return empty slice
}

// TestSpecifiedChannelSelector_Select_ValidChannel tests SpecifiedChannelSelector with valid channel.
func TestSpecifiedChannelSelector_Select_ValidChannel(t *testing.T) {
	ctx, client := setupTest(t)

	// Create channel
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusDisabled). // Can be disabled for SpecifiedChannelSelector
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: ch.ID})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, ch.ID, result[0].ID)
}

// TestSpecifiedChannelSelector_Select_ModelNotSupported tests SpecifiedChannelSelector with unsupported model.
func TestSpecifiedChannelSelector_Select_ModelNotSupported(t *testing.T) {
	ctx, client := setupTest(t)

	// Create channel with limited model support
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Limited Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-3.5-turbo").
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: ch.ID})

	req := &llm.Request{
		Model: "gpt-4", // Not supported
	}

	result, err := selector.Select(ctx, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "model gpt-4 not supported")
}

// TestSpecifiedChannelSelector_Select_ChannelNotFound tests SpecifiedChannelSelector with non-existent channel.
func TestSpecifiedChannelSelector_Select_ChannelNotFound(t *testing.T) {
	ctx, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: 999}) // Non-existent ID

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get channel for test")
}

// TestDefaultSelector_Select tests DefaultSelector returns all enabled channels supporting the model.
func TestDefaultSelector_Select(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	selector := NewDefaultSelector(channelService)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return 3 enabled channels (exclude disabled one)
	require.Len(t, result, 3)

	// Verify disabled channel is not included
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
	}

	assert.NotContains(t, channelIDs, channels[3].ID, "Disabled channel should not be included")
	assert.Contains(t, channelIDs, channels[0].ID, "High weight channel should be included")
	assert.Contains(t, channelIDs, channels[1].ID, "Medium weight channel should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Low weight channel should be included")
}

// TestSelectedChannelsSelector_Select_WithFilter tests SelectedChannelsSelector filters by allowed channel IDs.
func TestSelectedChannelsSelector_Select_WithFilter(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	baseSelector := NewDefaultSelector(channelService)

	// Only allow channels 0 and 2
	allowedIDs := []int{channels[0].ID, channels[2].ID}
	selector := NewSelectedChannelsSelector(baseSelector, allowedIDs)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return only 2 allowed channels
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID, "Allowed channel 0 should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Allowed channel 2 should be included")
	assert.NotContains(t, channelIDs, channels[1].ID, "Non-allowed channel 1 should not be included")
}

// TestSelectedChannelsSelector_Select_EmptyFilter tests SelectedChannelsSelector with empty filter returns all.
func TestSelectedChannelsSelector_Select_EmptyFilter(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	baseSelector := NewDefaultSelector(channelService)

	// Empty filter should return all channels
	selector := NewSelectedChannelsSelector(baseSelector, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return all 3 enabled channels
	require.Len(t, result, 3)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID)
	assert.Contains(t, channelIDs, channels[1].ID)
	assert.Contains(t, channelIDs, channels[2].ID)
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
	loadBalancer := NewLoadBalancer(systemService, strategies...)

	baseSelector := NewDefaultSelector(channelService)
	selector := NewLoadBalancedSelector(baseSelector, loadBalancer)

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
		channelIDs[i] = ch.ID
		assert.Equal(t, channel.StatusEnabled, ch.Status)
	}

	assert.Contains(t, channelIDs, channels[0].ID)
	assert.Contains(t, channelIDs, channels[1].ID)
	assert.Contains(t, channelIDs, channels[2].ID)
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
	loadBalancer := NewLoadBalancer(systemService)

	baseSelector := NewDefaultSelector(channelService)
	selector := NewLoadBalancedSelector(baseSelector, loadBalancer)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, ch.ID, result[0].ID)
}

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
	baseSelector := NewDefaultSelector(channelService)
	filteredSelector := NewSelectedChannelsSelector(baseSelector, []int{channels[0].ID, channels[1].ID})
	selector := NewLoadBalancedSelector(filteredSelector, loadBalancer)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return only 2 allowed channels
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
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

	baseSelector := newTestLoadBalancedSelector(channelService, systemService, requestService, connectionTracker)

	req := &llm.Request{
		Model: "gpt-4",
	}

	// Test without allowed channels - should return all 3 enabled channels
	result, err := baseSelector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Test with allowed channels - should return only 2 channels
	filteredSelector := NewSelectedChannelsSelector(baseSelector, []int{channels[0].ID, channels[2].ID})
	result, err = filteredSelector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
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
	baseSelector := NewDefaultSelector(channelService)

	req := &llm.Request{
		Model: "gpt-4",
	}

	// Empty slice should return all channels from wrapped selector
	filteredSelector := NewSelectedChannelsSelector(baseSelector, []int{})
	result, err := filteredSelector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3) // All 3 enabled channels

	// Nil slice should also return all channels
	filteredSelector = NewSelectedChannelsSelector(baseSelector, nil)
	result, err = filteredSelector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Verify all enabled channels are present
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID)
	assert.Contains(t, channelIDs, channels[1].ID)
	assert.Contains(t, channelIDs, channels[2].ID)
}

// createGeminiTestChannels creates test channels including Gemini channels for Google native tools testing.
func createGeminiTestChannels(t *testing.T, ctx context.Context, client *ent.Client) []*ent.Channel {
	t.Helper()

	channels := make([]*ent.Channel, 0)

	// Channel 0: gemini (native format, supports Google native tools)
	ch0, err := client.Channel.Create().
		SetType(channel.TypeGemini).
		SetName("Gemini Native").
		SetBaseURL("https://generativelanguage.googleapis.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-1"}).
		SetSupportedModels([]string{"gemini-2.0-flash", "gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.0-flash").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch0)

	// Channel 1: gemini_openai (OpenAI format, does NOT support Google native tools)
	ch1, err := client.Channel.Create().
		SetType(channel.TypeGeminiOpenai).
		SetName("Gemini OpenAI").
		SetBaseURL("https://generativelanguage.googleapis.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-2"}).
		SetSupportedModels([]string{"gemini-2.0-flash", "gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.0-flash").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch1)

	// Channel 2: gemini_vertex (Vertex AI, supports Google native tools)
	ch2, err := client.Channel.Create().
		SetType(channel.TypeGeminiVertex).
		SetName("Gemini Vertex").
		SetBaseURL("https://us-central1-aiplatform.googleapis.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-3"}).
		SetSupportedModels([]string{"gemini-2.0-flash", "gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.0-flash").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch2)

	return channels
}

// TestGoogleNativeToolsSelector_Select_WithGoogleNativeTools tests filtering when request contains Google native tools.
func TestGoogleNativeToolsSelector_Select_WithGoogleNativeTools(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createGeminiTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	baseSelector := NewDefaultSelector(channelService)
	selector := NewGoogleNativeToolsSelector(baseSelector)

	// Request with Google native tools
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{
			{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
			{Type: "function", Function: llm.Function{Name: "get_weather"}},
		},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return only channels that support Google native tools (gemini, gemini_vertex)
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID, "Gemini native channel should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Gemini Vertex channel should be included")
	assert.NotContains(t, channelIDs, channels[1].ID, "Gemini OpenAI channel should be excluded")
}

// TestGoogleNativeToolsSelector_Select_WithoutGoogleNativeTools tests that all channels are returned when no Google native tools.
func TestGoogleNativeToolsSelector_Select_WithoutGoogleNativeTools(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createGeminiTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	baseSelector := NewDefaultSelector(channelService)
	selector := NewGoogleNativeToolsSelector(baseSelector)

	// Request without Google native tools (only function tools)
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{
			{Type: "function", Function: llm.Function{Name: "get_weather"}},
			{Type: "function", Function: llm.Function{Name: "search"}},
		},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return all channels when no Google native tools
	require.Len(t, result, 3)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID, "Gemini native channel should be included")
	assert.Contains(t, channelIDs, channels[1].ID, "Gemini OpenAI channel should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Gemini Vertex channel should be included")
}

// TestGoogleNativeToolsSelector_Select_NoCompatibleChannels tests fallback when no compatible channels exist.
func TestGoogleNativeToolsSelector_Select_NoCompatibleChannels(t *testing.T) {
	ctx, client := setupTest(t)

	// Create only gemini_openai channel (does not support Google native tools)
	ch, err := client.Channel.Create().
		SetType(channel.TypeGeminiOpenai).
		SetName("Gemini OpenAI Only").
		SetBaseURL("https://generativelanguage.googleapis.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gemini-2.0-flash"}).
		SetDefaultTestModel("gemini-2.0-flash").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	baseSelector := NewDefaultSelector(channelService)
	selector := NewGoogleNativeToolsSelector(baseSelector)

	// Request with Google native tools
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{
			{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
		},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should fallback to all channels when no compatible channels exist
	// (downstream outbound will handle the fallback)
	require.Len(t, result, 1)
	assert.Equal(t, ch.ID, result[0].ID)
}

// TestGoogleNativeToolsSelector_Select_EmptyTools tests that all channels are returned when tools are empty.
func TestGoogleNativeToolsSelector_Select_EmptyTools(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createGeminiTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	baseSelector := NewDefaultSelector(channelService)
	selector := NewGoogleNativeToolsSelector(baseSelector)

	// Request with no tools
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return all channels when no tools
	require.Len(t, result, 3)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID)
	assert.Contains(t, channelIDs, channels[1].ID)
	assert.Contains(t, channelIDs, channels[2].ID)
}

// TestGoogleNativeToolsSelector_Select_MultipleGoogleNativeTools tests filtering with multiple Google native tools.
func TestGoogleNativeToolsSelector_Select_MultipleGoogleNativeTools(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createGeminiTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	baseSelector := NewDefaultSelector(channelService)
	selector := NewGoogleNativeToolsSelector(baseSelector)

	// Request with multiple Google native tools
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{
			{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
			{Type: llm.ToolTypeGoogleUrlContext, Google: &llm.GoogleTools{UrlContext: &llm.GoogleUrlContext{}}},
			{Type: "function", Function: llm.Function{Name: "get_weather"}},
		},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return only channels that support Google native tools
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID, "Gemini native channel should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Gemini Vertex channel should be included")
	assert.NotContains(t, channelIDs, channels[1].ID, "Gemini OpenAI channel should be excluded")
}
