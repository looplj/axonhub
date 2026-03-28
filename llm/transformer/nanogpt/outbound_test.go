package nanogpt

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestNewOutboundTransformerWithConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				BaseURL:        "https://nano-gpt.com/api/v1",
				APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
			},
			wantErr: false,
		},
		{
			name: "nil API key provider",
			config: &Config{
				BaseURL:        "https://nano-gpt.com/api/v1",
				APIKeyProvider: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformerWithConfig(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, transformer)
			assert.IsType(t, &OutboundTransformer{}, transformer)
		})
	}
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://nano-gpt.com/api/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)
	require.NotNil(t, transformer)

	tests := []struct {
		name         string
		httpResp     *httpclient.Response
		expectedErr  bool
		validateResp func(*testing.T, *llm.Response)
	}{
		{
			name: "valid response with reasoning field",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "test-123",
					"object": "chat.completion",
					"created": 1234567890,
					"model": "zai-org/glm-4.7:thinking",
					"choices": [{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "Hello!",
							"reasoning": "Thinking process..."
						},
						"finish_reason": "stop"
					}]
				}`),
			},
			expectedErr: false,
			validateResp: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Len(t, resp.Choices, 1)
				require.NotNil(t, resp.Choices[0].Message.Content)
				assert.Equal(t, "Hello!", *resp.Choices[0].Message.Content.Content)
				require.NotNil(t, resp.Choices[0].Message.ReasoningContent)
				assert.Equal(t, "Thinking process...", *resp.Choices[0].Message.ReasoningContent)
			},
		},
		{
			name: "valid response without reasoning field",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "test-456",
					"object": "chat.completion",
					"created": 1234567890,
					"model": "zai-org/glm-4.7",
					"choices": [{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "Hi there!"
						},
						"finish_reason": "stop"
					}]
				}`),
			},
			expectedErr: false,
			validateResp: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Len(t, resp.Choices, 1)
				require.NotNil(t, resp.Choices[0].Message.Content)
				assert.Equal(t, "Hi there!", *resp.Choices[0].Message.Content.Content)
				assert.Nil(t, resp.Choices[0].Message.ReasoningContent)
			},
		},
		{
			name:        "nil response",
			httpResp:    nil,
			expectedErr: true,
		},
		{
			name: "empty body",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte{},
			},
			expectedErr: true,
		},
		{
			name: "error status code",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusBadRequest,
				Body:       []byte(`{"error": {"message": "bad request"}}`),
			},
			expectedErr: true,
		},
		{
			name: "embedding request routes to embedded OpenAI transformer",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"object": "list",
					"data": [{
						"object": "embedding",
						"embedding": [0.1, 0.2, 0.3],
						"index": 0
					}],
					"model": "text-embedding-3-small",
					"usage": {
						"prompt_tokens": 10,
						"total_tokens": 10
					}
				}`),
				Request: &httpclient.Request{
					APIFormat: string(llm.APIFormatOpenAIEmbedding),
				},
			},
			expectedErr: false,
			validateResp: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.NotNil(t, resp.Embedding)
				require.Len(t, resp.Embedding.Data, 1)
				require.NotNil(t, resp.Embedding.Data[0].Embedding)
				assert.Len(t, resp.Embedding.Data[0].Embedding.Embedding, 3)
			},
		},
		{
			name: "chat request uses NanoGPT-specific parsing",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "chat-123",
					"object": "chat.completion",
					"created": 1234567890,
					"model": "zai-org/glm-4.7",
					"choices": [{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "Hello from NanoGPT!"
						},
						"finish_reason": "stop"
					}]
				}`),
				Request: &httpclient.Request{
					APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				},
			},
			expectedErr: false,
			validateResp: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Len(t, resp.Choices, 1)
				require.NotNil(t, resp.Choices[0].Message.Content)
				assert.Equal(t, "Hello from NanoGPT!", *resp.Choices[0].Message.Content.Content)
			},
		},
		{
			name: "nil request in httpResp uses NanoGPT parsing",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "chat-456",
					"object": "chat.completion",
					"created": 1234567890,
					"model": "zai-org/glm-4.7",
					"choices": [{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "Response without request info"
						},
						"finish_reason": "stop"
					}]
				}`),
				Request: nil,
			},
			expectedErr: false,
			validateResp: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Len(t, resp.Choices, 1)
				require.NotNil(t, resp.Choices[0].Message.Content)
				assert.Equal(t, "Response without request info", *resp.Choices[0].Message.Content.Content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := transformer.TransformResponse(context.Background(), tt.httpResp)
			if tt.expectedErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestOutboundTransformer_AggregateStreamChunks(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://nano-gpt.com/api/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"test","choices":[{"index":0,"delta":{"content":"Hello"}}]}`)},
		{Data: []byte(`{"id":"test","choices":[{"index":0,"delta":{"content":" World"}}]}`)},
	}

	data, meta, err := transformer.AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Hello")
	assert.Contains(t, string(data), "World")
	assert.NotNil(t, meta)
}
