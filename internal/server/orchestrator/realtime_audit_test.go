// 职责: 验证已计费响应审计失败后能够收尾且不会重复记账。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//	internal/server/biz/realtime_billing.go

package orchestrator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
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
	automatic := newConversation()
	_, err = automatic.Client([]byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`))
	require.NoError(t, err)
	firstAudio := automatic.pendingInput
	require.NotNil(t, firstAudio)
	_, err = automatic.Server([]byte(`{"type":"input_audio_buffer.committed"}`))
	require.NoError(t, err)
	require.Nil(t, automatic.pendingInput)
	require.Equal(t, []*realtimeResponse{firstAudio}, automatic.committedInputs)
	_, err = automatic.Client([]byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`))
	require.NoError(t, err)
	secondAudio := automatic.pendingInput
	require.NotNil(t, secondAudio)
	require.NotSame(t, firstAudio, secondAudio)
	_, err = automatic.Server([]byte(`{"type":"response.created","response":{"id":"automatic-audio"}}`))
	require.NoError(t, err)
	require.Same(t, firstAudio, automatic.responses["automatic-audio"])
	_, err = automatic.Server([]byte(`{"type":"response.done","response":{"id":"automatic-audio","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2,"input_token_details":{"text_tokens":1,"audio_tokens":0},"output_token_details":{"text_tokens":1,"audio_tokens":0}}}}`))
	require.NoError(t, err)
	automatic.Close()
	require.Equal(t, request.StatusCompleted, db.Request.GetX(ctx, firstAudio.request.ID).Status)
	require.Equal(t, request.StatusFailed, db.Request.GetX(ctx, secondAudio.request.ID).Status)

	// A's acknowledgement may arrive after B has been committed.
	ordered := newConversation()
	_, err = ordered.Client([]byte(`{"type":"response.create","event_id":"explicit-a"}`))
	require.NoError(t, err)
	explicit := ordered.pendingResponses[0]
	_, err = ordered.Client([]byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`))
	require.NoError(t, err)
	audio := ordered.pendingInput
	require.NotNil(t, audio)
	_, err = ordered.Client([]byte(`{"type":"input_audio_buffer.commit"}`))
	require.NoError(t, err)
	_, err = ordered.Server([]byte(`{"type":"response.created","response":{"id":"explicit-a"}}`))
	require.NoError(t, err)
	require.Same(t, explicit, ordered.responses["explicit-a"])
	require.Equal(t, []*realtimeResponse{audio}, ordered.committedInputs)
	ordered.Close()

	cleared := newConversation()
	_, err = cleared.Client([]byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`))
	require.NoError(t, err)
	require.NotNil(t, cleared.pendingInput)
	beforeClear := cleared.pendingInput
	_, err = cleared.Client([]byte(`{"type":"input_audio_buffer.clear"}`))
	require.NoError(t, err)
	_, err = cleared.Client([]byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`))
	require.NoError(t, err)
	afterClear := cleared.pendingInput
	require.NotSame(t, beforeClear, afterClear)
	_, err = cleared.Server([]byte(`{"type":"input_audio_buffer.cleared"}`))
	require.NoError(t, err)
	require.Same(t, afterClear, cleared.pendingInput)
	require.Equal(t, request.StatusCanceled, db.Request.GetX(ctx, beforeClear.request.ID).Status)
	cleared.Close()
	require.Equal(t, request.StatusFailed, db.Request.GetX(ctx, afterClear.request.ID).Status)
	require.NoError(t, usage.EnsureRealtimeUsageReconciled(ctx, key.ID))
	pipelined := newConversation()
	_, err = pipelined.Client([]byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`))
	require.NoError(t, err)
	firstBuffer := pipelined.pendingInput
	_, err = pipelined.Client([]byte(`{"type":"input_audio_buffer.commit"}`))
	require.NoError(t, err)
	_, err = pipelined.Client([]byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`))
	require.NoError(t, err)
	secondBuffer := pipelined.pendingInput
	require.NotSame(t, firstBuffer, secondBuffer)
	_, err = pipelined.Server([]byte(`{"type":"input_audio_buffer.committed"}`))
	require.NoError(t, err)
	require.Positive(t, pipelined.audioBytes, "the acknowledgement must not reset post-commit audio")
	_, err = pipelined.Client([]byte(`{"type":"response.create"}`))
	require.NoError(t, err)
	_, err = pipelined.Server([]byte(`{"type":"response.created","response":{"id":"manual-pipeline"}}`))
	require.NoError(t, err)
	require.Same(t, firstBuffer, pipelined.responses["manual-pipeline"])
	require.Same(t, secondBuffer, pipelined.pendingInput)
	_, err = pipelined.Server([]byte(`{"type":"response.done","response":{"id":"manual-pipeline","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2,"input_token_details":{"text_tokens":1},"output_token_details":{"text_tokens":1}}}}`))
	require.NoError(t, err)
	require.Equal(t, request.StatusCompleted, db.Request.GetX(ctx, firstBuffer.request.ID).Status)
	pipelined.Close()
	require.Equal(t, request.StatusFailed, db.Request.GetX(ctx, secondBuffer.request.ID).Status)

	interrupted := newConversation()
	_, err = interrupted.Client([]byte(`{"type":"response.create"}`))
	require.NoError(t, err)
	require.Len(t, interrupted.pendingResponses, 1)
	interrupted.Close()
	require.NoError(t, usage.EnsureRealtimeUsageReconciled(ctx, key.ID), "an interrupted response must not permanently lock the key")
	require.Equal(t, 3, db.UsageLog.Query().CountX(ctx), "pending requests must never become fabricated zero-cost usage")
}

func TestAutomaticRealtimeTurnChecksRequestQuota(t *testing.T) {
	ctx, db := setupTest(t)
	project := db.Project.Create().SetName("automatic-realtime-quota").SaveX(ctx)
	ctx = contexts.WithProjectID(ctx, project.ID)
	profiles := &objects.APIKeyProfiles{ActiveProfile: "limited", Profiles: []objects.APIKeyProfile{{
		Name: "limited",
		Quota: &objects.APIKeyQuota{
			Requests: lo.ToPtr(int64(1)),
			Period:   objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeAllTime},
		},
	}}}
	key := db.APIKey.Create().SetName("automatic-realtime-quota").SetKey("synthetic-quota").SetProjectID(project.ID).SetType("user").SetProfiles(profiles).SaveX(ctx)
	ctx = contexts.WithAPIKey(ctx, key)

	channelRow := db.Channel.Create().SetType("openai").SetName("automatic-realtime-quota").SetBaseURL("https://api.openai.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).SetSupportedModels([]string{"voice"}).SetDefaultTestModel("voice").SetStatus("enabled").SaveX(ctx)
	channels := newTestChannelServiceForChannels(db)
	channelService, err := channels.GetChannel(ctx, channelRow.ID)
	require.NoError(t, err)
	channels.SetEnabledChannelsForTest([]*biz.Channel{channelService})
	system := newTestSystemService(db)
	service := &RealtimeService{
		Channels: channels,
		Quota:    biz.NewQuotaService(db, system),
		Usage:    biz.NewUsageLogService(db, system, channels),
		Requests: newTestRequestServiceForChannels(db, system),
	}
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
	c := &RealtimeConversation{
		service: service, ctx: ctx, state: state, adapter: adapter,
		target: &RealtimeTarget{
			Channel: &biz.Channel{Channel: channelRow}, Model: "gpt-realtime-2.1-mini", ClientModel: "voice",
			APIFormat: llm.APIFormatOpenAIRealtimeWebSocket, APIKey: key,
			Price: &biz.RealtimePriceSnapshot{Price: price, ReferenceID: "automatic-quota"},
		},
		started: time.Now(), responses: map[string]*realtimeResponse{}, lastCheck: time.Now(),
	}
	_, err = c.Client([]byte(`{"type":"response.create"}`))
	require.NoError(t, err)
	_, err = c.Server([]byte(`{"type":"response.created","response":{"id":"r1"}}`))
	require.NoError(t, err)
	_, err = c.Server([]byte(`{"type":"response.done","response":{"id":"r1","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2,"input_token_details":{"text_tokens":1,"audio_tokens":0},"output_token_details":{"text_tokens":1,"audio_tokens":0}}}}`))
	require.NoError(t, err)
	require.Equal(t, 1, db.UsageLog.Query().CountX(ctx), "the first response should consume the only allowed request")

	_, err = c.Server([]byte(`{"type":"response.created","response":{"id":"r2"}}`))
	require.ErrorIs(t, err, rt.ErrQuotaExceeded, "an automatic turn without a client reservation must check request quota")
}

func TestQueuedResponseCreatedDuringSessionUpdate(t *testing.T) {
	unrequestedState := rt.NewState()
	require.NoError(t, unrequestedState.Connecting())
	require.NoError(t, unrequestedState.ObserveServer("session.created"))
	require.NoError(t, unrequestedState.ObserveServer("session.updated"))
	require.NoError(t, unrequestedState.Update())
	unrequestedAdapter, err := openairt.NewAdapter("gpt-realtime-2.1-mini")
	require.NoError(t, err)
	unrequested := &RealtimeConversation{state: unrequestedState, adapter: unrequestedAdapter, target: &RealtimeTarget{APIFormat: llm.APIFormatOpenAIRealtimeWebSocket}, responses: map[string]*realtimeResponse{}}
	_, err = unrequested.Server([]byte(`{"type":"response.created","response":{"id":"not-queued"}}`))
	require.Error(t, err, "an unrequested response must remain blocked during session.update")

	for name, clearing := range map[string]bool{"explicit create": false, "audio awaiting clear": true} {
		t.Run(name, func(t *testing.T) {
			state := rt.NewState()
			require.NoError(t, state.Connecting())
			require.NoError(t, state.ObserveServer("session.created"))
			require.NoError(t, state.ObserveServer("session.updated"))
			require.NoError(t, state.Update())
			adapter, err := openairt.NewAdapter("gpt-realtime-2.1-mini")
			require.NoError(t, err)
			pending := &realtimeResponse{}
			c := &RealtimeConversation{
				state: state, adapter: adapter,
				target:    &RealtimeTarget{APIFormat: llm.APIFormatOpenAIRealtimeWebSocket},
				responses: map[string]*realtimeResponse{},
			}

			if clearing {
				c.clearingInput = pending
				c.pendingClearID = "clear-before-update"
			} else {
				c.pendingResponses = []*realtimeResponse{pending}
			}

			_, err = c.Server([]byte(`{"type":"response.created","response":{"id":"queued-before-update"}}`))
			require.NoError(t, err, "the server must accept an acknowledgement for a response queued before session.update")
			require.Same(t, pending, c.responses["queued-before-update"])
			if clearing {
				require.Nil(t, c.clearingInput, "the response must consume the pre-clear reservation")
				_, err = c.Server([]byte(`{"type":"input_audio_buffer.cleared"}`))
				require.NoError(t, err)
			}
			_, err = c.Server([]byte(`{"type":"session.updated","session":{"type":"realtime"}}`))
			require.NoError(t, err)
			require.True(t, c.state.Ready())
		})
	}
}

func TestUnrelatedErrorPreservesPendingConfiguration(t *testing.T) {
	state := rt.NewState()
	require.NoError(t, state.Connecting())
	require.NoError(t, state.ObserveServer("session.created"))
	require.NoError(t, state.ObserveServer("session.updated"))
	require.NoError(t, state.Update())
	adapter, err := openairt.NewAdapter("gpt-realtime-2.1")
	require.NoError(t, err)
	c := &RealtimeConversation{state: state, adapter: adapter, pendingConfigurationID: "pending-update", target: &RealtimeTarget{APIFormat: llm.APIFormatOpenAIRealtimeWebSocket}, responses: map[string]*realtimeResponse{}}
	_, err = c.Server([]byte(`{"type":"error","error":{"event_id":"earlier-response","message":"rejected"}}`))
	require.NoError(t, err)
	_, err = c.Server([]byte(`{"type":"session.updated","session":{"type":"realtime"}}`))
	require.NoError(t, err)
	require.True(t, state.Ready())
	require.NoError(t, state.Update())
	c.pendingConfigurationID = "rejected-update"
	_, err = c.Server([]byte(`{"type":"error","error":{"event_id":"rejected-update","message":"invalid setting"}}`))
	require.NoError(t, err)
	require.True(t, state.Ready(), "a correlated configuration rejection restores the previous configuration")
	require.NoError(t, state.Update(), "a rejected update must not block the next update")
}

func TestRealtimeClosePreservesObservedUsageWithoutBodyStorage(t *testing.T) {
	ctx, db := setupTest(t)
	project := db.Project.Create().SetName("audit-no-body").SaveX(ctx)
	ctx = contexts.WithProjectID(ctx, project.ID)
	key := db.APIKey.Create().SetName("audit-no-body").SetKey("synthetic").SetProjectID(project.ID).SetType("user").SaveX(ctx)
	ctx = contexts.WithAPIKey(ctx, key)
	system := newTestSystemService(db)
	policy, err := system.StoragePolicy(ctx)
	require.NoError(t, err)
	policy.StoreResponseBody = false
	require.NoError(t, system.SetStoragePolicy(ctx, policy))
	service := &RealtimeService{Requests: newTestRequestServiceForChannels(db, system)}
	req, err := service.Requests.CreateRealtimeRequest(ctx, "voice", 1, "session", objects.JSONRawMessage(`{}`))
	require.NoError(t, err)
	usage := &llm.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	c := &RealtimeConversation{ctx: ctx, state: rt.NewState(), service: service, target: &RealtimeTarget{Model: "qwen3.5-omni-flash-realtime", Price: &biz.RealtimePriceSnapshot{ReferenceID: "frozen"}}, responses: map[string]*realtimeResponse{"r1": {request: req, observedUsage: usage, started: time.Now()}}}
	c.Close()
	stored := db.Request.GetX(ctx, req.ID)
	require.Contains(t, string(stored.ResponseBody), `"observed_usage"`)
	require.Contains(t, string(stored.ResponseBody), `"total_tokens":30`)
	require.Contains(t, string(stored.ResponseBody), `"price_reference_id":"frozen"`)
	require.Equal(t, request.StatusFailed, stored.Status)
	require.Zero(t, db.UsageLog.Query().CountX(ctx))
}

func TestRealtimeResponseReservationsKeepAudioClearOrder(t *testing.T) {
	before, clearing, after := &realtimeResponse{createEventID: "before"}, &realtimeResponse{}, &realtimeResponse{createEventID: "after"}
	c := &RealtimeConversation{
		pendingClearID: "clear", preClearCreates: 1, clearingInput: clearing,
		pendingResponses: []*realtimeResponse{before, after},
		responses:        map[string]*realtimeResponse{},
	}
	require.NoError(t, c.admitResponse("explicit-before"))
	require.Same(t, before, c.responses["explicit-before"])
	require.NoError(t, c.admitResponse("vad-before"))
	require.Same(t, clearing, c.responses["vad-before"])
	require.NoError(t, c.confirmAudioClear())
	require.NoError(t, c.admitResponse("explicit-after"))
	require.Same(t, after, c.responses["explicit-after"])
}

func TestServerVadCommitKeepsPreClearTurnAheadOfPostClearCreate(t *testing.T) {
	ctx, db := setupTest(t)
	project := db.Project.Create().SetName("vad-clear-order").SaveX(ctx)
	ctx = contexts.WithProjectID(ctx, project.ID)
	key := db.APIKey.Create().SetName("vad-clear-order").SetKey("vad-clear-order").SetProjectID(project.ID).SetType("user").SaveX(ctx)
	ctx = contexts.WithAPIKey(ctx, key)
	system := newTestSystemService(db)
	requests := newTestRequestServiceForChannels(db, system)
	requestRow, err := requests.CreateRealtimeRequest(ctx, "voice", 1, "vad-clear-order", objects.JSONRawMessage(`{}`))
	require.NoError(t, err)
	state := rt.NewState()
	require.NoError(t, state.Connecting())
	require.NoError(t, state.ObserveServer("session.created"))
	require.NoError(t, state.ObserveServer("session.updated"))
	adapter, err := openairt.NewAdapter("gpt-realtime-2.1-mini")
	require.NoError(t, err)
	beforeClear := &realtimeResponse{request: requestRow, reservationOrder: 1}
	postClearCreate := &realtimeResponse{createEventID: "post-clear-create", reservationOrder: 2}
	c := &RealtimeConversation{
		ctx: ctx, service: &RealtimeService{Requests: requests}, state: state, adapter: adapter, target: &RealtimeTarget{APIFormat: llm.APIFormatOpenAIRealtimeWebSocket},
		pendingClearID: "clear-event", clearingInput: beforeClear,
		pendingResponses: []*realtimeResponse{postClearCreate}, responses: map[string]*realtimeResponse{},
	}

	_, err = c.Server([]byte(`{"type":"input_audio_buffer.committed"}`))
	require.NoError(t, err)
	_, err = c.Server([]byte(`{"type":"input_audio_buffer.cleared"}`))
	require.NoError(t, err)
	_, err = c.Server([]byte(`{"type":"response.created","response":{"id":"pre-clear-vad"}}`))
	require.NoError(t, err)
	require.Same(t, beforeClear, c.responses["pre-clear-vad"])
	_, err = c.Server([]byte(`{"type":"response.created","response":{"id":"post-clear-manual"}}`))
	require.NoError(t, err)
	require.Same(t, postClearCreate, c.responses["post-clear-manual"])
}
