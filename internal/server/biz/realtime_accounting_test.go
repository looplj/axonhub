// 职责: 验证中断产生的未知费用会阻止新会话且确认记账后恢复。
// 关联文件:
//
//	internal/server/biz/realtime_accounting.go
//	internal/server/biz/realtime_billing.go
package biz

import (
	"context"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestRealtimeUnknownUsageCannotBypassNextSessionQuota(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))
	key := f.client.APIKey.Create().SetName("guard").SetKey("synthetic").SetProjectID(f.projectID).SetType("user").SaveX(f.ctx)
	require.NoError(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID))
	row := f.client.Request.Create().SetAPIKeyID(key.ID).SetProjectID(f.projectID).SetModelID(f.model).SetFormat("openai/realtime").SetStatus(request.StatusFailed).SetRequestBody(objects.JSONRawMessage(`{}`)).SaveX(f.ctx)
	require.Error(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID))
	require.NoError(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID+1))
	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)
	params := f.createParams(realtimeUsageFixture())
	params.RequestID = row.ID
	params.APIKeyID = &key.ID
	_, err = f.svc.CreateRealtimeUsage(f.ctx, params, snapshot)
	require.NoError(t, err)
	require.NoError(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID))
}
func TestRealtimePriceFreezesEffectiveScheduleAtConnection(t *testing.T) {
	price := realtimePrice("1", "2", "3", "4")
	price.Schedule = &objects.PriceSchedule{Timezone: "UTC", Overrides: []objects.PriceOverride{{Name: "all-day", Priority: 1, When: objects.OverrideWhen{Weekdays: []int{1, 2, 3, 4, 5, 6, 7}}, Items: realtimePrice("10", "20", "30", "40").Items}}}
	f := newRealtimeBillingFixture(t, price)
	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)
	require.Nil(t, snapshot.Price.Schedule)
	_, atStart := ComputeUsageCost(realtimeUsageFixture(), snapshot.Price, time.Now())
	_, afterWeek := ComputeUsageCost(realtimeUsageFixture(), snapshot.Price, time.Now().Add(7*24*time.Hour))
	require.True(t, atStart.Equal(afterWeek))
	require.Equal(t, "10", snapshot.Price.Items[0].Pricing.UsagePerUnit.String())
}

func TestRealtimeOrphanedProcessingAuditBlocksNewSession(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))
	key := f.client.APIKey.Create().SetName("orphan").SetKey("synthetic-orphan").SetProjectID(f.projectID).SetType("user").SaveX(f.ctx)
	f.client.Request.Create().SetAPIKeyID(key.ID).SetProjectID(f.projectID).SetModelID(f.model).SetFormat("openai/realtime").SetStatus(request.StatusProcessing).SetExternalID("rt-session-owner").SetRequestBody(objects.JSONRawMessage(`{}`)).SaveX(f.ctx)
	live := func(context.Context, string) (bool, error) { return true, nil }
	closed := func(context.Context, string) (bool, error) { return false, nil }
	require.NoError(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID, live))
	require.ErrorIs(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID, closed), ErrRealtimeUsageUnreconciled)
}
