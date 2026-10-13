package responses

import (
	"encoding/json"
	"strings"
)

func (pc *pooledWebSocketConn) rememberTurn(input []json.RawMessage, responseID string, maxInputBytes int) bool {
	if strings.TrimSpace(responseID) == "" {
		return false
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if pc.closed {
		return false
	}
	pc.lastResponseID = strings.TrimSpace(responseID)
	size := rawMessagesSize(input)
	if (maxInputBytes > 0 && size > maxInputBytes) || (pc.budget != nil && !pc.budget.Replace(pc.retainedBytes, size)) {
		if pc.budget != nil {
			pc.budget.Replace(pc.retainedBytes, 0)
		}
		pc.lastInput, pc.retainedBytes = nil, 0
		return false
	}
	pc.lastInput = cloneRawMessages(input)
	pc.retainedBytes = size
	return true
}

func (l *webSocketLease) rememberTurn(responseID string, output []json.RawMessage) bool {
	if l == nil || l.pooled == nil {
		return false
	}
	maxInputBytes := 0
	if l.owner != nil {
		maxInputBytes = l.owner.maxRetainedInput
	}
	if l.native || len(l.nextFullInput) == 0 {
		return l.pooled.rememberTurn(nil, responseID, maxInputBytes)
	}
	if maxInputBytes > 0 && rawMessagesSize(l.nextFullInput)+rawMessagesSize(output) > maxInputBytes {
		return l.pooled.rememberTurn(nil, responseID, maxInputBytes)
	}
	input := append(l.nextFullInput[:len(l.nextFullInput):len(l.nextFullInput)], output...)
	return l.pooled.rememberTurn(input, responseID, maxInputBytes)
}
