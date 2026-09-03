package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestInboundTransformer_TransformError_WrappedTypedNilReturnsInternalServerError(t *testing.T) {
	transformer := NewInboundTransformer()
	var typedNil *httpclient.Error
	wrapped := fmt.Errorf("wrap: %w", typedNil)

	result := transformer.TransformError(context.Background(), wrapped)

	require.Equal(t, http.StatusInternalServerError, result.StatusCode)
	require.Equal(t, http.StatusText(http.StatusInternalServerError), result.Status)
	require.JSONEq(t, `{"error":{"message":"An unexpected error occurred","type":"unexpected_error"}}`, string(result.Body))
}

func TestInboundTransformer_TransformError_WrappedTypedNilResponseErrorReturnsInternalServerError(t *testing.T) {
	transformer := NewInboundTransformer()
	var typedNil *llm.ResponseError
	wrapped := fmt.Errorf("wrap: %w", typedNil)

	result := transformer.TransformError(context.Background(), wrapped)

	require.Equal(t, http.StatusInternalServerError, result.StatusCode)
	require.Equal(t, http.StatusText(http.StatusInternalServerError), result.Status)
	require.JSONEq(t, `{"error":{"message":"An unexpected error occurred","type":"unexpected_error"}}`, string(result.Body))
}

func TestInboundTransformer_TransformError_StatuslessContextLengthExceededReturnsBadRequest(t *testing.T) {
	transformer := NewInboundTransformer()
	detail := llm.ErrorDetail{
		Code:    "context_length_exceeded",
		Message: "maximum context length exceeded",
		Type:    "invalid_request_error",
		Param:   "messages",
	}

	result := transformer.TransformError(context.Background(), &llm.ResponseError{Detail: detail})

	require.Equal(t, http.StatusBadRequest, result.StatusCode)
	require.Equal(t, http.StatusText(http.StatusBadRequest), result.Status)
	var body OpenAIError
	require.NoError(t, json.Unmarshal(result.Body, &body))
	require.Equal(t, detail, body.Detail)
}

func TestInboundTransformer_TransformError_NormalizesStatusCodes(t *testing.T) {
	httpBadRequest := &httpclient.Error{StatusCode: http.StatusBadRequest, Status: "client error"}
	httpServiceUnavailable := &httpclient.Error{StatusCode: http.StatusServiceUnavailable, Status: "server error"}

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantSame   *httpclient.Error
	}{
		{
			name:       "httpclient error preserves valid 4xx",
			err:        httpBadRequest,
			wantStatus: http.StatusBadRequest,
			wantSame:   httpBadRequest,
		},
		{
			name:       "httpclient error preserves valid 5xx",
			err:        httpServiceUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantSame:   httpServiceUnavailable,
		},
		{
			name:       "response error preserves valid 4xx",
			err:        &llm.ResponseError{StatusCode: http.StatusBadRequest},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "response error preserves valid 5xx",
			err:        &llm.ResponseError{StatusCode: http.StatusServiceUnavailable},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "httpclient error maps status 200 to bad gateway",
			err:        &httpclient.Error{StatusCode: http.StatusOK},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "httpclient error maps status 600 to bad gateway",
			err:        &httpclient.Error{StatusCode: 600},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "response error maps status 200 to bad gateway",
			err:        &llm.ResponseError{StatusCode: http.StatusOK},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "response error maps status 600 to bad gateway",
			err:        &llm.ResponseError{StatusCode: 600},
			wantStatus: http.StatusBadGateway,
		},
	}

	transformer := NewInboundTransformer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.TransformError(context.Background(), tt.err)

			require.Equal(t, tt.wantStatus, result.StatusCode)
			if tt.wantSame != nil {
				require.Same(t, tt.wantSame, result)
			}
		})
	}
}
