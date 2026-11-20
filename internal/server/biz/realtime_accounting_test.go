// 职责: 验证未知费用不锁定密钥且对账只由直接管理员执行。
// 关联文件:
//
//	internal/server/biz/realtime_accounting.go
//	internal/server/biz/realtime_billing.go

package biz

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
)

func TestRealtimeReconcileAuditsRequiresOwnerAndPreservesUsage(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))
	key := f.client.APIKey.Create().SetName("review").SetKey("review-key").SetProjectID(f.projectID).SetType("user").SaveX(f.ctx)
	other := f.client.APIKey.Create().SetName("other").SetKey("other-key").SetProjectID(f.projectID).SetType("user").SaveX(f.ctx)
	makeAudit := func(id int) *ent.Request {
		return f.client.Request.Create().SetAPIKeyID(id).SetProjectID(f.projectID).SetModelID(f.model).SetFormat("openai/realtime").SetStatus(request.StatusFailed).SetRequestBody(objects.JSONRawMessage(`{}`)).SetResponseBody(objects.JSONRawMessage(`{"usage_status":"unknown"}`)).SaveX(f.ctx)
	}
	selected, unselected, foreign := makeAudit(key.ID), makeAudit(key.ID), makeAudit(other.ID)
	owner := f.client.User.Create().SetEmail("reviewer@example.test").SetPassword("synthetic").SetIsOwner(true).SaveX(f.ctx)
	base := ent.NewContext(context.Background(), f.client)
	keyCtx := authz.NewAPIKeyContext(base, key.ID, f.projectID)
	_, err := f.svc.ReconcileRealtimeAudits(keyCtx, key.ID, nil, "checked")
	require.Error(t, err)
	nonowner := *owner
	nonowner.IsOwner = false
	userCtx := contexts.WithUser(authz.NewUserContext(base, owner.ID), &nonowner)
	_, err = f.svc.ReconcileRealtimeAudits(userCtx, key.ID, nil, "checked")
	require.Error(t, err)
	ownerCtx := contexts.WithUser(authz.NewUserContext(base, owner.ID), owner)
	count, err := f.svc.ReconcileRealtimeAudits(ownerCtx, key.ID, []int{selected.ID, foreign.ID}, "provider checked; no confirmed usage")
	require.NoError(t, err)
	require.Equal(t, 1, count)
	var metadata map[string]any
	require.NoError(t, json.Unmarshal(f.client.Request.GetX(f.ctx, selected.ID).ResponseBody, &metadata))
	require.Equal(t, "unknown", metadata["usage_status"])
	require.Equal(t, "reviewed", metadata["accounting_reconciliation"].(map[string]any)["status"])
	for _, row := range []*ent.Request{unselected, foreign} {
		require.JSONEq(t, `{"usage_status":"unknown"}`, string(f.client.Request.GetX(f.ctx, row.ID).ResponseBody))
	}
	require.Zero(t, f.client.UsageLog.Query().CountX(f.ctx), "review cannot invent consumption or costs")
}

func TestRealtimeUnknownUsageDoesNotDeclareDebtOrLockKey(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))
	key := f.client.APIKey.Create().SetName("guard").SetKey("synthetic").SetProjectID(f.projectID).SetType("user").SaveX(f.ctx)
	require.NoError(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID))
	row := f.client.Request.Create().SetAPIKeyID(key.ID).SetProjectID(f.projectID).SetModelID(f.model).SetFormat("openai/realtime").SetStatus(request.StatusFailed).SetRequestBody(objects.JSONRawMessage(`{}`)).SaveX(f.ctx)
	require.NoError(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID), "missing usage is uncertain, not confirmed debt")
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
	_, atStart := ComputeRealtimeUsageCost(realtimeUsageFixture(), snapshot.Price, time.Now())
	_, afterWeek := ComputeRealtimeUsageCost(realtimeUsageFixture(), snapshot.Price, time.Now().Add(7*24*time.Hour))
	require.True(t, atStart.Equal(afterWeek))
	require.Equal(t, "10", snapshot.Price.Items[0].Pricing.UsagePerUnit.String())
}

func TestRealtimeOrphanedProcessingAuditDoesNotPermanentlyBlockKey(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))
	key := f.client.APIKey.Create().SetName("orphan").SetKey("synthetic-orphan").SetProjectID(f.projectID).SetType("user").SaveX(f.ctx)
	f.client.Request.Create().SetAPIKeyID(key.ID).SetProjectID(f.projectID).SetModelID(f.model).SetFormat("openai/realtime").SetStatus(request.StatusProcessing).SetExternalID("rt-session-owner").SetRequestBody(objects.JSONRawMessage(`{}`)).SaveX(f.ctx)
	live := func(context.Context, string) (bool, error) { return true, nil }
	closed := func(context.Context, string) (bool, error) { return false, nil }
	require.NoError(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID, live))
	require.NoError(t, f.svc.EnsureRealtimeUsageReconciled(f.ctx, key.ID, closed))
}
