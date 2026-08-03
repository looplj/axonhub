package httpclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStreamEvent_IsBinaryAudioChunk(t *testing.T) {
	require.True(t, (&StreamEvent{Type: "audio/mpeg"}).IsBinaryAudioChunk())
	require.True(t, (&StreamEvent{Type: "application/octet-stream"}).IsBinaryAudioChunk())
	require.False(t, (&StreamEvent{Type: BinaryStreamDoneEventType}).IsBinaryAudioChunk())
	require.False(t, (&StreamEvent{Type: "text/event-stream"}).IsBinaryAudioChunk())
	require.False(t, (&StreamEvent{}).IsBinaryAudioChunk())

	var nilEvent *StreamEvent

	require.False(t, nilEvent.IsBinaryAudioChunk())
}

func TestSummarizeBinaryChunk(t *testing.T) {
	t.Run("audio chunk is summarized", func(t *testing.T) {
		orig := &StreamEvent{Type: "audio/mpeg", Data: []byte{0x01, 0x02, 0x03}}
		got := SummarizeBinaryChunk(orig)
		require.NotSame(t, orig, got)
		require.Equal(t, "audio/mpeg", got.Type)
		require.Empty(t, got.Data)
		require.Equal(t, 3, got.Size)
		// Original is left untouched so downstream consumers still get the payload.
		require.Equal(t, []byte{0x01, 0x02, 0x03}, orig.Data)
	})

	t.Run("non-binary chunk is returned as-is", func(t *testing.T) {
		orig := &StreamEvent{Type: "transcript.text.delta", Data: []byte(`{"delta":"hi"}`)}
		got := SummarizeBinaryChunk(orig)
		require.Same(t, orig, got)
	})

	t.Run("binary done sentinel is returned as-is", func(t *testing.T) {
		orig := &StreamEvent{Type: BinaryStreamDoneEventType}
		got := SummarizeBinaryChunk(orig)
		require.Same(t, orig, got)
	})

	t.Run("nil is returned as-is", func(t *testing.T) {
		require.Nil(t, SummarizeBinaryChunk(nil))
	})
}
