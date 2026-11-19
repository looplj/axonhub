// 职责: 阻止通过中断未结算的实时生成绕过费用配额。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//	internal/server/biz/realtime_billing.go
//	internal/server/biz/quota.go
//
// 注意事项:
//
//	缺失最终 usage 的失败响应不是免费调用；补录经核对的 UsageLog 前拒绝同密钥后续实时会话。
package biz

import (
	"context"
	"errors"
	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
)

var ErrRealtimeUsageUnreconciled = errors.New("Realtime usage is pending reconciliation; additional realtime sessions are blocked")

func (s *UsageLogService) EnsureRealtimeUsageReconciled(ctx context.Context, keyID int, activeSession ...func(context.Context, string) (bool, error)) error {
	unknown, err := authz.RunAsHost(ctx, s.entFromContext(ctx), "realtime-accounting-reconciliation", func(host context.Context) (bool, error) {
		return s.entFromContext(host).Request.Query().Where(request.APIKeyIDEQ(keyID), request.FormatEQ(llm.APIFormatOpenAIRealtime.String()), request.StatusEQ(request.StatusFailed), request.Not(request.HasUsageLogs())).Exist(host)
	})
	if err != nil {
		return errors.New("Realtime accounting reconciliation check unavailable")
	}
	if unknown {
		return ErrRealtimeUsageUnreconciled
	}
	if len(activeSession) > 0 {
		sessions, err := authz.RunAsHost(ctx, s.entFromContext(ctx), "realtime-pending-accounting", func(host context.Context) ([]*ent.Request, error) {
			return s.entFromContext(host).Request.Query().Where(request.APIKeyIDEQ(keyID), request.FormatEQ(llm.APIFormatOpenAIRealtime.String()), request.StatusEQ(request.StatusProcessing), request.Not(request.HasUsageLogs())).Select(request.FieldExternalID).All(host)
		})
		if err != nil {
			return errors.New("Realtime pending accounting check unavailable")
		}
		checked := map[string]bool{}
		for _, row := range sessions {
			if checked[row.ExternalID] {
				continue
			}
			live, err := activeSession[0](ctx, row.ExternalID)
			if err != nil {
				return errors.New("Realtime session ownership check unavailable")
			}
			if !live {
				return ErrRealtimeUsageUnreconciled
			}
			checked[row.ExternalID] = true
		}
	}
	return nil
}
