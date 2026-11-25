package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesContinuationDispatchesToNativeOwnerWithZeroRetries(t *testing.T) {
	ctx, client := setupTest(t)
	const scope = "api_key:1:project:1"
	ctx = shared.WithResponsesAPI(shared.WithSessionScope(ctx, scope))
	ctx = contexts.WithProjectID(ctx, createTestProject(t, ctx, client).ID)

	first := createTestChannel(t, ctx, client)
	owner, err := client.Channel.Create().
		SetType(first.Type).
		SetName("Continuation owner").
		SetBaseURL("http://127.0.0.1:9989/v1").
		SetCredentials(first.Credentials).
		SetSupportedModels(first.SupportedModels).
		SetDefaultTestModel(first.DefaultTestModel).
		Save(ctx)
	require.NoError(t, err)
	var candidates []*ChannelModelsCandidate
	for i, channel := range []*ent.Channel{first, owner} {
		outbound, err := responses.NewOutboundTransformer(channel.BaseURL, "fixture")
		require.NoError(t, err)
		candidates = append(candidates, &ChannelModelsCandidate{
			Channel:   &biz.Channel{Channel: channel, Outbound: outbound},
			Models:    []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
			APIFormat: llm.APIFormatOpenAIResponse.String(),
			Priority:  i,
		})
	}
	executor := &mockExecutor{response: &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"id":"resp_followup","status":"completed","object":"response","model":"gpt-4","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}]}`),
	}}
	orch := newTestOrchestrator(t, &staticChannelSelector{candidates: candidates}, client, executor)
	t.Cleanup(orch.ChannelService.Stop)
	orch.Inbound = responses.NewInboundTransformer()
	orch.adaptiveLoadBalancer = NewLoadBalancer(nil, nil, NewWeightStrategy())
	orch.responsesSessions = newResponsesSessionStore()
	origin := shared.ResponsesOrigin{
		ChannelID: owner.ID,
		Format:    llm.APIFormatOpenAIResponse.String(),
		Model:     "gpt-4",
		Endpoint:  owner.BaseURL + "/responses",
		Auth: responses.ContinuationAuthIdentity(&httpclient.Request{Headers: http.Header{
			"Authorization": {"Bearer fixture"},
			"Content-Type":  {"application/json"},
		}}),
	}
	orch.responsesSessions.byResponse[responsesSessionKey{scope: scope, responseID: "resp_owner"}] = &responsesSessionRecord{
		updatedAt: time.Now(),
		origin:    origin,
	}
	require.NoError(t, orch.SystemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled: true, MaxChannelRetries: 0, LoadBalancerStrategy: biz.LoadBalancerStrategyAdaptive,
	}))
	req := &httpclient.Request{Method: http.MethodPost, Body: []byte(`{"model":"gpt-4","previous_response_id":"resp_owner","input":"next"}`)}
	result, err := orch.Process(ctx, req)
	require.NoError(t, err)
	require.Equal(t, owner.ID, result.Route.ChannelID)
	require.EqualValues(t, 1, executor.requestCalls.Load())
	require.Equal(t, origin.Endpoint, executor.lastRequest.URL)
	require.Equal(t, "resp_owner", gjson.GetBytes(executor.lastRequest.Body, "previous_response_id").String())

	origin.Auth = "changed"
	orch.responsesSessions.byResponse[responsesSessionKey{scope: scope, responseID: "resp_owner"}] = &responsesSessionRecord{
		updatedAt: time.Now(),
		origin:    origin,
	}
	_, err = orch.Process(ctx, req)
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
	require.EqualValues(t, 1, executor.requestCalls.Load())
}

func TestResponsesContinuationSelectsEligibleOwnerBeforeDispatch(t *testing.T) {
	// Given a persisted incomplete continuation owned by candidate B while A is ranked first
	ctx := shared.WithSessionScope(t.Context(), "api_key:1:project:1")
	store := newResponsesSessionStore()
	store.byResponse[responsesSessionKey{scope: "api_key:1:project:1", responseID: "delta"}] = &responsesSessionRecord{
		updatedAt: time.Now(),
		origin:    shared.ResponsesOrigin{ChannelID: 2, Format: "openai/responses", Model: "gpt-5"},
	}
	selector := &responsesContinuationSelector{
		wrapped: &staticChannelSelector{candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}}, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}, modelAPIFormats: []string{"openai/responses"}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2}}, Models: []biz.ChannelModelEntry{{ActualModel: "other"}, {ActualModel: "gpt-5"}}, modelAPIFormats: []string{"openai/chat_completions", "openai/responses"}},
		}},
		continuation: newResponsesContinuation(store, []byte(`{"previous_response_id":"delta","input":"next"}`)),
	}
	policy := &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 0}
	balanced := WithLoadBalancedSelector(selector, nil, policy)

	// When the continuation owner is selected before outbound dispatch
	candidates, err := balanced.Select(ctx, &llm.Request{Model: "gpt-5"})

	// Then only the matching model on B reaches the retry-budget stage.
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, 2, candidates[0].Channel.ID)
	require.Equal(t, []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}, candidates[0].Models)
	require.Equal(t, []string{"openai/responses"}, candidates[0].modelAPIFormats)

	// A project or API-key filter can make the owner ineligible.
	selector.wrapped = WithSelectedChannelsSelector(selector.wrapped, []int{1})
	_, err = balanced.Select(ctx, &llm.Request{Model: "gpt-5"})
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
}

func TestResponsesCompleteHistoryKeepsRankedCandidate(t *testing.T) {
	// Given a complete persisted continuation owned by candidate B while A is ranked first
	ctx := shared.WithSessionScope(t.Context(), "api_key:1:project:1")
	store := newResponsesSessionStore()
	store.byResponse[responsesSessionKey{scope: "api_key:1:project:1", responseID: "complete"}] = &responsesSessionRecord{
		updatedAt: time.Now(),
		complete:  true,
		input:     []json.RawMessage{json.RawMessage(`{"role":"user","content":"root"}`)},
		origin:    shared.ResponsesOrigin{ChannelID: 2, Format: "openai/responses", Model: "gpt-5"},
	}
	selector := &responsesContinuationSelector{
		wrapped: &staticChannelSelector{candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}}, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}, modelAPIFormats: []string{"openai/responses"}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2}}, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}, modelAPIFormats: []string{"openai/responses"}},
		}},
		continuation: newResponsesContinuation(store, []byte(`{"previous_response_id":"complete","input":"next"}`)),
	}

	// When the continuation owner is considered before outbound dispatch
	candidates, err := selector.Select(ctx, &llm.Request{Model: "gpt-5"})

	// Then full-history replay leaves the ranked candidate order unchanged
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, 1, candidates[0].Channel.ID)
}

func TestResponsesPersistedDeltaIsNotExternalUnknownID(t *testing.T) {
	// Given
	ctx := shared.WithSessionScope(t.Context(), "api_key:1:project:1")
	store := newResponsesSessionStore()
	store.known = func(context.Context, string) (bool, error) { return true, nil }
	p := &PersistentOutboundTransformer{state: &PersistenceState{ResponsesContinuation: newResponsesContinuation(store, []byte(`{"previous_response_id":"persisted_delta","input":"delta"}`))}}
	// When
	_, err := prepareResponsesAttemptBody(ctx, p, responsesAttemptTarget{origin: shared.ResponsesOrigin{ChannelID: 1, Format: "openai/responses", Model: "gpt-5"}, forwardsID: true})
	// Then
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
	require.Empty(t, store.byResponse)
}

func TestResponsesKnownMissingOwnerFailsWithoutRecoveryWrite(t *testing.T) {
	// Given a known Responses continuation ID whose owner record is unavailable
	ctx := shared.WithSessionScope(t.Context(), "api_key:1:project:1")
	store := newResponsesSessionStore()
	store.known = func(context.Context, string) (bool, error) { return true, nil }
	p := &PersistentOutboundTransformer{state: &PersistenceState{ResponsesContinuation: newResponsesContinuation(store, []byte(`{"previous_response_id":"missing","input":"delta"}`))}}

	// When the HTTP attempt is prepared
	_, err := prepareResponsesAttemptBody(ctx, p, responsesAttemptTarget{origin: shared.ResponsesOrigin{ChannelID: 1, Format: "openai/responses", Model: "gpt-5"}, forwardsID: true})

	// Then recovery is rejected without creating a local record
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
	require.Empty(t, store.byResponse)
}

func TestResponsesHTTPNativeIDStaysOnOrigin(t *testing.T) {
	// Given
	ctx := shared.WithSessionScope(t.Context(), "api_key:1:project:1")
	c := newResponsesContinuation(newResponsesSessionStore(), []byte(`{"previous_response_id":"external","input":"delta"}`))
	p := &PersistentOutboundTransformer{state: &PersistenceState{ResponsesContinuation: c}}
	origin := shared.ResponsesOrigin{ChannelID: 1, Format: "openai/responses", Model: "gpt-5"}
	body, err := prepareResponsesAttemptBody(ctx, p, responsesAttemptTarget{origin: origin, forwardsID: true})
	require.NoError(t, err)
	require.Equal(t, c.original, body)
	c.firstOrigin = &origin
	// When
	_, err = prepareResponsesAttemptBody(ctx, p, responsesAttemptTarget{origin: shared.ResponsesOrigin{ChannelID: 2, Format: "openai/responses", Model: "gpt-5"}, forwardsID: true})
	// Then
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
}

func TestResponsesNativeLaneHasNoAdapterSnapshot(t *testing.T) {
	// Given
	lane := shared.NewResponsesLane("native", shared.NewResponsesBudget(64<<20))
	t.Cleanup(lane.Close)
	ctx := shared.WithResponsesAttempt(shared.WithResponsesLane(shared.WithResponsesWebSocket(t.Context()), lane), &shared.ResponsesAttempt{})
	shared.GetResponsesAttempt(ctx).SetOrigin(shared.ResponsesOrigin{Format: "openai/responses-ws", Generation: 1})
	c := newResponsesContinuation(nil, []byte(`{"input":"root"}`))
	c.complete = true
	c.retain = true
	c.capture.body = []byte(`{"id":"native_id","status":"completed","output":[]}`)
	// When
	c.accept(ctx)
	// Then
	require.Equal(t, "native_id", lane.Record().ID)
	require.Nil(t, lane.Record().Snapshot)
}
