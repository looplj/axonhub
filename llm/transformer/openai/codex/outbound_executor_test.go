package codex

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type captureHeadersExecutor struct {
	err  error
	seen http.Header
}

func (e *captureHeadersExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	return nil, errors.New("unexpected Do")
}

func (e *captureHeadersExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	e.seen = request.Headers.Clone()
	return nil, e.err
}

func TestCodexExecutor_SetsConversationIDInDo(t *testing.T) {
	inner := &captureHeadersExecutor{err: errors.New("stop")}
	exec := &codexExecutor{inner: inner, transformer: nil}

	req := &httpclient.Request{Headers: make(http.Header)}
	req.Headers.Set("Session_id", uuid.NewString())

	_, err := exec.Do(context.Background(), req)
	require.Error(t, err)

	require.Equal(t, req.Headers.Get("Session_id"), inner.seen.Get("Conversation_id"))
	require.Equal(t, "text/event-stream", inner.seen.Get("Accept"))
	require.Equal(t, UserAgent, inner.seen.Get("User-Agent"))
	require.Equal(t, "responses=experimental", inner.seen.Get("Openai-Beta"))
	require.Equal(t, "codex_cli_rs", inner.seen.Get("Originator"))
	require.Equal(t, codexDefaultVersion, inner.seen.Get("Version"))
}
