package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestStreamTerminalTracker_RequiresEveryObservedChoice(t *testing.T) {
	tracker := NewStreamTerminalTracker(0)
	require.False(t, tracker.Observe(&httpclient.StreamEvent{Data: []byte(`{"choices":[{"index":0,"finish_reason":"stop"},{"index":1,"finish_reason":null}]}`)}))
	require.True(t, tracker.Observe(&httpclient.StreamEvent{Data: []byte(`{"choices":[{"index":1,"finish_reason":"stop"}]}`)}))
}
