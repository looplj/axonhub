package biz

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xtime"
	"github.com/looplj/axonhub/llm/oauth"
)

// startPerformanceProcess starts the background goroutine to flush metrics to database.
func (svc *ChannelService) startPerformanceProcess() {
	ctx := authz.WithSystemBypass(context.Background(), "channel-record-performance")
	for perf := range svc.perfCh {
		svc.RecordPerformance(ctx, perf)
	}
}

func (svc *ChannelService) runSyncChannelModelsPeriodically(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "channel-run-model-sync")
	setting := svc.SystemService.ChannelSettingOrDefault(ctx)
	if !svc.shouldRunModelSync(xtime.UTCNow(), setting.AutoSync.Frequency) {
		return
	}

	svc.syncChannelModels(ctx)
}

func (svc *ChannelService) shouldRunModelSync(now time.Time, frequency AutoSyncFrequency) bool {
	intervalMinutes := getIntervalMinutesFromAutoSyncFrequency(frequency)
	alignedTime := now.Truncate(time.Duration(intervalMinutes) * time.Minute)

	svc.modelSyncMu.Lock()
	defer svc.modelSyncMu.Unlock()

	if !svc.lastModelSyncExecutionTime.IsZero() && svc.lastModelSyncExecutionTime.Equal(alignedTime) {
		return false
	}

	svc.lastModelSyncExecutionTime = alignedTime
	return true
}

func getIntervalMinutesFromAutoSyncFrequency(frequency AutoSyncFrequency) int {
	switch frequency {
	case AutoSyncFrequencyOneHour:
		return 60
	case AutoSyncFrequencySixHours:
		return 360
	case AutoSyncFrequencyOneDay:
		return 1440
	default:
		return 60
	}
}

func (svc *ChannelService) onCacheRefreshed(ctx context.Context, current []*Channel, lastUpdate time.Time) ([]*Channel, time.Time, bool, error) {
	ctx = authz.WithSystemBypass(ctx, "channel-refresh-cache")
	return svc.reloadEnabledChannels(ctx, current, lastUpdate)
}

func (svc *ChannelService) onTokenRefreshed(ch *ent.Channel) func(ctx context.Context, refreshed *oauth.OAuthCredentials) error {
	return func(ctx context.Context, refreshed *oauth.OAuthCredentials) error {
		ctx = authz.WithSystemBypass(ctx, "channel-refresh-cache")
		return svc.refreshOAuthToken(ctx, ch, refreshed)
	}
}

func (svc *ChannelService) initChannelPerformances(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "int-channel-load-performances")
	if err := svc.loadChannelPerformances(ctx); err != nil {
		log.Warn(ctx, "failed to load channel performances", log.Cause(err))
	}
}
