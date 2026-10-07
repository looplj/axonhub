package decisions

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestDecisionsOfflineHTTPStatusMatrixPreservesResponseContract(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		contentType string
		wantError   bool
	}{
		{name: "success", status: http.StatusOK, body: `{"model":"m","answers":[{"choice":true}]}`, contentType: "application/json"},
		{name: "no content", status: http.StatusNoContent},
		{name: "redirect", status: http.StatusTemporaryRedirect, body: `{"error":{"message":"redirect"}}`, contentType: "application/problem+json", wantError: true},
		{name: "bad request", status: http.StatusBadRequest, body: `{"error":{"message":"bad request","type":"invalid_request_error"}}`, contentType: "application/json", wantError: true},
		{name: "unauthorized", status: http.StatusUnauthorized, body: `{"error":{"message":"unauthorized","type":"authentication_error"}}`, contentType: "application/json", wantError: true},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"error":{"message":"rate limited","type":"rate_limit_error"}}`, contentType: "application/json", wantError: true},
		{name: "server error", status: http.StatusBadGateway, body: `{"error":{"message":"gateway","type":"api_error"}}`, contentType: "application/json", wantError: true},
		{name: "malformed success", status: http.StatusOK, body: `{not-json`, contentType: "application/json", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(test.status)
				_, err := io.WriteString(w, test.body)
				require.NoError(t, err)
			}))
			t.Cleanup(server.Close)
			client := httpclient.NewHttpClientWithClient(server.Client())
			outbound, err := NewOutboundTransformer(server.URL, "test-provider-key")
			require.NoError(t, err)
			request, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "m", Decisions: &llm.DecisionsRequest{Body: []byte(`{"model":"m","input":"text","questions":[{}]}`)}})
			require.NoError(t, err)

			response, err := client.Do(context.Background(), request)
			if test.status >= http.StatusBadRequest {
				require.Error(t, err)
				var httpErr *httpclient.Error
				require.ErrorAs(t, err, &httpErr)
				require.Equal(t, test.status, httpErr.StatusCode)
				response = nil
			} else {
				require.NoError(t, err)
			}
			if test.status >= http.StatusMultipleChoices {
				if response != nil {
					_, conversionErr := outbound.TransformResponse(context.Background(), response)
					require.Error(t, conversionErr)
					var responseErr *llm.ResponseError
					require.ErrorAs(t, conversionErr, &responseErr)
					require.Equal(t, test.status, responseErr.StatusCode)
				} else {
					responseErr := outbound.TransformError(context.Background(), &httpclient.Error{StatusCode: test.status, Body: []byte(test.body)})
					require.Equal(t, test.status, responseErr.StatusCode)
				}
			} else {
				converted, convertErr := outbound.TransformResponse(context.Background(), response)
				if test.wantError {
					require.Error(t, convertErr)
					require.ErrorIs(t, convertErr, transformer.ErrInvalidResponse)
				} else {
					require.NoError(t, convertErr)
					require.NotNil(t, converted.Decisions)
				}
			}
		})
	}
}

func TestDecisionsOfflineHTTPNetworkAndTimeoutErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	serverURL := server.URL
	server.Close()

	outbound, err := NewOutboundTransformer(serverURL, "test-provider-key")
	require.NoError(t, err)
	request, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "m", Decisions: &llm.DecisionsRequest{Body: []byte(`{"model":"m","input":"text","questions":[{}]}`)}})
	require.NoError(t, err)
	_, err = httpclient.NewHttpClientWithClient(&http.Client{Timeout: 50 * time.Millisecond}).Do(context.Background(), request)
	require.Error(t, err)

	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		slow.Close()
	})
	outbound, err = NewOutboundTransformer(slow.URL, "test-provider-key")
	require.NoError(t, err)
	request, err = outbound.TransformRequest(context.Background(), &llm.Request{Model: "m", Decisions: &llm.DecisionsRequest{Body: []byte(`{"model":"m","input":"text","questions":[{}]}`)}})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = httpclient.NewHttpClientWithClient(slow.Client()).Do(ctx, request)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "invalid decisions response body")
}
