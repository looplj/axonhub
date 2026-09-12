package requesty_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/requesty"
)

func TestOutboundTransformer_TransformError(t *testing.T) {
	transformer, err := requesty.NewOutboundTransformer("https://router.requesty.ai/v1", "test-api-key")
	require.NoError(t, err)

	tests := []struct {
		name        string
		statusCode  int
		body        string
		wantMessage string
	}{
		{
			name:        "requesty error message",
			statusCode:  http.StatusNotFound,
			body:        `{"error":{"origin":"router","message":"The requested model was not found."}}`,
			wantMessage: "The requested model was not found.",
		},
		{
			name:        "raw metadata preferred over message",
			statusCode:  http.StatusBadRequest,
			body:        `{"error":{"message":"Provider returned error","code":400,"metadata":{"raw":"context length exceeded"}}}`,
			wantMessage: "context length exceeded",
		},
		{
			name:        "empty json object falls back to status text",
			statusCode:  http.StatusBadGateway,
			body:        `{}`,
			wantMessage: http.StatusText(http.StatusBadGateway),
		},
		{
			name:        "unrelated json falls back to status text",
			statusCode:  http.StatusServiceUnavailable,
			body:        `{"message":"upstream gateway timeout"}`,
			wantMessage: http.StatusText(http.StatusServiceUnavailable),
		},
		{
			name:        "non json body falls back to status text",
			statusCode:  http.StatusNotFound,
			body:        `404 page not found`,
			wantMessage: http.StatusText(http.StatusNotFound),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respErr := transformer.TransformError(context.Background(), &httpclient.Error{
				StatusCode: tt.statusCode,
				Body:       []byte(tt.body),
			})

			require.NotNil(t, respErr)
			require.Equal(t, tt.statusCode, respErr.StatusCode)
			require.Equal(t, tt.wantMessage, respErr.Detail.Message)
			require.Equal(t, "api_error", respErr.Detail.Type)
		})
	}
}

func TestOutboundTransformer_TransformStreamChunkError(t *testing.T) {
	outbound, err := requesty.NewOutboundTransformer("https://router.requesty.ai/v1", "test-api-key")
	require.NoError(t, err)

	transformer, ok := outbound.(*requesty.OutboundTransformer)
	require.True(t, ok)

	tests := []struct {
		name        string
		data        string
		wantMessage string
	}{
		{
			name:        "error object uses message",
			data:        `{"error":{"origin":"provider","message":"Rate limit exceeded","code":429}}`,
			wantMessage: "Rate limit exceeded",
		},
		{
			name:        "error string is passed through",
			data:        `{"error":"stream aborted"}`,
			wantMessage: "stream aborted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := transformer.TransformStreamChunk(context.Background(), &httpclient.StreamEvent{
				Data: []byte(tt.data),
			})
			require.Nil(t, resp)
			require.Error(t, err)

			var respErr *llm.ResponseError

			require.True(t, errors.As(err, &respErr))
			require.Equal(t, tt.wantMessage, respErr.Detail.Message)
		})
	}
}
