package httpclient

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockStreamDecoder implements StreamDecoder for testing.
type mockStreamDecoder struct {
	rc     io.ReadCloser
	events []*StreamEvent
	index  int
	err    error
	closed bool
}

func newMockStreamDecoder(ctx context.Context, rc io.ReadCloser, events []*StreamEvent) *mockStreamDecoder {
	return &mockStreamDecoder{
		rc:     rc,
		events: events,
		index:  -1,
	}
}

func (m *mockStreamDecoder) Next() bool {
	if m.err != nil {
		return false
	}

	m.index++

	return m.index < len(m.events)
}

func (m *mockStreamDecoder) Current() *StreamEvent {
	if m.index < 0 || m.index >= len(m.events) {
		return nil
	}

	return m.events[m.index]
}

func (m *mockStreamDecoder) Err() error {
	return m.err
}

func (m *mockStreamDecoder) Close() error {
	m.closed = true
	return m.rc.Close()
}

// mockReadCloser for testing.
type mockReadCloser struct {
	*bytes.Reader

	closed bool
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func newMockReadCloser(data []byte) *mockReadCloser {
	return &mockReadCloser{
		Reader: bytes.NewReader(data),
		closed: false,
	}
}

func TestRegisterDecoder(t *testing.T) {
	// Save original state
	originalDecoders := make(map[string]StreamDecoderFactory)
	for k, v := range globalRegistry.decoders {
		originalDecoders[k] = v
	}

	// Clean up after test
	defer func() {
		globalRegistry.mu.Lock()
		globalRegistry.decoders = originalDecoders
		globalRegistry.mu.Unlock()
	}()

	// Test registering a new decoder
	testContentType := "application/test"
	testFactory := func(ctx context.Context, rc io.ReadCloser) StreamDecoder {
		return newMockStreamDecoder(ctx, rc, []*StreamEvent{})
	}

	RegisterDecoder(testContentType, testFactory)

	// Verify decoder was registered
	factory, exists := GetDecoder(testContentType)
	require.True(t, exists)
	require.NotNil(t, factory)

	// Test that the factory works
	ctx := context.Background()
	rc := newMockReadCloser([]byte("test"))
	decoder := factory(ctx, rc)
	require.NotNil(t, decoder)
	require.Implements(t, (*StreamDecoder)(nil), decoder)
}

func TestGetDecoder(t *testing.T) {
	// Test getting existing decoder (text/event-stream should be registered by default)
	factory, exists := GetDecoder("text/event-stream")
	require.True(t, exists)
	require.NotNil(t, factory)

	// Test getting non-existent decoder
	factory, exists = GetDecoder("application/non-existent")
	require.False(t, exists)
	require.Nil(t, factory)
}

func TestDefaultSSEDecoder(t *testing.T) {
	// Create a simple SSE stream
	sseData := "data: {\"type\": \"test\", \"message\": \"hello\"}\n\n"
	rc := newMockReadCloser([]byte(sseData))

	// Create decoder
	ctx := context.Background()
	decoder := NewDefaultSSEDecoder(ctx, rc)
	require.NotNil(t, decoder)
	require.Implements(t, (*StreamDecoder)(nil), decoder)

	// Test Next() and Current()
	hasNext := decoder.Next()
	require.True(t, hasNext)
	require.NoError(t, decoder.Err())

	event := decoder.Current()
	require.NotNil(t, event)
	require.Equal(t, "", event.Type) // Default SSE type
	require.Contains(t, string(event.Data), "hello")

	// Test Close()
	err := decoder.Close()
	require.NoError(t, err)
	require.True(t, rc.closed)
}

func TestDefaultSSEDecoder_EmptyStream(t *testing.T) {
	ctx := context.Background()
	rc := newMockReadCloser([]byte(""))
	decoder := NewDefaultSSEDecoder(ctx, rc)

	// Should return false for empty stream
	hasNext := decoder.Next()
	require.False(t, hasNext)

	// Current should return nil
	event := decoder.Current()
	require.Nil(t, event)

	// Close should work
	err := decoder.Close()
	require.NoError(t, err)
}

func TestStreamDecoderInterface(t *testing.T) {
	ctx := context.Background()
	// Test that our mock decoder implements the interface correctly
	events := []*StreamEvent{
		{Type: "test1", Data: []byte("data1")},
		{Type: "test2", Data: []byte("data2")},
	}

	rc := newMockReadCloser([]byte("test"))
	decoder := newMockStreamDecoder(ctx, rc, events)

	// Test Next() and Current() for multiple events
	require.True(t, decoder.Next())
	event1 := decoder.Current()
	require.Equal(t, "test1", event1.Type)
	require.Equal(t, []byte("data1"), event1.Data)

	require.True(t, decoder.Next())
	event2 := decoder.Current()
	require.Equal(t, "test2", event2.Type)
	require.Equal(t, []byte("data2"), event2.Data)

	// No more events
	require.False(t, decoder.Next())
	require.Nil(t, decoder.Current())

	// Test error handling
	require.NoError(t, decoder.Err())

	// Test close
	err := decoder.Close()
	require.NoError(t, err)
	require.True(t, decoder.closed)
	require.True(t, rc.closed)
}
