package responses

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestResponsesOutboundStreamResponseFailedStatus(t *testing.T) {
	for _, tt := range []struct {
		status string
		want   int
	}{
		{"null", http.StatusBadGateway},
		{"0", http.StatusBadGateway},
		{"200", http.StatusBadGateway},
		{"399", http.StatusBadGateway},
		{"600", http.StatusBadGateway},
		{"-1", http.StatusBadGateway},
		{`"failed"`, http.StatusBadGateway},
		{"400", 400},
		{"429", 429},
		{"500", 500},
		{"599", 599},
	} {
		t.Run(tt.status, func(t *testing.T) {
			// Given
			stream := newResponsesOutboundStream(streams.SliceStream([]*httpclient.StreamEvent{{
				Data: fmt.Appendf(nil, `{"type":"response.failed","status":%s,"response":{"error":{"type":"server_error","code":"server_is_overloaded","message":"server overloaded"}}}`, tt.status),
			}}))

			// When
			chunks, err := streams.All(stream)

			// Then
			require.Empty(t, chunks)
			var responseErr *llm.ResponseError
			require.ErrorAs(t, err, &responseErr)
			require.Equal(t, tt.want, responseErr.StatusCode)
			require.ErrorIs(t, err, llm.ErrEmptyResponse)
		})
	}
}

func TestResponsesOutboundStreamResponseFailedUsageBeforeError(t *testing.T) {
	// Given: a later content event must not override the failed attempt.
	source := &responsesErrorAfterStream{items: []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.failed","response":{"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12,"input_tokens_details":{"cached_tokens":3},"output_tokens_details":{"reasoning_tokens":1}},"error":{"type":"server_error","code":"server_is_overloaded","message":"server overloaded"}}}`)},
		{Data: []byte(`{"type":"response.output_text.delta","delta":"late content"}`)},
	}}
	stream := newResponsesOutboundStream(source)

	// When
	require.True(t, stream.Next(), "usage must be available before stream termination")
	chunk := stream.Current()

	// Then
	require.Empty(t, chunk.Choices)
	require.NotNil(t, chunk.Usage)
	require.Equal(t, int64(10), chunk.Usage.PromptTokens)
	require.Equal(t, int64(2), chunk.Usage.CompletionTokens)
	require.Equal(t, int64(12), chunk.Usage.TotalTokens)
	require.Equal(t, int64(3), chunk.Usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, int64(1), chunk.Usage.CompletionTokensDetails.ReasoningTokens)
	require.False(t, stream.contentCommitted)
	require.False(t, stream.Next())
	require.ErrorIs(t, stream.Err(), llm.ErrEmptyResponse)
	require.EqualError(t, stream.Err(), "error: server overloaded, code: server_is_overloaded, type: server_error")
	require.False(t, stream.Next(), "the failure must remain terminal after usage drains")
	require.Equal(t, 1, source.index, "do not consume events after the failure")
}
