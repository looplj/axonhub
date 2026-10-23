package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type terminalCloseProbe struct {
	streams.Stream[*httpclient.StreamEvent]

	reads, closes int
	err           error
}

func (s *terminalCloseProbe) Next() bool   { s.reads++; return s.Stream.Next() }
func (s *terminalCloseProbe) Close() error { s.closes++; s.err = context.Canceled; return nil }
func (s *terminalCloseProbe) Err() error   { return s.err }

// TestAnthropicTerminalStream closes the transport at semantic completion while
// retaining errors and incomplete EOF before a terminal event.
func TestAnthropicTerminalStream(t *testing.T) {
	upstreamErr := errors.New("upstream failed")
	for _, tc := range []struct {
		name, body string
		err        error
		complete   bool
	}{
		{"complete", `{"type":"message_stop"}`, nil, true},
		{"incomplete", `{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`, nil, false},
		{"failed", `{"type":"error"}`, upstreamErr, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := &terminalCloseProbe{Stream: streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte(tc.body)}}), err: tc.err}
			s := &anthropicTerminalStream{Stream: source}
			require.True(t, s.Next())
			require.JSONEq(t, tc.body, string(s.Current().Data))
			require.False(t, s.Next())
			if tc.complete {
				require.Equal(t, 1, source.reads, "must not wait for transport EOF")
				require.Equal(t, 1, source.closes)
				require.NoError(t, s.Err(), "closing a completed source is cleanup")
			} else {
				require.Equal(t, 0, source.closes)
				require.Equal(t, tc.err, s.Err())
			}
			require.NoError(t, s.Close())
			require.NoError(t, s.Close())
			require.Equal(t, 1, source.closes)
		})
	}
}

// TestAnthropicTerminalMiddlewareScope avoids interpreting other providers'
// event payloads as an Anthropic protocol boundary.
func TestAnthropicTerminalMiddlewareScope(t *testing.T) {
	for _, format := range []llm.APIFormat{llm.APIFormatAnthropicMessage, llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIChatCompletion} {
		source := streams.SliceStream([]*httpclient.StreamEvent{})
		outbound := &PersistentOutboundTransformer{state: &PersistenceState{RawProviderRequest: &httpclient.Request{APIFormat: string(format)}}}
		wrapped, err := closeCompletedAnthropicStream(outbound).OnOutboundRawStream(t.Context(), source)
		require.NoError(t, err)
		if format != llm.APIFormatAnthropicMessage {
			require.Same(t, source, wrapped)
		} else {
			require.NotSame(t, source, wrapped)
		}
	}
}
