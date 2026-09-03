package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestIsUpstreamTransportError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unexpected EOF", err: io.ErrUnexpectedEOF, want: true},
		{name: "wrapped unexpected EOF", err: fmt.Errorf("HTTP stream request failed: %w", io.ErrUnexpectedEOF), want: true},
		{name: "EOF", err: io.EOF, want: true},
		{name: "llm stream incomplete", err: llm.ErrStreamIncomplete, want: true},
		{name: "orchestrator stream incomplete", err: ErrStreamIncomplete, want: true},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: false},
		{name: "plain error", err: errors.New("boom"), want: false},
		{name: "http error", err: &httpclient.Error{StatusCode: http.StatusBadGateway, Body: []byte(`{}`)}, want: false},
		{name: "response error", err: &llm.ResponseError{StatusCode: http.StatusTooManyRequests}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsUpstreamTransportError(tt.err))
		})
	}
}

func TestClassifyUpstreamTransportError(t *testing.T) {
	cause := fmt.Errorf("read body: %w", io.ErrUnexpectedEOF)

	err := ClassifyUpstreamTransportError(cause)

	respErr := &llm.ResponseError{}
	require.True(t, errors.As(err, &respErr))
	require.Equal(t, http.StatusBadGateway, respErr.StatusCode)
	require.Equal(t, ErrTypeUpstreamError, respErr.Detail.Type)
	require.Equal(t, ErrCodeUpstreamStreamInterrupted, respErr.Detail.Code)
	require.Contains(t, respErr.Detail.Message, "unexpected EOF")
	require.True(t, errors.Is(err, io.ErrUnexpectedEOF), "the cause must stay matchable through the classified error")

	// Idempotent for already classified errors, transparent for everything else.
	require.Same(t, err, ClassifyUpstreamTransportError(err))

	plain := errors.New("boom")
	require.Equal(t, plain, ClassifyUpstreamTransportError(plain))
	require.Equal(t, context.Canceled, ClassifyUpstreamTransportError(context.Canceled))
}

func TestExtractErrorMessageAndInfo_ClassifiedTransportError(t *testing.T) {
	err := ClassifyUpstreamTransportError(io.ErrUnexpectedEOF)

	require.Equal(t,
		"Upstream provider closed the connection before the response completed: unexpected EOF",
		ExtractErrorMessage(err))

	info := ExtractErrorInfo(err)
	require.NotNil(t, info)
	require.NotNil(t, info.StatusCode)
	require.Equal(t, http.StatusBadGateway, *info.StatusCode)

	// Plain errors keep the previous behavior.
	require.Equal(t, "boom", ExtractErrorMessage(errors.New("boom")))
	require.Nil(t, ExtractErrorInfo(errors.New("boom")))
}
