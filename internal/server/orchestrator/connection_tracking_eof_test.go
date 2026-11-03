package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestChannelLimiterMiddleware_ReleasesAtEOFBeforeDownstreamClose(t *testing.T) {
	ch := channelWithLimit(17, "review-shared-channel", 1, 1)
	mgr := NewChannelLimiterManager()
	m := withChannelLimiter(newTestOutbound(ch), mgr, nil).(*channelLimiterMiddleware)
	lim := mgr.GetOrCreate(ch)
	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)
	recorder := &hardModeStreamRecorder{lim: lim}
	wrapped, err := m.OnOutboundLlmStream(t.Context(), recorder)
	require.NoError(t, err)
	require.False(t, wrapped.Next())
	require.True(t, recorder.closeCalled)
	require.Equal(t, 1, recorder.inFlightAtClose, "upstream closes before its slot is released")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, lim.Acquire(ctx), "reviewer must acquire without waiting for downstream Close")
	require.NoError(t, wrapped.Close())
	inFlight, _ := lim.Stats()
	require.Equal(t, 1, inFlight, "late downstream Close must not release the reviewer's slot")
	lim.Release()
}
