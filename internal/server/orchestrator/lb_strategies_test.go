package orchestrator

import (
	"context"

	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

// mockStrategy is a test strategy that returns a fixed score.
type mockStrategy struct {
	name  string
	score float64
}

func (m *mockStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	return m.score
}

func (m *mockStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	return m.score, StrategyScore{
		StrategyName: m.name,
		Score:        m.score,
		Details:      map[string]any{"fixed_score": m.score},
	}
}

func (m *mockStrategy) Name() string {
	return m.name
}

// mockMetricsProvider is a mock implementation of ChannelMetricsProvider for testing.
type mockMetricsProvider struct {
	metrics map[int]*biz.AggregatedMetrics
	err     error
}

func (m *mockMetricsProvider) GetChannelMetrics(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error) {
	if m.err != nil {
		return nil, m.err
	}

	if metrics, ok := m.metrics[channelID]; ok {
		return metrics, nil
	}

	return &biz.AggregatedMetrics{}, nil
}

type mockRetryPolicyProvider struct {
	policy *biz.RetryPolicy
}

func (m *mockRetryPolicyProvider) RetryPolicyOrDefault(ctx context.Context) *biz.RetryPolicy {
	return m.policy
}

type mockSelectionTracker struct {
	selections map[int]int
}

func (m *mockSelectionTracker) IncrementChannelSelection(channelID int) {
	if m.selections == nil {
		m.selections = make(map[int]int)
	}

	m.selections[channelID]++
}

// mockTraceProvider is a mock implementation of ChannelTraceProvider for testing.
type mockTraceProvider struct {
	lastSuccessChannel map[int]int // traceID -> channelID
	err                error
}

func (m *mockTraceProvider) GetLastSuccessfulChannelID(ctx context.Context, traceID int) (int, error) {
	if m.err != nil {
		return 0, m.err
	}

	if channelID, ok := m.lastSuccessChannel[traceID]; ok {
		return channelID, nil
	}

	return 0, nil
}

// newTestChannelService creates a minimal channel service for testing.
// It bypasses the normal initialization to avoid requiring a ScheduledExecutor.
func newTestChannelService(client *ent.Client) *biz.ChannelService {
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})

	return biz.NewChannelService(biz.ChannelServiceParams{
		Executor:      executors.NewPoolScheduleExecutor(),
		Ent:           client,
		SystemService: systemService,
	})
}

// newTestRequestService creates a minimal request service for testing.
func newTestRequestService(client *ent.Client) *biz.RequestService {
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	dataStorageService := biz.NewDataStorageService(biz.DataStorageServiceParams{
		Client:        client,
		SystemService: systemService,
		CacheConfig:   xcache.Config{},
		Executor:      executors.NewPoolScheduleExecutor(),
	})
	channelService := biz.NewChannelServiceForTest(client)
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	return biz.NewRequestService(client, systemService, usageLogService, dataStorageService)
}
