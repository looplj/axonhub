package gql

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/usagelog"
)

// dashboardUsageAggregateDimension identifies the grouping dimension for the
// shared dashboard usage aggregate. The dashboard renders request counts,
// cost and token totals for the same dimension in separate widgets, so one
// grouped scan can serve all of them.
type dashboardUsageAggregateDimension string

const (
	dashboardUsageByChannel dashboardUsageAggregateDimension = "channel"
	dashboardUsageByModel   dashboardUsageAggregateDimension = "model"
	dashboardUsageByAPIKey  dashboardUsageAggregateDimension = "apiKey"
)

const dashboardUsageAggregateLimit = 10

// dashboardUsageAggregate is one row of the shared grouped scan. ID is the
// channel or API key ID; Name is the channel name, API key name, or model ID.
type dashboardUsageAggregate struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	RequestCount    int     `json:"request_count"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CachedTokens    int64   `json:"cached_tokens"`
	ReasoningTokens int64   `json:"reasoning_tokens"`
	TotalCost       float64 `json:"total_cost"`
}

func (a dashboardUsageAggregate) totalTokens() int64 {
	// Reasoning tokens are already included in completion tokens, so adding
	// them separately would double count. This matches the token widgets.
	return a.InputTokens + a.OutputTokens
}

// normalizeFastestChannelsInput applies the same defaults and clamps that the
// resolvers use, before the values are put into a cache key. Without this, a
// nil limit and limit=5 would compute the same result under different keys,
// and an unknown time window could collide with the explicit "day" default.
func normalizeFastestChannelsInput(input FastestChannelsInput) FastestChannelsInput {
	switch input.TimeWindow {
	case "day", "week", "month":
	default:
		input.TimeWindow = "day"
	}

	limit := 5
	if input.Limit != nil {
		limit = *input.Limit
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 100 {
		limit = 100
	}
	input.Limit = new(limit)

	return input
}

func canonicalDashboardTimeWindow(timeWindow *string) string {
	if timeWindow == nil {
		return "allTime"
	}

	switch strings.TrimSpace(*timeWindow) {
	case "day":
		return "day"
	case "week":
		return "week"
	case "month":
		return "month"
	default:
		// parseTimeWindow treats empty, allTime and unknown values as all time.
		return "allTime"
	}
}

// loadDashboardUsageAggregates returns one grouped row per channel, model or
// API key for the requested window. Concurrent callers for the same dimension
// share a single scan and its cached slice; callers must sort a copy instead
// of mutating the returned rows.
func (r *queryResolver) loadDashboardUsageAggregates(
	ctx context.Context,
	dimension dashboardUsageAggregateDimension,
	timeWindow *string,
) ([]dashboardUsageAggregate, error) {
	window := canonicalDashboardTimeWindow(timeWindow)
	location := r.systemService.TimeLocation(ctx).String()
	key := fmt.Sprintf("dashboard-usage:%s:%s:%s:%p", dimension, window, location, r.client)

	// Use the canonical window for both the cache key and the query so
	// equivalent inputs such as "day" and " day " cannot populate different
	// results under the same key. The canonical unknown/empty value is
	// "allTime", matching parseTimeWindow's fallback.
	normalizedWindow := window

	// One grouped scan serves the request, cost and token widgets for this
	// dimension. The shared cache also collapses concurrent loads from
	// multiple dashboard viewers.
	return cachedDashboardQuery(ctx, key, dashboardCacheTTL, func(ctx context.Context) ([]dashboardUsageAggregate, error) {
		return r.queryDashboardUsageAggregates(ctx, dimension, &normalizedWindow)
	})
}

func (r *queryResolver) queryDashboardUsageAggregates(
	ctx context.Context,
	dimension dashboardUsageAggregateDimension,
	timeWindow *string,
) ([]dashboardUsageAggregate, error) {
	since, applyFilter := r.parseTimeWindow(ctx, timeWindow)
	query := r.client.UsageLog.Query()

	switch dimension {
	case dashboardUsageByChannel:
		return r.queryChannelUsageAggregates(ctx, query, since, applyFilter)
	case dashboardUsageByModel:
		return r.queryModelUsageAggregates(ctx, query, since, applyFilter)
	case dashboardUsageByAPIKey:
		return r.queryAPIKeyUsageAggregates(ctx, query, since, applyFilter)
	default:
		return nil, fmt.Errorf("unsupported dashboard usage dimension %q", dimension)
	}
}

func (r *queryResolver) queryChannelUsageAggregates(
	ctx context.Context,
	query *ent.UsageLogQuery,
	since time.Time,
	applyFilter bool,
) ([]dashboardUsageAggregate, error) {
	var rows []dashboardUsageAggregate

	err := query.
		Modify(func(s *sql.Selector) {
			channelTable := sql.Table(channel.Table)
			s.Join(channelTable).On(
				s.C(usagelog.FieldChannelID),
				channelTable.C(channel.FieldID),
			)

			// Match the existing dashboard behavior: soft-deleted channels are
			// excluded from usage widgets.
			s.Where(sql.EQ(channelTable.C(channel.FieldDeletedAt), 0))
			if applyFilter {
				s.Where(sql.GTE(s.C(usagelog.FieldCreatedAt), since))
			}

			s.GroupBy(channelTable.C(channel.FieldID), channelTable.C(channel.FieldName))
			s.Select(append([]string{
				sql.As(channelTable.C(channel.FieldID), "id"),
				sql.As(channelTable.C(channel.FieldName), "name"),
			}, dashboardUsageAggregateSelects(s)...)...)
		}).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate usage by channel: %w", err)
	}

	return rows, nil
}

func (r *queryResolver) queryModelUsageAggregates(
	ctx context.Context,
	query *ent.UsageLogQuery,
	since time.Time,
	applyFilter bool,
) ([]dashboardUsageAggregate, error) {
	var rows []dashboardUsageAggregate

	err := query.
		Modify(func(s *sql.Selector) {
			if applyFilter {
				s.Where(sql.GTE(s.C(usagelog.FieldCreatedAt), since))
			}

			s.GroupBy(s.C(usagelog.FieldModelID))
			s.Select(append([]string{
				sql.As(s.C(usagelog.FieldModelID), "name"),
			}, dashboardUsageAggregateSelects(s)...)...)
		}).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate usage by model: %w", err)
	}

	return rows, nil
}

func (r *queryResolver) queryAPIKeyUsageAggregates(
	ctx context.Context,
	query *ent.UsageLogQuery,
	since time.Time,
	applyFilter bool,
) ([]dashboardUsageAggregate, error) {
	var rows []dashboardUsageAggregate

	err := query.
		Where(usagelog.APIKeyIDNotNil()).
		Modify(func(s *sql.Selector) {
			apiKeyTable := sql.Table(apikey.Table)
			s.Join(apiKeyTable).On(
				s.C(usagelog.FieldAPIKeyID),
				apiKeyTable.C(apikey.FieldID),
			)

			// Match the previous behavior: soft-deleted API keys are not shown.
			s.Where(sql.EQ(apiKeyTable.C(apikey.FieldDeletedAt), 0))
			if applyFilter {
				s.Where(sql.GTE(s.C(usagelog.FieldCreatedAt), since))
			}

			s.GroupBy(apiKeyTable.C(apikey.FieldID), apiKeyTable.C(apikey.FieldName))
			s.Select(append([]string{
				sql.As(apiKeyTable.C(apikey.FieldID), "id"),
				sql.As(apiKeyTable.C(apikey.FieldName), "name"),
			}, dashboardUsageAggregateSelects(s)...)...)
		}).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate usage by API key: %w", err)
	}

	return rows, nil
}

func dashboardUsageAggregateSelects(s *sql.Selector) []string {
	return []string{
		sql.As(sql.Count(s.C(usagelog.FieldID)), "request_count"),
		sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptTokens)), "input_tokens"),
		sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionTokens)), "output_tokens"),
		sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptCachedTokens)), "cached_tokens"),
		sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionReasoningTokens)), "reasoning_tokens"),
		sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalCost)), "total_cost"),
	}
}

// topDashboardUsageAggregates sorts a copy of rows by score and keeps the
// dashboard's top-N limit. Ties are stable by ID and name.
func topDashboardUsageAggregates(
	rows []dashboardUsageAggregate,
	score func(dashboardUsageAggregate) float64,
) []dashboardUsageAggregate {
	top := slices.Clone(rows)
	sort.SliceStable(top, func(i, j int) bool {
		left, right := score(top[i]), score(top[j])
		if left != right {
			return left > right
		}
		if top[i].ID != top[j].ID {
			return top[i].ID < top[j].ID
		}
		return top[i].Name < top[j].Name
	})

	if len(top) > dashboardUsageAggregateLimit {
		top = top[:dashboardUsageAggregateLimit]
	}

	return top
}
