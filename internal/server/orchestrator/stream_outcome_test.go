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
			name: "aggregation evidence cannot override transport error",
			configure: func(outcome *streamOutcome) {
				outcome.observeAggregatedCompletion(true)
				outcome.observeTransportError(transportErr)
			},
			want: streamTerminalFailed,
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

func TestStreamOutcomeFinalDecisionSelectsCause(t *testing.T) {
	transportErr := errors.New("unexpected EOF")

	tests := []struct {
		name      string
		configure func(*streamOutcome)
		wantState streamTerminalState
		wantCause error
	}{
		{
			name: "substantive transport error beats client cancellation",
			configure: func(outcome *streamOutcome) {
				outcome.observeTransportError(transportErr)
				outcome.observeContextError(context.Canceled)
			},
			wantState: streamTerminalFailed,
			wantCause: transportErr,
		},
		{
			name: "server deadline beats transport and client cancellation",
			configure: func(outcome *streamOutcome) {
				outcome.observeTransportError(transportErr)
				outcome.observeContextError(context.Canceled)
				outcome.observeContextError(context.DeadlineExceeded)
			},
			wantState: streamTerminalFailed,
			wantCause: context.DeadlineExceeded,
		},
		{
			name: "client cancellation is the selected cause when no stronger cause exists",
			configure: func(outcome *streamOutcome) {
				outcome.observeTransportError(context.Canceled)
				outcome.observeContextError(context.Canceled)
			},
			wantState: streamTerminalCanceled,
			wantCause: context.Canceled,
		},
		{
			name: "source cancellation is the selected cause when wrapper context is alive",
			configure: func(outcome *streamOutcome) {
				outcome.observeTransportError(context.Canceled)
			},
			wantState: streamTerminalCanceled,
			wantCause: context.Canceled,
		},
		{
			name: "source deadline is failed when wrapper context is alive",
			configure: func(outcome *streamOutcome) {
				outcome.observeTransportError(context.DeadlineExceeded)
			},
			wantState: streamTerminalFailed,
			wantCause: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := streamOutcome{}
			tt.configure(&outcome)
			decision := outcome.finalDecision()
			require.Equal(t, tt.wantState, decision.state)
			require.ErrorIs(t, decision.cause, tt.wantCause)
		})
	}
}
