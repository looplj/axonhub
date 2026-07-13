package gql

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/usagelog"
)

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
