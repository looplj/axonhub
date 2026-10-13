package orchestrator

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesCaptureCommitsActualProviderIdentity(t *testing.T) {
	// Given
	lane := shared.NewResponsesLane("lane", shared.NewResponsesBudget(64<<20))
	t.Cleanup(lane.Close)
	ctx := shared.WithResponsesLane(t.Context(), lane)
	continuation := newResponsesContinuation(nil, []byte(`{"input":"root"}`))
	continuation.complete = true
	continuation.retain = true
	continuation.capture.observe(&httpclient.StreamEvent{Data: []byte(`{"type":"response.completed","response":{"id":"actual_provider_id","status":"completed","output":[{"id":"actual_item_id","type":"message"}]}}`)})
	// When
	continuation.accept(ctx)
	// Then
	record := lane.Record()
	require.Equal(t, "actual_provider_id", record.ID)
	require.NotNil(t, record.Snapshot)
	require.JSONEq(t, `{"id":"actual_item_id","type":"message"}`, string(record.Snapshot.Output[0]))
}

func TestResponsesCapture16MiBBoundary(t *testing.T) {
	for _, size := range []int{(16 << 20) - 1, (16 << 20) + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			// Given
			terminal := []byte(`{"type":"response.completed","response":{"id":"provider_id","status":"completed","output":[]}}`)
			capture := &responsesCompletion{}
			deltaPrefix := `{"type":"response.output_text.delta","delta":"`
			delta := []byte(deltaPrefix + strings.Repeat("x", size-len(terminal)-len(deltaPrefix)-2) + `"}`)
			capture.observe(&httpclient.StreamEvent{Data: delta})
			// When
			capture.observe(&httpclient.StreamEvent{Data: terminal})
			// Then
			require.Equal(t, size, capture.bytes)
			require.Equal(t, size > 16<<20, capture.overflow)
		})
	}
}

func TestResponsesCaptureOverflowNeverCertifiesPrefix(t *testing.T) {
	// Given
	lane := shared.NewResponsesLane("lane", shared.NewResponsesBudget(64<<20))
	t.Cleanup(lane.Close)
	ctx := shared.WithResponsesLane(t.Context(), lane)
	c := newResponsesContinuation(nil, []byte(`{"input":"root"}`))
	c.complete = true
	c.capture.bytes = 16 << 20
	// When
	c.capture.observe(&httpclient.StreamEvent{Data: []byte(`{"type":"response.completed","response":{"id":"provider_id","status":"completed","output":[]}}`)})
	c.accept(ctx)
	// Then
	require.Equal(t, "provider_id", lane.Record().ID)
	require.Nil(t, lane.Record().Snapshot)
}

func TestResponsesCaptureRetainsFullSnapshotAtAggregateLimit(t *testing.T) {
	// Given
	body := []byte(`{"id":"provider_id","status":"completed","output":[{"type":"message"}]}`)
	input := []json.RawMessage{json.RawMessage(`{"type":"message","content":"input"}`)}
	inputBytes := len(input[0])
	budget := shared.NewResponsesBudget(inputBytes + len(body))
	lane := shared.NewResponsesLane("lane", budget)
	t.Cleanup(lane.Close)
	continuation := newResponsesContinuation(nil, []byte(`{"input":[]}`))
	continuation.complete = true
	continuation.retain = true
	continuation.input = input
	continuation.retentionBytes = inputBytes
	continuation.lane = lane
	continuation.capture.body = body
	continuation.capture.budget = budget
	continuation.capture.retained = len(body)
	require.True(t, budget.Replace(0, inputBytes+len(body)))
	ctx := shared.WithResponsesLane(t.Context(), lane)

	// When
	continuation.accept(ctx)

	// Then
	record := lane.Record()
	require.NotNil(t, record.Snapshot)
	require.Len(t, record.Snapshot.Input, 1)
	require.Len(t, record.Snapshot.Output, 1)
}

func TestResponsesCaptureSharedBudgetRetainsMultipleLanes(t *testing.T) {
	// Given
	body := []byte(`{"id":"provider_id","status":"completed","output":[{"type":"message"}]}`)
	input := []json.RawMessage{json.RawMessage(`{"type":"message","content":"input"}`)}
	inputBytes := len(input[0])
	reservation := inputBytes + len(body)
	budget := shared.NewResponsesBudget(2 * reservation)
	lanes := []*shared.ResponsesLane{
		shared.NewResponsesLane("lane-1", budget),
		shared.NewResponsesLane("lane-2", budget),
	}
	for _, lane := range lanes {
		t.Cleanup(lane.Close)
	}

	for _, lane := range lanes {
		continuation := newResponsesContinuation(nil, []byte(`{"input":[]}`))
		continuation.complete = true
		continuation.retain = true
		continuation.input = input
		continuation.retentionBytes = inputBytes
		continuation.lane = lane
		continuation.capture.body = body
		continuation.capture.budget = budget
		continuation.capture.retained = len(body)
		require.True(t, budget.Replace(0, reservation))

		// When
		continuation.accept(shared.WithResponsesLane(t.Context(), lane))

		// Then
		require.NotNil(t, lane.Record().Snapshot)
	}
}

func TestResponsesCaptureErrorCleanupReleasesReservationsOnce(t *testing.T) {
	// Given
	body := []byte(`{"id":"provider_id","status":"completed"}`)
	inputBytes := 32
	budget := shared.NewResponsesBudget(inputBytes + len(body))
	lane := shared.NewResponsesLane("lane", budget)
	t.Cleanup(lane.Close)
	continuation := newResponsesContinuation(nil, []byte(`{"input":[]}`))
	continuation.retentionBytes = inputBytes
	continuation.lane = lane
	continuation.capture.body = []byte(`not-json`)
	continuation.capture.budget = budget
	continuation.capture.retained = len(continuation.capture.body)
	require.True(t, budget.Replace(0, inputBytes+continuation.capture.retained))

	// When
	continuation.accept(shared.WithResponsesLane(t.Context(), lane))
	continuation.releaseRetention()
	continuation.releaseCapture()
	continuation.releaseRetention()
	continuation.releaseCapture()

	// Then
	require.True(t, budget.Replace(0, inputBytes+len(continuation.capture.body)))
	require.True(t, budget.Replace(inputBytes+len(continuation.capture.body), 0))
}

func TestResponsesCaptureCloseReleasesReservationsOnce(t *testing.T) {
	// Given
	body := []byte(`{"id":"provider_id","status":"completed"}`)
	inputBytes := 32
	budget := shared.NewResponsesBudget(inputBytes + len(body))
	lane := shared.NewResponsesLane("lane", budget)
	t.Cleanup(lane.Close)
	continuation := newResponsesContinuation(nil, []byte(`{"input":[]}`))
	continuation.retentionBytes = inputBytes
	continuation.lane = lane
	continuation.capture.budget = budget
	continuation.capture.retained = len(body)
	require.True(t, budget.Replace(0, inputBytes+len(body)))
	stream := &responsesAcceptedStream{Stream: &consumingResponsesStream{}, continuation: continuation}

	// When
	require.NoError(t, stream.Close())
	require.NoError(t, stream.Close())

	// Then
	require.True(t, budget.Replace(0, inputBytes+len(body)))
	require.True(t, budget.Replace(inputBytes+len(body), 0))
}
