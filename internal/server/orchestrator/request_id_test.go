package orchestrator

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestApplyUpstreamRequestID_ReusedClientIDGetsUniqueUpstreamIDs(t *testing.T) {
	middleware := applyUpstreamRequestID()

	first, err := middleware.OnOutboundRawRequest(
		contexts.WithRequestID(t.Context(), "ar-first-request"),
		&httpclient.Request{Headers: http.Header{
			upstreamRequestIDHeader: []string{"shared-agent-trace"},
		}},
	)
	require.NoError(t, err)

	second, err := middleware.OnOutboundRawRequest(
		contexts.WithRequestID(t.Context(), "ar-second-request"),
		&httpclient.Request{Headers: http.Header{
			upstreamRequestIDHeader: []string{"shared-agent-trace"},
		}},
	)
	require.NoError(t, err)

	assert.Equal(t, "ar-first-request", first.Headers.Get(upstreamRequestIDHeader))
	assert.Equal(t, "ar-second-request", second.Headers.Get(upstreamRequestIDHeader))
	assert.NotEqual(t, first.Headers.Get(upstreamRequestIDHeader), second.Headers.Get(upstreamRequestIDHeader))
}

func TestApplyUpstreamRequestID_InitializesHeaders(t *testing.T) {
	ctx := contexts.WithRequestID(t.Context(), "ar-unique-request")

	got, err := applyUpstreamRequestID().OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.NoError(t, err)
	assert.Equal(t, "ar-unique-request", got.Headers.Get(upstreamRequestIDHeader))
}

func TestApplyUpstreamRequestID_PreservesHeaderWithoutContextID(t *testing.T) {
	request := &httpclient.Request{Headers: http.Header{
		upstreamRequestIDHeader: []string{"client-request-id"},
	}}

	got, err := applyUpstreamRequestID().OnOutboundRawRequest(t.Context(), request)
	require.NoError(t, err)
	assert.Equal(t, "client-request-id", got.Headers.Get(upstreamRequestIDHeader))
}
