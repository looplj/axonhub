package responses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestOutboundTransformer_TransformStream_accepts_string_status(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://example.test", "test-key")
	require.NoError(t, err)
	events := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","sequence_number":0,"status":"in_progress","response":{"id":"resp_1","model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","sequence_number":1,"status":"completed","response":{"id":"resp_1","model":"gpt-5","status":"completed","output":[]}}`)},
	}

	stream, err := transformer.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)
	responses, err := streams.All(stream)

	require.NoError(t, err)
	require.NotEmpty(t, responses)
	require.Equal(t, llm.DoneResponse, responses[len(responses)-1])
}

func TestStreamEvent_Unmarshal_preserves_numeric_status(t *testing.T) {
	data := []byte(`{"type":"error","status":429}`)

	var event StreamEvent
	err := json.Unmarshal(data, &event)

	require.NoError(t, err)
	require.Equal(t, 429, event.Status)
}
