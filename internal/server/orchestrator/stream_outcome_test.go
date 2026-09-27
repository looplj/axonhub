package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStreamOutcomeFinalState(t *testing.T) {
	transportErr := errors.New("http2: response body closed")

	tests := []struct {
		name      string
		configure func(*streamOutcome)
		want      streamTerminalState
	}{
		{
			name: "provider completion outranks trailing transport error",
			configure: func(outcome *streamOutcome) {
				outcome.observeTerminal(streamTerminalCompleted)
				outcome.observeTransportError(transportErr)
			},
			want: streamTerminalCompleted,
		},
		{
			name: "provider failure preserves failure",
			configure: func(outcome *streamOutcome) {
				outcome.observeTerminal(streamTerminalFailed)
				outcome.observeTransportError(transportErr)
			},
			want: streamTerminalFailed,
		},
		{
			name: "explicit aggregation evidence permits completion",
			configure: func(outcome *streamOutcome) {
				outcome.observeAggregatedCompletion(true)
				outcome.observeTransportError(transportErr)
			},
			want: streamTerminalCompleted,
		},
		{
			name: "transport error without terminal evidence fails",
			configure: func(outcome *streamOutcome) {
				outcome.observeTransportError(transportErr)
			},
			want: streamTerminalFailed,
		},
		{
			name: "client cancellation is canceled",
			configure: func(outcome *streamOutcome) {
				outcome.observeContextError(context.Canceled)
			},
			want: streamTerminalCanceled,
		},
		{
			name: "server deadline is failed rather than canceled",
			configure: func(outcome *streamOutcome) {
				outcome.observeContextError(context.DeadlineExceeded)
			},
			want: streamTerminalFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := streamOutcome{}
			tt.configure(&outcome)
			require.Equal(t, tt.want, outcome.finalState())
		})
	}
}
