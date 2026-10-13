package orchestrator

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesContinuationSelectsEligibleOwnerBeforeDispatch(t *testing.T) {
	// Given a persisted incomplete continuation owned by candidate B while A is ranked first
	ctx := shared.WithSessionScope(t.Context(), "api_key:1:project:1")
	store := newResponsesSessionStore()
	store.byResponse[responsesSessionKey{scope: "api_key:1:project:1", responseID: "delta"}] = &responsesSessionRecord{
		updatedAt: time.Now(),
		origin:    shared.ResponsesOrigin{ChannelID: 2, Format: "openai/responses", Model: "gpt-5"},
	}
	p := &PersistentOutboundTransformer{state: &PersistenceState{
		ChannelModelsCandidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}}, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}, modelAPIFormats: []string{"openai/responses"}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2}}, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}, modelAPIFormats: []string{"openai/responses"}},
		},
		ResponsesContinuation: newResponsesContinuation(store, []byte(`{"previous_response_id":"delta","input":"next"}`)),
	}}

	// When the continuation owner is selected before outbound dispatch
	err := selectResponsesContinuationCandidate(ctx, p, &llm.Request{Model: "gpt-5"})

	// Then candidate B is selected without reordering the ranked list
	require.NoError(t, err)
	require.Equal(t, 1, p.state.CurrentCandidateIndex)
	require.Equal(t, 2, p.state.CurrentCandidate.Channel.ID)
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
	p := &PersistentOutboundTransformer{state: &PersistenceState{
		ChannelModelsCandidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}}, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}, modelAPIFormats: []string{"openai/responses"}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2}}, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}, modelAPIFormats: []string{"openai/responses"}},
		},
		ResponsesContinuation: newResponsesContinuation(store, []byte(`{"previous_response_id":"complete","input":"next"}`)),
	}}

	// When the continuation owner is considered before outbound dispatch
	err := selectResponsesContinuationCandidate(ctx, p, &llm.Request{Model: "gpt-5"})

	// Then full-history replay leaves the ranked candidate order unchanged
	require.NoError(t, err)
	require.Equal(t, 0, p.state.CurrentCandidateIndex)
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
