package pipeline

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/decisions"
)

type decisionsRetryableOutbound struct {
	*decisions.OutboundTransformer
}

func (o *decisionsRetryableOutbound) CanRetry(error) bool { return true }

func (o *decisionsRetryableOutbound) PrepareForRetry(context.Context) error { return nil }

func TestDecisionsEmptyResponseRetryStopsAfterConfiguredAttempts(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, err := io.WriteString(w, `{"model":"m","answers":[]}`)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	outbound, err := decisions.NewOutboundTransformer(server.URL, "test-provider-key")
	require.NoError(t, err)
	p := NewFactory(httpclient.NewHttpClientWithClient(server.Client())).Pipeline(
		decisions.NewInboundTransformer(),
		&decisionsRetryableOutbound{OutboundTransformer: outbound},
		WithRetry(0, 1, 0),
		WithEmptyResponseDetection(),
	)

	_, err = p.Process(context.Background(), &httpclient.Request{
		Method: http.MethodPost,
		Body:   []byte(`{"model":"m","input":"text","questions":[{}]}`),
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	})

	require.ErrorIs(t, err, ErrEmptyResponse)
	require.Equal(t, 2, calls)
}

var _ interface {
	CanRetry(error) bool
	PrepareForRetry(context.Context) error
} = (*decisionsRetryableOutbound)(nil)
