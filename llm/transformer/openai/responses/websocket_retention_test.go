package responses

import (
	"encoding/json"
	"github.com/looplj/axonhub/llm/transformer/shared"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestResponsesWebSocketRetentionBoundaries(t *testing.T) {
	for _, size := range []int{(1 << 20) - 1, (1 << 20) + 1, (16 << 20) - 1, (16 << 20) + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			// Given
			pc := &pooledWebSocketConn{budget: shared.NewResponsesBudget(64 << 20)}
			input := []json.RawMessage{make([]byte, size)}
			// When
			retained := pc.rememberTurn(input, "provider_id", defaultWebSocketMaxRetainedIn)
			// Then
			require.Equal(t, size <= 16<<20, retained)
			_, id := pc.previousTurn()
			require.Equal(t, "provider_id", id)
		})
	}
}

func TestResponsesWebSocketPoolPayloadPressurePreservesMetadata(t *testing.T) {
	// Given
	budget := shared.NewResponsesBudget(64 << 20)
	input := []json.RawMessage{make([]byte, 16<<20)}
	connections := make([]*pooledWebSocketConn, 5)
	for i := range connections {
		connections[i] = &pooledWebSocketConn{budget: budget}
	}
	for _, pc := range connections[:4] {
		require.True(t, pc.rememberTurn(input, "kept", 16<<20))
	}
	// When
	retained := connections[4].rememberTurn(input, "still_native", 16<<20)
	// Then
	require.False(t, retained)
	_, id := connections[4].previousTurn()
	require.Equal(t, "still_native", id)
	for _, pc := range connections {
		require.True(t, pc.rememberTurn(nil, "released", 16<<20))
	}
	require.True(t, budget.Replace(0, 64<<20))
}
