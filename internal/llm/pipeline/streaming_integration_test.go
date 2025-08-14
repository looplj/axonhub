package pipeline_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

// TestPipeline_Streaming_OpenAI_to_OpenAI tests streaming pipeline with OpenAI inbound and outbound transformers.
func TestPipeline_Streaming_OpenAI_to_OpenAI(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Load test data using xtest
	streamEvents, err := xtest.LoadStreamChunks("openai-tool.stream.jsonl")
	require.NoError(t, err)

	// Create mock executor for streaming
	executor := &mockExecutor{
		doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			// Verify request format
			require.Equal(t, http.MethodPost, request.Method)
			require.Contains(t, request.URL, "/chat/completions")

			// Verify auth configuration
			require.NotNil(t, request.Auth)
			require.Equal(t, "bearer", request.Auth.Type)
			require.Equal(t, "test-api-key", request.Auth.APIKey)

			// Return mock stream
			return streams.SliceStream(streamEvents), nil
		},
	}

	// Create pipeline
	factory := pipeline.NewFactory(executor)
	pipeline := factory.Pipeline(inbound, outbound)

	// Create test request (OpenAI format with streaming)
	requestBody := map[string]interface{}{
		"model":  "gpt-4",
		"stream": true,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "What is the weather in New York City?",
			},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "get_weather",
					"description": "Get weather at the given location",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []string{"location"},
					},
				},
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
	require.True(t, result.Stream)
	require.NotNil(t, result.EventStream)

	// Collect all events from the stream
	var collectedEvents []*httpclient.StreamEvent

	for result.EventStream.Next() {
		event := result.EventStream.Current()
		collectedEvents = append(collectedEvents, event)
	}

	// Verify we got events
	require.NotEmpty(t, collectedEvents)

	// Verify the last event is [DONE]
	lastEvent := collectedEvents[len(collectedEvents)-1]
	require.Equal(t, "[DONE]", string(lastEvent.Data))
}

// TestPipeline_Streaming_OpenAI_to_Anthropic tests streaming pipeline with OpenAI inbound and Anthropic outbound transformers.
func TestPipeline_Streaming_OpenAI_to_Anthropic(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := anthropic.NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	// Load test data using xtest
	streamEvents, err := xtest.LoadStreamChunks("anthropic-tool.stream.jsonl")
	require.NoError(t, err)

	// Create mock executor for streaming
	executor := &mockExecutor{
		doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			// Verify request format (should be Anthropic format)
			require.Equal(t, http.MethodPost, request.Method)
			require.Contains(t, request.URL, "/v1/messages")
			require.Equal(t, "2023-06-01", request.Headers.Get("Anthropic-Version"))

			// Verify auth configuration
			require.NotNil(t, request.Auth)
			require.Equal(t, "api_key", request.Auth.Type)
			require.Equal(t, "test-api-key", request.Auth.APIKey)
			require.Equal(t, "X-API-Key", request.Auth.HeaderKey)

			// Return mock stream
			return streams.SliceStream(streamEvents), nil
		},
	}

	// Create pipeline
	factory := pipeline.NewFactory(executor)
	pipeline := factory.Pipeline(inbound, outbound)

	// Create test request (OpenAI format with streaming)
	requestBody := map[string]interface{}{
		"model":      "gpt-4",
		"stream":     true,
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "What is the weather in San Francisco, CA?",
			},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "get_weather",
					"description": "Get weather at the given location",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []string{"location"},
					},
				},
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
	require.True(t, result.Stream)
	require.NotNil(t, result.EventStream)

	// Collect all events from the stream
	var collectedEvents []*httpclient.StreamEvent

	for result.EventStream.Next() {
		event := result.EventStream.Current()
		collectedEvents = append(collectedEvents, event)
	}

	// Verify we got events
	require.NotEmpty(t, collectedEvents)

	// Verify the last event is [DONE]
	lastEvent := collectedEvents[len(collectedEvents)-1]
	require.Equal(t, "[DONE]", string(lastEvent.Data))
}

// TestPipeline_Streaming_Anthropic_to_OpenAI tests streaming pipeline with Anthropic inbound and OpenAI outbound transformers.
func TestPipeline_Streaming_Anthropic_to_OpenAI(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := anthropic.NewInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	// Load test data using xtest
	streamEvents, err := xtest.LoadStreamChunks("openai-tool.stream.jsonl")
	require.NoError(t, err)

	// Create mock executor for streaming
	executor := &mockExecutor{
		doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			// Verify request format (should be OpenAI format)
			require.Equal(t, http.MethodPost, request.Method)
			require.Contains(t, request.URL, "/chat/completions")

			// Verify auth configuration
			require.NotNil(t, request.Auth)
			require.Equal(t, "bearer", request.Auth.Type)
			require.Equal(t, "test-api-key", request.Auth.APIKey)

			// Return mock stream
			return streams.SliceStream(streamEvents), nil
		},
	}

	// Create pipeline
	factory := pipeline.NewFactory(executor)
	pipeline := factory.Pipeline(inbound, outbound)

	// Create test request (Anthropic format with streaming)
	requestBody := map[string]interface{}{
		"model":      "claude-3-sonnet-20240229",
		"max_tokens": 1024,
		"stream":     true,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "What is the weather in New York City?",
			},
		},
		"tools": []map[string]interface{}{
			{
				"name":        "get_weather",
				"description": "Get weather at the given location",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"location"},
				},
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
	require.True(t, result.Stream)
	require.NotNil(t, result.EventStream)

	// Collect all events from the stream
	var collectedEvents []*httpclient.StreamEvent

	for result.EventStream.Next() {
		event := result.EventStream.Current()
		collectedEvents = append(collectedEvents, event)
	}

	// Verify we got events
	require.NotEmpty(t, collectedEvents)

	// The last event should be a message_stop event in Anthropic format
	lastEvent := collectedEvents[len(collectedEvents)-1]
	require.NotEmpty(t, lastEvent.Data)
}

// TestPipeline_Streaming_Anthropic_to_Anthropic tests streaming pipeline with Anthropic inbound and outbound transformers.
func TestPipeline_Streaming_Anthropic_to_Anthropic(t *testing.T) {
	ctx := context.Background()

	// Create transformers
	inbound := anthropic.NewInboundTransformer()
	outbound, err := anthropic.NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	// Load test data using xtest
	streamEvents, err := xtest.LoadStreamChunks("anthropic-tool.stream.jsonl")
	require.NoError(t, err)

	// Create mock executor for streaming
	executor := &mockExecutor{
		doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			// Verify request format (should be Anthropic format)
			require.Equal(t, http.MethodPost, request.Method)
			require.Contains(t, request.URL, "/v1/messages")
			require.Equal(t, "2023-06-01", request.Headers.Get("Anthropic-Version"))

			// Verify auth configuration
			require.NotNil(t, request.Auth)
			require.Equal(t, "api_key", request.Auth.Type)
			require.Equal(t, "test-api-key", request.Auth.APIKey)
			require.Equal(t, "X-API-Key", request.Auth.HeaderKey)

			// Return mock stream
			return streams.SliceStream(streamEvents), nil
		},
	}

	// Create pipeline
	factory := pipeline.NewFactory(executor)
	pipeline := factory.Pipeline(inbound, outbound)

	// Create test request (Anthropic format with streaming)
	requestBody := map[string]interface{}{
		"model":      "claude-3-sonnet-20240229",
		"max_tokens": 1024,
		"stream":     true,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "What is the weather in San Francisco, CA?",
			},
		},
		"tools": []map[string]interface{}{
			{
				"name":        "get_weather",
				"description": "Get weather at the given location",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"location"},
				},
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
	require.True(t, result.Stream)
	require.NotNil(t, result.EventStream)

	// Collect all events from the stream
	var collectedEvents []*httpclient.StreamEvent

	for result.EventStream.Next() {
		event := result.EventStream.Current()
		collectedEvents = append(collectedEvents, event)
	}

	// Verify we got events
	require.NotEmpty(t, collectedEvents)

	// The last event should be a message_stop event in Anthropic format
	lastEvent := collectedEvents[len(collectedEvents)-1]
	require.NotEmpty(t, lastEvent.Data)
}

// TestPipeline_Streaming_WithTestData tests streaming with actual test data files.
func TestPipeline_Streaming_WithTestData(t *testing.T) {
	tests := []struct {
		name                string
		inboundType         string
		outboundType        string
		inputStreamFile     string
		expectedOutputCheck func(t *testing.T, events []*httpclient.StreamEvent)
	}{
		{
			name:            "OpenAI to OpenAI with tool calls",
			inboundType:     "openai",
			outboundType:    "openai",
			inputStreamFile: "openai-tool.stream.jsonl",
			expectedOutputCheck: func(t *testing.T, events []*httpclient.StreamEvent) {
				t.Helper()
				require.NotEmpty(t, events)
				lastEvent := events[len(events)-1]
				require.Equal(t, "[DONE]", string(lastEvent.Data))
			},
		},
		{
			name:            "Anthropic to Anthropic with tool calls",
			inboundType:     "anthropic",
			outboundType:    "anthropic",
			inputStreamFile: "anthropic-tool.stream.jsonl",
			expectedOutputCheck: func(t *testing.T, events []*httpclient.StreamEvent) {
				t.Helper()
				require.NotEmpty(t, events)
				// Anthropic streams end with message_stop event
				lastEvent := events[len(events)-1]
				require.NotEmpty(t, lastEvent.Data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create transformers based on test type
			var (
				inbound  transformer.Inbound
				outbound transformer.Outbound
			)

			switch tt.inboundType {
			case "openai":
				inbound = openai.NewInboundTransformer()
			case "anthropic":
				inbound = anthropic.NewInboundTransformer()
			}

			switch tt.outboundType {
			case "openai":
				var err error

				outbound, err = openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
				require.NoError(t, err)
			case "anthropic":
				var err error

				outbound, err = anthropic.NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
				require.NoError(t, err)
			}

			// Load test data using xtest
			streamEvents, err := xtest.LoadStreamChunks(tt.inputStreamFile)
			require.NoError(t, err)

			// Create mock executor
			executor := &mockExecutor{
				doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
					return streams.SliceStream(streamEvents), nil
				},
			}

			// Create pipeline
			factory := pipeline.NewFactory(executor)
			pipeline := factory.Pipeline(inbound, outbound)

			// Create appropriate test request
			var (
				requestBody map[string]interface{}
				requestURL  string
			)

			if tt.inboundType == "openai" {
				requestBody = map[string]interface{}{
					"model":  "gpt-4",
					"stream": true,
					"messages": []map[string]interface{}{
						{
							"role":    "user",
							"content": "Test message",
						},
					},
				}
				requestURL = "/v1/chat/completions"
			} else {
				requestBody = map[string]interface{}{
					"model":      "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"stream":     true,
					"messages": []map[string]interface{}{
						{
							"role":    "user",
							"content": "Test message",
						},
					},
				}
				requestURL = "/v1/messages"
			}

			requestBodyBytes, err := json.Marshal(requestBody)
			require.NoError(t, err)

			httpRequest := &httpclient.Request{
				Method: http.MethodPost,
				URL:    requestURL,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: requestBodyBytes,
			}

			// Process the request
			result, err := pipeline.Process(ctx, httpRequest)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.True(t, result.Stream)
			require.NotNil(t, result.EventStream)

			// Collect all events from the stream
			var collectedEvents []*httpclient.StreamEvent

			for result.EventStream.Next() {
				event := result.EventStream.Current()
				collectedEvents = append(collectedEvents, event)
			}

			// Run the expected output check
			tt.expectedOutputCheck(t, collectedEvents)
		})
	}
}
