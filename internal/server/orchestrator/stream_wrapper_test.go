package orchestrator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

type mockResponseStream struct {
	items      []*llm.Response
	idx        int
	err        error
	closeErr   error
	closed     bool
	closeCount int32
}

type mockResponseStreamWithErr struct {
	items  []*llm.Response
	idx    int
	err    error
	closed bool
}

func (m *mockResponseStream) Current() *llm.Response {
	if m.idx < len(m.items) {
		item := m.items[m.idx]
		m.idx++
		return item
	}

	return nil
}

func (m *mockResponseStream) Next() bool {
	return m.idx < len(m.items)
}

func (m *mockResponseStream) Close() error {
	atomic.AddInt32(&m.closeCount, 1)
	m.closed = true
	return m.closeErr
}

func (m *mockResponseStream) Err() error {
	return nil
}

func (m *mockResponseStreamWithErr) Current() *llm.Response {
	if m.idx < len(m.items) {
		item := m.items[m.idx]
		m.idx++
		return item
	}
	return nil
}

func (m *mockResponseStreamWithErr) Next() bool {
	return m.idx < len(m.items)
}

func (m *mockResponseStreamWithErr) Close() error {
	m.closed = true
	return nil
}

func (m *mockResponseStreamWithErr) Err() error {
	return m.err
}

func TestOnCloseStream_ExhaustionTriggersOnClose(t *testing.T) {
	var onCloseCount int32

	s := newOnCloseStream(&mockResponseStream{items: []*llm.Response{{}}}, func() {
		atomic.AddInt32(&onCloseCount, 1)
	})

	assert.True(t, s.Next())
	_ = s.Current()
	assert.False(t, s.Next())
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should fire once when stream is exhausted")
}

func TestOnCloseStream_ExplicitCloseTriggersOnClose(t *testing.T) {
	var onCloseCount int32

	s := newOnCloseStream(&mockResponseStream{items: []*llm.Response{{}, {}}}, func() {
		atomic.AddInt32(&onCloseCount, 1)
	})

	err := s.Close()
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should fire once on explicit Close")
}

func TestOnCloseStream_CloseThenNextDoesNotDoubleFire(t *testing.T) {
	var onCloseCount int32

	s := newOnCloseStream(&mockResponseStream{items: []*llm.Response{{}}}, func() {
		atomic.AddInt32(&onCloseCount, 1)
	})

	s.Next()
	_ = s.Current()
	s.Close()
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should not double-fire after Close following Next")
}

func TestOnCloseStream_NextExhaustionThenCloseDoesNotDoubleFire(t *testing.T) {
	var onCloseCount int32

	s := newOnCloseStream(&mockResponseStream{items: []*llm.Response{{}}}, func() {
		atomic.AddInt32(&onCloseCount, 1)
	})

	s.Next()
	_ = s.Current()
	streamExhausted := !s.Next()
	assert.True(t, streamExhausted, "stream should be exhausted")
	s.Close()
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should fire exactly once")
}

func TestOnCloseStream_CloseClosesUnderlyingStream(t *testing.T) {
	ms := &mockResponseStream{items: []*llm.Response{{}}}
	s := newOnCloseStream(ms, func() {})

	err := s.Close()
	require.NoError(t, err)
	assert.True(t, ms.closed, "underlying stream should be closed")
}

func TestOnCloseStream_CloseReturnsStreamError(t *testing.T) {
	expectedErr := errors.New("stream error")
	ms := &mockResponseStream{items: []*llm.Response{{}}, closeErr: expectedErr}
	s := newOnCloseStream(ms, func() {})

	err := s.Close()
	assert.ErrorIs(t, err, expectedErr, "Close should return the underlying stream error")
}

func TestOnCloseStream_ExhaustThenCloseDoesNotDoubleCloseUnderlyingStream(t *testing.T) {
	ms := &mockResponseStream{items: []*llm.Response{{}}}
	s := newOnCloseStream(ms, func() {})

	s.Next()
	_ = s.Current()
	streamExhausted := !s.Next()
	assert.True(t, streamExhausted)
	assert.True(t, ms.closed, "underlying stream should be closed after exhaustion")

	err := s.Close()
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&ms.closeCount), "underlying stream Close() should be called exactly once")
}

func TestOnCloseStream_NextExhaustWithStreamError(t *testing.T) {
	streamErr := errors.New("network error")
	ms := &mockResponseStreamWithErr{items: []*llm.Response{{}}, err: streamErr}
	var onCloseCount int32
	s := newOnCloseStream(ms, func() {
		atomic.AddInt32(&onCloseCount, 1)
	})

	s.Next()
	_ = s.Current()
	streamExhausted := !s.Next()
	assert.True(t, streamExhausted)
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should fire even when stream has an error")
	assert.ErrorIs(t, s.Err(), streamErr, "Err() should propagate the underlying stream error")
}

func TestOnCloseStream_ConcurrentCloseAndNext(t *testing.T) {
	var onCloseCount int32

	ms := &mockResponseStream{items: []*llm.Response{{}, {}, {}}}
	s := newOnCloseStream(ms, func() {
		atomic.AddInt32(&onCloseCount, 1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		for s.Next() {
			_ = s.Current()
		}
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-done:
		}
	}()

	<-done

	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should fire exactly once under concurrent access")
	assert.Equal(t, int32(1), atomic.LoadInt32(&ms.closeCount), "underlying stream Close should be called exactly once")
}
