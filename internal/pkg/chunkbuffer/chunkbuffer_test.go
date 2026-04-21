package chunkbuffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestBuffer_Append(t *testing.T) {
	buffer := New()

	chunk1 := &httpclient.StreamEvent{Type: "test", Data: []byte("data1")}
	chunk2 := &httpclient.StreamEvent{Type: "test", Data: []byte("data2")}

	assert.True(t, buffer.Append(chunk1))
	assert.True(t, buffer.Append(chunk2))
	assert.Equal(t, 2, buffer.Len())

	assert.False(t, buffer.Append(nil))
	assert.Equal(t, 2, buffer.Len())
}

func TestBuffer_Slice(t *testing.T) {
	buffer := New()

	chunk1 := &httpclient.StreamEvent{Type: "test", Data: []byte("data1")}
	chunk2 := &httpclient.StreamEvent{Type: "test", Data: []byte("data2")}

	buffer.Append(chunk1)
	buffer.Append(chunk2)

	slice := buffer.Slice()
	assert.Len(t, slice, 2)
	assert.Equal(t, chunk1, slice[0])
	assert.Equal(t, chunk2, slice[1])

	slice[0] = nil
	assert.NotNil(t, buffer.Slice()[0])
}

func TestBuffer_Close(t *testing.T) {
	buffer := New()

	assert.False(t, buffer.IsClosed())

	chunk := &httpclient.StreamEvent{Type: "test", Data: []byte("data")}
	assert.True(t, buffer.Append(chunk))

	buffer.Close()
	assert.True(t, buffer.IsClosed())

	assert.False(t, buffer.Append(&httpclient.StreamEvent{Type: "test2", Data: []byte("data2")}))
	assert.Equal(t, 1, buffer.Len())
}

func TestBuffer_ConcurrentAccess(t *testing.T) {
	buffer := New()

	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) {
			chunk := &httpclient.StreamEvent{
				Type: "test",
				Data: []byte{byte(n)},
			}
			buffer.Append(chunk)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	assert.Equal(t, 100, buffer.Len())
}

func TestBuffer_Len(t *testing.T) {
	buffer := New()

	assert.Equal(t, 0, buffer.Len())

	chunk := &httpclient.StreamEvent{Type: "test", Data: []byte("data")}
	buffer.Append(chunk)

	assert.Equal(t, 1, buffer.Len())
}

func TestBuffer_Read(t *testing.T) {
	buffer := New()

	chunk := &httpclient.StreamEvent{Type: "test", Data: []byte("data")}
	require.True(t, buffer.Append(chunk))

	got, nextIndex, closed, ok := buffer.Read(0)
	require.True(t, ok)
	require.False(t, closed)
	require.Equal(t, 1, nextIndex)
	require.Equal(t, chunk, got)

	buffer.Close()

	got, nextIndex, closed, ok = buffer.Read(1)
	require.False(t, ok)
	require.True(t, closed)
	require.Equal(t, 1, nextIndex)
	require.Nil(t, got)
}

func TestBuffer_SubscribeFromCurrent(t *testing.T) {
	buffer := New()

	require.True(t, buffer.Append(&httpclient.StreamEvent{Type: "test", Data: []byte("data1")}))

	notifyCh, replayUntil, unsubscribe := buffer.SubscribeFromCurrent()
	t.Cleanup(unsubscribe)

	require.Equal(t, 1, replayUntil)

	require.True(t, buffer.Append(&httpclient.StreamEvent{Type: "test", Data: []byte("data2")}))

	select {
	case <-notifyCh:
	default:
		t.Fatal("expected subscriber to be notified after append")
	}
}
