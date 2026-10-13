package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesAdapterBudgetsLargeHistory(t *testing.T) {
	// Given
	ctx := shared.WithSessionScope(context.Background(), "api_key:1:project:1")
	store := newResponsesSessionStore()
	body := []byte(`{"input":[{"type":"function_call_output","call_id":"call_canary","output":"` + strings.Repeat("x", 4<<20) + `"}]}`)
	// When
	store.record(ctx, body, []byte(`{"id":"large","status":"completed","output":[]}`))
	// Then
	record := store.lookup(ctx, "large")
	require.NotNil(t, record)
	require.Greater(t, record.size, 4<<20)
}

func TestResponsesSnapshotRejectsUnresolvedAncestor(t *testing.T) {
	// Given
	ctx := shared.WithSessionScope(context.Background(), "api_key:1:project:1")
	store := newResponsesSessionStore()
	store.record(ctx, []byte(`{"previous_response_id":"missing","input":"delta"}`), []byte(`{"id":"delta","status":"completed","output":[]}`))
	// When
	record := store.lookup(ctx, "delta")
	// Then
	require.NotNil(t, record)
	require.False(t, record.complete)
}

func TestResponsesCodexHTTPUnknownIDFailsBeforeDispatch(t *testing.T) {
	// Given
	ctx := shared.WithSessionScope(context.Background(), "api_key:1:project:1")
	state := &PersistenceState{ResponsesContinuation: newResponsesContinuation(newResponsesSessionStore(), []byte(`{"model":"gpt-5","previous_response_id":"unknown","input":"delta"}`))}
	p := &PersistentOutboundTransformer{state: state}
	// When
	_, err := prepareResponsesAttemptBody(ctx, p, responsesAttemptTarget{origin: shared.ResponsesOrigin{ChannelID: 1, Format: "openai/responses", Model: "gpt-5"}})
	// Then
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
	var wire *httpclient.Error
	require.ErrorAs(t, err, &wire)
	require.Equal(t, 400, wire.StatusCode)
	var parsed struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(wire.Body, &parsed))
	require.Equal(t, "previous_response_not_found", parsed.Error.Code)
}
