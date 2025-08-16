package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestFakeTransformer_CustomizeExecutor(t *testing.T) {
	fake := NewFakeTransformer()
	executor := fake.CustomizeExecutor(nil)

	assert.NotNil(t, executor)
	assert.IsType(t, &fakeExecutor{}, executor)
}

func TestFakeExecutor_Do(t *testing.T) {
	executor := &fakeExecutor{}
	ctx := context.Background()
	req := &httpclient.Request{}

	resp, err := executor.Do(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify response structure
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Headers["Content-Type"][0])

	// Verify response body contains expected OpenAI structure
	var responseData map[string]interface{}

	err = json.Unmarshal(resp.Body, &responseData)
	require.NoError(t, err)

	// Check for OpenAI response structure
	assert.Contains(t, responseData, "id")
	assert.Contains(t, responseData, "model")
	assert.Contains(t, responseData, "object")
	assert.Contains(t, responseData, "choices")
	assert.Equal(t, "chat.completion", responseData["object"])
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
	assert.NoError(t, stream.Err())

	// Verify we have events
	assert.Greater(t, len(events), 0)

	// Verify first event structure
	firstEvent := events[0]
	assert.NotNil(t, firstEvent.Data)

	// Parse the first event data to verify it's valid OpenAI chunk format
	var chunkData map[string]interface{}

	err = json.Unmarshal(firstEvent.Data, &chunkData)
	require.NoError(t, err)

	// Check for OpenAI chunk structure
	assert.Contains(t, chunkData, "id")
	assert.Contains(t, chunkData, "model")
	assert.Contains(t, chunkData, "object")
	assert.Contains(t, chunkData, "choices")
	assert.Equal(t, "chat.completion.chunk", chunkData["object"])

	// Close the stream
	stream.Close()
}
