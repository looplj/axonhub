package gql

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelprobe"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/gql/qb"
)

const topPerformersLimit = 6

type probeStats struct {
	DateStr      string  `json:"date"`
	ChannelID    int     `json:"channel_id"`
	RequestCount int     `json:"request_count"`
	Throughput   float64 `json:"throughput"`
	TTFTMs       float64 `json:"ttft_ms"`
}

type channelInfo struct {
	channelID    int
	requestCount int64
}

func (r *queryResolver) queryChannelProbeStats(ctx context.Context, startTimestamp int64, locName string, offsetSeconds int) ([]probeStats, error) {
	var probeResults []probeStats
	const maxThroughputCap = 2000.0

	err := r.client.ChannelProbe.Query().
		Where(
			channelprobe.TimestampGTE(startTimestamp),
			channelprobe.AvgTokensPerSecondLTE(maxThroughputCap),
		).
		Modify(func(s *sql.Selector) {
			timestampCol := s.C(channelprobe.FieldTimestamp)
			dateExpr := buildDateExpression(s.Dialect(), timestampCol, offsetSeconds, locName)
			selects := buildProbeQuerySelects(s, dateExpr)
			s.Select(selects...).
				GroupBy(dateExpr, s.C(channelprobe.FieldChannelID)).
				OrderBy(dateExpr, s.C(channelprobe.FieldChannelID))
		}).
		Scan(ctx, &probeResults)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel performance stats from probes: %w", err)
	}

	return probeResults, nil
}

func aggregateProbeStats(probeResults []probeStats) map[string]map[int]*probeStats {
	statsMap := make(map[string]map[int]*probeStats)
	for i := range probeResults {
		res := probeResults[i]
		if statsMap[res.DateStr] == nil {
			statsMap[res.DateStr] = make(map[int]*probeStats)
		}
		statsMap[res.DateStr][res.ChannelID] = &probeResults[i]
	}

	return statsMap
}

func calculateChannelTotals(statsMap map[string]map[int]*probeStats) map[int]int64 {
	channelTotals := make(map[int]int64)
	for _, dayStats := range statsMap {
		for chID, stats := range dayStats {
			channelTotals[chID] += int64(stats.RequestCount)
		}
	}

	return channelTotals
}

func getTopChannelIDs(statsMap map[string]map[int]*probeStats, limit int) map[int]struct{} {
	channelTotals := calculateChannelTotals(statsMap)

	channels := lo.MapToSlice(channelTotals, func(chID int, total int64) channelInfo {
		return channelInfo{
			channelID:    chID,
			requestCount: total,
		}
	})

	topChannels := calculateConfidenceAndSort(
		channels,
		func(c channelInfo) int64 { return c.requestCount },
		func(c channelInfo) float64 { return float64(c.requestCount) },
		limit,
	)

	topChannelIDs := make(map[int]struct{})
	for _, item := range topChannels {
		topChannelIDs[item.stats.channelID] = struct{}{}
	}

	return topChannelIDs
}

func filterStatsByTopChannels(statsMap map[string]map[int]*probeStats, topChannelIDs map[int]struct{}) {
	for dateStr, dayStats := range statsMap {
		for chID := range dayStats {
			if _, ok := topChannelIDs[chID]; !ok {
				delete(dayStats, chID)
			}
		}
		if len(dayStats) == 0 {
			delete(statsMap, dateStr)
		}
	}
}

func extractChannelIDsFromStats(statsMap map[string]map[int]*probeStats) []int {
	channelIDSet := make(map[int]struct{})
	for _, dayStats := range statsMap {
		for chID := range dayStats {
			channelIDSet[chID] = struct{}{}
		}
	}

	return lo.Keys(channelIDSet)
}

func (r *queryResolver) fetchChannelNames(ctx context.Context, channelIDs []int) map[int]string {
	channelNames := make(map[int]string)
	if len(channelIDs) == 0 {
		return channelNames
	}

	queriedChannels, err := r.client.Channel.Query().
		Where(channel.IDIn(channelIDs...)).
		Select(channel.FieldID, channel.FieldName).
		All(ctx)
	if err != nil {
		log.Error(ctx, "failed to query channel names for performance stats",
			log.Any("channelIDs", channelIDs),
			log.Cause(err))

		return channelNames
	}

	for _, ch := range queriedChannels {
		channelNames[ch.ID] = ch.Name
	}

	return channelNames
}

func buildChannelPerformanceResponse(
	statsMap map[string]map[int]*probeStats,
	channelNames map[int]string,
	startDateLocal time.Time,
	daysCount int,
) []*ChannelPerformanceStat {
	response := make([]*ChannelPerformanceStat, 0)

	for i := range daysCount {
		date := startDateLocal.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")

		if dayStats, exists := statsMap[dateStr]; exists && len(dayStats) > 0 {
			for chID, stats := range dayStats {
				channelName := channelNames[chID]
				if channelName == "" {
					channelName = fmt.Sprintf("channel-%d", chID)
				}

				var throughput *float64
				if stats.Throughput > 0 {
					throughput = &stats.Throughput
				}

				var ttftMs *float64
				if stats.TTFTMs > 0 {
					value := stats.TTFTMs
					ttftMs = &value
				}

				response = append(response, &ChannelPerformanceStat{
					Date:         dateStr,
					ChannelID:    fmt.Sprintf("%d", chID),
					ChannelName:  channelName,
					Throughput:   throughput,
					TtftMs:       ttftMs,
					RequestCount: stats.RequestCount,
				})
			}
		}
	}

	return response
}

func (r *queryResolver) buildChannelPerformanceStatsFromExecutions(ctx context.Context, startDateLocal time.Time, offsetSeconds int, daysCount int) ([]*ChannelPerformanceStat, error) {
	dbDriver := r.client.Driver()
	sqlDB, ok := dbDriver.(*sql.Driver)
	if !ok {
		return nil, fmt.Errorf("failed to get underlying SQL driver")
	}

	dialectName := sqlDB.Dialect()
	useDollarPlaceholders := dialectName == dialect.Postgres

	placeholder := "?"
	if useDollarPlaceholders {
		placeholder = "$1"
	}

	queryMode := qb.ThroughputModeRowNumber
	if !useDollarPlaceholders {
		queryMode = qb.ThroughputModeMaxID
	}

	query := qb.BuildDailyPerformanceStatsQuery(
		dialectName,
		r.systemService.TimeLocation(ctx).String(),
		offsetSeconds,
		qb.DailyThroughputByChannel,
		placeholder,
		queryMode,
	)

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}

	rows, err := sqlDB.DB().QueryContext(ctx, query, startDateLocal.UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to query channel performance stats from executions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type rawStat struct {
		Date         string
		ChannelID    int
		TokensCount  int64
		LatencyMs    int64
		FirstTokenMs *float64
		RequestCount int64
		Throughput   *float64
	}

	statsMap := make(map[string]map[int]*rawStat)
	channelTotals := make(map[int]int64)

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled: %w", err)
		}

		var stat rawStat
		if err := rows.Scan(
			&stat.Date,
			&stat.ChannelID,
			&stat.TokensCount,
			&stat.LatencyMs,
			&stat.FirstTokenMs,
			&stat.RequestCount,
			&stat.Throughput,
		); err != nil {
			return nil, fmt.Errorf("failed to scan channel performance stats: %w", err)
		}

		if statsMap[stat.Date] == nil {
			statsMap[stat.Date] = make(map[int]*rawStat)
		}
		statsMap[stat.Date][stat.ChannelID] = &stat
		channelTotals[stat.ChannelID] += stat.RequestCount
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	var channels []channelInfo
	for chID, total := range channelTotals {
		channels = append(channels, channelInfo{
			channelID:    chID,
			requestCount: total,
		})
	}

	topChannels := calculateConfidenceAndSort(
		channels,
		func(c channelInfo) int64 { return c.requestCount },
		func(c channelInfo) float64 { return float64(c.requestCount) },
		topPerformersLimit,
	)

	topChannelIDs := make(map[int]struct{})
	for _, item := range topChannels {
		topChannelIDs[item.stats.channelID] = struct{}{}
	}

	for dateStr, dayStats := range statsMap {
		for chID := range dayStats {
			if _, ok := topChannelIDs[chID]; !ok {
				delete(dayStats, chID)
			}
		}
		if len(dayStats) == 0 {
			delete(statsMap, dateStr)
		}
	}

	var validChannelIDs []int
	for chID := range topChannelIDs {
		validChannelIDs = append(validChannelIDs, chID)
	}

	channelNames := make(map[int]string)
	if len(validChannelIDs) > 0 {
		queriedChannels, err := r.client.Channel.Query().
			Where(channel.IDIn(validChannelIDs...)).
			Select(channel.FieldID, channel.FieldName).
			All(ctx)
		if err != nil {
			log.Error(ctx, "failed to query channel names for performance stats",
				log.Any("channelIDs", validChannelIDs),
				log.Cause(err))
		} else {
			for _, ch := range queriedChannels {
				channelNames[ch.ID] = ch.Name
			}
		}
	}

	response := make([]*ChannelPerformanceStat, 0)
	for i := range daysCount {
		date := startDateLocal.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")

		if dayStats, exists := statsMap[dateStr]; exists && len(dayStats) > 0 {
			for chID, stats := range dayStats {
				channelName := channelNames[chID]
				if channelName == "" {
					channelName = fmt.Sprintf("channel-%d", chID)
				}

				var throughput *float64
				if stats.Throughput != nil && *stats.Throughput > 0 {
					throughput = stats.Throughput
				}

				var ttftMs *float64
				if stats.FirstTokenMs != nil && *stats.FirstTokenMs > 0 {
					value := *stats.FirstTokenMs
					ttftMs = &value
				}

				response = append(response, &ChannelPerformanceStat{
					Date:         dateStr,
					ChannelID:    fmt.Sprintf("%d", chID),
					ChannelName:  channelName,
					Throughput:   throughput,
					TtftMs:       ttftMs,
					RequestCount: safeIntFromInt64(stats.RequestCount),
				})
			}
		}
	}

	return response, nil
}
