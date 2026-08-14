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

// backfillBatchSize controls how many aggregate rows are written per bulk
// upsert. Package-level so tests can shrink it to exercise multi-batch runs.
var backfillBatchSize = 2000

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

	// Paginate by ID cursor: each page must be strictly after the last ID of
	// the previous page, otherwise pages repeat forever once the log count
	// exceeds the batch size.
	var lastID int
	for {
		q := client.UsageLog.Query().
			Order(ent.Asc(usagelog.FieldID)).
			Limit(backfillBatchSize)
		if lastID > 0 {
			q = q.Where(usagelog.IDGT(lastID))
		}
		logs, err := q.All(ctx)
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

		lastID = logs[len(logs)-1].ID
		processed += len(logs)
		if len(logs) < backfillBatchSize {
			break
		}
	}

	// Collect aggregate rows first (builders must be created on the
	// transactional client so the whole write is atomic — a failed batch
	// cannot leave partial aggregates behind).
	type aggRow struct {
		key statKey
		val *statVal
	}
	rows := make([]aggRow, 0, len(agg))
	for key, v := range agg {
		rows = append(rows, aggRow{key: key, val: v})
	}

	tx, err := client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("failed to start backfill transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	for i := 0; i < len(rows); i += backfillBatchSize {
		end := min(i+backfillBatchSize, len(rows))
		builders := make([]*ent.UsageStatCreate, 0, end-i)
		for _, r := range rows[i:end] {
			builders = append(builders, txClient.UsageStat.Create().
				SetDate(r.key.date).
				SetAPIKeyID(r.key.apiKeyID).
				SetProjectID(r.key.projectID).
				SetChannelID(r.key.channelID).
				SetModelID(r.key.modelID).
				SetRequestCount(r.val.requestCount).
				SetPromptTokens(r.val.promptTokens).
				SetCompletionTokens(r.val.completionTokens).
				SetTotalTokens(r.val.totalTokens).
				SetPromptCachedTokens(r.val.promptCachedTokens).
				SetPromptWriteCachedTokens(r.val.promptWriteCachedTokens).
				SetCompletionReasoningTokens(r.val.completionReasoningTokens).
				SetTotalCost(r.val.totalCost))
		}
		// Backfill only runs when the table is empty, so conflicts should
		// never happen; UpdateNewValues keeps re-runs idempotent.
		if err := txClient.UsageStat.CreateBulk(builders...).
			OnConflict(sql.ConflictColumns(usageStatConflictColumns...)).
			UpdateNewValues().
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to bulk insert usage stats: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit backfill: %w", err)
	}

	log.Info(ctx, "Usage stats backfill completed",
		log.Int("usage_logs_processed", processed),
		log.Int("aggregate_rows", len(agg)))

	return nil
}
