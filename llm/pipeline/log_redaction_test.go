package pipeline

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRedactedResponseLogSummary_DoesNotExposeTransformerMetadataValues(t *testing.T) {
	resp := &llm.Response{
		ID:    "resp_123",
		Model: "gpt-5.4",
		TransformerMetadata: map[string]any{
			"raw_tool_output": "secret tool output",
		},
	}

	summary := redactedResponseLogSummary(resp)
	rendered := fmt.Sprintf("%+v", summary)

	require.Equal(t, 1, summary.TransformerMetadataKey)
	require.NotContains(t, rendered, "secret tool output")
	require.NotContains(t, rendered, "raw_tool_output")
}

func TestRedactedStreamEventLogSummary_DoesNotExposeEventData(t *testing.T) {
	event := &httpclient.StreamEvent{
		Type: "response.output_text.delta",
		Data: []byte(`{"delta":"secret stream data"}`),
	}

	summary := redactedStreamEventLogSummary(event)
	rendered := fmt.Sprintf("%+v", summary)

	require.Equal(t, len(event.Data), summary.DataBytes)
	require.NotContains(t, rendered, "secret stream data")
}
