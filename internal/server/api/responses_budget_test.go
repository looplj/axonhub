package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesAdapterBudgetsQueue128MiB(t *testing.T) {
	// Given
	d := newResponsesWebSocketDispatcher(context.Background(), func() {}, nil, nil, nil, 0, nil, nil)
	// When
	accepted := d.reserve(128 << 20)
	// Then
	require.True(t, accepted)
	require.False(t, d.reserve(1))
	d.queueBudget.Replace(128<<20, 0)
	<-d.pending
	require.True(t, d.reserve(128<<20))
	require.Equal(t, 128<<20, responsesWebSocketMaxMessageSize)
}

func TestResponsesAdapterBudgetsWarmupSharesConnection64MiB(t *testing.T) {
	// Given
	budget := shared.NewResponsesBudget(64 << 20)
	require.True(t, budget.Replace(0, (64<<20)-64))
	session := &responsesWebSocketSession{budget: budget}
	frame := []byte(`{"type":"response.create","generate":false,"model":"gpt-5","input":"this warmup exceeds the remaining retained byte budget of the connection"}`)
	// When
	_, _, err := session.prepareRequest(httptest.NewRequest(http.MethodGet, "/responses", nil), frame)
	// Then
	require.NotNil(t, err)
	require.Empty(t, session.warmupID)
}

func TestResponsesAdapterBudgetsTranscriptOverflowDropsEligibility(t *testing.T) {
	// Given
	budget := shared.NewResponsesBudget(64 << 20)
	lane := shared.NewResponsesLane("lane", budget)
	budget.Replace(0, (64<<20)-2)
	// When
	lane.Remember(shared.ResponsesRecord{ID: "successful_id", Snapshot: &shared.ResponsesSnapshot{Input: []json.RawMessage{json.RawMessage(`{"canary":true}`)}}})
	// Then
	require.Equal(t, "successful_id", lane.Record().ID)
	require.Nil(t, lane.Record().Snapshot)
	lane.Close()
}
