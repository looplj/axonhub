package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xtime"
	"github.com/looplj/axonhub/llm/oauth"
)

// startPerformanceProcess starts the background goroutine to flush metrics to database.
func (svc *ChannelService) startPerformanceProcess() {
	defer svc.perfWg.Done()

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

	// Use singleflight to prevent thundering herd if called from multiple goroutines
	_, err, shared := svc.refreshSingleflight.Do("init-historical", func() (any, error) {
		windowDays := svc.histWindowDays
		if windowDays <= 0 {
			windowDays = DefaultHistoricalWindowDays
		}
		windowDuration := time.Duration(windowDays) * 24 * time.Hour

		return nil, svc.loadChannelPerformances(ctx, windowDuration)
	})
	if shared {
		log.Debug(ctx, "init channel performances deduplicated via singleflight")
		return
	}
	if err != nil {
		log.Warn(ctx, "failed to load channel performances", log.Cause(err))
	}
}

func (svc *ChannelService) ReloadEnabledChannelsCache(ctx context.Context) error {
	ctx = authz.WithSystemBypass(ctx, "channel-reload-enabled-channels-cache")
	if err := svc.enabledChannelsCache.Load(ctx, true); err != nil {
		return fmt.Errorf("failed to reload enabled channels cache: %w", err)
	}

	return nil
}

// startHistoricalRefresh starts the background goroutine for periodic historical performance refresh.
// If refreshInterval is <= 0, the refresh is disabled and no background goroutine is started.
// Otherwise, it creates a ticker with the specified interval and calls loadChannelPerformances on each tick.
// Implements retry with exponential backoff (3 retries max) and uses singleflight to prevent
// concurrent refreshes.
func (svc *ChannelService) startHistoricalRefresh(refreshInterval time.Duration) {
	if refreshInterval <= 0 {
		// Historical refresh is disabled
		log.Info(context.Background(), "historical refresh disabled (interval <= 0)")
		return
	}

	svc.refreshTicker = time.NewTicker(refreshInterval)
	svc.refreshStopCh = make(chan struct{})
	svc.refreshWg.Add(1)

	go svc.historicalRefreshWorker()
}

// historicalRefreshWorker is the background goroutine that listens for ticker events and stop signals.
func (svc *ChannelService) historicalRefreshWorker() {
	defer svc.refreshWg.Done()

	for {
		select {
		case <-svc.refreshStopCh:
			return
		case <-svc.refreshTicker.C:
			svc.doHistoricalRefreshWithRetry()
		}
	}
}

// doHistoricalRefreshWithRetry performs the historical refresh with exponential backoff retry logic.
// Retries up to 3 times with backoff: 1s, 2s, 4s. After 3 failures, waits for next tick.
func (svc *ChannelService) doHistoricalRefreshWithRetry() {
	ctx := authz.WithSystemBypass(context.Background(), "channel-historical-refresh")

	maxRetries := 3
	baseDelay := time.Second

	windowDays := svc.histWindowDays
	if windowDays <= 0 {
		windowDays = 7
	}
	windowDuration := time.Duration(windowDays) * 24 * time.Hour

	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err, shared := svc.refreshSingleflight.Do("historical-refresh", func() (any, error) {
			return nil, svc.loadChannelPerformances(ctx, windowDuration)
		})

		if shared {
			log.Debug(ctx, "historical refresh deduplicated via singleflight")
			return
		}

		if err == nil {
			log.Info(ctx, "historical refresh completed successfully")
			return
		}

		log.Warn(ctx, "historical refresh failed", log.Int("attempt", attempt+1), log.Cause(err))

		if attempt < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<attempt)
			select {
			case <-svc.refreshStopCh:
				return
			case <-time.After(delay):
			}
		}
	}

	log.Error(ctx, "historical refresh failed after all retries, waiting for next tick")
}

// stopHistoricalRefresh stops the background refresh goroutine gracefully.
// This method is idempotent and safe for concurrent calls.
func (svc *ChannelService) stopHistoricalRefresh() {
	svc.refreshMu.Lock()
	defer svc.refreshMu.Unlock()

	if svc.refreshTicker != nil {
		svc.refreshTicker.Stop()
		svc.refreshTicker = nil
	}

	if svc.refreshStopCh != nil {
		select {
		case <-svc.refreshStopCh:
		default:
			close(svc.refreshStopCh)
		}
	}

	done := make(chan struct{})
	go func() {
		svc.refreshWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
	}
}
