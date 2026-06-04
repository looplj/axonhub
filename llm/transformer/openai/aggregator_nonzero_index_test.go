package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestAggregateStreamChunksNonZeroChoiceIndex ensures that a stream whose only
// choice carries a non-zero index aggregates without panicking. choicesAggs is
// keyed by the choice index, so a sparse/non-zero-based index must not be
// looked up positionally.
func TestAggregateStreamChunksNonZeroChoiceIndex(t *testing.T) {
	chunk := `{"id":"chatcmpl-1","model":"gpt-4o-mini","object":"chat.completion.chunk","created":1,` +
		`"choices":[{"index":1,"delta":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`

	chunks := []*httpclient.StreamEvent{{Data: []byte(chunk)}}

	gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var got llm.Response
	require.NoError(t, json.Unmarshal(gotBytes, &got))
	require.Len(t, got.Choices, 1)
	require.Equal(t, 1, got.Choices[0].Index)
	require.NotNil(t, got.Choices[0].Message.Content.Content)
	require.Equal(t, "hi", *got.Choices[0].Message.Content.Content)
}
