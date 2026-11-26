// 职责: 在 PostgreSQL 和 SQLite 上批量读取各 API Key 的最近缓存用量样本。
// 关联文件:
//
//	internal/server/biz/plugin_cache_usage.go
//	internal/ent/schema/request.go
//	internal/ent/schema/usage_log.go
//
// 注意事项:
//
//	固定 SQL 跳过逐实体 Ent privacy，只在受控的 System bypass 中运行。
package biz

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
)

func canBatchCacheUsage(ctx context.Context, d string) bool {
	if d != dialect.Postgres && d != dialect.SQLite {
		return false
	}
	info, ok := authz.GetBypassInfo(ctx)
	if !ok || !info.Principal.IsSystem() || authz.RequireSystemPrincipal(ctx) != nil {
		return false
	}
	decision, ok := privacy.DecisionFromContext(ctx)
	return ok && decision == nil
}

func queryCacheUsageBatch(ctx context.Context, driver dialect.Driver, q CacheUsageQuery, ids []int) ([]KeyCacheUsage, error) {
	if !canBatchCacheUsage(ctx, driver.Dialect()) {
		return nil, privacy.Denyf("batch cache usage requires a System bypass")
	}
	items := make([]KeyCacheUsage, len(ids))
	byKey := make(map[int]*KeyCacheUsage, len(ids))
	for i, id := range ids {
		items[i].KeyID = id
		byKey[id] = &items[i]
	}
	statement, args := cacheUsageBatchSQL(driver.Dialect(), q, ids)
	var rows sql.Rows
	if err := driver.Query(ctx, statement, args, &rows); err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var keyID int
		var createdAt time.Time
		var usage cacheUsageTokens
		if err := rows.Scan(&keyID, &usage.RequestID, &createdAt, &usage.Status, &usage.PromptTokens, &usage.CachedTokens); err != nil {
			return nil, err
		}
		item := byKey[keyID]
		if item == nil {
			return nil, fmt.Errorf("unexpected cache usage key %d", keyID)
		}
		if item.Count >= q.LastRequests {
			item.Truncated = true
			continue
		}
		if err := item.add(q, usage.RequestID, createdAt, usage); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].finish()
	}
	return items, nil
}

func cacheUsageBatchSQL(d string, q CacheUsageQuery, ids []int) (string, []any) {
	var b sql.Builder
	b.SetDialect(d)
	const columns = `r.id, r.api_key_id, r.project_id, r.created_at`
	const order = `r.created_at DESC, r.id DESC`
	recent := func(writeKey func()) {
		b.WriteString(`SELECT `).WriteString(columns).WriteString(` FROM requests AS r WHERE r.api_key_id = `)
		writeKey()
		b.WriteString(` AND r.project_id = `).Arg(q.ProjectID).
			WriteString(` AND r.status = `).Arg(request.StatusCompleted).
			WriteString(` AND r.source = `).Arg(request.SourceAPI).
			WriteString(` AND r.created_at >= `).Arg(q.since()).
			WriteString(` AND r.created_at <= `).Arg(q.End).
			WriteString(` ORDER BY `).WriteString(order).WriteString(` LIMIT `).Arg(q.LastRequests + 1)
	}
	b.WriteString(`WITH samples AS (`)
	if d == dialect.Postgres {
		// The key/created_at index can stop the scan early for each key.
		b.WriteString(`SELECT r.* FROM (VALUES `)
		for i, id := range ids {
			if i != 0 {
				b.Comma()
			}
			b.WriteString(`(CAST(`).Arg(id).WriteString(` AS BIGINT))`)
		}
		b.WriteString(`) AS k(id) CROSS JOIN LATERAL (`)
		recent(func() { b.WriteString(`k.id`) })
		b.WriteString(`) AS r`)
	} else {
		// SQLite has no LATERAL. An indexed LIMIT per branch avoids ranking the
		// whole window. 100 keys fit within 500 UNION branches and 999 parameters.
		for i, id := range ids {
			if i != 0 {
				b.WriteString(` UNION ALL `)
			}
			b.WriteString(`SELECT * FROM (`)
			recent(func() { b.Arg(id) })
			b.WriteString(`)`)
		}
	}
	// Rank the requests first. Missing or untrusted usage consumes its place in the sample.
	b.WriteString(`) SELECT r.api_key_id, r.id, r.created_at, u.`).WriteString(usagelog.FieldCacheReadStatus).
		WriteString(`, u.prompt_tokens, u.prompt_cached_tokens FROM samples AS r
 LEFT JOIN usage_logs AS u ON u.id = (
   SELECT MAX(l.id) FROM usage_logs AS l
   WHERE l.request_id = r.id AND l.project_id = r.project_id AND l.api_key_id = r.api_key_id AND l.source = `).
		Arg(usagelog.SourceAPI).WriteString(`
 ) ORDER BY r.api_key_id, `).WriteString(order)
	return b.Query()
}
