package pipeline_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// mockExecutor implements the Executor interface for testing.
type mockExecutor struct {
	doFunc       func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error)
	doStreamFunc func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error)
}

func (m *mockExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	if m.doFunc != nil {
		return m.doFunc(ctx, request)
	}

	return nil, nil
}

func (m *mockExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	if m.doStreamFunc != nil {
		return m.doStreamFunc(ctx, request)
	}

	return nil, nil
}

// TestPipeline_OpenAI_to_OpenAI tests the pipeline with OpenAI inbound and outbound transformers.
func TestPipeline_OpenAI_to_OpenAI(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Mock OpenAI response
	mockResponse := &llm.Response{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-4",
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello! How can I help you today?"),
					},
				},
				FinishReason: lo.ToPtr("stop"),
			},
		},
		Usage: &llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 8,
			TotalTokens:      18,
		},
	}

	// Create mock executor
	executor := &mockExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			// Verify request format
			require.Equal(t, http.MethodPost, request.Method)
			require.Contains(t, request.URL, "/chat/completions")

			// Verify auth configuration
			require.NotNil(t, request.Auth)
			require.Equal(t, "bearer", request.Auth.Type)
			require.Equal(t, "test-api-key", request.Auth.APIKey)

			responseBody, err := json.Marshal(mockResponse)
			require.NoError(t, err)

			return &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: responseBody,
			}, nil
		},
	}

	// Create pipeline
	factory := pipeline.NewFactory(executor)
	pipeline := factory.Pipeline(inbound, outbound)

	// Create test request (OpenAI format)
	requestBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, how are you?",
			},
		},
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	httpRequest := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: requestBodyBytes,
	}

	// Process the request
	result, err := pipeline.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)
	require.NotNil(t, result.Response)

	// Verify response
	require.Equal(t, http.StatusOK, result.Response.StatusCode)
	require.Equal(t, "application/json", result.Response.Headers.Get("Content-Type"))

	var finalResponse llm.Response

	err = json.Unmarshal(result.Response.Body, &finalResponse)
	require.NoError(t, err)
	require.Equal(t, "chatcmpl-123", finalResponse.ID)
	require.Equal(t, "gpt-4", finalResponse.Model)
	require.Equal(t, "Hello! How can I help you today?", *finalResponse.Choices[0].Message.Content.Content)
}

// TestPipeline_OpenAI_to_Anthropic tests the pipeline with OpenAI inbound and Anthropic outbound transformers.
func TestPipeline_OpenAI_to_Anthropic(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := anthropic.NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	// Mock Anthropic response
	mockAnthropicResponse := &anthropic.Message{
		ID:   "msg_123",
		Type: "message",
		Role: "assistant",
		Content: []anthropic.MessageContentBlock{
			{
				Type: "text",
				Text: "Hello! I'm Claude, how can I assist you today?",
			},
		},
		Model:      "claude-3-sonnet-20240229",
		StopReason: lo.ToPtr("end_turn"),
		Usage: &anthropic.Usage{
			InputTokens:  10,
			OutputTokens: 12,
		},
	}

	// Create mock executor
	executor := &mockExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			// Verify request format (should be Anthropic format)
			require.Equal(t, http.MethodPost, request.Method)
			require.Contains(t, request.URL, "/v1/messages")
			require.Equal(t, "2023-06-01", request.Headers.Get("Anthropic-Version"))

			// Verify auth configuration
			require.NotNil(t, request.Auth)
			require.Equal(t, "api_key", request.Auth.Type)
			require.Equal(t, "test-api-key", request.Auth.APIKey)
			require.Equal(t, "X-API-Key", request.Auth.HeaderKey)

			responseBody, err := json.Marshal(mockAnthropicResponse)
			require.NoError(t, err)

			return &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: responseBody,
			}, nil
		},
	}

	// Create pipeline
	factory := pipeline.NewFactory(executor)
	pipeline := factory.Pipeline(inbound, outbound)

	// Create test request (OpenAI format)
	requestBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, how are you?",
			},
		},
		"max_tokens": 1024,
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	httpRequest := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: requestBodyBytes,
	}

	// Process the request
	result, err := pipeline.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)
	require.NotNil(t, result.Response)

	// Verify response (should be in OpenAI format)
	require.Equal(t, http.StatusOK, result.Response.StatusCode)
	require.Equal(t, "application/json", result.Response.Headers.Get("Content-Type"))

	var finalResponse llm.Response

	err = json.Unmarshal(result.Response.Body, &finalResponse)
	require.NoError(t, err)
	require.Equal(t, "msg_123", finalResponse.ID)
	require.Equal(t, "claude-3-sonnet-20240229", finalResponse.Model)
	require.Equal(t, "Hello! I'm Claude, how can I assist you today?", *finalResponse.Choices[0].Message.Content.Content)
	require.Equal(t, "stop", *finalResponse.Choices[0].FinishReason)
}

// TestPipeline_Anthropic_to_OpenAI tests the pipeline with Anthropic inbound and OpenAI outbound transformers.
func TestPipeline_Anthropic_to_OpenAI(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := anthropic.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Mock OpenAI response
	mockOpenAIResponse := &llm.Response{
		ID:      "chatcmpl-456",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-4",
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello! I'm GPT-4, how can I help you?"),
					},
				},
				FinishReason: lo.ToPtr("stop"),
			},
		},
		Usage: &llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
		},
	}

	// Create mock executor
	executor := &mockExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			// Verify request format (should be OpenAI format)
			require.Equal(t, http.MethodPost, request.Method)
			require.Contains(t, request.URL, "/chat/completions")

			// Verify auth configuration
			require.NotNil(t, request.Auth)
			require.Equal(t, "bearer", request.Auth.Type)
			require.Equal(t, "test-api-key", request.Auth.APIKey)

			responseBody, err := json.Marshal(mockOpenAIResponse)
			require.NoError(t, err)

			return &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: responseBody,
			}, nil
		},
	}

	// Create pipeline
	factory := pipeline.NewFactory(executor)
	pipeline := factory.Pipeline(inbound, outbound)

	// Create test request (Anthropic format)
	requestBody := map[string]interface{}{
		"model":      "claude-3-sonnet-20240229",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, how are you?",
			},
		},
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	httpRequest := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/messages",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: requestBodyBytes,
	}

	// Process the request
	result, err := pipeline.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)
	require.NotNil(t, result.Response)

	// Verify response (should be in Anthropic format)
	require.Equal(t, http.StatusOK, result.Response.StatusCode)
	require.Equal(t, "application/json", result.Response.Headers.Get("Content-Type"))

	var finalResponse anthropic.Message

	err = json.Unmarshal(result.Response.Body, &finalResponse)
	require.NoError(t, err)
	require.Equal(t, "chatcmpl-456", finalResponse.ID)
	require.Equal(t, "gpt-4", finalResponse.Model)
	require.Equal(t, "assistant", finalResponse.Role)
	require.Len(t, finalResponse.Content, 1)
	require.Equal(t, "text", finalResponse.Content[0].Type)
	require.Equal(t, "Hello! I'm GPT-4, how can I help you?", finalResponse.Content[0].Text)
}

// TestPipeline_Anthropic_to_Anthropic tests the pipeline with Anthropic inbound and outbound transformers.
func TestPipeline_Anthropic_to_Anthropic(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := anthropic.NewInboundTransformer()
	outbound, err := anthropic.NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	// Mock Anthropic response
	mockAnthropicResponse := &anthropic.Message{
		ID:   "msg_789",
		Type: "message",
		Role: "assistant",
		Content: []anthropic.MessageContentBlock{
			{
				Type: "text",
				Text: "Hello! I'm Claude, nice to meet you!",
			},
		},
		Model:      "claude-3-sonnet-20240229",
		StopReason: lo.ToPtr("end_turn"),
		Usage: &anthropic.Usage{
			InputTokens:  8,
			OutputTokens: 9,
		},
	}

	// Create mock executor
	executor := &mockExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			// Verify request format (should be Anthropic format)
			require.Equal(t, http.MethodPost, request.Method)
			require.Contains(t, request.URL, "/v1/messages")
			require.Equal(t, "2023-06-01", request.Headers.Get("Anthropic-Version"))

			// Verify auth configuration
			require.NotNil(t, request.Auth)
			require.Equal(t, "api_key", request.Auth.Type)
			require.Equal(t, "test-api-key", request.Auth.APIKey)
			require.Equal(t, "X-API-Key", request.Auth.HeaderKey)

			responseBody, err := json.Marshal(mockAnthropicResponse)
			require.NoError(t, err)

			return &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: responseBody,
			}, nil
		},
	}

	// Create pipeline
	factory := pipeline.NewFactory(executor)
	pipeline := factory.Pipeline(inbound, outbound)

	// Create test request (Anthropic format)
	requestBody := map[string]interface{}{
		"model":      "claude-3-sonnet-20240229",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, nice to meet you!",
			},
		},
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	httpRequest := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/messages",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: requestBodyBytes,
	}

	// Process the request
	result, err := pipeline.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)
	require.NotNil(t, result.Response)

	// Verify response (should be in Anthropic format)
	require.Equal(t, http.StatusOK, result.Response.StatusCode)
	require.Equal(t, "application/json", result.Response.Headers.Get("Content-Type"))

	var finalResponse anthropic.Message

	err = json.Unmarshal(result.Response.Body, &finalResponse)
	require.NoError(t, err)
	require.Equal(t, "msg_789", finalResponse.ID)
	require.Equal(t, "claude-3-sonnet-20240229", finalResponse.Model)
	require.Equal(t, "assistant", finalResponse.Role)
	require.Len(t, finalResponse.Content, 1)
	require.Equal(t, "text", finalResponse.Content[0].Type)
	require.Equal(t, "Hello! I'm Claude, nice to meet you!", finalResponse.Content[0].Text)
}
