package biz

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/ent/usagestat"
	"github.com/looplj/axonhub/internal/log"
)

// usageStatConflictColumns are the natural key of a day-granularity usage
// aggregate row. api_key_id / channel_id are stored as 0 when absent so the
// unique index works on every database (NULLs would never conflict).
var usageStatConflictColumns = []string{
	usagestat.FieldDate,
	usagestat.FieldAPIKeyID,
	usagestat.FieldModelID,
	usagestat.FieldChannelID,
	usagestat.FieldProjectID,
}

// LocalDateString renders a time in the given location as YYYY-MM-DD.
func LocalDateString(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("2006-01-02")
}

// upsertUsageStat increments the day aggregate row for a freshly created
// usage log. Failure is non-fatal: stats are best-effort and the detail log
// write is the source of truth (a later backfill can repair gaps).
func (s *UsageLogService) upsertUsageStat(ctx context.Context, ul *ent.UsageLog) error {
	if ul == nil {
		return nil
	}

	loc := s.SystemService.TimeLocation(ctx)
	date := LocalDateString(ul.CreatedAt, loc)

	// ul.APIKeyID is 0 when unset; keep 0 as "no API key" so the unique
	// index works uniformly (NULLs would never conflict on PG).
	apiKeyID := ul.APIKeyID
	channelID := ul.ChannelID

	totalCost := 0.0
	if ul.TotalCost != nil {
		totalCost = *ul.TotalCost
	}

	err := s.entFromContext(ctx).UsageStat.Create().
		SetDate(date).
		SetAPIKeyID(apiKeyID).
		SetProjectID(ul.ProjectID).
		SetChannelID(channelID).
		SetModelID(ul.ModelID).
		SetRequestCount(1).
		SetPromptTokens(ul.PromptTokens).
		SetCompletionTokens(ul.CompletionTokens).
		SetTotalTokens(ul.TotalTokens).
		SetPromptCachedTokens(ul.PromptCachedTokens).
		SetPromptWriteCachedTokens(ul.PromptWriteCachedTokens).
		SetCompletionReasoningTokens(ul.CompletionReasoningTokens).
		SetTotalCost(totalCost).
		OnConflict(sql.ConflictColumns(usageStatConflictColumns...)).
		Update(func(u *ent.UsageStatUpsert) {
			u.AddRequestCount(1)
			u.AddPromptTokens(ul.PromptTokens)
			u.AddCompletionTokens(ul.CompletionTokens)
			u.AddTotalTokens(ul.TotalTokens)
			u.AddPromptCachedTokens(ul.PromptCachedTokens)
			u.AddPromptWriteCachedTokens(ul.PromptWriteCachedTokens)
			u.AddCompletionReasoningTokens(ul.CompletionReasoningTokens)
			u.AddTotalCost(totalCost)
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to upsert usage stat: %w", err)
	}

	return nil
}

// BackfillUsageStats builds day aggregates from the existing usage_logs
// detail table. It is safe to call repeatedly: it skips when aggregates
// already exist, and each batch upsert is idempotent. Run it once after
// upgrading to the usage_stats version so historical statistics survive
// subsequent GC log cleanup.
func (s *UsageLogService) BackfillUsageStats(ctx context.Context) error {
	client := s.entFromContext(ctx)

	existing, err := client.UsageStat.Query().Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to count existing usage stats: %w", err)
	}
	if existing > 0 {
		log.Info(ctx, "Usage stats backfill skipped: aggregates already present",
			log.Int("existing_rows", existing))
		return nil
	}

	logsTotal, err := client.UsageLog.Query().Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to count usage logs: %w", err)
	}
	if logsTotal == 0 {
		log.Info(ctx, "Usage stats backfill skipped: no usage logs to aggregate")
		return nil
	}

	const batchSize = 2000
	loc := s.SystemService.TimeLocation(ctx)

	type statKey struct {
		date      string
		apiKeyID  int
		projectID int
		channelID int
		modelID   string
	}

	type statVal struct {
		requestCount              int64
		promptTokens              int64
		completionTokens          int64
		totalTokens               int64
		promptCachedTokens        int64
		promptWriteCachedTokens   int64
		completionReasoningTokens int64
		totalCost                 float64
	}

	agg := make(map[statKey]*statVal)
	processed := 0

	for {
		logs, err := client.UsageLog.Query().
			Order(ent.Asc(usagelog.FieldID)).
			Limit(batchSize).
			All(ctx)
		if err != nil {
			return fmt.Errorf("failed to query usage logs for backfill: %w", err)
		}
		if len(logs) == 0 {
			break
		}

		for _, ul := range logs {
			key := statKey{
				date:      LocalDateString(ul.CreatedAt, loc),
				apiKeyID:  ul.APIKeyID,
				projectID: ul.ProjectID,
				channelID: ul.ChannelID,
				modelID:   ul.ModelID,
			}
			v, ok := agg[key]
			if !ok {
				v = &statVal{}
				agg[key] = v
			}
			v.requestCount++
			v.promptTokens += ul.PromptTokens
			v.completionTokens += ul.CompletionTokens
			v.totalTokens += ul.TotalTokens
			v.promptCachedTokens += ul.PromptCachedTokens
			v.promptWriteCachedTokens += ul.PromptWriteCachedTokens
			v.completionReasoningTokens += ul.CompletionReasoningTokens
			if ul.TotalCost != nil {
				v.totalCost += *ul.TotalCost
			}
		}

		processed += len(logs)
		if len(logs) < batchSize {
			break
		}
	}

	// Upsert aggregates in batches (idempotent on conflict).
	bulk := make([]*ent.UsageStatCreate, 0, len(agg))
	for key, v := range agg {
		bulk = append(bulk, client.UsageStat.Create().
			SetDate(key.date).
			SetAPIKeyID(key.apiKeyID).
			SetProjectID(key.projectID).
			SetChannelID(key.channelID).
			SetModelID(key.modelID).
			SetRequestCount(v.requestCount).
			SetPromptTokens(v.promptTokens).
			SetCompletionTokens(v.completionTokens).
			SetTotalTokens(v.totalTokens).
			SetPromptCachedTokens(v.promptCachedTokens).
			SetPromptWriteCachedTokens(v.promptWriteCachedTokens).
			SetCompletionReasoningTokens(v.completionReasoningTokens).
			SetTotalCost(v.totalCost))
	}

	for i := 0; i < len(bulk); i += batchSize {
		end := i + batchSize
		if end > len(bulk) {
			end = len(bulk)
		}
		// Backfill only runs when the table is empty, so conflicts should
		// never happen; UpdateNewValues keeps re-runs idempotent.
		if err := client.UsageStat.CreateBulk(bulk[i:end]...).
			OnConflict(sql.ConflictColumns(usageStatConflictColumns...)).
			UpdateNewValues().
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to bulk insert usage stats: %w", err)
		}
	}

	log.Info(ctx, "Usage stats backfill completed",
		log.Int("usage_logs_processed", processed),
		log.Int("aggregate_rows", len(agg)))

	return nil
}
