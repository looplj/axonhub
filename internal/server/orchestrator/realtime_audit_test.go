// 职责: 验证已计费响应审计失败后能够收尾且不会重复记账。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//	internal/server/biz/realtime_billing.go
package orchestrator

import (
	"context"
	"errors"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestRealtimeAuditRetryDoesNotRepeatUsage(t *testing.T) {
	ctx, db := setupTest(t)
	p := db.Project.Create().SetName("audit-retry").SaveX(ctx)
	ctx = contexts.WithProjectID(ctx, p.ID)
	key := db.APIKey.Create().SetName("audit-retry").SetKey("synthetic").SetProjectID(p.ID).SetType("user").SaveX(ctx)
	ctx = contexts.WithAPIKey(ctx, key)
	channels := newTestChannelServiceForChannels(db)
	system := newTestSystemService(db)
	usage := biz.NewUsageLogService(db, system, channels)
	service := &RealtimeService{Channels: channels, Usage: usage, Requests: newTestRequestServiceForChannels(db, system)}
	price := objects.ModelPrice{}
	for _, code := range objects.RealtimePriceItemCodes() {
		rate := decimal.NewFromInt(1)
		price.Items = append(price.Items, objects.ModelPriceItem{ItemCode: code, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &rate}})
	}
	state := rt.NewState()
	require.NoError(t, state.Connecting())
	require.NoError(t, state.ObserveServer("session.created"))
	require.NoError(t, state.ObserveServer("session.updated"))
	adapter, err := openairt.NewAdapter("gpt-realtime-2.1-mini")
	require.NoError(t, err)
	c := &RealtimeConversation{service: service, ctx: ctx, state: state, adapter: adapter, target: &RealtimeTarget{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}}, Model: "gpt-realtime-2.1-mini", ClientModel: "voice", APIFormat: llm.APIFormatOpenAIRealtimeWebSocket, APIKey: key, Price: &biz.RealtimePriceSnapshot{Price: price, ReferenceID: "audit-v1"}}, started: time.Now(), responses: map[string]*realtimeResponse{}}
	service.Channels.SetEnabledChannelsForTest([]*biz.Channel{c.target.Channel})
	c.lastCheck = time.Now()
	_, err = c.Client([]byte(`{"type":"response.create"}`))
	require.NoError(t, err)
	require.Equal(t, 1, db.Request.Query().CountX(ctx), "audit must exist before upstream creates the response")
	_, err = c.Server([]byte(`{"type":"response.created","response":{"id":"r1"}}`))
	require.NoError(t, err)
	var failed atomic.Bool
	db.Request.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			if failed.CompareAndSwap(false, true) {
				return nil, errors.New("synthetic audit outage")
			}
			return next.Mutate(ctx, m)
		})
	})
	_, err = c.Server([]byte(`{"type":"response.done","response":{"id":"r1","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2,"input_token_details":{"text_tokens":1,"audio_tokens":0},"output_token_details":{"text_tokens":1,"audio_tokens":0}}}}`))
	require.Error(t, err)
	require.Equal(t, 1, db.UsageLog.Query().CountX(ctx))
	require.False(t, c.responses["r1"].auditFinalized)
	c.Close()
	require.True(t, c.responses["r1"].auditFinalized)
	require.Equal(t, request.StatusCompleted, db.Request.GetX(ctx, c.responses["r1"].request.ID).Status)
	require.Equal(t, 1, db.UsageLog.Query().CountX(ctx))
	newConversation := func() *RealtimeConversation {
		state := rt.NewState()
		_ = state.Connecting()
		_ = state.ObserveServer("session.created")
		_ = state.ObserveServer("session.updated")
		adapter, _ := openairt.NewAdapter("gpt-realtime-2.1-mini")
		return &RealtimeConversation{service: service, ctx: ctx, target: c.target, state: state, adapter: adapter, responses: map[string]*realtimeResponse{}, lastCheck: time.Now()}
	}
	cleared := newConversation()
	_, err = cleared.Client([]byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`))
	require.NoError(t, err)
	require.NotNil(t, cleared.pendingInput)
	_, err = cleared.Client([]byte(`{"type":"input_audio_buffer.clear"}`))
	require.NoError(t, err)
	_, err = cleared.Server([]byte(`{"type":"input_audio_buffer.cleared"}`))
	require.NoError(t, err)
	require.Nil(t, cleared.pendingInput)
	cleared.Close()
	require.NoError(t, usage.EnsureRealtimeUsageReconciled(ctx, key.ID))
	interrupted := newConversation()
	_, err = interrupted.Client([]byte(`{"type":"response.create"}`))
	require.NoError(t, err)
	require.Len(t, interrupted.pendingResponses, 1)
	interrupted.Close()
	require.ErrorIs(t, usage.EnsureRealtimeUsageReconciled(ctx, key.ID), biz.ErrRealtimeUsageUnreconciled)
	require.Equal(t, 1, db.UsageLog.Query().CountX(ctx), "pending requests must never become fabricated zero-cost usage")

}
