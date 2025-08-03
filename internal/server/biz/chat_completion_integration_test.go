package biz

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestChatCompletionProcessor_Integration(t *testing.T) {
	// Create a simple processor with real transformers but mock services
	processor := &ChatCompletionProcessor{
		Inbound: openai.NewInboundTransformer(),
		// Note: We're not testing the full Process method here since it requires
		// database connections and external services. This is just testing the
		// basic request conversion functionality.
	}

	// Test data - valid OpenAI format request
	requestBody := `{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Hello, how are you?"}
		],
		"stream": false,
		"temperature": 0.7
	}`

	req, err := http.NewRequest("POST", "http://localhost:8080/v1/chat/completions", bytes.NewBufferString(requestBody))
	assert.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	// Test convertToGenericRequest
	genericReq, err := httpclient.ReadHTTPRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, genericReq)

	// Test that the inbound transformer can parse the request
	ctx := context.Background()
	chatReq, err := processor.Inbound.TransformRequest(ctx, genericReq)
	assert.NoError(t, err)
	assert.NotNil(t, chatReq)
	assert.Equal(t, "gpt-4", chatReq.Model)
	assert.Len(t, chatReq.Messages, 1)
	assert.Equal(t, "user", chatReq.Messages[0].Role)
	assert.NotNil(t, chatReq.Stream)
	assert.False(t, *chatReq.Stream)
	assert.NotNil(t, chatReq.Temperature)
	assert.Equal(t, 0.7, *chatReq.Temperature)
}

func TestChatCompletionProcessor_NewConstructor(t *testing.T) {
	// Mock services (in real usage these would be properly initialized)
	var channelService *ChannelService
	var requestService *RequestService
	var httpClient httpclient.HttpClient

	// Test that the constructor creates a processor with the right components
	processor := NewChatCompletionProcessor(channelService, requestService, httpClient, openai.NewInboundTransformer())

	assert.NotNil(t, processor)
	assert.Equal(t, channelService, processor.ChannelService)
	assert.Equal(t, requestService, processor.RequestService)
	assert.Equal(t, httpClient, processor.HttpClient)
	assert.NotNil(t, processor.Inbound)
}
