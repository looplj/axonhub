package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
)

func (svc *ChannelService) markChannelUnavailable(ctx context.Context, channelID int, errorStatusCode int) {
	ctx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	_, err := svc.db.Channel.UpdateOneID(channelID).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(deriveErrorMessage(errorStatusCode)).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to disable channel on unrecoverable error",
			log.Int("channel_id", channelID),
			log.Int("error_code", errorStatusCode),
			log.Cause(err),
		)

		return
	}

	log.Warn(ctx, "Channel disabled due to unrecoverable error",
		log.Int("channel_id", channelID),
		log.Int("error_code", errorStatusCode),
	)

	// Reload channels to reflect the change in load balancer
	svc.asyncReloadChannels()
}

// checkAndHandleChannelError checks if the channel should be disabled based on the error status code.
func (svc *ChannelService) checkAndHandleChannelError(ctx context.Context, perf *PerformanceRecord, policy *RetryPolicy) bool {
	for _, statusConfig := range policy.AutoDisableChannel.Statuses {
		if statusConfig.Status != perf.ErrorStatusCode {
			continue
		}

		svc.channelErrorCountsLock.Lock()

		if svc.channelErrorCounts[perf.ChannelID] == nil {
			svc.channelErrorCounts[perf.ChannelID] = make(map[int]int)
		}

		svc.channelErrorCounts[perf.ChannelID][perf.ErrorStatusCode]++
		count := svc.channelErrorCounts[perf.ChannelID][perf.ErrorStatusCode]
		svc.channelErrorCountsLock.Unlock()

		if count >= statusConfig.Times {
			svc.markChannelUnavailable(ctx, perf.ChannelID, perf.ErrorStatusCode)
			svc.channelErrorCountsLock.Lock()
			delete(svc.channelErrorCounts, perf.ChannelID)
			svc.channelErrorCountsLock.Unlock()

			return true
		}
	}

	return false
}

// checkAndHandleAPIKeyError checks if the API key should be disabled based on the error status code.
// Returns true if the API key was disabled.
func (svc *ChannelService) checkAndHandleAPIKeyError(ctx context.Context, perf *PerformanceRecord, policy *RetryPolicy) bool {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	for _, statusConfig := range policy.AutoDisableChannel.Statuses {
		if statusConfig.Status != perf.ErrorStatusCode {
			continue
		}

		svc.apiKeyErrorCountsLock.Lock()

		if svc.apiKeyErrorCounts[perf.ChannelID] == nil {
			svc.apiKeyErrorCounts[perf.ChannelID] = make(map[string]map[int]int)
		}

		if svc.apiKeyErrorCounts[perf.ChannelID][perf.APIKey] == nil {
			svc.apiKeyErrorCounts[perf.ChannelID][perf.APIKey] = make(map[int]int)
		}

		svc.apiKeyErrorCounts[perf.ChannelID][perf.APIKey][perf.ErrorStatusCode]++
		count := svc.apiKeyErrorCounts[perf.ChannelID][perf.APIKey][perf.ErrorStatusCode]
		svc.apiKeyErrorCountsLock.Unlock()

		if count >= statusConfig.Times {
			reason := fmt.Sprintf("Auto-disabled after %d consecutive errors with status %d", count, perf.ErrorStatusCode)
			if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, perf.ErrorStatusCode, reason); err != nil {
				log.Error(ctx, "Failed to disable API key",
					log.Int("channel_id", perf.ChannelID),
					log.Int("error_code", perf.ErrorStatusCode),
					log.Cause(err),
				)

				return false
			}

			svc.apiKeyErrorCountsLock.Lock()
			delete(svc.apiKeyErrorCounts[perf.ChannelID], perf.APIKey)
			svc.apiKeyErrorCountsLock.Unlock()

			return true
		}
	}

	return false
}
