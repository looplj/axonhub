package api

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm/httpclient"
)

func responsesWarmupBytes(payload map[string]json.RawMessage) int {
	total := 0
	for key, value := range payload {
		total += len(key) + len(value)
	}
	return total
}

func (s *responsesWebSocketSession) releaseWarmup() {
	if s.budget != nil {
		s.budget.Replace(s.warmupBytes, 0)
	}
	s.warmupBytes = 0
	s.warmupID, s.warmupPayload = "", nil
}

func (s *responsesWebSocketSession) releaseConsumedWarmup() {
	if s.warmupID == "" {
		s.releaseWarmup()
	}
}

func responsesWebSocketRetentionLimitError() *httpclient.Error {
	return responsesWebSocketRequestError("This WebSocket connection has reached its retained context byte limit (64 MiB).", "input", "websocket_context_limit_reached")
}
