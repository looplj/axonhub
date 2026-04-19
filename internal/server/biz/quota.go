package biz

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/shopspring/decimal"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xtime"
)

type QuotaWindow struct {
	Start *time.Time
	End   *time.Time
}

type QuotaUsage struct {
	RequestCount int64
	TotalTokens  int64
	TotalCost    decimal.Decimal
}

type QuotaCheckResult struct {
	Allowed bool
	Message string
	Window  QuotaWindow
}

type QuotaResult struct {
	Window QuotaWindow
	Usage  QuotaUsage
}

type QuotaService struct {
	ent    *ent.Client
	system *SystemService
}

func NewQuotaService(entClient *ent.Client, systemService *SystemService) *QuotaService {
	return &QuotaService{ent: entClient, system: systemService}
}

func (s *QuotaService) CheckAPIKeyQuota(ctx context.Context, apiKeyID int, quota *objects.APIKeyQuota) (QuotaCheckResult, error) {
	if quota == nil {
		return QuotaCheckResult{Allowed: true}, nil
	}

	loc := s.system.TimeLocation(ctx)

	window, err := quotaWindow(xtime.UTCNow(), quota.Period, loc)
	if err != nil {
		return QuotaCheckResult{}, err
	}

	if quota.Requests != nil {
		reqCount, err := authz.RunWithSystemBypass(ctx, "quota-request-count", func(bypassCtx context.Context) (int64, error) {
			return s.requestCount(bypassCtx, apiKeyID, window)
		})
		if err != nil {
			return QuotaCheckResult{}, err
		}

		if reqCount >= *quota.Requests {
			return QuotaCheckResult{
				Allowed: false,
				Message: fmt.Sprintf("requests quota exceeded: %d/%d", reqCount, *quota.Requests),
				Window:  window,
			}, nil
		}
	}

	if quota.TotalTokens == nil && quota.Cost == nil {
		return QuotaCheckResult{
			Allowed: true,
			Window:  window,
		}, nil
	}

	usageAgg, err := authz.RunWithSystemBypass(ctx, "quota-usage-agg", func(bypassCtx context.Context) (usageAggResult, error) {
		return s.usageAgg(bypassCtx, apiKeyID, window, quota.TotalTokens != nil, quota.Cost != nil)
	})
	if err != nil {
		return QuotaCheckResult{}, err
	}

	if quota.TotalTokens != nil && usageAgg.TotalTokens >= *quota.TotalTokens {
		return QuotaCheckResult{
			Allowed: false,
			Message: fmt.Sprintf("total_tokens quota exceeded: %d/%d", usageAgg.TotalTokens, *quota.TotalTokens),
			Window:  window,
		}, nil
	}

	if quota.Cost != nil && usageAgg.TotalCost.GreaterThanOrEqual(*quota.Cost) {
		return QuotaCheckResult{
			Allowed: false,
			Message: fmt.Sprintf("cost quota exceeded: %s/%s", usageAgg.TotalCost.String(), quota.Cost.String()),
			Window:  window,
		}, nil
	}

	return QuotaCheckResult{
		Allowed: true,
		Window:  window,
	}, nil
}

func (s *QuotaService) GetQuota(ctx context.Context, apiKeyID int, quota *objects.APIKeyQuota) (QuotaResult, error) {
	if quota == nil {
		return QuotaResult{}, nil
	}

	loc := s.system.TimeLocation(ctx)

	window, err := quotaWindow(xtime.UTCNow(), quota.Period, loc)
	if err != nil {
		return QuotaResult{}, err
	}

	reqCount, err := authz.RunWithSystemBypass(ctx, "quota-request-count", func(bypassCtx context.Context) (int64, error) {
		return s.requestCount(bypassCtx, apiKeyID, window)
	})
	if err != nil {
		return QuotaResult{}, err
	}

	usageAgg, err := authz.RunWithSystemBypass(ctx, "quota-usage-agg", func(bypassCtx context.Context) (usageAggResult, error) {
		return s.usageAgg(bypassCtx, apiKeyID, window, true, true)
	})
	if err != nil {
		return QuotaResult{}, err
	}

	return QuotaResult{
		Window: window,
		Usage: QuotaUsage{
			RequestCount: reqCount,
			TotalTokens:  usageAgg.TotalTokens,
			TotalCost:    usageAgg.TotalCost,
		},
	}, nil
}

func quotaWindow(now time.Time, period objects.APIKeyQuotaPeriod, loc *time.Location) (QuotaWindow, error) {
	if loc == nil {
		loc = time.UTC
	}

	switch period.Type {
	case objects.APIKeyQuotaPeriodTypeAllTime:
		end := now
		return QuotaWindow{End: &end}, nil
	case objects.APIKeyQuotaPeriodTypePastDuration:
		if period.PastDuration == nil {
			return QuotaWindow{}, fmt.Errorf("pastDuration is required")
		}

		if period.PastDuration.Value <= 0 {
			return QuotaWindow{}, fmt.Errorf("pastDuration.value must be positive")
		}

		var d time.Duration

		switch period.PastDuration.Unit {
		case objects.APIKeyQuotaPastDurationUnitMinute:
			d = time.Duration(period.PastDuration.Value) * time.Minute
		case objects.APIKeyQuotaPastDurationUnitHour:
			d = time.Duration(period.PastDuration.Value) * time.Hour
		case objects.APIKeyQuotaPastDurationUnitDay:
			d = time.Duration(period.PastDuration.Value) * 24 * time.Hour
		default:
			return QuotaWindow{}, fmt.Errorf("unknown pastDuration.unit: %s", period.PastDuration.Unit)
		}

		start := now.Add(-d)
		end := now

		return QuotaWindow{Start: &start, End: &end}, nil
	case objects.APIKeyQuotaPeriodTypeCalendarDuration:
		if period.CalendarDuration == nil {
			return QuotaWindow{}, fmt.Errorf("calendarDuration is required")
		}

		switch period.CalendarDuration.Unit {
		case objects.APIKeyQuotaCalendarDurationUnitDay:
			nowLocal := now.In(loc)
			startLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
			endLocal := startLocal.AddDate(0, 0, 1)
			start := startLocal.UTC()
			end := endLocal.UTC()

			return QuotaWindow{Start: &start, End: &end}, nil
		case objects.APIKeyQuotaCalendarDurationUnitMonth:
			nowLocal := now.In(loc)
			startLocal := time.Date(nowLocal.Year(), nowLocal.Month(), 1, 0, 0, 0, 0, loc)
			endLocal := startLocal.AddDate(0, 1, 0)
			start := startLocal.UTC()
			end := endLocal.UTC()

			return QuotaWindow{Start: &start, End: &end}, nil
		default:
			return QuotaWindow{}, fmt.Errorf("unknown calendarDuration.unit: %s", period.CalendarDuration.Unit)
		}
	default:
		return QuotaWindow{}, fmt.Errorf("unknown period.type: %s", period.Type)
	}
}

func (s *QuotaService) requestCount(ctx context.Context, apiKeyID int, window QuotaWindow) (int64, error) {
	q := s.ent.UsageLog.Query().Where(usagelog.APIKeyIDEQ(apiKeyID))

	if window.Start != nil {
		q = q.Where(usagelog.CreatedAtGTE(*window.Start))
	}

	if window.End != nil {
		q = q.Where(usagelog.CreatedAtLT(*window.End))
	}

	n, err := q.Count(ctx)
	if err != nil {
		return 0, err
	}

	return int64(n), nil
}

type usageAggResult struct {
	TotalTokens int64
	TotalCost   decimal.Decimal
}

func (s *QuotaService) usageAgg(ctx context.Context, apiKeyID int, window QuotaWindow, needTokens bool, needCost bool) (usageAggResult, error) {
	if !needTokens && !needCost {
		return usageAggResult{}, nil
	}

	queryAgg := func(q *ent.UsageLogQuery) (usageAggResult, error) {
		if window.Start != nil {
			q = q.Where(usagelog.CreatedAtGTE(*window.Start))
		}

		if window.End != nil {
			q = q.Where(usagelog.CreatedAtLT(*window.End))
		}

		switch {
		case needTokens && needCost:
			type row struct {
				TotalTokens int64   `json:"total_tokens"`
				TotalCost   float64 `json:"total_cost"`
			}

			var rows []row

			err := q.Modify(func(s *sql.Selector) {
				s.Select(
					sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalTokens)), "total_tokens"),
					sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalCost)), "total_cost"),
				)
			}).Scan(ctx, &rows)
			if err != nil {
				return usageAggResult{}, err
			}

			if len(rows) == 0 {
				return usageAggResult{TotalCost: decimal.Zero}, nil
			}

			return usageAggResult{
				TotalTokens: rows[0].TotalTokens,
				TotalCost:   decimal.NewFromFloat(rows[0].TotalCost),
			}, nil
		case needTokens:
			type row struct {
				TotalTokens int64 `json:"total_tokens"`
			}

			var rows []row

			err := q.Modify(func(s *sql.Selector) {
				s.Select(
					sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalTokens)), "total_tokens"),
				)
			}).Scan(ctx, &rows)
			if err != nil {
				return usageAggResult{}, err
			}

			if len(rows) == 0 {
				return usageAggResult{TotalCost: decimal.Zero}, nil
			}

			return usageAggResult{TotalTokens: rows[0].TotalTokens, TotalCost: decimal.Zero}, nil
		default:
			type row struct {
				TotalCost float64 `json:"total_cost"`
			}

			var rows []row

			err := q.Modify(func(s *sql.Selector) {
				s.Select(
					sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalCost)), "total_cost"),
				)
			}).Scan(ctx, &rows)
			if err != nil {
				return usageAggResult{}, err
			}

			if len(rows) == 0 {
				return usageAggResult{TotalCost: decimal.Zero}, nil
			}

			return usageAggResult{TotalCost: decimal.NewFromFloat(rows[0].TotalCost)}, nil
		}
	}

	agg1, err := queryAgg(s.ent.UsageLog.Query().Where(usagelog.APIKeyIDEQ(apiKeyID)))
	if err != nil {
		return usageAggResult{}, err
	}

	//  Compatible with old usage log without api_key_id.
	//  DO NOT NEED FOR NOW.
	// agg2, err := queryAgg(s.ent.UsageLog.Query().Where(
	// 	usagelog.APIKeyIDIsNil(),
	// 	usagelog.HasRequestWith(request.APIKeyIDEQ(apiKeyID)),
	// ))
	// if err != nil {
	// 	return usageAggResult{}, err
	// }

	return usageAggResult{
		TotalTokens: agg1.TotalTokens, // + agg2.TotalTokens,
		TotalCost:   agg1.TotalCost,   // .Add(agg2.TotalCost),
	}, nil
}

// GetChannelCost returns the total cost for a channel within the specified time window.
// Note: This query should have a composite index on (channel_id, created_at) for optimal
// performance. Without it, large tables may incur full table scans.
func (s *QuotaService) GetChannelCost(ctx context.Context, channelID int, window QuotaWindow) (float64, error) {
	return authz.RunWithSystemBypass(ctx, "channel-cost", func(bypassCtx context.Context) (float64, error) {
		q := s.ent.UsageLog.Query().Where(
			usagelog.ChannelIDEQ(channelID),
			usagelog.SourceEQ(usagelog.SourceAPI),
		)

		if window.Start != nil {
			q = q.Where(usagelog.CreatedAtGTE(*window.Start))
		}

		if window.End != nil {
			q = q.Where(usagelog.CreatedAtLT(*window.End))
		}

		type row struct {
			TotalCost float64 `json:"total_cost"`
		}

		var rows []row

		err := q.Modify(func(sel *sql.Selector) {
			sel.Select(
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", sel.C(usagelog.FieldTotalCost)), "total_cost"),
			)
		}).Scan(bypassCtx, &rows)
		if err != nil {
			return 0, err
		}

		if len(rows) == 0 {
			return 0, nil
		}

		return rows[0].TotalCost, nil
	})
}

// GetChannelCostAllSources returns the total cost for a channel across all sources (not just API).
// This matches the RateLimitAwareStrategy behavior in the load balancer.
func (s *QuotaService) GetChannelCostAllSources(ctx context.Context, channelID int, window QuotaWindow) (float64, error) {
	return authz.RunWithSystemBypass(ctx, "channel-cost-all", func(bypassCtx context.Context) (float64, error) {
		q := s.ent.UsageLog.Query().Where(
			usagelog.ChannelIDEQ(channelID),
		)

		if window.Start != nil {
			q = q.Where(usagelog.CreatedAtGTE(*window.Start))
		}

		if window.End != nil {
			q = q.Where(usagelog.CreatedAtLT(*window.End))
		}

		type row struct {
			TotalCost float64 `json:"total_cost"`
		}

		var rows []row

		err := q.Modify(func(sel *sql.Selector) {
			sel.Select(
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", sel.C(usagelog.FieldTotalCost)), "total_cost"),
			)
		}).Scan(bypassCtx, &rows)
		if err != nil {
			return 0, err
		}

		if len(rows) == 0 {
			return 0, nil
		}

		return rows[0].TotalCost, nil
	})
}

func (s *QuotaService) GetChannelRequestCount(ctx context.Context, channelID int, window QuotaWindow) (int64, error) {
	return authz.RunWithSystemBypass(ctx, "channel-request-count", func(bypassCtx context.Context) (int64, error) {
		return s.channelRequestCount(bypassCtx, channelID, window, usagelog.SourceAPI)
	})
}

func (s *QuotaService) GetChannelRequestCountAllSources(ctx context.Context, channelID int, window QuotaWindow) (int64, error) {
	return authz.RunWithSystemBypass(ctx, "channel-request-count-all", func(bypassCtx context.Context) (int64, error) {
		return s.channelRequestCount(bypassCtx, channelID, window, "")
	})
}

func (s *QuotaService) channelRequestCount(ctx context.Context, channelID int, window QuotaWindow, source usagelog.Source) (int64, error) {
	q := s.ent.UsageLog.Query().Where(
		usagelog.ChannelIDEQ(channelID),
	)
	if source != "" {
		q = q.Where(usagelog.SourceEQ(source))
	}

	if window.Start != nil {
		q = q.Where(usagelog.CreatedAtGTE(*window.Start))
	}

	if window.End != nil {
		q = q.Where(usagelog.CreatedAtLT(*window.End))
	}

	type row struct {
		Count int64 `json:"count"`
	}

	var rows []row

	err := q.Modify(func(sel *sql.Selector) {
		sel.Select(
			sql.As(fmt.Sprintf("COALESCE(COUNT(%s), 0)", sel.C(usagelog.FieldID)), "count"),
		)
	}).Scan(ctx, &rows)
	if err != nil {
		return 0, err
	}

	if len(rows) == 0 {
		return 0, nil
	}

	return rows[0].Count, nil
}

func (s *QuotaService) GetChannelTokenCount(ctx context.Context, channelID int, window QuotaWindow) (int64, error) {
	return authz.RunWithSystemBypass(ctx, "channel-token-count", func(bypassCtx context.Context) (int64, error) {
		return s.channelTokenCount(bypassCtx, channelID, window, usagelog.SourceAPI)
	})
}

func (s *QuotaService) GetChannelTokenCountAllSources(ctx context.Context, channelID int, window QuotaWindow) (int64, error) {
	return authz.RunWithSystemBypass(ctx, "channel-token-count-all", func(bypassCtx context.Context) (int64, error) {
		return s.channelTokenCount(bypassCtx, channelID, window, "")
	})
}

func (s *QuotaService) channelTokenCount(ctx context.Context, channelID int, window QuotaWindow, source usagelog.Source) (int64, error) {
	q := s.ent.UsageLog.Query().Where(
		usagelog.ChannelIDEQ(channelID),
	)
	if source != "" {
		q = q.Where(usagelog.SourceEQ(source))
	}

	if window.Start != nil {
		q = q.Where(usagelog.CreatedAtGTE(*window.Start))
	}

	if window.End != nil {
		q = q.Where(usagelog.CreatedAtLT(*window.End))
	}

	type row struct {
		TotalTokens int64 `json:"total_tokens"`
	}

	var rows []row

	err := q.Modify(func(sel *sql.Selector) {
		sel.Select(
			sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", sel.C(usagelog.FieldTotalTokens)), "total_tokens"),
		)
	}).Scan(ctx, &rows)
	if err != nil {
		return 0, err
	}

	if len(rows) == 0 {
		return 0, nil
	}

	return rows[0].TotalTokens, nil
}

// GetBatchChannelRequestCount returns request counts for multiple channels in a single query.
// Uses GROUP BY channel_id to fetch all counts at once. Returns a map of channelID to count.
func (s *QuotaService) GetBatchChannelRequestCount(ctx context.Context, channelIDs []int, window QuotaWindow) (map[int]int64, error) {
	if len(channelIDs) == 0 {
		return map[int]int64{}, nil
	}

	return authz.RunWithSystemBypass(ctx, "batch-channel-request-count", func(bypassCtx context.Context) (map[int]int64, error) {
		q := s.ent.UsageLog.Query().Where(
			usagelog.ChannelIDIn(channelIDs...),
		)

		if window.Start != nil {
			q = q.Where(usagelog.CreatedAtGTE(*window.Start))
		}

		if window.End != nil {
			q = q.Where(usagelog.CreatedAtLT(*window.End))
		}

		type row struct {
			ChannelID int   `json:"channel_id"`
			Count     int64 `json:"count"`
		}

		var rows []row

		err := q.Modify(func(sel *sql.Selector) {
			sel.Select(
				sel.C(usagelog.FieldChannelID),
				sql.As(fmt.Sprintf("COALESCE(COUNT(%s), 0)", sel.C(usagelog.FieldID)), "count"),
			)
			sel.GroupBy(sel.C(usagelog.FieldChannelID))
		}).Scan(bypassCtx, &rows)
		if err != nil {
			return nil, err
		}

		result := make(map[int]int64, len(rows))
		for _, r := range rows {
			result[r.ChannelID] = r.Count
		}

		return result, nil
	})
}

// GetBatchChannelTokenCount returns token counts for multiple channels in a single query.
// Uses GROUP BY channel_id to fetch all counts at once. Returns a map of channelID to token count.
func (s *QuotaService) GetBatchChannelTokenCount(ctx context.Context, channelIDs []int, window QuotaWindow) (map[int]int64, error) {
	if len(channelIDs) == 0 {
		return map[int]int64{}, nil
	}

	return authz.RunWithSystemBypass(ctx, "batch-channel-token-count", func(bypassCtx context.Context) (map[int]int64, error) {
		q := s.ent.UsageLog.Query().Where(
			usagelog.ChannelIDIn(channelIDs...),
		)

		if window.Start != nil {
			q = q.Where(usagelog.CreatedAtGTE(*window.Start))
		}

		if window.End != nil {
			q = q.Where(usagelog.CreatedAtLT(*window.End))
		}

		type row struct {
			ChannelID   int   `json:"channel_id"`
			TotalTokens int64 `json:"total_tokens"`
		}

		var rows []row

		err := q.Modify(func(sel *sql.Selector) {
			sel.Select(
				sel.C(usagelog.FieldChannelID),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", sel.C(usagelog.FieldTotalTokens)), "total_tokens"),
			)
			sel.GroupBy(sel.C(usagelog.FieldChannelID))
		}).Scan(bypassCtx, &rows)
		if err != nil {
			return nil, err
		}

		result := make(map[int]int64, len(rows))
		for _, r := range rows {
			result[r.ChannelID] = r.TotalTokens
		}

		return result, nil
	})
}

// GetBatchChannelCost returns cost totals for multiple channels in a single query.
// Uses GROUP BY channel_id to fetch all costs at once. Returns a map of channelID to cost.
func (s *QuotaService) GetBatchChannelCost(ctx context.Context, channelIDs []int, window QuotaWindow) (map[int]float64, error) {
	if len(channelIDs) == 0 {
		return map[int]float64{}, nil
	}

	return authz.RunWithSystemBypass(ctx, "batch-channel-cost", func(bypassCtx context.Context) (map[int]float64, error) {
		q := s.ent.UsageLog.Query().Where(
			usagelog.ChannelIDIn(channelIDs...),
		)

		if window.Start != nil {
			q = q.Where(usagelog.CreatedAtGTE(*window.Start))
		}

		if window.End != nil {
			q = q.Where(usagelog.CreatedAtLT(*window.End))
		}

		type row struct {
			ChannelID int     `json:"channel_id"`
			TotalCost float64 `json:"total_cost"`
		}

		var rows []row

		err := q.Modify(func(sel *sql.Selector) {
			sel.Select(
				sel.C(usagelog.FieldChannelID),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", sel.C(usagelog.FieldTotalCost)), "total_cost"),
			)
			sel.GroupBy(sel.C(usagelog.FieldChannelID))
		}).Scan(bypassCtx, &rows)
		if err != nil {
			return nil, err
		}

		result := make(map[int]float64, len(rows))
		for _, r := range rows {
			result[r.ChannelID] = r.TotalCost
		}

		return result, nil
	})
}
