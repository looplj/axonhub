package codex

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

func TestCodexExecutorRetriesInvalidEncryptedContent(t *testing.T) {
	stub := &codexEncryptedRetryStub{}
	executor := &codexExecutor{inner: stub, httpExecutor: stub}
	request := &httpclient.Request{
		Method:    http.MethodPost,
		APIFormat: llm.APIFormatOpenAIResponse.String(),
		Body:      []byte(`{"input":[{"type":"reasoning","encrypted_content":"gAAAA"}]}`),
	}

	stream, err := executor.DoStream(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, stream)
	require.True(t, stream.Next())
	require.Equal(t, "response.completed", stream.Current().Type)
	require.Len(t, stub.requests, 2)
	require.Contains(t, string(stub.requests[0].Body), "gAAAA")
	require.NotContains(t, string(stub.requests[1].Body), "gAAAA")
}

type codexEncryptedRetryStub struct {
	requests []*httpclient.Request
}

var _ pipeline.Executor = (*codexEncryptedRetryStub)(nil)

func (s *codexEncryptedRetryStub) Do(_ context.Context, _ *httpclient.Request) (*httpclient.Response, error) {
	return nil, errors.New("unexpected non-streaming call")
}

func (s *codexEncryptedRetryStub) DoStream(_ context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	copyRequest := *request
	copyRequest.Body = append([]byte(nil), request.Body...)
	s.requests = append(s.requests, &copyRequest)

	if len(s.requests) == 1 {
		return nil, &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"error":{"code":"invalid_encrypted_content"}}`),
		}
	}

	return streams.SliceStream([]*httpclient.StreamEvent{{
		Type: "response.completed",
		Data: []byte(`{"type":"response.completed"}`),
	}}), nil
}
