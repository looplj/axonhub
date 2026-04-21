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
	m.closed = true
	return m.err
}

func (m *mockResponseStream) Err() error {
	return nil
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
	ms := &mockResponseStream{items: []*llm.Response{{}}, err: expectedErr}
	s := &onCloseStream{
		stream:  ms,
		onClose: func() {},
	}

	err := s.Close()
	assert.ErrorIs(t, err, expectedErr, "Close should return the underlying stream error")
}
