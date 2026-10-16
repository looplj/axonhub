package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestStreamDeliveryBaseline(t *testing.T) {
	t.Run("legacy raw completion survives cancel", func(t *testing.T) {
		// Given: existing execution evidence, without delivery acknowledgement.
		outcome := streamOutcome{}
		outcome.observeTerminal(streamTerminalCompleted)
		// When
		outcome.observeContextError(context.Canceled)
		// Then: preserve this legacy decision until task 6 integrates delivery.
		require.Equal(t, streamTerminalCompleted, outcome.finalState())
	})
	t.Run("all choices required", func(t *testing.T) {
		// Given
		tracker := NewStreamTerminalTracker(2)
		first := &httpclient.StreamEvent{Data: []byte(`{"choices":[{"index":0,"finish_reason":"stop"}]}`)}
		second := &httpclient.StreamEvent{Data: []byte(`{"choices":[{"index":1,"finish_reason":"stop"}]}`)}
		// When / Then
		require.False(t, tracker.Observe(first))
		require.True(t, tracker.Observe(second))
		require.True(t, tracker.AllChoicesFinished())
	})
	t.Run("usage does not establish completion", func(t *testing.T) {
		// Given
		meta := llm.ResponseMeta{Usage: &llm.Usage{PromptTokens: 12, CompletionTokens: 34}}
		// When / Then
		require.False(t, isCompletedAggregated(meta))
	})
}

func TestStreamDeliveryContract(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*streamOutcome)
		want      streamTerminalState
	}{
		{
			name: "observed terminal without acknowledgement stays incomplete",
			configure: func(outcome *streamOutcome) {
				outcome.observeTerminal(streamTerminalCompleted)
			},
			want: streamTerminalIncomplete,
		},
		{
			name: "acknowledged success survives later cancellation",
			configure: func(outcome *streamOutcome) {
				outcome.observeTerminal(streamTerminalCompleted)
				outcome.observeDeliveryAcknowledgement()
				outcome.observeContextError(context.Canceled)
			},
			want: streamTerminalCompleted,
		},
		{
			name: "provider failure remains unsuccessful after acknowledgement",
			configure: func(outcome *streamOutcome) {
				outcome.observeTerminal(streamTerminalFailed)
				outcome.observeDeliveryAcknowledgement()
			},
			want: streamTerminalFailed,
		},
		{
			name: "anthropic message stop needs no done event",
			configure: func(outcome *streamOutcome) {
				outcome.observeTerminal(streamTerminalCompleted)
				outcome.observeDeliveryAcknowledgement()
			},
			want: streamTerminalCompleted,
		},
		{
			name: "validated clean eof needs acknowledgement",
			configure: func(outcome *streamOutcome) {
				outcome.observeValidatedCompletion()
				outcome.observeDeliveryAcknowledgement()
			},
			want: streamTerminalCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := streamOutcome{}
			tt.configure(&outcome)
			require.Equal(t, tt.want, outcome.deliveryState())
		})
	}

	t.Run("accounting finality is independent of usage value", func(t *testing.T) {
		finalZero := llm.ResponseMeta{
			Usage:              &llm.Usage{},
			AccountingEvidence: llm.AccountingEvidenceExplicitFinal,
		}
		require.True(t, finalZero.AccountingFinal(llm.SourceEndUnknown))
		require.False(t, (llm.ResponseMeta{
			Usage: &llm.Usage{TotalTokens: 34},
		}).AccountingFinal(llm.SourceEndUnknown))
		require.True(t, (llm.ResponseMeta{
			AccountingEvidence: llm.AccountingEvidenceCleanEndCandidate,
		}).AccountingFinal(llm.SourceEndSemantic))
		require.False(t, (llm.ResponseMeta{
			AccountingEvidence: llm.AccountingEvidenceCleanEndCandidate,
		}).AccountingFinal(llm.SourceEndError))
	})
}
