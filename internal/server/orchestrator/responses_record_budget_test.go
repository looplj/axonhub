package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesAdapterBudgetsHTTPRecord16MiB(t *testing.T) {
	for _, size := range []int{(1 << 20) - 1, (1 << 20) + 1, (16 << 20) - 1, (16 << 20) + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			// Given
			ctx := shared.WithSessionScope(context.Background(), "api_key:1:project:1")
			store := newResponsesSessionStore()
			item := `{"role":"user","content":"` + strings.Repeat("x", size-len(`{"role":"user","content":""}`)) + `"}`
			require.Len(t, item, size)
			// When
			store.record(ctx, []byte(`{"input":[`+item+`]}`), []byte(`{"id":"provider_id","status":"completed","output":[]}`))
			// Then
			record := store.lookup(ctx, "provider_id")
			require.NotNil(t, record)
			require.Equal(t, size <= 16<<20, record.complete)
		})
	}
}

func TestResponsesAdapterBudgetsHTTPPressurePreservesOrigin(t *testing.T) {
	// Given
	ctx := shared.WithSessionScope(t.Context(), "api_key:1:project:1")
	store := newResponsesSessionStore()
	input := []byte(`{"input":[{"role":"user","content":"` + strings.Repeat("x", (16<<20)-100) + `"}]}`)
	for i := range 4 {
		store.record(ctx, input, fmt.Appendf(nil, `{"id":"response_%d","status":"completed","output":[]}`, i))
	}
	// When
	store.record(ctx, input, []byte(`{"id":"response_4","status":"completed","output":[]}`))
	// Then
	record := store.lookup(ctx, "response_0")
	require.NotNil(t, record)
	require.False(t, record.complete)
	require.LessOrEqual(t, store.totalBytes, 64<<20)
}

func TestResponsesWarmupReservationReleasedBeforeTranscriptRetention(t *testing.T) {
	budget := shared.NewResponsesBudget(256)
	lane := shared.NewResponsesLane("lane", budget)
	ctx := shared.WithResponsesLane(t.Context(), lane)
	require.True(t, budget.Replace(0, 200)) // Original warmup and merged body each hold 100 bytes.
	accepted := false
	ctx = WithResponsesWarmupAccepted(ctx, func() { accepted = true; require.True(t, budget.Replace(100, 0)) })
	c := newResponsesContinuation(nil, []byte(`{"input":"`+strings.Repeat("x", 80)+`"}`))
	p := &PersistentOutboundTransformer{state: &PersistenceState{ResponsesContinuation: c}}
	_, err := prepareResponsesAttemptBody(ctx, p, responsesAttemptTarget{})
	require.NoError(t, err)
	require.False(t, accepted, "a prepared attempt can still be rejected by the channel queue")
	require.Zero(t, c.retentionBytes)
	_, err = captureResponsesProvider(p).OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.NoError(t, err)
	require.True(t, accepted)
	c.capture.body = []byte(`{"id":"resp","status":"completed","output":[]}`)
	c.accept(ctx)
	require.NotNil(t, lane.Record().Snapshot)
}
