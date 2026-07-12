package gql

import (
	"context"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
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
