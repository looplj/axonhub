package responses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/coder/websocket"

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

// Match Codex's upload fragments so a pong can be sent between data frames.
const webSocketRequestFragmentSize = 1 << 20

var errWebSocketClosedBeforeSend = errors.New("websocket closed before sending response.create")

func (l *webSocketLease) write(ctx context.Context, payload map[string]any) error {
	if l.pooled != nil {
		l.pooled.mu.Lock()
		defer l.pooled.mu.Unlock()
		if l.pooled.closed || l.pooled.generation != l.generation {
			return errWebSocketClosedBeforeSend
		}
		if id := explicitPreviousResponseID(payload); id != "" && id != l.pooled.lastResponseID {
			return shared.ResponsesRecoveryError()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var err error
	l.readTarget, err = l.receiver.attach()
	if err != nil {
		return fmt.Errorf("%w: %w", errWebSocketClosedBeforeSend, err)
	}
	writeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return json.NewEncoder(webSocketRequestWriter{ctx: writeCtx, conn: l.conn}).Encode(payload)
}

// The JSON encoder supplies one serialized response.create to Write.
type webSocketRequestWriter struct {
	ctx  context.Context
	conn *websocket.Conn
}

func (w webSocketRequestWriter) Write(data []byte) (int, error) {
	if len(data) <= webSocketRequestFragmentSize {
		if err := w.conn.Write(w.ctx, websocket.MessageText, data); err != nil {
			return 0, err
		}
		return len(data), nil
	}

	writer, err := w.conn.Writer(w.ctx, websocket.MessageText)
	if err != nil {
		return 0, err
	}
	written := 0
	for fragment := range slices.Chunk(data, webSocketRequestFragmentSize) {
		n, err := writer.Write(fragment)
		written += n
		if err != nil {
			// The caller retires this socket. Closing the writer would send
			// the final fragment of a partially written request.
			return written, err
		}
	}
	return written, writer.Close()
}
