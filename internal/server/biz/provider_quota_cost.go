package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

// fillPeriodQuotas prices each limit period of a channel: it sums what the
// channel cost according to AxonHub usage logs since the period started, and
// derives the period's total money quota from the usage ratio the provider
// reported (see provider_quota.EstimatePeriodQuota).
//
// Only limits whose checker could pin down a period start participate; the rest
// keep a nil estimate. Any aggregation failure is logged and skipped so the
// quota status itself is still saved.
func (svc *ProviderQuotaService) fillPeriodQuotas(
	ctx context.Context,
	channelID int,
	quotaData *provider_quota.QuotaData,
	now time.Time,
) {
	// Providers repeat the same period start across limits (Claude Code reports
	// a 5h and a 7d window, Cline three windows), so the cost of each distinct
	// period is only queried once.
	costs := make(map[time.Time]float64)

	for i := range quotaData.Limits {
		limit := &quotaData.Limits[i]
		limit.PeriodCost = nil

		start := limit.PeriodStart
		if start == nil || !start.Before(now) {
			continue
		}

		cost, ok := costs[*start]
		if !ok {
			aggregated, err := svc.channelCostSince(ctx, channelID, *start, now)
			if err != nil {
				log.Warn(ctx, "Failed to aggregate channel cost for quota period",
					log.Int("channel_id", channelID),
					log.Time("period_start", *start),
					log.Cause(err))

				continue
			}

			cost = aggregated
			costs[*start] = cost
		}

		limit.PeriodCost = &cost
	}

	quotaData.FillPeriodQuotas()
}

// channelCostSince sums the cost of the usage logs a channel produced in
// [since, until). The usage_logs_by_channel_id_created_at index covers it.
func (svc *ProviderQuotaService) channelCostSince(
	ctx context.Context,
	channelID int,
	since time.Time,
	until time.Time,
) (float64, error) {
	// Manual checks run from a GraphQL mutation, whose context carries a user
	// principal that cannot read usage logs across projects; the scheduled path
	// already runs under a system bypass.
	metadata, err := authz.RunWithSystemBypass(ctx, "provider-quota-period-cost", func(ctx context.Context) (*UsageMetadata, error) {
		return aggregateUsageMetadata(ctx, svc.db.UsageLog.Query().Where(
			usagelog.ChannelIDEQ(channelID),
			usagelog.CreatedAtGTE(since),
			usagelog.CreatedAtLT(until),
		))
	})
	if err != nil {
		return 0, fmt.Errorf("failed to aggregate channel period cost: %w", err)
	}

	return metadata.TotalCost.InexactFloat64(), nil
}
