package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestFakeTransformer_CustomizeExecutor(t *testing.T) {
	fake := NewFakeTransformer()
	executor := fake.CustomizeExecutor(nil)

	require.NotNil(t, executor)
	require.IsType(t, &fakeExecutor{}, executor)
}

func TestFakeExecutor_Do(t *testing.T) {
	executor := &fakeExecutor{}
	ctx := context.Background()
	req := &httpclient.Request{}

	resp, err := executor.Do(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify response structure
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "application/json", resp.Headers["Content-Type"][0])

	// Verify response body contains expected OpenAI structure
	var responseData map[string]interface{}

	err = json.Unmarshal(resp.Body, &responseData)
	require.NoError(t, err)

	// Check for OpenAI response structure
	require.Contains(t, responseData, "id")
	require.Contains(t, responseData, "model")
	require.Contains(t, responseData, "object")
	require.Contains(t, responseData, "choices")
	require.Equal(t, "chat.completion", responseData["object"])
}

func TestFakeExecutor_DoStream(t *testing.T) {
	executor := &fakeExecutor{}
	ctx := context.Background()
	req := &httpclient.Request{}

	stream, err := executor.DoStream(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Collect all events from the stream
	var events []*httpclient.StreamEvent

	for stream.Next() {
		event := stream.Current()
		events = append(events, event)
	}

	// Verify no error occurred during streaming
	require.NoError(t, stream.Err())

	// Verify we have events
	require.Greater(t, len(events), 0)

	// Verify first event structure
	firstEvent := events[0]
	require.NotNil(t, firstEvent.Data)

	// Parse the first event data to verify it's valid OpenAI chunk format
	var chunkData map[string]interface{}

	err = json.Unmarshal(firstEvent.Data, &chunkData)
	require.NoError(t, err)

	// Check for OpenAI chunk structure
	require.Contains(t, chunkData, "id")
	require.Contains(t, chunkData, "model")
	require.Contains(t, chunkData, "object")
	require.Contains(t, chunkData, "choices")
	require.Equal(t, "chat.completion.chunk", chunkData["object"])

	// Close the stream
	stream.Close()
}
