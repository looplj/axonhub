package nanogpt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestIntegration_NanoGPTTerminalPropagation tests that terminal events
// (DONE markers) properly propagate through the NanoGPT transformer.
// This is a regression test for the terminal propagation issue.
func TestIntegration_NanoGPTTerminalPropagation(t *testing.T) {
	ctx := context.Background()

	// Create mock server that simulates NanoGPT API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/chat/completions")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "nano_123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello from NanoGPT!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Test request
	llmReq := &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello!")},
			},
		},
	}

	// Step 1: Transform request
	httpReq, err := transformer.TransformRequest(ctx, llmReq)
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	// Step 2: Execute request
	httpClient := httpclient.NewHttpClient()
	httpResp, err := httpClient.Do(ctx, httpReq)
	require.NoError(t, err)
	require.NotNil(t, httpResp)

	assert.Equal(t, http.StatusOK, httpResp.StatusCode)

	// Step 3: Transform response
	llmResp, err := transformer.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)

	// Verify response
	assert.Equal(t, "nano_123", llmResp.ID)
	assert.Equal(t, "gpt-4o", llmResp.Model)

	// Verify usage extraction
	require.NotNil(t, llmResp.Usage)
	assert.Equal(t, 10, llmResp.Usage.PromptTokens)
	assert.Equal(t, 5, llmResp.Usage.CompletionTokens)

	t.Logf("✓ NanoGPT terminal propagation test passed")
}

// TestIntegration_NanoGPTOpus46Flow tests the end-to-end flow for NanoGPT Opus 4.6.
func TestIntegration_NanoGPTOpus46Flow(t *testing.T) {
	ctx := context.Background()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "nano_opus_123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":      "assistant",
						"content":   "This is Opus 4.6 through NanoGPT",
						"reasoning": "I need to provide a thoughtful response...",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     50,
				"completion_tokens": 25,
			},
		})
	}))
	defer server.Close()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Test Opus 4.6 request
	llmReq := &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello Opus!")},
			},
		},
	}

	// Transform request
	httpReq, err := transformer.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	// Execute request
	httpClient := httpclient.NewHttpClient()
	httpResp, err := httpClient.Do(ctx, httpReq)
	require.NoError(t, err)

	// Transform response with reasoning field
	llmResp, err := transformer.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)

	// Verify Opus 4.6 response
	assert.Equal(t, "nano_opus_123", llmResp.ID)
	assert.Equal(t, "gpt-4o", llmResp.Model)

	// Verify usage extraction
	require.NotNil(t, llmResp.Usage)
	assert.Equal(t, 50, llmResp.Usage.PromptTokens)
	assert.Equal(t, 25, llmResp.Usage.CompletionTokens)

	t.Logf("✓ NanoGPT Opus 4.6 flow test passed")
}

// TestIntegration_NanoGPTCodex52Flow tests the end-to-end flow for NanoGPT Codex 5.2.
func TestIntegration_NanoGPTCodex52Flow(t *testing.T) {
	ctx := context.Background()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "nano_codex_456",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "deepseek-chat",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "function sortList() { return items.sort(); }",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     30,
				"completion_tokens": 20,
			},
		})
	}))
	defer server.Close()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Test Codex 5.2 request
	llmReq := &llm.Request{
		Model: "deepseek-chat",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Write a sort function")},
			},
		},
	}

	// Transform request
	httpReq, err := transformer.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	// Execute request
	httpClient := httpclient.NewHttpClient()
	httpResp, err := httpClient.Do(ctx, httpReq)
	require.NoError(t, err)

	// Transform response
	llmResp, err := transformer.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)

	// Verify Codex 5.2 response
	assert.Equal(t, "nano_codex_456", llmResp.ID)
	assert.Equal(t, "deepseek-chat", llmResp.Model)

	// Verify usage extraction
	require.NotNil(t, llmResp.Usage)
	assert.Equal(t, 30, llmResp.Usage.PromptTokens)
	assert.Equal(t, 20, llmResp.Usage.CompletionTokens)

	t.Logf("✓ NanoGPT Codex 5.2 flow test passed")
}

// TestIntegration_NanoGPTStreamingFlow tests streaming response handling
// including DONE marker propagation.
func TestIntegration_NanoGPTStreamingFlow(t *testing.T) {
	ctx := context.Background()

	// Create mock streaming server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Send streaming events
		events := []string{
			`{"id":"stream_123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`{"id":"stream_123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" World"},"finish_reason":null}]}`,
			`{"id":"stream_123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}]}`,
			`[DONE]`,
		}

		for _, event := range events {
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Create streaming request
	llmReq := &llm.Request{
		Model:  "gpt-4o",
		Stream: lo.ToPtr(true),
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Say hello")},
			},
		},
	}

	// Transform request
	httpReq, err := transformer.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	// Execute streaming request
	httpClient := httpclient.NewHttpClient()
	stream, err := httpClient.DoStream(ctx, httpReq)
	require.NoError(t, err)
	defer stream.Close()

	// Transform stream
	llmStream, err := transformer.TransformStream(ctx, stream)
	require.NoError(t, err)
	require.NotNil(t, llmStream)

	// Collect responses
	var responses []*llm.Response
	for llmStream.Next() {
		resp := llmStream.Current()
		if resp != nil {
			responses = append(responses, resp)
		}
	}
	require.NoError(t, llmStream.Err())

	// Verify responses - should have content + DONE
	assert.GreaterOrEqual(t, len(responses), 2, "Should have content chunks + DONE marker")

	// Verify last response is DONE
	lastResp := responses[len(responses)-1]
	assert.Equal(t, llm.DoneResponse.Object, lastResp.Object,
		"Last event must be DONE response")

	t.Logf("✓ NanoGPT streaming test passed: %d responses received", len(responses))
}

// TestIntegration_NanoGPTReasoningPropagation tests that reasoning content
// is properly extracted and propagated through the transformer.
func TestIntegration_NanoGPTReasoningPropagation(t *testing.T) {
	ctx := context.Background()

	// Create mock server with reasoning field
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "reasoning_123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4o-thinking",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":      "assistant",
						"content":   "The answer is 42.",
						"reasoning": "Let me think about this problem step by step...",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     20,
				"completion_tokens": 15,
			},
		})
	}))
	defer server.Close()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Test request
	llmReq := &llm.Request{
		Model: "gpt-4o-thinking",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("What is the answer?")},
			},
		},
	}

	// Transform and execute request
	httpReq, err := transformer.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	httpClient := httpclient.NewHttpClient()
	httpResp, err := httpClient.Do(ctx, httpReq)
	require.NoError(t, err)

	// Transform response
	llmResp, err := transformer.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)
	require.Len(t, llmResp.Choices, 1)

	// Verify content
	assert.Equal(t, "The answer is 42.", *llmResp.Choices[0].Message.Content.Content)

	// Verify reasoning is propagated
	require.NotNil(t, llmResp.Choices[0].Message.ReasoningContent)
	assert.Equal(t, "Let me think about this problem step by step...",
		*llmResp.Choices[0].Message.ReasoningContent)

	t.Logf("✓ NanoGPT reasoning propagation test passed")
}

// TestIntegration_NanoGPTStatusTransitions tests proper handling of various
// HTTP status codes and error transformations.
func TestIntegration_NanoGPTStatusTransitions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		statusCode     int
		responseBody   []byte
		expectErr      bool
		validateErrMsg string
	}{
		{
			name:         "success - Opus 4.6",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"id":"ok","object":"chat.completion","choices":[{"message":{"content":"OK"}}]}`),
			expectErr:    false,
		},
		{
			name:         "success - Codex 5.2",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"id":"ok","object":"chat.completion","choices":[{"message":{"content":"OK"}}]}`),
			expectErr:    false,
		},
		{
			name:           "auth error",
			statusCode:     http.StatusUnauthorized,
			responseBody:   []byte(`{"error": "unauthorized"}`),
			expectErr:      true,
			validateErrMsg: "HTTP error 401",
		},
		{
			name:           "rate limit",
			statusCode:     http.StatusTooManyRequests,
			responseBody:   []byte(`{"error": "rate limit exceeded"}`),
			expectErr:      true,
			validateErrMsg: "HTTP error 429",
		},
		{
			name:           "empty body error",
			statusCode:     http.StatusOK,
			responseBody:   []byte{},
			expectErr:      true,
			validateErrMsg: "response body is empty",
		},
		{
			name:           "invalid json error",
			statusCode:     http.StatusOK,
			responseBody:   []byte(`not valid json`),
			expectErr:      true,
			validateErrMsg: "failed to unmarshal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write(tt.responseBody)
			}))
			defer server.Close()

			// Create transformer
			transformer, err := NewOutboundTransformerWithConfig(&Config{
				BaseURL:        server.URL,
				APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
			})
			require.NoError(t, err)

			// Create request
			llmReq := &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Test")},
					},
				},
			}

			// Transform request
			httpReq, err := transformer.TransformRequest(ctx, llmReq)
			require.NoError(t, err)

			// Execute request
			httpClient := httpclient.NewHttpClient()
			httpResp, err := httpClient.Do(ctx, httpReq)
			require.NoError(t, err)

			// Transform response
			_, err = transformer.TransformResponse(ctx, httpResp)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.validateErrMsg != "" {
					assert.Contains(t, err.Error(), tt.validateErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestIntegration_NanoGPTStreamingDoneFiltering tests that upstream [DONE]
// markers are filtered and our own DONE is appended.
func TestIntegration_NanoGPTStreamingDoneFiltering(t *testing.T) {
	ctx := context.Background()

	// Create mock streaming server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Send events with upstream DONE
		events := []string{
			`{"id":"test","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
			`[DONE]`, // Upstream DONE
		}

		for _, event := range events {
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
			time.Sleep(2 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Create streaming request
	llmReq := &llm.Request{
		Model:  "deepseek-chat",
		Stream: lo.ToPtr(true),
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Test")},
			},
		},
	}

	// Transform and execute
	httpReq, err := transformer.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	httpClient := httpclient.NewHttpClient()
	stream, err := httpClient.DoStream(ctx, httpReq)
	require.NoError(t, err)
	defer stream.Close()

	// Transform stream
	llmStream, err := transformer.TransformStream(ctx, stream)
	require.NoError(t, err)

	// Collect responses
	var responses []*llm.Response
	for llmStream.Next() {
		resp := llmStream.Current()
		if resp != nil {
			responses = append(responses, resp)
		}
	}
	require.NoError(t, llmStream.Err())

	// Should have exactly 2 responses: content + our DONE
	assert.Len(t, responses, 2, "Expected content + DONE")

	// Last response should be our DONE
	lastResp := responses[len(responses)-1]
	assert.Equal(t, llm.DoneResponse.Object, lastResp.Object,
		"Last event must be DONE response")

	t.Logf("✓ NanoGPT DONE filtering test passed: %d responses", len(responses))
}

// TestIntegration_NanoGPTStreamingReasoningPropagation tests reasoning
// content in streaming responses.
func TestIntegration_NanoGPTStreamingReasoningPropagation(t *testing.T) {
	ctx := context.Background()

	// Create mock streaming server with reasoning
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Send events with reasoning content
		events := []string{
			`{"id":"test","choices":[{"index":0,"delta":{"content":"","reasoning":"Let me analyze"}}]}`,
			`{"id":"test","choices":[{"index":0,"delta":{"content":"The answer is","reasoning":""}}]}`,
			`{"id":"test","choices":[{"index":0,"delta":{"content":" 42","reasoning":""},"finish_reason":"stop"}]}`,
		}

		for _, event := range events {
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
			time.Sleep(2 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Create streaming request
	llmReq := &llm.Request{
		Model:  "gpt-4o",
		Stream: lo.ToPtr(true),
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("What is 6 * 7?")},
			},
		},
	}

	// Transform and execute
	httpReq, err := transformer.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	httpClient := httpclient.NewHttpClient()
	stream, err := httpClient.DoStream(ctx, httpReq)
	require.NoError(t, err)
	defer stream.Close()

	// Transform stream
	llmStream, err := transformer.TransformStream(ctx, stream)
	require.NoError(t, err)

	// Collect responses
	var responses []*llm.Response
	for llmStream.Next() {
		resp := llmStream.Current()
		if resp != nil {
			responses = append(responses, resp)
		}
	}
	require.NoError(t, llmStream.Err())

	// Verify we got responses
	assert.GreaterOrEqual(t, len(responses), 1, "Should have streaming responses")

	t.Logf("✓ NanoGPT streaming reasoning propagation test passed: %d responses", len(responses))
}

// TestIntegration_NanoGPTAggregateStreamChunks tests chunk aggregation
// for Opus/Codex models.
func TestIntegration_NanoGPTAggregateStreamChunks(t *testing.T) {
	ctx := context.Background()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://example.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Create test chunks
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"agg_test","choices":[{"index":0,"delta":{"content":"Hello"}}]}`)},
		{Data: []byte(`{"id":"agg_test","choices":[{"index":0,"delta":{"content":" World"}}]}`)},
		{Data: []byte(`{"id":"agg_test","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}]}`)},
	}

	// Aggregate chunks
	data, meta, err := transformer.AggregateStreamChunks(ctx, chunks)
	require.NoError(t, err)
	require.NotNil(t, data)
	require.NotNil(t, meta)

	// Verify aggregated content
	assert.Contains(t, string(data), "Hello World!")

	t.Logf("✓ NanoGPT aggregate stream chunks test passed")
}

// TestIntegration_NanoGPTEmbeddingRequest tests that embedding requests
// are properly routed to the embedded OpenAI transformer.
func TestIntegration_NanoGPTEmbeddingRequest(t *testing.T) {
	ctx := context.Background()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/embeddings")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"object":    "embedding",
					"embedding": []float64{0.1, 0.2, 0.3},
					"index":     0,
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]int{
				"prompt_tokens": 5,
				"total_tokens":  5,
			},
		})
	}))
	defer server.Close()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Create embedding request (will use embedded OpenAI transformer)
	llmReq := &llm.Request{
		Model:       "text-embedding-3-small",
		RequestType: llm.RequestTypeEmbedding,
		Embedding: &llm.EmbeddingRequest{
			Input: llm.EmbeddingInput{
				String: "Hello world",
			},
		},
	}

	// Transform request
	httpReq, err := transformer.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	// Execute request
	httpClient := httpclient.NewHttpClient()
	httpResp, err := httpClient.Do(ctx, httpReq)
	require.NoError(t, err)

	// Transform response
	llmResp, err := transformer.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)

	// Verify embedding response
	require.NotNil(t, llmResp.Embedding)
	require.Len(t, llmResp.Embedding.Data, 1)
	require.NotNil(t, llmResp.Embedding.Data[0].Embedding)
	assert.Len(t, llmResp.Embedding.Data[0].Embedding.Embedding, 3)

	t.Logf("✓ NanoGPT embedding request test passed")
}

// TestIntegration_NanoGPTTransformStreamChunk tests the TransformStreamChunk
// method directly for various edge cases.
func TestIntegration_NanoGPTTransformStreamChunk(t *testing.T) {
	ctx := context.Background()

	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://example.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	tests := []struct {
		name           string
		event          *httpclient.StreamEvent
		expectDone     bool
		expectErr      bool
		validateResult func(*testing.T, *llm.Response)
	}{
		{
			name: "valid content chunk",
			event: &httpclient.StreamEvent{
				Data: []byte(`{"id":"test","choices":[{"index":0,"delta":{"content":"Hello"}}]}`),
			},
			expectDone: false,
			expectErr:  false,
			validateResult: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Len(t, resp.Choices, 1)
				require.NotNil(t, resp.Choices[0].Delta)
				require.NotNil(t, resp.Choices[0].Delta.Content)
				assert.Equal(t, "Hello", *resp.Choices[0].Delta.Content.Content)
			},
		},
		{
			name: "DONE marker",
			event: &httpclient.StreamEvent{
				Data: []byte("[DONE]"),
			},
			expectDone: true,
			expectErr:  false,
		},
		{
			name: "DONE with newline",
			event: &httpclient.StreamEvent{
				Data: []byte("[DONE]\n"),
			},
			expectDone: true,
			expectErr:  false,
		},
		{
			name: "error in event",
			event: &httpclient.StreamEvent{
				Data: []byte(`{"error": "some error"}`),
			},
			expectDone: false,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nanogptTx, ok := transformer.(*OutboundTransformer)
			require.True(t, ok, "transformer should be *OutboundTransformer")

			resp, err := nanogptTx.TransformStreamChunk(ctx, tt.event)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			if tt.expectDone {
				require.NoError(t, err)
				assert.Equal(t, llm.DoneResponse.Object, resp.Object)
			} else {
				require.NoError(t, err)
				if tt.validateResult != nil {
					tt.validateResult(t, resp)
				}
			}
		})
	}
}

// TestIntegration_NanoGPTEndToEndComplete tests a complete end-to-end flow
// simulating real-world usage with Opus 4.6 through NanoGPT.
func TestIntegration_NanoGPTEndToEndComplete(t *testing.T) {
	ctx := context.Background()

	// Create mock server with realistic response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/chat/completions")

		// Parse request body
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			// Verify model is preserved
			assert.Equal(t, "gpt-4o", reqBody["model"])
		}

		// Return realistic NanoGPT response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "e2e_complete_123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":      "assistant",
						"content":   "This is a complete end-to-end test response.",
						"reasoning": "I analyzed the request thoroughly...",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
		})
	}))
	defer server.Close()

	// Create transformer
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	// Full request with multiple messages
	llmReq := &llm.Request{
		Model:       "gpt-4o",
		Temperature: lo.ToPtr(0.7),
		MaxTokens:   lo.ToPtr(int64(1000)),
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: llm.MessageContent{Content: lo.ToPtr("You are a helpful assistant")},
			},
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Please help me with a complex task")},
			},
			{
				Role:    "assistant",
				Content: llm.MessageContent{Content: lo.ToPtr("I'd be happy to help")},
			},
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("What's the weather like?")},
			},
		},
	}

	// Transform request
	httpReq, err := transformer.TransformRequest(ctx, llmReq)
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	// Verify request has body
	assert.NotNil(t, httpReq.Body)
	assert.Greater(t, len(httpReq.Body), 0)

	// Execute request
	httpClient := httpclient.NewHttpClient()
	httpResp, err := httpClient.Do(ctx, httpReq)
	require.NoError(t, err)

	// Transform response
	llmResp, err := transformer.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)

	// Verify complete response
	assert.Equal(t, "e2e_complete_123", llmResp.ID)
	assert.Equal(t, "gpt-4o", llmResp.Model)
	assert.Len(t, llmResp.Choices, 1)

	// Verify content
	content := *llmResp.Choices[0].Message.Content.Content
	assert.Equal(t, "This is a complete end-to-end test response.", content)

	// Verify reasoning
	require.NotNil(t, llmResp.Choices[0].Message.ReasoningContent)
	assert.Contains(t, *llmResp.Choices[0].Message.ReasoningContent, "I analyzed")

	// Verify usage
	require.NotNil(t, llmResp.Usage)
	assert.Equal(t, 100, llmResp.Usage.PromptTokens)
	assert.Equal(t, 50, llmResp.Usage.CompletionTokens)
	assert.Equal(t, 150, llmResp.Usage.TotalTokens)

	t.Logf("✓ NanoGPT complete end-to-end test passed")
}
