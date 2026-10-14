package responses

import (
	"context"
	"encoding/json"
	"time"

	"github.com/looplj/axonhub/llm/httpclient"

	"github.com/looplj/axonhub/llm/transformer/shared"
)

func ContinuationAuthIdentity(request *httpclient.Request) string {
	headers := request.Headers
	return hashSecret(authPoolIdentity(request.Auth, headers) + "\x00" + headerPoolIdentity(headers) + "\x00" + headers.Get(webSocketAccountIDHeader) + "\x00" + headers.Get(webSocketOrgHeader) + "\x00" + headers.Get(webSocketProjectHeader))
}

func isWebSocketRecoveryEvent(raw []byte) bool {
	var event struct {
		Code  string `json:"code"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	return json.Unmarshal(raw, &event) == nil && (event.Code == "previous_response_not_found" || event.Error.Code == "previous_response_not_found")
}

func (l *webSocketLease) write(ctx context.Context, payload map[string]any) error {
	if l.pooled != nil {
		l.pooled.mu.Lock()
		defer l.pooled.mu.Unlock()
		if l.pooled.closed || l.pooled.generation != l.generation {
			return shared.ResponsesRecoveryError()
		}
		if id := explicitPreviousResponseID(payload); id != "" && id != l.pooled.lastResponseID {
			return shared.ResponsesRecoveryError()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = l.conn.Close() })
	defer stop()
	deadline := time.Now().Add(30 * time.Second)
	if expires, ok := ctx.Deadline(); ok && expires.Before(deadline) {
		deadline = expires
	}
	if err := l.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return l.conn.WriteJSON(payload)
}
