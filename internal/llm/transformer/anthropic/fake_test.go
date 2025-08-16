package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

func TestFakeTransformer_CustomizeExecutor(t *testing.T) {
	fake := NewFakeTransformer()

	// Verify it implements ChannelCustomizedExecutor
	var _ pipeline.ChannelCustomizedExecutor = fake

	// Create a mock original executor
	originalExecutor := &mockExecutor{}

	// Get the customized executor
	customExecutor := fake.CustomizeExecutor(originalExecutor)

	// Verify it returns a different executor (the fake one)
	assert.NotEqual(t, originalExecutor, customExecutor)
	assert.IsType(t, &fakeExecutor{}, customExecutor)
}

func TestFakeExecutor_Do(t *testing.T) {
	ctx := context.Background()
	executor := &fakeExecutor{}

	// Create a test request
	request := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.anthropic.com/v1/messages",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"claude-3-sonnet-20240229","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}`),
	}

	// Execute the request
	response, err := executor.Do(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response properties
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "application/json", response.Headers.Get("Content-Type"))
	assert.NotEmpty(t, response.Body)

	// Verify response body is valid JSON and contains expected structure
	var responseData map[string]interface{}

	err = json.Unmarshal(response.Body, &responseData)
	require.NoError(t, err)

	// Check for expected fields in the response
	assert.Contains(t, responseData, "id")
	assert.Contains(t, responseData, "type")
	assert.Contains(t, responseData, "role")
	assert.Contains(t, responseData, "content")
	assert.Contains(t, responseData, "model")
	assert.Equal(t, "message", responseData["type"])
	assert.Equal(t, "assistant", responseData["role"])
}

func TestFakeExecutor_DoStream(t *testing.T) {
	ctx := context.Background()
	executor := &fakeExecutor{}

	// Create a test request
	request := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.anthropic.com/v1/messages",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"claude-3-sonnet-20240229","max_tokens":1024,"stream":true,"messages":[{"role":"user","content":"Hello"}]}`),
	}

	// Execute the streaming request
	stream, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Collect all events from the stream
	var events []*httpclient.StreamEvent
	for stream.Next() {
		events = append(events, stream.Current())
	}

	// Verify no errors occurred
	assert.NoError(t, stream.Err())
	assert.NoError(t, stream.Close())

	// Verify we got some events
	assert.NotEmpty(t, events)

	// Verify the first event is message_start
	assert.Equal(t, "message_start", events[0].Type)

	// Verify the last event is message_stop
	lastEvent := events[len(events)-1]
	assert.Equal(t, "message_stop", lastEvent.Type)

	// Verify we have content_block events
	hasContentBlock := false

	for _, event := range events {
		if event.Type == "content_block_start" || event.Type == "content_block_delta" {
			hasContentBlock = true
			break
		}
	}

	assert.True(t, hasContentBlock, "Should have content block events")
}

// mockExecutor for testing.
type mockExecutor struct{}

func (m *mockExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	return &httpclient.Response{StatusCode: 200}, nil
}

func (m *mockExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.SliceStream([]*httpclient.StreamEvent{}), nil
}
