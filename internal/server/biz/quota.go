package biz

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
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

	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	loc := s.system.TimeLocation(ctx)

	window, err := quotaWindow(xtime.Now(), quota.Period, loc)
	if err != nil {
		return QuotaCheckResult{}, err
	}

	if quota.Requests != nil {
		reqCount, err := s.requestCount(ctx, apiKeyID, window)
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

	usageAgg, err := s.usageAgg(ctx, apiKeyID, window, quota.TotalTokens != nil, quota.Cost != nil)
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

	window, err := quotaWindow(xtime.Now(), quota.Period, loc)
	if err != nil {
		return QuotaResult{}, err
	}

	reqCount, err := s.requestCount(ctx, apiKeyID, window)
	if err != nil {
		return QuotaResult{}, err
	}

	usageAgg, err := s.usageAgg(ctx, apiKeyID, window, true, true)
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
		return QuotaWindow{}, nil
	case objects.APIKeyQuotaPeriodTypePastDuration:
		if period.PastDuration == nil {
			return QuotaWindow{}, fmt.Errorf("pastDuration is required")
		}

		if period.PastDuration.Value <= 0 {
			return QuotaWindow{}, fmt.Errorf("pastDuration.value must be positive")
		}

		var d time.Duration

		switch period.PastDuration.Unit {
		case objects.APIKeyQuotaPastDurationUnitHour:
			d = time.Duration(period.PastDuration.Value) * time.Hour
		case objects.APIKeyQuotaPastDurationUnitDay:
			d = time.Duration(period.PastDuration.Value) * 24 * time.Hour
		default:
			return QuotaWindow{}, fmt.Errorf("unknown pastDuration.unit: %s", period.PastDuration.Unit)
		}

		start := now.Add(-d)

		return QuotaWindow{Start: &start}, nil
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
	q := s.ent.Request.Query().Where(request.APIKeyIDEQ(apiKeyID))

	if window.Start != nil {
		q = q.Where(request.CreatedAtGTE(*window.Start))
	}

	if window.End != nil {
		q = q.Where(request.CreatedAtLT(*window.End))
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
				TotalTokens sql.NullInt64   `json:"total_tokens"`
				TotalCost   sql.NullFloat64 `json:"total_cost"`
			}

			var rows []row

			err := q.Aggregate(
				ent.As(ent.Sum(usagelog.FieldTotalTokens), "total_tokens"),
				ent.As(ent.Sum(usagelog.FieldTotalCost), "total_cost"),
			).Scan(ctx, &rows)
			if err != nil {
				return usageAggResult{}, err
			}

			if len(rows) == 0 {
				return usageAggResult{TotalCost: decimal.Zero}, nil
			}

			tokens := lo.Ternary(rows[0].TotalTokens.Valid, rows[0].TotalTokens.Int64, int64(0))
			costFloat := lo.Ternary(rows[0].TotalCost.Valid, rows[0].TotalCost.Float64, float64(0))

			return usageAggResult{
				TotalTokens: tokens,
				TotalCost:   decimal.NewFromFloat(costFloat),
			}, nil
		case needTokens:
			type row struct {
				TotalTokens sql.NullInt64 `json:"total_tokens"`
			}

			var rows []row

			err := q.Aggregate(
				ent.As(ent.Sum(usagelog.FieldTotalTokens), "total_tokens"),
			).Scan(ctx, &rows)
			if err != nil {
				return usageAggResult{}, err
			}

			if len(rows) == 0 {
				return usageAggResult{TotalCost: decimal.Zero}, nil
			}

			tokens := lo.Ternary(rows[0].TotalTokens.Valid, rows[0].TotalTokens.Int64, int64(0))

			return usageAggResult{TotalTokens: tokens, TotalCost: decimal.Zero}, nil
		default:
			type row struct {
				TotalCost sql.NullFloat64 `json:"total_cost"`
			}

			var rows []row

			err := q.Aggregate(
				ent.As(ent.Sum(usagelog.FieldTotalCost), "total_cost"),
			).Scan(ctx, &rows)
			if err != nil {
				return usageAggResult{}, err
			}

			if len(rows) == 0 {
				return usageAggResult{TotalCost: decimal.Zero}, nil
			}

			costFloat := lo.Ternary(rows[0].TotalCost.Valid, rows[0].TotalCost.Float64, float64(0))

			return usageAggResult{TotalCost: decimal.NewFromFloat(costFloat)}, nil
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
