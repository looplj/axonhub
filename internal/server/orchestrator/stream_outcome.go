package orchestrator

import (
	"context"
	"errors"
)

// streamOutcome is the single monotonic view of a stream's terminal facts.
// Provider terminal outcomes outrank transport and context errors observed
// after them; a transport error can only be replaced by a completed outcome
// when aggregation provides explicit completion evidence.
type streamOutcome struct {
	terminalState       streamTerminalState
	providerTerminal    bool
	aggregatedCompleted bool
	transportErr        error
	contextErr          error
}

func (o *streamOutcome) observeTerminal(state streamTerminalState) {
	if o.providerTerminal || state == streamTerminalNone {
		return
	}

	o.providerTerminal = true
	o.terminalState = state
}

func (o *streamOutcome) observeAggregatedCompletion(completed bool) {
	if !o.providerTerminal && completed {
		o.aggregatedCompleted = true
	}
}

func (o *streamOutcome) observeTransportError(err error) {
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		o.transportErr = err
	}
}

func (o *streamOutcome) observeContextError(err error) {
	if err != nil {
		o.contextErr = err
	}
}

func (o streamOutcome) finalState() streamTerminalState {
	if o.providerTerminal {
		return o.terminalState
	}
	if o.aggregatedCompleted {
		return streamTerminalCompleted
	}
	if o.contextErr != nil {
		return streamTerminalCanceled
	}
	if o.transportErr != nil {
		return streamTerminalFailed
	}
	return streamTerminalIncomplete
}
