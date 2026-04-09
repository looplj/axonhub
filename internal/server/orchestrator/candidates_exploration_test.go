package orchestrator

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestNeedsExploration_DetectsColdChannel(t *testing.T) {
	ctx := context.Background()

	perfStrategy := &PerformanceAwareStrategy{
		maxScore: 150.0,
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			return nil, nil
		},
	}

	loadBalancer := NewLoadBalancer(nil, nil, perfStrategy)
	selector := &LoadBalancedSelector{
		loadBalancer: loadBalancer,
	}

	candidate := &ChannelModelsCandidate{
		Channel: &biz.Channel{
			Channel: &ent.Channel{ID: 1, Name: "cold-channel"},
		},
	}

	require.True(t, selector.needsExploration(ctx, candidate, "gpt-4"), "channel with no metrics should need exploration")
}

func TestNeedsExploration_SkipsWarmChannel(t *testing.T) {
	ctx := context.Background()

	perfStrategy := &PerformanceAwareStrategy{
		maxScore: 150.0,
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			if model == "gpt-4" {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 100
				return m, nil
			}
			return nil, nil
		},
	}

	loadBalancer := NewLoadBalancer(nil, nil, perfStrategy)
	selector := &LoadBalancedSelector{
		loadBalancer: loadBalancer,
	}

	candidate := &ChannelModelsCandidate{
		Channel: &biz.Channel{
			Channel: &ent.Channel{ID: 1, Name: "warm-channel"},
		},
	}

	require.False(t, selector.needsExploration(ctx, candidate, "gpt-4"), "channel with metrics should not need exploration")
}

func TestSelectExplorationCandidate_RoundRobin(t *testing.T) {
	selector := &LoadBalancedSelector{
		explorationState: struct {
			mu     sync.Mutex
			counts map[string]int
		}{
			counts: make(map[string]int),
		},
	}

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "channel-1"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "channel-2"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "channel-3"}}},
	}

	model := "gpt-4"
	selectedIDs := make([]int, 6)
	for i := 0; i < 6; i++ {
		candidate := selector.selectExplorationCandidate(model, candidates)
		selectedIDs[i] = candidate.Channel.ID
	}

	expected := []int{1, 2, 3, 1, 2, 3}
	require.Equal(t, expected, selectedIDs, "round-robin should cycle through candidates")
}

func TestSelectExplorationCandidate_PerModelIsolation(t *testing.T) {
	selector := &LoadBalancedSelector{
		explorationState: struct {
			mu     sync.Mutex
			counts map[string]int
		}{
			counts: make(map[string]int),
		},
	}

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2}}},
	}

	c1 := selector.selectExplorationCandidate("model-1", candidates)
	require.Equal(t, 1, c1.Channel.ID)

	c2 := selector.selectExplorationCandidate("model-1", candidates)
	require.Equal(t, 2, c2.Channel.ID)

	c3 := selector.selectExplorationCandidate("model-2", candidates)
	require.Equal(t, 1, c3.Channel.ID, "different model should have isolated counter")
}

func TestSelectExplorationCandidate_ThreadSafety(t *testing.T) {
	selector := &LoadBalancedSelector{
		explorationState: struct {
			mu     sync.Mutex
			counts map[string]int
		}{
			counts: make(map[string]int),
		},
	}

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3}}},
	}

	const goroutines = 10
	const selectionsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	counts := make(map[int]int)
	var countsMu sync.Mutex

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < selectionsPerGoroutine; j++ {
				candidate := selector.selectExplorationCandidate("gpt-4", candidates)
				countsMu.Lock()
				counts[candidate.Channel.ID]++
				countsMu.Unlock()
			}
		}()
	}

	wg.Wait()

	totalSelections := goroutines * selectionsPerGoroutine
	for _, count := range counts {
		require.InDelta(t, float64(totalSelections)/3, float64(count), float64(totalSelections)/10,
			"distribution should be roughly equal across channels")
	}
}

func TestExplorationMechanism_Integration(t *testing.T) {
	ctx, client := setupTest(t)

	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Hot Channel").
		SetBaseURL("https://api.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(100).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Warm Channel").
		SetBaseURL("https://api2.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Cold Channel").
		SetBaseURL("https://api3.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key3"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(10).
		Save(ctx)
	require.NoError(t, err)

	perfStrategy := &PerformanceAwareStrategy{
		maxScore: 150.0,
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			if channelID == ch3.ID {
				return nil, nil
			}
			m := &biz.AggregatedMetrics{}
			m.RequestCount = 100
			return m, nil
		},
	}

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	modelService := newTestModelService(client)

	loadBalancer := NewLoadBalancer(systemService, nil, perfStrategy, NewWeightRoundRobinStrategy(channelService))
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)
	selector := WithLoadBalancedSelector(baseSelector, loadBalancer, systemService, systemService, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)

	channelIDs := make([]int, len(result))
	for i, c := range result {
		channelIDs[i] = c.Channel.ID
	}

	require.Contains(t, channelIDs, ch1.ID, "hot channel should be included")
	require.Contains(t, channelIDs, ch2.ID, "warm channel should be included")
	require.Contains(t, channelIDs, ch3.ID, "cold channel should be included via exploration")
}

func TestExplorationMechanism_NoExplorationWhenAllWarm(t *testing.T) {
	ctx, client := setupTest(t)

	for i := 0; i < 3; i++ {
		_, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName(string(rune('W' + i))).
			SetBaseURL("https://api.example.com").
			SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetStatus(channel.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)
	}

	perfStrategy := &PerformanceAwareStrategy{
		maxScore: 150.0,
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			m := &biz.AggregatedMetrics{}
			m.RequestCount = 100
			return m, nil
		},
	}

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	modelService := newTestModelService(client)

	loadBalancer := NewLoadBalancer(systemService, nil, perfStrategy, NewWeightRoundRobinStrategy(channelService))
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)
	selector := WithLoadBalancedSelector(baseSelector, loadBalancer, systemService, systemService, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 3)
}

func TestExplorationMechanism_RequiresMultipleSlots(t *testing.T) {
	ctx, client := setupTest(t)

	for i := 0; i < 3; i++ {
		_, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName(string(rune('X' + i))).
			SetBaseURL("https://api.example.com").
			SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetStatus(channel.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)
	}

	ch4, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Cold Channel").
		SetBaseURL("https://api4.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key4"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	perfStrategy := &PerformanceAwareStrategy{
		maxScore: 150.0,
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			if channelID == ch4.ID {
				return nil, nil
			}
			m := &biz.AggregatedMetrics{}
			m.RequestCount = 100
			return m, nil
		},
	}

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	modelService := newTestModelService(client)

	loadBalancer := NewLoadBalancer(systemService, nil, perfStrategy, NewWeightRoundRobinStrategy(channelService))
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)
	selector := WithLoadBalancedSelector(baseSelector, loadBalancer, systemService, systemService, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	_ = result
}
