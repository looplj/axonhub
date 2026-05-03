package responses

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactedStreamEventLogSummary_DoesNotExposeDeltaOrErrorMessage(t *testing.T) {
	event := StreamEvent{
		Type:    StreamEventTypeOutputTextDelta,
		Delta:   "secret delta",
		Message: "secret error detail",
	}

	summary := redactedStreamEventLogSummary(event)
	rendered := fmt.Sprintf("%+v", summary)

	require.Equal(t, len(event.Delta), summary.DeltaBytes)
	require.True(t, summary.HasError)
	require.NotContains(t, rendered, "secret delta")
	require.NotContains(t, rendered, "secret error detail")
}
