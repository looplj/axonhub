// 职责: 按项目和 API Key 查询有界的缓存读取指标。
// 关联文件:
//
//	internal/server/biz/plugin_cache_usage_batch.go
//	internal/server/plugin/hostapi_cache_usage.go
//	internal/ent/schema/usage_log.go
//	internal/ent/schema/request.go
//
// 注意事项:
//
//	样本依请求创建时间和 ID 排序。缺失或未报告的用量仍占用样本位置。
package biz

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/llm"
)

const (
	maxCacheUsageKeys         = 100
	maxCacheUsageLastRequests = 100
	maxCacheUsageWindow       = 90 * 24 * 60 * 60
	maxCacheUsageSafeInt      = 1<<53 - 1
)

var ErrInvalidCacheUsageQuery = errors.New("invalid cache usage query")

type CacheUsageQuery struct {
	ProjectID      int       `json:"projectId"`
	KeyIDs         []int     `json:"keyIds,omitempty"`
	AllKeys        bool      `json:"allKeys,omitempty"`
	AfterKeyID     int       `json:"afterKeyId,omitempty"`
	Limit          int       `json:"limit,omitempty"`
	End            time.Time `json:"end,omitempty"`
	WindowSeconds  int       `json:"windowSeconds,omitempty"`
	LastRequests   int       `json:"lastRequests,omitempty"`
	IncludeSamples bool      `json:"includeSamples,omitempty"`
}

type CacheUsagePage struct {
	ProjectID      int             `json:"projectId"`
	End            time.Time       `json:"end"`
	Items          []KeyCacheUsage `json:"items"`
	NextAfterKeyID int             `json:"nextAfterKeyId,omitempty"`
}

type KeyCacheUsage struct {
	KeyID        int                `json:"keyId"`
	Count        int                `json:"count"`
	ValidCount   int                `json:"validCount"`
	UnknownCount int                `json:"unknownCount"`
	InvalidCount int                `json:"invalidCount"`
	PromptTokens int64              `json:"promptTokens"`
	CachedTokens int64              `json:"cachedTokens"`
	CacheRate    *float64           `json:"cacheRate"`
	Truncated    bool               `json:"truncated"`
	Samples      []CacheUsageSample `json:"samples,omitempty"`
}

type CacheUsageSample struct {
	RequestID    int       `json:"requestId"`
	CreatedAt    time.Time `json:"createdAt"`
	Status       string    `json:"status"`
	PromptTokens *int64    `json:"promptTokens,omitempty"`
	CachedTokens *int64    `json:"cachedTokens,omitempty"`
}

// QueryCacheUsage preserves ordinary Ent privacy. An authorized detached host
// context can use the fixed batch query on supported dialects.
func QueryCacheUsage(ctx context.Context, client *ent.Client, q CacheUsageQuery) (CacheUsagePage, error) {
	if err := q.normalize(); err != nil {
		return CacheUsagePage{}, err
	}
	page := CacheUsagePage{ProjectID: q.ProjectID, End: q.End, Items: []KeyCacheUsage{}}
	keys := client.APIKey.Query().Where(apikey.ProjectIDEQ(q.ProjectID), apikey.IDGT(q.AfterKeyID))
	if !q.AllKeys {
		keys.Where(apikey.IDIn(q.KeyIDs...))
	}
	ids, err := keys.Order(apikey.ByID()).Limit(q.Limit + 1).IDs(ctx)
	if err != nil {
		return CacheUsagePage{}, fmt.Errorf("select cache usage keys: %w", err)
	}
	if len(ids) > q.Limit {
		ids = ids[:q.Limit]
		page.NextAfterKeyID = ids[len(ids)-1]
	}
	if len(ids) == 0 {
		return page, nil
	}
	for _, id := range ids {
		if !validCacheUsageID(id) {
			return CacheUsagePage{}, errors.New("cache usage key ID is outside the safe integer range")
		}
	}
	if canBatchCacheUsage(ctx, client.Driver().Dialect()) {
		page.Items, err = queryCacheUsageBatch(ctx, client.Driver(), q, ids)
	} else {
		page.Items, err = queryCacheUsageEnt(ctx, client, q, ids)
	}
	if err != nil {
		return CacheUsagePage{}, fmt.Errorf("query cache usage: %w", err)
	}
	return page, nil
}

func (q *CacheUsageQuery) normalize() error {
	if !validCacheUsageID(q.ProjectID) || q.AfterKeyID < 0 || int64(q.AfterKeyID) > maxCacheUsageSafeInt {
		return fmt.Errorf("%w: invalid projectId or afterKeyId", ErrInvalidCacheUsageQuery)
	}
	if q.AllKeys == (len(q.KeyIDs) != 0) || len(q.KeyIDs) > maxCacheUsageKeys {
		return fmt.Errorf("%w: set allKeys or supply 1–100 keyIds", ErrInvalidCacheUsageQuery)
	}
	for _, id := range q.KeyIDs {
		if !validCacheUsageID(id) {
			return fmt.Errorf("%w: invalid keyId", ErrInvalidCacheUsageQuery)
		}
	}
	if q.Limit < 0 || q.Limit > maxCacheUsageKeys || q.LastRequests < 0 || q.LastRequests > maxCacheUsageLastRequests ||
		q.WindowSeconds < 0 || q.WindowSeconds > maxCacheUsageWindow {
		return fmt.Errorf("%w: limit, lastRequests, or windowSeconds is out of range", ErrInvalidCacheUsageQuery)
	}
	if q.Limit == 0 {
		q.Limit = 50
	}
	if q.LastRequests == 0 {
		q.LastRequests = maxCacheUsageLastRequests
	}
	if q.WindowSeconds == 0 {
		q.WindowSeconds = 24 * 60 * 60
	}
	if q.End.IsZero() {
		q.End = time.Now()
	}
	q.End = q.End.UTC()
	if q.since().Year() < 1 || q.End.Year() > 9999 {
		return fmt.Errorf("%w: end is out of range", ErrInvalidCacheUsageQuery)
	}
	return nil
}

func (q CacheUsageQuery) since() time.Time {
	return q.End.Add(-time.Duration(q.WindowSeconds) * time.Second)
}

func validCacheUsageID(id int) bool {
	return id > 0 && int64(id) <= maxCacheUsageSafeInt
}

type cacheUsageTokens struct {
	RequestID    int     `json:"request_id"`
	Status       *string `json:"cache_read_status"`
	PromptTokens *int64  `json:"prompt_tokens"`
	CachedTokens *int64  `json:"prompt_cached_tokens"`
}

func queryCacheUsageEnt(ctx context.Context, client *ent.Client, q CacheUsageQuery, ids []int) ([]KeyCacheUsage, error) {
	items := make([]KeyCacheUsage, 0, len(ids))
	for _, id := range ids {
		rows, err := client.Request.Query().Where(
			request.ProjectIDEQ(q.ProjectID), request.APIKeyIDEQ(id), request.SourceEQ(request.SourceAPI),
			request.StatusEQ(request.StatusCompleted), request.CreatedAtGTE(q.since()), request.CreatedAtLTE(q.End),
		).Order(request.ByCreatedAt(sql.OrderDesc()), request.ByID(sql.OrderDesc())).
			Limit(q.LastRequests+1).Select(request.FieldID, request.FieldCreatedAt).All(ctx)
		if err != nil {
			return nil, err
		}
		item := KeyCacheUsage{KeyID: id, Truncated: len(rows) > q.LastRequests}
		if item.Truncated {
			rows = rows[:q.LastRequests]
		}
		if len(rows) == 0 {
			items = append(items, item)
			continue
		}
		requestIDs := make([]int, len(rows))
		for i := range rows {
			requestIDs[i] = rows[i].ID
		}
		logs := client.UsageLog.Query().Where(
			usagelog.ProjectIDEQ(q.ProjectID), usagelog.APIKeyIDEQ(id), usagelog.SourceEQ(usagelog.SourceAPI),
			usagelog.RequestIDIn(requestIDs...),
		)
		var latest []struct {
			RequestID int `json:"request_id"`
			Max       int `json:"max"`
		}
		if err := logs.Clone().GroupBy(usagelog.FieldRequestID).Aggregate(ent.Max(usagelog.FieldID)).Scan(ctx, &latest); err != nil {
			return nil, err
		}
		usageIDs := make([]int, len(latest))
		for i, row := range latest {
			usageIDs[i] = row.Max
		}
		var usage []cacheUsageTokens
		err = logs.Where(usagelog.IDIn(usageIDs...)).
			Select(usagelog.FieldRequestID, usagelog.FieldCacheReadStatus, usagelog.FieldPromptTokens, usagelog.FieldPromptCachedTokens).
			Scan(ctx, &usage)
		if err != nil {
			return nil, err
		}
		byRequest := make(map[int]cacheUsageTokens, len(usage))
		for _, row := range usage {
			byRequest[row.RequestID] = row
		}
		for _, row := range rows {
			if err := item.add(q, row.ID, row.CreatedAt, byRequest[row.ID]); err != nil {
				return nil, err
			}
		}
		item.finish()
		items = append(items, item)
	}
	return items, nil
}

func (item *KeyCacheUsage) add(q CacheUsageQuery, requestID int, createdAt time.Time, usage cacheUsageTokens) error {
	if !validCacheUsageID(requestID) {
		return errors.New("cache usage request ID is outside the safe integer range")
	}
	sample := CacheUsageSample{RequestID: requestID, CreatedAt: createdAt.UTC(), Status: "unknown"}
	item.Count++
	if usage.Status == nil || *usage.Status != string(llm.CacheReadReported) {
		item.UnknownCount++
	} else if usage.PromptTokens == nil || usage.CachedTokens == nil || *usage.PromptTokens <= 0 ||
		*usage.PromptTokens > maxCacheUsageSafeInt || *usage.CachedTokens < 0 || *usage.CachedTokens > *usage.PromptTokens {
		sample.Status = "invalid"
		item.InvalidCount++
	} else {
		prompt, cached := *usage.PromptTokens, *usage.CachedTokens
		if item.PromptTokens > maxCacheUsageSafeInt-prompt {
			return errors.New("cache usage token total exceeds the safe integer range")
		}
		sample.Status, sample.PromptTokens, sample.CachedTokens = "reported", usage.PromptTokens, usage.CachedTokens
		item.ValidCount++
		item.PromptTokens += prompt
		item.CachedTokens += cached
	}
	if q.IncludeSamples {
		item.Samples = append(item.Samples, sample)
	}
	return nil
}

func (item *KeyCacheUsage) finish() {
	if item.PromptTokens > 0 {
		rate := float64(item.CachedTokens) / float64(item.PromptTokens)
		item.CacheRate = &rate
	}
}
