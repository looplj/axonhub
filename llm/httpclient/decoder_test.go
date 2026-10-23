package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultSSEDecoder_TerminalEventAtUnterminatedEOF pins the synthetic.new
// shape: the upstream ends the body with "data: [DONE]" and no trailing
// newline. The final field is never terminated, so go-sse's field parser never
// sees it as a field and the [DONE] event is lost. The terminated fixture
// (identical bytes plus "\n\n") must yield it, proving the fixture pair only
// differs in line termination.
func TestDefaultSSEDecoder_TerminalEventAtUnterminatedEOF(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		wantDone bool
	}{
		{
			name:     "unterminated final line drops [DONE]",
			fixture:  "synthetic_upstream_unterminated_final_line.raw",
			wantDone: true,
		},
		{
			name:     "terminated final line yields [DONE]",
			fixture:  "synthetic_upstream_terminated_final_line.raw",
			wantDone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.fixture))
			require.NoError(t, err)

			decoder := NewDefaultSSEDecoder(t.Context(), newMockReadCloser(data))
			t.Cleanup(func() { _ = decoder.Close() })

			var last []byte
			events := 0
			for decoder.Next() {
				events++
				last = decoder.Current().Data
			}
			require.NoError(t, decoder.Err())

			t.Logf("fixture=%s events=%d last=%q", tt.fixture, events, last)
			require.Equal(t, tt.wantDone, bytes.Equal(last, []byte("[DONE]")),
				"last event data %q", last)
		})
	}
}

// TestDefaultSSEDecoder_UnterminatedDoneOnlyYieldsDone verifies the minimal
// synthetic.new shape: a body consisting solely of "data: [DONE]" with no
// trailing newline still yields a terminal [DONE] event.
func TestDefaultSSEDecoder_UnterminatedDoneOnlyYieldsDone(t *testing.T) {
	decoder := NewDefaultSSEDecoder(t.Context(), newMockReadCloser([]byte("data: [DONE]")))
	t.Cleanup(func() { _ = decoder.Close() })

	var last []byte
	events := 0
	for decoder.Next() {
		events++
		last = decoder.Current().Data
	}
	require.NoError(t, decoder.Err())
	require.Equal(t, 1, events)
	require.Equal(t, "[DONE]", string(last))
}

// TestDefaultSSEDecoder_TruncatedMidJSONDoesNotFabricateTerminal verifies the
// negative control: a body truncated mid-JSON (no complete final field) must
// not synthesize a terminal [DONE] event. The partial line, once
// newline-terminated at EOF, may surface as a truncated data event, but it must
// never be rewritten into a [DONE] terminal.
func TestDefaultSSEDecoder_TruncatedMidJSONDoesNotFabricateTerminal(t *testing.T) {
	body := []byte("data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"index\":0,\"delta\":{\"con")
	decoder := NewDefaultSSEDecoder(t.Context(), newMockReadCloser(body))
	t.Cleanup(func() { _ = decoder.Close() })

	var sawDone bool
	for decoder.Next() {
		if string(decoder.Current().Data) == "[DONE]" {
			sawDone = true
		}
	}
	require.False(t, sawDone, "truncated JSON must not fabricate a [DONE] terminal")
}

// eofWithDataReader returns its complete payload and io.EOF in a single Read.
type eofWithDataReader struct {
	data []byte
	done bool
}

func (r *eofWithDataReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), io.EOF
}
func (*eofWithDataReader) Close() error { return nil }

func TestNewlineTerminatingReaderEOFWithData(t *testing.T) {
	body := []byte("data: [DONE]")
	reader := newNewlineTerminatingReader(&eofWithDataReader{data: body})

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, append(bytes.Clone(body), '\n'), got)
	buf := make([]byte, 1)
	n, err := reader.Read(buf)
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF)
}

func TestNewlineTerminatingReaderLoneCarriageReturn(t *testing.T) {
	body := []byte("data: [DONE]\r")
	reader := newNewlineTerminatingReader(newMockReadCloser(body))

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, body, got)
}

func TestNewlineTerminatingReaderZeroLengthRead(t *testing.T) {
	body := []byte("data: [DONE]")
	reader := newNewlineTerminatingReader(newMockReadCloser(body))

	n, err := reader.Read(nil)
	require.NoError(t, err)
	require.Zero(t, n)

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, append(bytes.Clone(body), '\n'), got)

	emptyReader := newNewlineTerminatingReader(newMockReadCloser(nil))
	n, err = emptyReader.Read(nil)
	require.NoError(t, err)
	require.Zero(t, n)
	got, err = io.ReadAll(emptyReader)
	require.NoError(t, err)
	require.Empty(t, got, "empty body must not synthesize a newline")
}

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

type readChunkThenError struct {
	data []byte
	err  error
	read bool
}

func (r *readChunkThenError) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}

	r.read = true
	n := copy(p, r.data)

	return n, r.err
}

func (r *readChunkThenError) Close() error {
	return nil
}

func TestRegisterDecoder(t *testing.T) {
	// Save original state
	originalDecoders := make(map[string]StreamDecoderFactory)
	maps.Copy(originalDecoders, globalRegistry.decoders)

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

func TestDefaultSSEDecoder_LastEventAtEOFDoesNotPanicOnRepeatedNext(t *testing.T) {
	decoder := NewDefaultSSEDecoder(t.Context(), newMockReadCloser([]byte("data: trailing-event\n")))

	require.True(t, decoder.Next())
	require.Equal(t, []byte("trailing-event"), decoder.Current().Data)

	for range 3 {
		require.NotPanics(t, func() {
			require.False(t, decoder.Next())
		})
		require.NoError(t, decoder.Err())
	}
}

func TestDefaultSSEDecoder_RejectsOversizedEventAtParserBoundary(t *testing.T) {
	const limit = 1024
	raw := "data: " + strings.Repeat("x", limit+1) + "\n\n"
	decoder := NewSSEDecoderWithMaxEventSize(t.Context(), newMockReadCloser([]byte(raw)), limit)

	require.False(t, decoder.Next())
	require.Nil(t, decoder.Current())
	require.ErrorIs(t, decoder.Err(), ErrStreamEventTooLarge)
}

func TestDefaultSSEDecoder_DoesNotClassifyErrorTextAsOversizedEvent(t *testing.T) {
	wantErr := errors.New("upstream token too long due proxy")
	decoder := NewSSEDecoderWithMaxEventSize(t.Context(), &readChunkThenError{err: wantErr}, 1024)

	require.False(t, decoder.Next())
	require.ErrorIs(t, decoder.Err(), wantErr)
	require.NotErrorIs(t, decoder.Err(), ErrStreamEventTooLarge)
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

func TestDefaultSSEDecoder_NextAfterClose(t *testing.T) {
	// Create a simple SSE stream
	sseData := "data: {\"type\": \"test\", \"message\": \"hello\"}\n\n"
	rc := newMockReadCloser([]byte(sseData))

	ctx := context.Background()
	decoder := NewDefaultSSEDecoder(ctx, rc)

	err := decoder.Close()
	require.NoError(t, err)
	require.True(t, rc.closed)

	hasNext := decoder.Next()
	require.False(t, hasNext)
	require.NoError(t, decoder.Err())
}

func TestBinaryChunkDecoder(t *testing.T) {
	ctx := context.Background()
	rc := newMockReadCloser([]byte("abcdefgh"))
	decoder := NewBinaryChunkDecoder("audio/mpeg")(ctx, rc)

	require.True(t, decoder.Next())
	first := decoder.Current()
	require.NotNil(t, first)
	require.Equal(t, "audio/mpeg", first.Type)
	require.Equal(t, []byte("abcdefgh"), first.Data)

	require.True(t, decoder.Next())
	done := decoder.Current()
	require.NotNil(t, done)
	require.Equal(t, BinaryStreamDoneEventType, done.Type)
	require.Empty(t, done.Data)

	require.False(t, decoder.Next())
	require.NoError(t, decoder.Err())
	require.NoError(t, decoder.Close())
	require.True(t, rc.closed)
}

func TestBinaryChunkDecoder_PreservesReadErrorAfterChunk(t *testing.T) {
	ctx := context.Background()
	streamErr := errors.New("truncated audio stream")
	decoder := NewBinaryChunkDecoder("audio/mpeg")(ctx, &readChunkThenError{
		data: []byte("abc"),
		err:  streamErr,
	})

	require.True(t, decoder.Next())
	chunk := decoder.Current()
	require.NotNil(t, chunk)
	require.Equal(t, "audio/mpeg", chunk.Type)
	require.Equal(t, []byte("abc"), chunk.Data)
	require.NoError(t, decoder.Err())

	require.False(t, decoder.Next())
	require.ErrorIs(t, decoder.Err(), streamErr)
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
