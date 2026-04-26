package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// newTestOutbound returns an outbound transformer whose GetCurrentChannel returns ch.
func newTestOutbound(ch *biz.Channel) *PersistentOutboundTransformer {
	return &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{Channel: ch},
		},
	}
}

func channelWithLimit(id int, name string, max, queue int64) *biz.Channel {
	rl := &objects.ChannelRateLimit{
		MaxConcurrent: lo.ToPtr(max),
	}
	if queue > 0 {
		rl.QueueSize = lo.ToPtr(queue)
	}

	return &biz.Channel{
		Channel: &ent.Channel{
			ID:   id,
			Name: name,
			Settings: &objects.ChannelSettings{
				RateLimit: rl,
			},
		},
	}
}

func TestChannelLimiterMiddleware_AcquireAndReleaseOnResponse(t *testing.T) {
	t.Parallel()

	ch := channelWithLimit(1, "k", 2, 0) // soft mode is enough for counting
	mgr := NewChannelLimiterManager()
	out := newTestOutbound(ch)
	m := withChannelLimiter(out, mgr, nil).(*channelLimiterMiddleware)

	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)
	require.NotNil(t, m.acquired)

	inFlight, _ := m.acquired.Stats()
	assert.Equal(t, 1, inFlight)

	_, err = m.OnOutboundLlmResponse(t.Context(), &llm.Response{})
	require.NoError(t, err)

	inFlight, _ = m.acquired.Stats()
	assert.Equal(t, 0, inFlight, "Release on response must drop in-flight")
}

func TestChannelLimiterMiddleware_NoLimitChannelBypasses(t *testing.T) {
	t.Parallel()

	ch := &biz.Channel{Channel: &ent.Channel{ID: 9, Name: "open"}} // no settings
	mgr := NewChannelLimiterManager()
	out := newTestOutbound(ch)
	m := withChannelLimiter(out, mgr, nil).(*channelLimiterMiddleware)

	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)
	assert.Nil(t, m.acquired, "channels without rate limit must not engage limiter")
}

func TestChannelLimiterMiddleware_QueueFullReturnsTypedError(t *testing.T) {
	t.Parallel()

	ch := channelWithLimit(2, "kimi", 1, 0) // soft mode treats acquisitions as instant; switch to hard mode below
	// Use hard mode with queue=0 to force ErrChannelQueueFull on the second acquire? No —
	// queueSize must be > 0 for hard mode. Use queue=1 with capacity already saturated.
	ch = channelWithLimit(2, "kimi", 1, 1)

	mgr := NewChannelLimiterManager()

	// Pre-saturate capacity and queue.
	lim := mgr.GetOrCreate(ch)
	require.NoError(t, lim.Acquire(t.Context()))

	queueSlotCtx, cancelQueueSlot := context.WithCancel(t.Context())
	defer cancelQueueSlot()

	queueDone := make(chan struct{})
	go func() {
		_ = lim.Acquire(queueSlotCtx) // sits in queue until ctx cancel
		close(queueDone)
	}()

	require.Eventually(t, func() bool {
		_, w := lim.Stats()
		return w == 1
	}, time.Second, 5*time.Millisecond)

	// Now invoke middleware — capacity 1 + queue 1 are taken, so the call must fail.
	out := newTestOutbound(ch)
	m := withChannelLimiter(out, mgr, nil).(*channelLimiterMiddleware)

	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.Error(t, err)

	var queueErr *ChannelQueueError
	require.ErrorAs(t, err, &queueErr)
	assert.Equal(t, channelQueueReasonFull, queueErr.Reason)
	assert.Equal(t, ch.ID, queueErr.ChannelID)
	assert.Nil(t, m.acquired, "no slot must be retained after Acquire failure")

	cancelQueueSlot()
	<-queueDone
	lim.Release()
}

func TestChannelLimiterMiddleware_OnceProtection(t *testing.T) {
	t.Parallel()

	ch := channelWithLimit(3, "x", 5, 0)
	mgr := NewChannelLimiterManager()
	out := newTestOutbound(ch)
	m := withChannelLimiter(out, mgr, nil).(*channelLimiterMiddleware)

	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	inFlight, _ := m.acquired.Stats()
	require.Equal(t, 1, inFlight)

	// Trigger every release path concurrently — sync.Once must keep inFlight from
	// going negative.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, _ = m.OnOutboundLlmResponse(t.Context(), &llm.Response{})
			m.OnOutboundRawError(t.Context(), errors.New("boom"))

			s, _ := m.OnOutboundLlmStream(t.Context(), &emptyResponseStream{})
			_ = s.Close()
		})
	}
	wg.Wait()

	inFlight, _ = m.acquired.Stats()
	assert.Equal(t, 0, inFlight, "release must run exactly once across all paths")
}

func TestChannelLimiterMiddleware_StreamCloseReleases(t *testing.T) {
	t.Parallel()

	ch := channelWithLimit(4, "y", 3, 0)
	mgr := NewChannelLimiterManager()
	out := newTestOutbound(ch)
	m := withChannelLimiter(out, mgr, nil).(*channelLimiterMiddleware)

	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	wrappedStream, err := m.OnOutboundLlmStream(t.Context(), &emptyResponseStream{})
	require.NoError(t, err)
	require.NoError(t, wrappedStream.Close())

	inFlight, _ := m.acquired.Stats()
	assert.Equal(t, 0, inFlight, "stream Close must release the slot")
}

// emptyResponseStream is a minimal Stream[*llm.Response] used only as a passthrough.
type emptyResponseStream struct{}

func (e *emptyResponseStream) Current() *llm.Response { return nil }
func (e *emptyResponseStream) Next() bool             { return false }
func (e *emptyResponseStream) Close() error           { return nil }
func (e *emptyResponseStream) Err() error             { return nil }

var _ streams.Stream[*llm.Response] = (*emptyResponseStream)(nil)
