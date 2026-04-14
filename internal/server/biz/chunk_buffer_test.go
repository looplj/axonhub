package biz

import (
	"testing"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/stretchr/testify/assert"
)

func TestChunkBuffer_Append(t *testing.T) {
	buffer := NewChunkBuffer()

	// Append chunks
	chunk1 := &httpclient.StreamEvent{Type: "test", Data: []byte("data1")}
	chunk2 := &httpclient.StreamEvent{Type: "test", Data: []byte("data2")}

	assert.True(t, buffer.Append(chunk1))
	assert.True(t, buffer.Append(chunk2))
	assert.Equal(t, 2, buffer.Len())

	// Nil chunk should be ignored
	assert.False(t, buffer.Append(nil))
	assert.Equal(t, 2, buffer.Len())
}

func TestChunkBuffer_Slice(t *testing.T) {
	buffer := NewChunkBuffer()

	chunk1 := &httpclient.StreamEvent{Type: "test", Data: []byte("data1")}
	chunk2 := &httpclient.StreamEvent{Type: "test", Data: []byte("data2")}

	buffer.Append(chunk1)
	buffer.Append(chunk2)

	slice := buffer.Slice()
	assert.Len(t, slice, 2)
	assert.Equal(t, chunk1, slice[0])
	assert.Equal(t, chunk2, slice[1])

	// Verify it's a copy
	slice[0] = nil
	assert.NotNil(t, buffer.Slice()[0])
}

func TestChunkBuffer_Close(t *testing.T) {
	buffer := NewChunkBuffer()

	assert.False(t, buffer.IsClosed())

	chunk := &httpclient.StreamEvent{Type: "test", Data: []byte("data")}
	assert.True(t, buffer.Append(chunk))

	buffer.Close()
	assert.True(t, buffer.IsClosed())

	// Appends should fail after close
	assert.False(t, buffer.Append(&httpclient.StreamEvent{Type: "test2", Data: []byte("data2")}))
	assert.Equal(t, 1, buffer.Len())
}

func TestChunkBuffer_ConcurrentAccess(t *testing.T) {
	buffer := NewChunkBuffer()

	// Simulate concurrent appends
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

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	assert.Equal(t, 100, buffer.Len())
}

func TestChunkBuffer_SnapshotLen(t *testing.T) {
	buffer := NewChunkBuffer()

	assert.Equal(t, 0, buffer.SnapshotLen())

	chunk := &httpclient.StreamEvent{Type: "test", Data: []byte("data")}
	buffer.Append(chunk)

	assert.Equal(t, 1, buffer.SnapshotLen())
}

func TestChunkBuffer_ChunksPointer(t *testing.T) {
	buffer := NewChunkBuffer()

	chunk := &httpclient.StreamEvent{Type: "test", Data: []byte("data")}
	buffer.Append(chunk)

	ptr := buffer.ChunksPointer()
	assert.NotNil(t, ptr)
	assert.Len(t, *ptr, 1)
}
