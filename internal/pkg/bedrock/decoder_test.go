package bedrock

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream/eventstreamapi"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// mockReadCloser implements io.ReadCloser for testing.
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

// createEventStreamMessage creates a test AWS EventStream message.
func createEventStreamMessage(messageType, eventType string, payload []byte) []byte {
	headers := []eventstream.Header{
		{
			Name:  eventstreamapi.MessageTypeHeader,
			Value: eventstream.StringValue(messageType),
		},
	}

	if eventType != "" {
		headers = append(headers, eventstream.Header{
			Name:  eventstreamapi.EventTypeHeader,
			Value: eventstream.StringValue(eventType),
		})
	}

	msg := eventstream.Message{
		Headers: eventstream.Headers(headers),
		Payload: payload,
	}

	var buf bytes.Buffer

	encoder := eventstream.NewEncoder()

	err := encoder.Encode(&buf, msg)
	if err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// createChunkPayload creates a test chunk payload.
func createChunkPayload(data map[string]interface{}) []byte {
	// Create the inner data
	innerData, _ := json.Marshal(data)

	// Base64 encode it
	encodedData := base64.StdEncoding.EncodeToString(innerData)

	// Create the chunk structure
	chunk := eventstreamChunk{
		Bytes: encodedData,
		P:     "test",
	}

	payload, _ := json.Marshal(chunk)

	return payload
}

func TestNewAWSEventStreamDecoder(t *testing.T) {
	ctx := context.Background()
	rc := newMockReadCloser([]byte{})
	decoder := NewAWSEventStreamDecoder(ctx, rc)

	require.NotNil(t, decoder)
	require.Implements(t, (*httpclient.StreamDecoder)(nil), decoder)
}

func TestAWSEventStreamDecoder_EventMessage(t *testing.T) {
	// Create test data
	testData := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":      "test-id",
			"content": "Hello, World!",
		},
	}

	// Create chunk payload
	chunkPayload := createChunkPayload(testData)

	// Create EventStream message
	msgData := createEventStreamMessage(eventstreamapi.EventMessageType, "chunk", chunkPayload)

	// Create decoder
	ctx := context.Background()
	rc := newMockReadCloser(msgData)
	decoder := NewAWSEventStreamDecoder(ctx, rc).(*AWSEventStreamDecoder)

	// Test Next()
	hasNext := decoder.Next()
	require.True(t, hasNext)
	require.NoError(t, decoder.Err())

	// Test Current()
	event := decoder.Current()
	require.NotNil(t, event)
	require.Equal(t, "message_start", event.Type)
	require.Contains(t, string(event.Data), "Hello, World!")

	// Test Close()
	err := decoder.Close()
	require.NoError(t, err)
	require.True(t, rc.closed)
}

func TestAWSEventStreamDecoder_ExceptionMessage(t *testing.T) {
	// Create exception payload
	exceptionPayload := map[string]interface{}{
		"__type":  "ValidationException",
		"message": "Invalid input parameter",
	}
	payload, _ := json.Marshal(exceptionPayload)

	// Create EventStream message with exception
	headers := []eventstream.Header{
		{
			Name:  eventstreamapi.MessageTypeHeader,
			Value: eventstream.StringValue(eventstreamapi.ExceptionMessageType),
		},
		{
			Name:  eventstreamapi.ExceptionTypeHeader,
			Value: eventstream.StringValue("ValidationException"),
		},
	}

	msg := eventstream.Message{
		Headers: eventstream.Headers(headers),
		Payload: payload,
	}

	var buf bytes.Buffer

	encoder := eventstream.NewEncoder()
	err := encoder.Encode(&buf, msg)
	require.NoError(t, err)

	// Create decoder
	ctx := context.Background()
	rc := newMockReadCloser(buf.Bytes())
	decoder := NewAWSEventStreamDecoder(ctx, rc).(*AWSEventStreamDecoder)

	// Test Next() - should return false due to exception
	hasNext := decoder.Next()
	require.False(t, hasNext)
	require.Error(t, decoder.Err())
	require.Contains(t, decoder.Err().Error(), "ValidationException")
	require.Contains(t, decoder.Err().Error(), "Invalid input parameter")
}

func TestAWSEventStreamDecoder_ErrorMessage(t *testing.T) {
	ctx := context.Background()
	// Create EventStream error message
	headers := []eventstream.Header{
		{
			Name:  eventstreamapi.MessageTypeHeader,
			Value: eventstream.StringValue(eventstreamapi.ErrorMessageType),
		},
		{
			Name:  eventstreamapi.ErrorCodeHeader,
			Value: eventstream.StringValue("InternalError"),
		},
		{
			Name:  eventstreamapi.ErrorMessageHeader,
			Value: eventstream.StringValue("Internal server error occurred"),
		},
	}

	msg := eventstream.Message{
		Headers: eventstream.Headers(headers),
		Payload: []byte{},
	}

	var buf bytes.Buffer

	encoder := eventstream.NewEncoder()
	err := encoder.Encode(&buf, msg)
	require.NoError(t, err)

	// Create decoder
	rc := newMockReadCloser(buf.Bytes())
	decoder := NewAWSEventStreamDecoder(ctx, rc).(*AWSEventStreamDecoder)

	// Test Next() - should return false due to error
	hasNext := decoder.Next()
	require.False(t, hasNext)
	require.Error(t, decoder.Err())
	require.Contains(t, decoder.Err().Error(), "InternalError")
	require.Contains(t, decoder.Err().Error(), "Internal server error occurred")
}

func TestAWSEventStreamDecoder_InvalidMessage(t *testing.T) {
	// Create invalid message without required headers
	msg := eventstream.Message{
		Headers: eventstream.Headers{},
		Payload: []byte{},
	}

	var buf bytes.Buffer

	encoder := eventstream.NewEncoder()
	err := encoder.Encode(&buf, msg)
	require.NoError(t, err)

	// Create decoder
	ctx := context.Background()
	rc := newMockReadCloser(buf.Bytes())
	decoder := NewAWSEventStreamDecoder(ctx, rc).(*AWSEventStreamDecoder)

	// Test Next() - should return false due to missing headers
	hasNext := decoder.Next()
	require.False(t, hasNext)
	require.Error(t, decoder.Err())
	require.Contains(t, decoder.Err().Error(), "event header not present")
}

func TestAWSEventStreamDecoder_InvalidChunkData(t *testing.T) {
	// Create invalid chunk payload (not valid JSON)
	invalidPayload := []byte("invalid json")

	// Create EventStream message
	msgData := createEventStreamMessage(eventstreamapi.EventMessageType, "chunk", invalidPayload)

	// Create decoder
	ctx := context.Background()
	rc := newMockReadCloser(msgData)
	decoder := NewAWSEventStreamDecoder(ctx, rc).(*AWSEventStreamDecoder)

	// Test Next() - should return false due to invalid JSON
	hasNext := decoder.Next()
	require.False(t, hasNext)
	require.Error(t, decoder.Err())
}

func TestAWSEventStreamDecoder_InvalidBase64(t *testing.T) {
	ctx := context.Background()
	// Create chunk with invalid base64 data
	chunk := eventstreamChunk{
		Bytes: "invalid-base64!",
		P:     "test",
	}
	payload, _ := json.Marshal(chunk)

	// Create EventStream message
	msgData := createEventStreamMessage(eventstreamapi.EventMessageType, "chunk", payload)

	// Create decoder
	rc := newMockReadCloser(msgData)
	decoder := NewAWSEventStreamDecoder(ctx, rc).(*AWSEventStreamDecoder)

	// Test Next() - should return false due to invalid base64
	hasNext := decoder.Next()
	require.False(t, hasNext)
	require.Error(t, decoder.Err())
}

func TestAWSEventStreamDecoder_MultipleEvents(t *testing.T) {
	ctx := context.Background()
	// Create multiple test events
	event1Data := map[string]interface{}{
		"type":    "message_start",
		"message": map[string]interface{}{"id": "1"},
	}
	event2Data := map[string]interface{}{
		"type":  "content_block_delta",
		"delta": map[string]interface{}{"text": "Hello"},
	}

	// Create chunk payloads
	chunk1Payload := createChunkPayload(event1Data)
	chunk2Payload := createChunkPayload(event2Data)

	// Create EventStream messages
	msg1Data := createEventStreamMessage(eventstreamapi.EventMessageType, "chunk", chunk1Payload)
	msg2Data := createEventStreamMessage(eventstreamapi.EventMessageType, "chunk", chunk2Payload)

	// Combine messages
	combinedData := append(msg1Data, msg2Data...)

	// Create decoder
	rc := newMockReadCloser(combinedData)
	decoder := NewAWSEventStreamDecoder(ctx, rc).(*AWSEventStreamDecoder)

	// Test first event
	hasNext := decoder.Next()
	require.True(t, hasNext)
	require.NoError(t, decoder.Err())

	event1 := decoder.Current()
	require.NotNil(t, event1)
	require.Equal(t, "message_start", event1.Type)

	// Test second event
	hasNext = decoder.Next()
	require.True(t, hasNext)
	require.NoError(t, decoder.Err())

	event2 := decoder.Current()
	require.NotNil(t, event2)
	require.Equal(t, "content_block_delta", event2.Type)

	// Test no more events
	hasNext = decoder.Next()
	require.False(t, hasNext)
}
