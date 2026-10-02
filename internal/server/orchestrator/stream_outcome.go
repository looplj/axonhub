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
	validatedCompleted  bool
	transportErr        error
	contextErr          error
	contextState        streamTerminalState
}

type streamOutcomeDecision struct {
	state streamTerminalState
	cause error
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

func (o *streamOutcome) observeValidatedCompletion() {
	if !o.providerTerminal {
		o.validatedCompleted = true
	}
}

func (o *streamOutcome) observeTransportError(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		o.observeContextError(err)
		return
	}
	o.transportErr = err
}

func (o *streamOutcome) observeContextError(err error) {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			o.contextState = streamTerminalFailed
			o.contextErr = err
			return
		}
		if o.contextState == streamTerminalNone {
			o.contextState = streamTerminalCanceled
			o.contextErr = err
		}
	}
}

func (o streamOutcome) finalDecision() streamOutcomeDecision {
	if o.providerTerminal {
		return streamOutcomeDecision{state: o.terminalState}
	}
	if o.validatedCompleted {
		return streamOutcomeDecision{state: streamTerminalCompleted}
	}
	if o.contextState == streamTerminalFailed {
		return streamOutcomeDecision{state: streamTerminalFailed, cause: o.contextErr}
	}
	if o.transportErr != nil {
		return streamOutcomeDecision{state: streamTerminalFailed, cause: o.transportErr}
	}
	if o.contextState == streamTerminalCanceled {
		return streamOutcomeDecision{state: streamTerminalCanceled, cause: o.contextErr}
	}
	if o.aggregatedCompleted {
		return streamOutcomeDecision{state: streamTerminalCompleted}
	}

	return streamOutcomeDecision{state: streamTerminalIncomplete, cause: ErrStreamIncomplete}
}

func (o streamOutcome) finalState() streamTerminalState {
	return o.finalDecision().state
}

func (o streamOutcome) hasFinalEvidence() bool {
	return o.providerTerminal || o.aggregatedCompleted || o.validatedCompleted
}
