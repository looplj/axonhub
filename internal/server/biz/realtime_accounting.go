// 职责: 保存实时待核对审计状态并提供管理员对账操作。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//	internal/server/biz/realtime_billing.go
//	internal/server/biz/quota.go
//
// 注意事项:
//
//	缺失 usage 表示未知，不表示欠费；连接准入只由并发限制和已确认消费配额决定。
//	标记对账不会伪造 UsageLog 或将未知消费写成零费用。

package biz

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/llm"
)

// EnsureRealtimeUsageReconciled retains the internal call contract, but uncertain
// audits do not veto admission. QuotaService enforces confirmed consumption;
// Admission owns the bounded concurrent-session limit, including crashed leases.
func (s *UsageLogService) EnsureRealtimeUsageReconciled(ctx context.Context, keyID int, _ ...func(context.Context, string) (bool, error)) error {
	_, err := authz.RunAsHost(ctx, s.entFromContext(ctx), "realtime-accounting-availability", func(host context.Context) (int, error) {
		return s.entFromContext(host).Request.Query().Where(request.APIKeyIDEQ(keyID), request.FormatEQ(llm.APIFormatOpenAIRealtime.String()), request.StatusEQ(request.StatusFailed), request.Not(request.HasUsageLogs())).Count(host)
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return errors.New("Realtime accounting reconciliation check unavailable")
	}
	return nil
}

// ReconcileRealtimeAudits marks terminal, unpriced audits reviewed. Only a direct
// owner may acknowledge them; it changes neither usage nor monetary balances.
func (s *UsageLogService) ReconcileRealtimeAudits(ctx context.Context, keyID int, ids []int, note string) (int, error) {
	if err := authz.RequireDirectUser(ctx); err != nil {
		return 0, err
	}
	if !scopes.IsDirectOwner(ctx) {
		return 0, errors.New("direct owner required")
	}
	note = strings.TrimSpace(note)
	if keyID <= 0 || len(ids) > 500 || note == "" || len(note) > 1024 {
		return 0, errors.New("invalid reconciliation input")
	}
	user, _ := contexts.GetUser(ctx)
	return authz.RunAsHost(ctx, s.entFromContext(ctx), "realtime-reconcile-audits", func(host context.Context) (int, error) {
		count := 0
		err := s.RunInTransaction(host, func(txctx context.Context) error {
			q := s.entFromContext(txctx).Request.Query().Where(request.APIKeyIDEQ(keyID), request.FormatEQ(llm.APIFormatOpenAIRealtime.String()), request.StatusIn(request.StatusFailed, request.StatusCanceled), request.Not(request.HasUsageLogs()))
			if len(ids) > 0 {
				q = q.Where(request.IDIn(ids...))
			}
			rows, err := q.Limit(500).All(txctx)
			if err != nil {
				return err
			}
			for _, row := range rows {
				body := map[string]any{}
				if len(row.ResponseBody) > 0 {
					if err := json.Unmarshal(row.ResponseBody, &body); err != nil {
						return err
					}
				}
				if body == nil {
					body = map[string]any{}
				}
				body["accounting_reconciliation"] = map[string]any{"status": "reviewed", "note": note, "reviewer_user_id": user.ID, "reviewed_at": time.Now().UTC().Format(time.RFC3339Nano)}
				data, err := json.Marshal(body)
				if err != nil {
					return err
				}
				if _, err := s.entFromContext(txctx).Request.UpdateOneID(row.ID).SetResponseBody(data).Save(txctx); err != nil {
					return err
				}
				count++
			}
			return nil
		})
		return count, err
	})
}
