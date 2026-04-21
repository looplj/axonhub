package orchestrator

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

type mockResponseStream struct {
	items       []*llm.Response
	idx         int
	err         error
	closeErr    error
	closed      bool
	closeCount  int32
}

type mockResponseStreamWithErr struct {
	items  []*llm.Response
	idx    int
	err    error
	closed bool
}

func (m *mockResponseStream) Current() *llm.Response {
	if m.idx > 0 && m.idx <= len(m.items) {
		return m.items[m.idx-1]
	}

	return nil
}

func (m *mockResponseStream) Next() bool {
	if m.idx >= len(m.items) {
		return false
	}

	m.idx++

	return true
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
	if m.idx > 0 && m.idx <= len(m.items) {
		return m.items[m.idx-1]
	}
	return nil
}

func (m *mockResponseStreamWithErr) Next() bool {
	if m.idx >= len(m.items) {
		return false
	}
	m.idx++
	return true
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

	s := &onCloseStream{
		stream: &mockResponseStream{items: []*llm.Response{{}}},
		onClose: func() {
			atomic.AddInt32(&onCloseCount, 1)
		},
	}

	assert.True(t, s.Next())
	assert.False(t, s.Next())
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should fire once when stream is exhausted")
}

func TestOnCloseStream_ExplicitCloseTriggersOnClose(t *testing.T) {
	var onCloseCount int32

	s := &onCloseStream{
		stream: &mockResponseStream{items: []*llm.Response{{}, {}}},
		onClose: func() {
			atomic.AddInt32(&onCloseCount, 1)
		},
	}

	err := s.Close()
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should fire once on explicit Close")
}

func TestOnCloseStream_CloseThenNextDoesNotDoubleFire(t *testing.T) {
	var onCloseCount int32

	s := &onCloseStream{
		stream: &mockResponseStream{items: []*llm.Response{{}}},
		onClose: func() {
			atomic.AddInt32(&onCloseCount, 1)
		},
	}

	s.Next()
	s.Close()
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should not double-fire after Close following Next")
}

func TestOnCloseStream_NextExhaustionThenCloseDoesNotDoubleFire(t *testing.T) {
	var onCloseCount int32

	s := &onCloseStream{
		stream: &mockResponseStream{items: []*llm.Response{{}}},
		onClose: func() {
			atomic.AddInt32(&onCloseCount, 1)
		},
	}

	s.Next()
	streamExhausted := !s.Next()
	assert.True(t, streamExhausted, "stream should be exhausted")
	s.Close()
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should fire exactly once")
}

func TestOnCloseStream_CloseClosesUnderlyingStream(t *testing.T) {
	ms := &mockResponseStream{items: []*llm.Response{{}}}
	s := &onCloseStream{
		stream:  ms,
		onClose: func() {},
	}

	err := s.Close()
	require.NoError(t, err)
	assert.True(t, ms.closed, "underlying stream should be closed")
}

func TestOnCloseStream_CloseReturnsStreamError(t *testing.T) {
	expectedErr := errors.New("stream error")
	ms := &mockResponseStream{items: []*llm.Response{{}}, closeErr: expectedErr}
	s := &onCloseStream{
		stream:  ms,
		onClose: func() {},
	}

	err := s.Close()
	assert.ErrorIs(t, err, expectedErr, "Close should return the underlying stream error")
}

func TestOnCloseStream_ExhaustThenCloseDoesNotDoubleCloseUnderlyingStream(t *testing.T) {
	ms := &mockResponseStream{items: []*llm.Response{{}}}
	s := &onCloseStream{
		stream:  ms,
		onClose: func() {},
	}

	s.Next()
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
	s := &onCloseStream{
		stream: ms,
		onClose: func() {
			atomic.AddInt32(&onCloseCount, 1)
		},
	}

	s.Next()
	streamExhausted := !s.Next()
	assert.True(t, streamExhausted)
	assert.Equal(t, int32(1), atomic.LoadInt32(&onCloseCount), "onClose should fire even when stream has an error")
	assert.ErrorIs(t, s.Err(), streamErr, "Err() should propagate the underlying stream error")
}
