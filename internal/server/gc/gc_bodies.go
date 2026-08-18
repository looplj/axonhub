package gc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/trace"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

var (
	emptyJSONObject = objects.JSONRawMessage(`{}`)
	emptyJSONArray  = []objects.JSONRawMessage{}
)

type bodyPayloadKind string

const (
	bodyPayloadRequest  bodyPayloadKind = biz.CleanupResourceRequestBodies
	bodyPayloadResponse bodyPayloadKind = biz.CleanupResourceResponseBodies
	bodyPayloadChunks   bodyPayloadKind = biz.CleanupResourceResponseChunks
)

func (w *Worker) cleanupRequestBodies(ctx context.Context, cleanupDays int) error {
	return w.cleanupBodyPayloads(ctx, bodyPayloadRequest, cleanupDays)
}

func (w *Worker) cleanupResponseBodies(ctx context.Context, cleanupDays int) error {
	return w.cleanupBodyPayloads(ctx, bodyPayloadResponse, cleanupDays)
}

func (w *Worker) cleanupResponseChunks(ctx context.Context, cleanupDays int) error {
	return w.cleanupBodyPayloads(ctx, bodyPayloadChunks, cleanupDays)
}

func (w *Worker) cleanupBodyPayloads(ctx context.Context, kind bodyPayloadKind, cleanupDays int) error {
	if cleanupDays <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -cleanupDays)

	primaryID, err := w.primaryStorageID(ctx)
	if err != nil {
		return err
	}

	reqCount, reqErr := w.stripRequestPayloads(ctx, kind, cutoff, primaryID)
	execCount, execErr := w.stripExecutionPayloads(ctx, kind, cutoff, primaryID)

	log.Debug(ctx, "Stripped stored payloads",
		log.String("resource", string(kind)),
		log.Int("requests", reqCount),
		log.Int("executions", execCount),
		log.Time("cutoff_time", cutoff),
	)

	return errors.Join(reqErr, execErr)
}

func (w *Worker) previewBodyCleanup(ctx context.Context, resourceType string, days int, now time.Time) (GcCleanupPreviewItem, error) {
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.AddDate(0, 0, -days)

	// Preview must not inspect JSON/TOAST columns. CAST(jsonb AS TEXT) on a
	// multi-GB body table blocks the admin dialog until the proxy times out.
	// created_at is indexed; the count is an upper bound (already-stripped
	// placeholders are included). Actual GC still uses applyBodyPayloadPredicate.
	reqCount, err := w.Ent.Request.Query().
		Where(
			request.CreatedAtLT(cutoff),
			request.Not(request.HasTraceWith(trace.StatusEQ(trace.StatusRetained))),
		).
		Count(ctx)
	if err != nil {
		return GcCleanupPreviewItem{}, fmt.Errorf("failed to count %s on requests: %w", resourceType, err)
	}

	execCount, err := w.Ent.RequestExecution.Query().
		Where(
			requestexecution.CreatedAtLT(cutoff),
			requestexecution.Not(requestexecution.HasRequestWith(
				request.HasTraceWith(trace.StatusEQ(trace.StatusRetained)),
			)),
		).
		Count(ctx)
	if err != nil {
		return GcCleanupPreviewItem{}, fmt.Errorf("failed to count %s on executions: %w", resourceType, err)
	}

	return GcCleanupPreviewItem{
		ResourceType:   resourceType,
		EstimatedCount: reqCount + execCount,
		CutoffTime:     cutoff,
		RetentionDays:  days,
	}, nil
}

func (w *Worker) stripRequestPayloads(ctx context.Context, kind bodyPayloadKind, cutoff time.Time, primaryID int) (int, error) {
	batchSize := w.getBatchSize()
	total := 0
	failed := 0
	skipIDs := make([]int, 0)
	cache := make(map[int]*ent.DataStorage)

	for {
		query := w.Ent.Request.Query().
			Select(
				request.FieldID,
				request.FieldProjectID,
				request.FieldDataStorageID,
			).
			Where(
				request.CreatedAtLT(cutoff),
				request.Not(request.HasTraceWith(trace.StatusEQ(trace.StatusRetained))),
			)
		if len(skipIDs) > 0 {
			query = query.Where(request.IDNotIn(skipIDs...))
		}

		reqs, err := query.
			Modify(func(s *sql.Selector) {
				applyBodyPayloadPredicate(s, kind, primaryID, request.FieldRequestBody, request.FieldRequestHeaders, request.FieldResponseBody, request.FieldResponseChunks, request.FieldDataStorageID)
			}).
			Order(ent.Asc(request.FieldID)).
			Limit(batchSize).
			All(ctx)
		if err != nil {
			return total, fmt.Errorf("failed to query requests for %s cleanup: %w", kind, err)
		}

		if len(reqs) == 0 {
			if failed > 0 {
				return total, fmt.Errorf("external payload delete failed for %d request %s rows", failed, kind)
			}

			return total, nil
		}

		ids := make([]int, 0, len(reqs))
		for _, req := range reqs {
			if err := w.deleteRequestExternalPayload(ctx, req, kind, cache); err != nil {
				// 外存还在时不能打 DB 已剥标记，否则 Load* 仍读对象且后续 GC 选不中。
				log.Warn(ctx, "Skip stripping request payload; external delete failed",
					log.Cause(err),
					log.Int("request_id", req.ID),
					log.String("resource", string(kind)),
				)
				skipIDs = append(skipIDs, req.ID)
				failed++

				continue
			}

			ids = append(ids, req.ID)
		}

		if len(ids) == 0 {
			continue
		}

		if err := w.clearRequestPayloadColumns(ctx, kind, ids); err != nil {
			return total, err
		}

		total += len(ids)
	}
}

func (w *Worker) stripExecutionPayloads(ctx context.Context, kind bodyPayloadKind, cutoff time.Time, primaryID int) (int, error) {
	batchSize := w.getBatchSize()
	total := 0
	failed := 0
	skipIDs := make([]int, 0)
	cache := make(map[int]*ent.DataStorage)

	for {
		query := w.Ent.RequestExecution.Query().
			Select(
				requestexecution.FieldID,
				requestexecution.FieldProjectID,
				requestexecution.FieldRequestID,
				requestexecution.FieldDataStorageID,
			).
			Where(
				requestexecution.CreatedAtLT(cutoff),
				requestexecution.Not(requestexecution.HasRequestWith(
					request.HasTraceWith(trace.StatusEQ(trace.StatusRetained)),
				)),
			)
		if len(skipIDs) > 0 {
			query = query.Where(requestexecution.IDNotIn(skipIDs...))
		}

		execs, err := query.
			Modify(func(s *sql.Selector) {
				applyBodyPayloadPredicate(s, kind, primaryID, requestexecution.FieldRequestBody, requestexecution.FieldRequestHeaders, requestexecution.FieldResponseBody, requestexecution.FieldResponseChunks, requestexecution.FieldDataStorageID)
			}).
			Order(ent.Asc(requestexecution.FieldID)).
			Limit(batchSize).
			All(ctx)
		if err != nil {
			return total, fmt.Errorf("failed to query executions for %s cleanup: %w", kind, err)
		}

		if len(execs) == 0 {
			if failed > 0 {
				return total, fmt.Errorf("external payload delete failed for %d execution %s rows", failed, kind)
			}

			return total, nil
		}

		ids := make([]int, 0, len(execs))
		for _, exec := range execs {
			if err := w.deleteExecutionExternalPayload(ctx, exec, kind, cache); err != nil {
				log.Warn(ctx, "Skip stripping execution payload; external delete failed",
					log.Cause(err),
					log.Int("execution_id", exec.ID),
					log.String("resource", string(kind)),
				)
				skipIDs = append(skipIDs, exec.ID)
				failed++

				continue
			}

			ids = append(ids, exec.ID)
		}

		if len(ids) == 0 {
			continue
		}

		if err := w.clearExecutionPayloadColumns(ctx, kind, ids); err != nil {
			return total, err
		}

		total += len(ids)
	}
}

func (w *Worker) clearRequestPayloadColumns(ctx context.Context, kind bodyPayloadKind, ids []int) error {
	upd := w.Ent.Request.Update().Where(request.IDIn(ids...))

	switch kind {
	case bodyPayloadRequest:
		// request_body 是 Ent Immutable JSON，只能走 Modify。
		// 必须写方言 JSON 字面量：[]byte 会被 pgx 绑成 bytea，Postgres jsonb 列会失败。
		upd = upd.ClearRequestHeaders().Modify(func(u *sql.UpdateBuilder) {
			setImmutableJSONObject(u, request.FieldRequestBody)
		})
	case bodyPayloadResponse:
		upd = upd.SetResponseBody(emptyJSONObject)
	case bodyPayloadChunks:
		upd = upd.SetResponseChunks(emptyJSONArray)
	}

	if _, err := upd.Save(ctx); err != nil {
		return fmt.Errorf("failed to clear request %s: %w", kind, err)
	}

	return nil
}

func (w *Worker) clearExecutionPayloadColumns(ctx context.Context, kind bodyPayloadKind, ids []int) error {
	upd := w.Ent.RequestExecution.Update().Where(requestexecution.IDIn(ids...))

	switch kind {
	case bodyPayloadRequest:
		upd = upd.ClearRequestHeaders().Modify(func(u *sql.UpdateBuilder) {
			setImmutableJSONObject(u, requestexecution.FieldRequestBody)
		})
	case bodyPayloadResponse:
		upd = upd.SetResponseBody(emptyJSONObject)
	case bodyPayloadChunks:
		upd = upd.SetResponseChunks(emptyJSONArray)
	}

	if _, err := upd.Save(ctx); err != nil {
		return fmt.Errorf("failed to clear execution %s: %w", kind, err)
	}

	return nil
}

func (w *Worker) deleteRequestExternalPayload(ctx context.Context, req *ent.Request, kind bodyPayloadKind, cache map[int]*ent.DataStorage) error {
	if req == nil || req.DataStorageID == 0 || w.DataStorageService == nil {
		return nil
	}

	ds, err := w.getDataStorageCached(ctx, req.DataStorageID, cache)
	if err != nil {
		return fmt.Errorf("load data storage for request %d: %w", req.ID, err)
	}

	if ds == nil || ds.Primary {
		return nil
	}

	var keys []string

	switch kind {
	case bodyPayloadRequest:
		keys = []string{biz.GenerateRequestBodyKey(req.ProjectID, req.ID)}
	case bodyPayloadResponse:
		keys = []string{biz.GenerateResponseBodyKey(req.ProjectID, req.ID)}
	case bodyPayloadChunks:
		keys = []string{biz.GenerateResponseChunksKey(req.ProjectID, req.ID)}
	}

	for _, key := range keys {
		if err := w.DataStorageService.DeleteData(ctx, ds, key); err != nil {
			return fmt.Errorf("delete request %d key %s: %w", req.ID, key, err)
		}
	}

	return nil
}

func (w *Worker) deleteExecutionExternalPayload(ctx context.Context, exec *ent.RequestExecution, kind bodyPayloadKind, cache map[int]*ent.DataStorage) error {
	if exec == nil || exec.DataStorageID == 0 || w.DataStorageService == nil {
		return nil
	}

	ds, err := w.getDataStorageCached(ctx, exec.DataStorageID, cache)
	if err != nil {
		return fmt.Errorf("load data storage for execution %d: %w", exec.ID, err)
	}

	if ds == nil || ds.Primary {
		return nil
	}

	var keys []string

	switch kind {
	case bodyPayloadRequest:
		keys = []string{biz.GenerateExecutionRequestBodyKey(exec.ProjectID, exec.RequestID, exec.ID)}
	case bodyPayloadResponse:
		keys = []string{biz.GenerateExecutionResponseBodyKey(exec.ProjectID, exec.RequestID, exec.ID)}
	case bodyPayloadChunks:
		keys = []string{biz.GenerateExecutionResponseChunksKey(exec.ProjectID, exec.RequestID, exec.ID)}
	}

	for _, key := range keys {
		if err := w.DataStorageService.DeleteData(ctx, ds, key); err != nil {
			return fmt.Errorf("delete execution %d key %s: %w", exec.ID, key, err)
		}
	}

	return nil
}

func (w *Worker) primaryStorageID(ctx context.Context) (int, error) {
	if w.DataStorageService == nil {
		return 0, nil
	}

	ds, err := w.DataStorageService.GetPrimaryDataStorage(ctx)
	if err != nil {
		return 0, fmt.Errorf("get primary data storage for body cleanup: %w", err)
	}

	if ds == nil {
		return 0, nil
	}

	return ds.ID, nil
}

func applyBodyPayloadPredicate(
	s *sql.Selector,
	kind bodyPayloadKind,
	primaryID int,
	bodyCol, headersCol, responseCol, chunksCol, storageCol string,
) {
	s.Where(sql.P(func(b *sql.Builder) {
		b.WriteByte('(')

		switch kind {
		case bodyPayloadRequest:
			appendJSONNotPlaceholder(b, s, bodyCol, "{}", "null", "")
			b.WriteString(" OR ")
			appendJSONNotPlaceholder(b, s, headersCol, "{}", "null", "")

			if primaryID > 0 {
				// 生产外存：DB 里 request_body 是 {}，靠 headers 还在判断对象未剥。
				b.WriteString(" OR (")
				b.WriteString(s.C(storageCol))
				b.WriteString(" IS NOT NULL AND ")
				b.WriteString(s.C(storageCol))
				b.WriteString(" <> ")
				b.Arg(primaryID)
				b.WriteString(" AND ")
				b.WriteString(s.C(headersCol))
				b.WriteString(" IS NOT NULL)")
			}
		case bodyPayloadResponse:
			appendJSONNotPlaceholder(b, s, responseCol, "{}", "null", "")
			appendExternalNullPayload(b, s, primaryID, storageCol, responseCol)
		case bodyPayloadChunks:
			appendJSONNotPlaceholder(b, s, chunksCol, "[]", "{}", "null", "")
			appendExternalNullPayload(b, s, primaryID, storageCol, chunksCol)
		}

		b.WriteByte(')')
	}))
}

func appendExternalNullPayload(b *sql.Builder, s *sql.Selector, primaryID int, storageCol, payloadCol string) {
	if primaryID <= 0 {
		return
	}

	// 历史外存行：对象在 S3/FS，DB 列是 NULL。窗口内补剥一次后打成 {} / []，下次不再入选。
	b.WriteString(" OR (")
	b.WriteString(s.C(storageCol))
	b.WriteString(" IS NOT NULL AND ")
	b.WriteString(s.C(storageCol))
	b.WriteString(" <> ")
	b.Arg(primaryID)
	b.WriteString(" AND ")
	b.WriteString(s.C(payloadCol))
	b.WriteString(" IS NULL)")
}

func appendJSONNotPlaceholder(b *sql.Builder, s *sql.Selector, column string, placeholders ...string) {
	col := s.C(column)
	text := jsonTextCast(s, column)

	b.WriteByte('(')
	b.WriteString(col)
	b.WriteString(" IS NOT NULL AND ")
	b.WriteString(text)
	b.WriteString(" NOT IN (")

	for i, placeholder := range placeholders {
		if i > 0 {
			b.WriteByte(',')
		}

		b.Arg(placeholder)
	}

	b.WriteString("))")
}

func jsonTextCast(s *sql.Selector, column string) string {
	col := s.C(column)
	if s.Dialect() == dialect.MySQL {
		return fmt.Sprintf("CAST(%s AS CHAR)", col)
	}

	return fmt.Sprintf("CAST(%s AS TEXT)", col)
}

// setImmutableJSONObject writes JSON {} without binding []byte.
// Postgres + pgx 会把 []byte 编成 bytea，jsonb 列会报类型错误。
func setImmutableJSONObject(u *sql.UpdateBuilder, column string) {
	u.Set(column, jsonObjectSQL(u.Dialect()))
}

func jsonObjectSQL(d string) sql.Querier {
	if d == dialect.Postgres {
		return sql.Expr("'{}'::jsonb")
	}

	return sql.Expr("'{}'")
}
