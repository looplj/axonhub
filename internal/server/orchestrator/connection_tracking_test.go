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

	ch := channelWithLimit(1, "k", 2, 0)
	mgr := NewChannelLimiterManager()
	out := newTestOutbound(ch)
	m := withChannelLimiter(out, mgr, nil).(*channelLimiterMiddleware)
	lim := mgr.GetOrCreate(ch)

	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)
	require.NotNil(t, m.current.Load())

	inFlight, _ := lim.Stats()
	assert.Equal(t, 1, inFlight)

	_, err = m.OnOutboundLlmResponse(t.Context(), &llm.Response{})
	require.NoError(t, err)

	inFlight, _ = lim.Stats()
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
	assert.Nil(t, m.current.Load(), "channels without rate limit must not engage limiter")
}

func TestChannelLimiterMiddleware_QueueFullReturnsTypedError(t *testing.T) {
	t.Parallel()

	// capacity 1 + bounded queue 1: once both are taken, the next acquire must
	// return ErrChannelQueueFull immediately.
	ch := channelWithLimit(2, "kimi", 1, 1)

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
	assert.Nil(t, m.current.Load(), "no slot must be retained after Acquire failure")

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
	lim := mgr.GetOrCreate(ch)

	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	inFlight, _ := lim.Stats()
	require.Equal(t, 1, inFlight)

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

	inFlight, _ = lim.Stats()
	assert.Equal(t, 0, inFlight, "release must run exactly once across all paths")
}

func TestChannelLimiterMiddleware_StreamCloseReleases(t *testing.T) {
	t.Parallel()

	ch := channelWithLimit(4, "y", 3, 0)
	mgr := NewChannelLimiterManager()
	out := newTestOutbound(ch)
	m := withChannelLimiter(out, mgr, nil).(*channelLimiterMiddleware)
	lim := mgr.GetOrCreate(ch)

	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	wrappedStream, err := m.OnOutboundLlmStream(t.Context(), &emptyResponseStream{})
	require.NoError(t, err)
	require.NoError(t, wrappedStream.Close())

	inFlight, _ := lim.Stats()
	assert.Equal(t, 0, inFlight, "stream Close must release the slot")
}

// Pipeline.Process re-enters OnOutboundRawRequest on same-channel retry and
// channel switch. A struct-scoped Once would short-circuit every release after
// the first, leaking the slot permanently; per-attempt slots must not.
func TestChannelLimiterMiddleware_RetryReacquireDoesNotLeak(t *testing.T) {
	t.Parallel()

	ch := channelWithLimit(7, "retry-ch", 2, 0)
	mgr := NewChannelLimiterManager()
	out := newTestOutbound(ch)
	m := withChannelLimiter(out, mgr, nil).(*channelLimiterMiddleware)
	lim := mgr.GetOrCreate(ch)

	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)
	m.OnOutboundRawError(t.Context(), errors.New("upstream 429"))

	inFlight, _ := lim.Stats()
	require.Equal(t, 0, inFlight, "attempt 1 must release after error")

	_, err = m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	inFlight, _ = lim.Stats()
	require.Equal(t, 1, inFlight, "attempt 2 must hold a slot")

	_, err = m.OnOutboundLlmResponse(t.Context(), &llm.Response{})
	require.NoError(t, err)

	inFlight, _ = lim.Stats()
	require.Equal(t, 0, inFlight, "attempt 2 success must release; pre-fix this leaked")

	_, err = m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	wrapped, err := m.OnOutboundLlmStream(t.Context(), &emptyResponseStream{})
	require.NoError(t, err)
	require.NoError(t, wrapped.Close())

	inFlight, _ = lim.Stats()
	assert.Equal(t, 0, inFlight, "stream Close on retry must release; pre-fix this leaked")
}

// emptyResponseStream is a minimal Stream[*llm.Response] used only as a passthrough.
type emptyResponseStream struct{}

func (e *emptyResponseStream) Current() *llm.Response { return nil }
func (e *emptyResponseStream) Next() bool             { return false }
func (e *emptyResponseStream) Close() error           { return nil }
func (e *emptyResponseStream) Err() error             { return nil }

var _ streams.Stream[*llm.Response] = (*emptyResponseStream)(nil)

// hardModeStreamRecorder is a Stream[*llm.Response] whose Close captures the
// limiter's live state at the exact moment THIS request's upstream stream is
// being torn down. It lets the test observe whether a queued waiter has already
// been admitted (slot handed over) BEFORE the upstream connection was closed.
type hardModeStreamRecorder struct {
	lim             *ChannelLimiter
	closeCalled     bool
	inFlightAtClose int
	waitingAtClose  int
}

func (s *hardModeStreamRecorder) Current() *llm.Response { return nil }
func (s *hardModeStreamRecorder) Next() bool             { return false }
func (s *hardModeStreamRecorder) Err() error             { return nil }

func (s *hardModeStreamRecorder) Close() error {
	s.closeCalled = true
	s.inFlightAtClose, s.waitingAtClose = s.lim.Stats()

	return nil
}

var _ streams.Stream[*llm.Response] = (*hardModeStreamRecorder)(nil)

// TestChannelLimiterMiddleware_StreamCloseDoesNotAdmitWaiterBeforeUpstreamClosed
// guards the per-channel concurrency cap at the PHYSICAL upstream connection when a
// bounded queue is configured (queueSize > 0).
//
// It guards against a release-before-close ordering bug: if channelLimiterStream.Close
// released the limiter slot BEFORE closing s.Stream, a queued waiter B would be granted
// A's slot — and be free to dial a brand-new upstream connection — while A's own
// upstream stream/connection had NOT yet been closed. For that window the channel would
// momentarily hold capacity+1 live upstream connections even though the inFlight counter
// stays <= capacity. On a client abort (Close before EOF) the old connection is still
// actively open, so the overlap is real, not just bookkeeping.
//
// Repro is deterministic: capacity=1, queue=1. Request A holds the only slot and waiter
// B is parked in the queue. When A's wrapped stream is closed, the recorder captures the
// limiter state at the instant the upstream Close runs. Correct behavior is that B is
// STILL queued (waiting == 1) when A's upstream is closed — the slot must only be handed
// over AFTER the upstream is torn down.
//
//   - waiting == 1: PASS — A's upstream closed first, then B was admitted (correct).
//   - waiting == 0: FAIL — B was admitted before A's upstream closed (the bug).
//
// The guarantee comes from channelLimiterStream.Close tearing down the upstream first
// and releasing the slot afterwards (`defer s.release(); return s.Stream.Close()`).
func TestChannelLimiterMiddleware_StreamCloseDoesNotAdmitWaiterBeforeUpstreamClosed(t *testing.T) {
	t.Parallel()

	ch := channelWithLimit(11, "stream-handoff", 1, 1) // bounded queue: capacity 1, queue 1
	mgr := NewChannelLimiterManager()
	out := newTestOutbound(ch)
	m := withChannelLimiter(out, mgr, nil).(*channelLimiterMiddleware)
	lim := mgr.GetOrCreate(ch)

	// Request A takes the only slot and establishes its (recorded) stream via the
	// real middleware path, so the channelLimiterStream under test is the production one.
	_, err := m.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	recorder := &hardModeStreamRecorder{lim: lim}

	wrapped, err := m.OnOutboundLlmStream(t.Context(), recorder)
	require.NoError(t, err)

	inFlight, _ := lim.Stats()
	require.Equal(t, 1, inFlight, "request A must hold the only slot")

	// Waiter B parks in the queue behind A.
	bCtx, cancelB := context.WithCancel(t.Context())
	defer cancelB()

	bAcquired := make(chan error, 1)
	go func() { bAcquired <- lim.Acquire(bCtx) }()

	require.Eventually(t, func() bool {
		_, waiting := lim.Stats()
		return waiting == 1
	}, time.Second, 5*time.Millisecond, "waiter B must enqueue before A closes")

	// Close A's stream. This is the handoff: the slot must not be transferred to B
	// until A's upstream stream has actually been closed.
	require.NoError(t, wrapped.Close())

	require.True(t, recorder.closeCalled, "upstream stream Close must run")
	assert.Equal(t, 1, recorder.waitingAtClose,
		"queued waiter must still be parked when A's upstream is closed; waiting==0 means "+
			"the slot was handed to B (which then dials a new upstream connection) BEFORE A's "+
			"connection was torn down — the release-before-close over-admission window")

	// Sanity/cleanup: B is admitted once A's slot is released, then drains cleanly.
	select {
	case err := <-bAcquired:
		require.NoError(t, err, "waiter B must be admitted after A's slot is released")
	case <-time.After(time.Second):
		t.Fatal("waiter B was never admitted after A closed")
	}

	lim.Release() // release B's transferred slot

	inFlight, waiting := lim.Stats()
	assert.Equal(t, 0, inFlight, "no slot leak after handoff")
	assert.Equal(t, 0, waiting, "no waiter leak after handoff")
}
