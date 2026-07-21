package gql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
)

// parseDateStr 解析 YYYY-MM-DD 日期字符串为系统时区午夜时间（保持 loc 时区）
// 与仪表盘 GetCalendarPeriods 中 todayStart 的构建方式一致：
// time.Date(year, month, day, 0, 0, 0, 0, loc)
// 不转 UTC，因为后续 fill-missing-dates 循环需要 loc 时区的 Year/Month/Day.
func parseDateStr(dateStr string, loc *time.Location) time.Time {
	parts := strings.Split(dateStr, "-")
	if len(parts) != 3 {
		return time.Time{}
	}
	y, m, d := 0, 0, 0
	for i, p := range parts {
		n := 0
		for _, c := range p {
			n = n*10 + int(c-'0')
		}
		switch i {
		case 0:
			y = n
		case 1:
			m = n
		case 2:
			d = n
		}
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, loc)
}

func (r *queryResolver) buildAnalyticsWhere(s *sql.Selector, filter *AnalyticsFilter, apiKeyIDs []int, hasUserFilter bool, loc *time.Location) {
	if filter == nil {
		return
	}

	if filter.StartTime != nil {
		startDate := parseDateStr(*filter.StartTime, loc)
		if !startDate.IsZero() {
			// 同仪表盘：本地午夜转 UTC 再比较，数据库 created_at 是 UTC
			s.Where(sql.GTE(s.C(usagelog.FieldCreatedAt), startDate.UTC()))
		}
	}

	if filter.EndTime != nil {
		endDate := parseDateStr(*filter.EndTime, loc)
		if !endDate.IsZero() {
			endDateNext := endDate.AddDate(0, 0, 1)
			s.Where(sql.LT(s.C(usagelog.FieldCreatedAt), endDateNext.UTC()))
		}
	}

	if len(filter.ProjectIDs) > 0 {
		ids := lo.Map(filter.ProjectIDs, func(g *objects.GUID, _ int) int { return g.ID })
		s.Where(sql.InInts(usagelog.FieldProjectID, ids...))
	}

	if len(filter.ChannelIDs) > 0 {
		ids := lo.Map(filter.ChannelIDs, func(g *objects.GUID, _ int) int { return g.ID })
		s.Where(sql.InInts(usagelog.FieldChannelID, ids...))
	}

	if len(filter.ModelIDs) > 0 {
		vals := make([]any, len(filter.ModelIDs))
		for i, v := range filter.ModelIDs {
			vals[i] = v
		}
		s.Where(sql.In(usagelog.FieldModelID, vals...))
	}

	// API key / user filtering:
	// - apiKeyIDs > 0: filter by specific API keys
	// - apiKeyIDs == 0 && hasUserFilter: user filter matched no API keys → return empty
	// - apiKeyIDs == 0 && !hasUserFilter: no API key filter → show all
	if len(apiKeyIDs) > 0 {
		s.Where(sql.InInts(usagelog.FieldAPIKeyID, apiKeyIDs...))
	} else if hasUserFilter {
		s.Where(sql.False())
	}
}

// resolveFilterAPIKeyIDs resolves the effective API key IDs from filter.
// When both APIKeyIDs and UserIDs are present, returns the intersection (AND logic).
// Returns (api key IDs, whether a user filter was applied).
func (r *queryResolver) resolveFilterAPIKeyIDs(ctx context.Context, filter *AnalyticsFilter) ([]int, bool) {
	if filter == nil {
		return nil, false
	}

	hasExplicitKeys := len(filter.APIKeyIDs) > 0
	hasUserFilter := len(filter.UserIDs) > 0

	if !hasExplicitKeys && !hasUserFilter {
		return nil, false
	}

	// Resolve user IDs to API key IDs
	var userKeyIDs []int

	if hasUserFilter {
		userIDs := lo.Map(filter.UserIDs, func(g *objects.GUID, _ int) int { return g.ID })
		apiKeys, err := r.client.APIKey.Query().
			Where(apikey.UserIDIn(userIDs...)).
			All(ctx)
		if err != nil {
			return nil, hasUserFilter
		}
		userKeyIDs = lo.Map(apiKeys, func(ak *ent.APIKey, _ int) int { return ak.ID })
	}

	explicitKeyIDs := lo.Map(filter.APIKeyIDs, func(g *objects.GUID, _ int) int { return g.ID })

	switch {
	case hasExplicitKeys && hasUserFilter:
		// AND logic: intersect explicit keys with user's keys
		userKeySet := make(map[int]bool, len(userKeyIDs))
		for _, id := range userKeyIDs {
			userKeySet[id] = true
		}
		var intersection []int
		for _, id := range explicitKeyIDs {
			if userKeySet[id] {
				intersection = append(intersection, id)
			}
		}
		return intersection, true
	case hasExplicitKeys:
		return explicitKeyIDs, false
	default:
		// Only user filter
		return userKeyIDs, true
	}
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// dimStats holds aggregated dimension statistics from raw SQL queries.
type dimStats struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	RequestCount int     `json:"request_count"`
	InputTokens  int64   `json:"input_tokens"`
	CachedTokens int64   `json:"cached_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	Cost         float64 `json:"cost"`
}

func (r *queryResolver) queryChannelStats(ctx context.Context, filter *AnalyticsFilter, apiKeyIDs []int, hasUserFilter bool, loc *time.Location) ([]dimStats, error) {
	var results []dimStats

	err := r.client.UsageLog.Query().
		Modify(func(s *sql.Selector) {
			channelTable := sql.Table(channel.Table)
			s.Join(channelTable).On(
				s.C(usagelog.FieldChannelID),
				channelTable.C(channel.FieldID),
			)
			s.Where(sql.EQ(channelTable.C(channel.FieldDeletedAt), 0))

			r.buildAnalyticsWhere(s, filter, apiKeyIDs, hasUserFilter, loc)

			s.Select(
				sql.As(fmt.Sprintf("CAST(%s AS TEXT)", s.C(usagelog.FieldChannelID)), "id"),
				sql.As(channelTable.C(channel.FieldName), "name"),
				sql.As(sql.Count(s.C(usagelog.FieldID)), "request_count"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptTokens)), "input_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptCachedTokens)), "cached_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionTokens)), "output_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalTokens)), "total_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalCost)), "cost"),
			).
				GroupBy(s.C(usagelog.FieldChannelID), channelTable.C(channel.FieldName)).
				OrderBy(sql.Desc("total_tokens"))
		}).
		Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics stats by channel: %w", err)
	}

	return results, nil
}

func (r *queryResolver) queryModelStats(ctx context.Context, filter *AnalyticsFilter, apiKeyIDs []int, hasUserFilter bool, loc *time.Location) ([]dimStats, error) {
	var results []dimStats

	err := r.client.UsageLog.Query().
		Modify(func(s *sql.Selector) {
			r.buildAnalyticsWhere(s, filter, apiKeyIDs, hasUserFilter, loc)

			s.Select(
				sql.As(s.C(usagelog.FieldModelID), "id"),
				sql.As(s.C(usagelog.FieldModelID), "name"),
				sql.As(sql.Count(s.C(usagelog.FieldID)), "request_count"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptTokens)), "input_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptCachedTokens)), "cached_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionTokens)), "output_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalTokens)), "total_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalCost)), "cost"),
			).
				GroupBy(s.C(usagelog.FieldModelID)).
				OrderBy(sql.Desc("total_tokens"))
		}).
		Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics stats by model: %w", err)
	}

	return results, nil
}

func (r *queryResolver) queryAPIKeyStats(ctx context.Context, filter *AnalyticsFilter, apiKeyIDs []int, hasUserFilter bool, loc *time.Location) ([]dimStats, error) {
	type apiKeyStatsRaw struct {
		APIKeyID     int     `json:"api_key_id"`
		RequestCount int     `json:"request_count"`
		InputTokens  int64   `json:"input_tokens"`
		CachedTokens int64   `json:"cached_tokens"`
		OutputTokens int64   `json:"output_tokens"`
		TotalTokens  int64   `json:"total_tokens"`
		Cost         float64 `json:"cost"`
	}

	var rawResults []apiKeyStatsRaw

	err := r.client.UsageLog.Query().
		Where(usagelog.APIKeyIDNotNil()).
		Modify(func(s *sql.Selector) {
			r.buildAnalyticsWhere(s, filter, apiKeyIDs, hasUserFilter, loc)

			s.Select(
				sql.As(s.C(usagelog.FieldAPIKeyID), "api_key_id"),
				sql.As(sql.Count(s.C(usagelog.FieldID)), "request_count"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptTokens)), "input_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptCachedTokens)), "cached_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionTokens)), "output_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalTokens)), "total_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalCost)), "cost"),
			).
				GroupBy(s.C(usagelog.FieldAPIKeyID)).
				OrderBy(sql.Desc("total_tokens"))
		}).
		Scan(ctx, &rawResults)
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics stats by apiKey: %w", err)
	}

	var results []dimStats

	if len(rawResults) > 0 {
		akIDs := lo.Map(rawResults, func(item apiKeyStatsRaw, _ int) int { return item.APIKeyID })
		apiKeys, qErr := r.client.APIKey.Query().Where(apikey.IDIn(akIDs...)).All(ctx)
		if qErr != nil {
			return nil, fmt.Errorf("failed to get API key details: %w", qErr)
		}
		apiKeyMap := lo.SliceToMap(apiKeys, func(ak *ent.APIKey) (int, *ent.APIKey) { return ak.ID, ak })

		for _, raw := range rawResults {
			name := fmt.Sprintf("API Key #%d", raw.APIKeyID)
			if ak, ok := apiKeyMap[raw.APIKeyID]; ok {
				name = ak.Name
			}
			results = append(results, dimStats{
				ID:           fmt.Sprintf("%d", raw.APIKeyID),
				Name:         name,
				RequestCount: raw.RequestCount,
				InputTokens:  raw.InputTokens,
				CachedTokens: raw.CachedTokens,
				OutputTokens: raw.OutputTokens,
				TotalTokens:  raw.TotalTokens,
				Cost:         raw.Cost,
			})
		}
	}

	return results, nil
}

func (r *queryResolver) queryUserStats(ctx context.Context, filter *AnalyticsFilter, apiKeyIDs []int, hasUserFilter bool, loc *time.Location) ([]dimStats, error) {
	type userStatsRaw struct {
		UserID       int     `json:"user_id"`
		FirstName    string  `json:"first_name"`
		LastName     string  `json:"last_name"`
		Email        string  `json:"email"`
		RequestCount int     `json:"request_count"`
		InputTokens  int64   `json:"input_tokens"`
		CachedTokens int64   `json:"cached_tokens"`
		OutputTokens int64   `json:"output_tokens"`
		TotalTokens  int64   `json:"total_tokens"`
		Cost         float64 `json:"cost"`
	}

	var rawResults []userStatsRaw

	err := r.client.UsageLog.Query().
		Where(usagelog.APIKeyIDNotNil()).
		Modify(func(s *sql.Selector) {
			apiKeyTable := sql.Table(apikey.Table)
			userTable := sql.Table("users")

			s.Join(apiKeyTable).On(
				s.C(usagelog.FieldAPIKeyID),
				apiKeyTable.C(apikey.FieldID),
			)
			s.Join(userTable).On(
				apiKeyTable.C(apikey.FieldUserID),
				userTable.C("id"),
			)
			s.Where(sql.EQ(apiKeyTable.C(apikey.FieldDeletedAt), 0))

			r.buildAnalyticsWhere(s, filter, apiKeyIDs, hasUserFilter, loc)

			s.Select(
				sql.As(userTable.C("id"), "user_id"),
				sql.As(userTable.C("first_name"), "first_name"),
				sql.As(userTable.C("last_name"), "last_name"),
				sql.As(userTable.C("email"), "email"),
				sql.As(sql.Count(s.C(usagelog.FieldID)), "request_count"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptTokens)), "input_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptCachedTokens)), "cached_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionTokens)), "output_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalTokens)), "total_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalCost)), "cost"),
			).
				GroupBy(
					userTable.C("id"),
					userTable.C("first_name"),
					userTable.C("last_name"),
					userTable.C("email"),
				).
				OrderBy(sql.Desc("total_tokens"))
		}).
		Scan(ctx, &rawResults)
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics stats by user: %w", err)
	}

	results := make([]dimStats, 0, len(rawResults))

	for _, raw := range rawResults {
		userName := fmt.Sprintf("%s %s", raw.FirstName, raw.LastName)
		userName = trimSpace(userName)
		if userName == "" {
			userName = raw.Email
		}
		results = append(results, dimStats{
			ID:           fmt.Sprintf("%d", raw.UserID),
			Name:         userName,
			RequestCount: raw.RequestCount,
			InputTokens:  raw.InputTokens,
			CachedTokens: raw.CachedTokens,
			OutputTokens: raw.OutputTokens,
			TotalTokens:  raw.TotalTokens,
			Cost:         raw.Cost,
		})
	}

	return results, nil
}

func dimStatsToDimensionStats(items []dimStats) []*AnalyticsDimensionStat {
	return lo.Map(items, func(item dimStats, _ int) *AnalyticsDimensionStat {
		return &AnalyticsDimensionStat{
			ID:                item.ID,
			Name:              item.Name,
			RequestCount:      item.RequestCount,
			InputTokens:       safeIntFromInt64(item.InputTokens),
			CachedInputTokens: safeIntFromInt64(item.CachedTokens),
			OutputTokens:      safeIntFromInt64(item.OutputTokens),
			TotalTokens:       safeIntFromInt64(item.TotalTokens),
			Cost:              item.Cost,
		}
	})
}
