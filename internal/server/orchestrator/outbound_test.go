package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
)

// mockTransformer is a simple mock transformer for testing.
type mockTransformer struct{}

func (m *mockTransformer) TransformRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	body, err := json.Marshal(map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"temperature": 0.5,
		"max_tokens":  1000,
	})
	if err != nil {
		return nil, err
	}

	return &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
		Body:   body,
	}, nil
}

func (m *mockTransformer) TransformResponse(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (m *mockTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, nil
}

func (m *mockTransformer) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return nil
}

func (m *mockTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, nil
}

func (m *mockTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIChatCompletion
}

// TestOverrideParameters tests that TransformRequest works correctly.
// Note: Override parameters are now applied via OnRawRequest middleware,
// so this test only verifies the base transformation without overrides.
func TestOverrideParameters(t *testing.T) {
	ctx := context.Background()

	// Create mock channel
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:              1,
			Name:            "test-channel",
			SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			Settings:        nil,
		},
		Outbound: &mockTransformer{},
	}

	// Create processor
	processor := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: channel,
			Channels:       []*biz.Channel{channel},
			ChannelIndex:   0,
			RequestExec:    &ent.RequestExecution{ID: 1}, // Dummy to skip creation
		},
	}

	// Create test request
	text := "Hello"
	llmRequest := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: &text,
				},
			},
		},
	}

	// Transform request
	channelRequest, err := processor.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)
	require.NotNil(t, channelRequest)

	// Verify base transformation works
	bodyStr := string(channelRequest.Body)
	temperature := gjson.Get(bodyStr, "temperature")
	assert.Equal(t, 0.5, temperature.Float())
}

// TestOverrideParametersInvalidJSON and TestOverrideParametersEmptySettings
// are now covered by the middleware tests (TestOverrideParametersMiddleware_InvalidJSON
// and TestOverrideParametersMiddleware_EmptySettings)

func TestPersistentOutboundTransformer_TransformRequest_OriginalModelRestoration(t *testing.T) {
	tests := []struct {
		name               string
		originalModel      string
		inputModel         string
		expectedFinalModel string
	}{
		{
			name:               "no original model - should use input model",
			originalModel:      "",
			inputModel:         "gpt-4",
			expectedFinalModel: "gpt-4",
		},
		{
			name:               "has original model - should restore original",
			originalModel:      "gpt-3.5-turbo",
			inputModel:         "mapped-gpt-4",
			expectedFinalModel: "gpt-3.5-turbo",
		},
		{
			name:               "original and input are same - should remain unchanged",
			originalModel:      "gpt-4",
			inputModel:         "gpt-4",
			expectedFinalModel: "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctx := context.Background()

			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:              1,
					Name:            "test-channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings:        nil,
				},
				Outbound: &mockTransformer{},
			}

			processor := &PersistentOutboundTransformer{
				wrapped: &mockTransformer{},
				state: &PersistenceState{
					OriginalModel:  tt.originalModel,
					CurrentChannel: channel,
					Channels:       []*biz.Channel{channel},
					ChannelIndex:   0,
					RequestExec:    &ent.RequestExecution{ID: 1}, // Dummy to skip creation
				},
			}

			text := "Hello"
			llmRequest := &llm.Request{
				Model: tt.inputModel,
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: &text,
						},
					},
				},
			}

			// Execute
			channelRequest, err := processor.TransformRequest(ctx, llmRequest)

			// Assert
			require.NoError(t, err)
			require.NotNil(t, channelRequest)

			// Verify model restoration in the request body
			bodyStr := string(channelRequest.Body)
			model := gjson.Get(bodyStr, "model")
			assert.Equal(t, tt.expectedFinalModel, model.String())

			// Also verify the llmRequest was modified
			assert.Equal(t, tt.expectedFinalModel, llmRequest.Model)
		})
	}
}

func TestPersistentOutboundTransformer_TransformRequest_WithChannelSelection(t *testing.T) {
	// Setup
	ctx := context.Background()

	// Pre-populate channels (now done by inbound transformer)
	testChannel := &biz.Channel{
		Channel: &ent.Channel{
			ID:              1,
			Name:            "test-channel",
			SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"}, // Add gpt-3.5-turbo
			Settings:        nil,
		},
		Outbound: &mockTransformer{},
	}

	processor := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			OriginalModel: "gpt-3.5-turbo",
			Channels:      []*biz.Channel{testChannel}, // Pre-populated by inbound
			ChannelIndex:  0,
			RequestExec:   &ent.RequestExecution{ID: 1}, // Dummy to skip creation
		},
	}

	text := "Hello"
	llmRequest := &llm.Request{
		Model: "mapped-gpt-4", // This was mapped by inbound transformer
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: &text,
				},
			},
		},
	}

	// Execute
	channelRequest, err := processor.TransformRequest(ctx, llmRequest)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, channelRequest)

	// Verify original model was restored
	assert.Equal(t, "gpt-3.5-turbo", llmRequest.Model)

	// Verify channel was used
	assert.Equal(t, testChannel, processor.state.CurrentChannel)
}

// mockChannelSelector for testing.
type mockChannelSelector struct {
	selectFunc func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error)
}

func (m *mockChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	if m.selectFunc != nil {
		return m.selectFunc(ctx, req)
	}

	return []*ChannelModelCandidate{}, nil
}

func TestOverrideParametersMiddleware(t *testing.T) {
	tests := []struct {
		name               string
		overrideParameters map[string]any
		expectedValues     map[string]any
		unexpectedKeys     []string
	}{
		{
			name: "override temperature",
			overrideParameters: map[string]any{
				"temperature": 0.9,
			},
			expectedValues: map[string]any{
				"temperature": 0.9,
				"max_tokens":  float64(1000),
			},
		},
		{
			name: "override multiple parameters",
			overrideParameters: map[string]any{
				"temperature": 0.7,
				"max_tokens":  2000,
				"top_p":       0.95,
			},
			expectedValues: map[string]any{
				"temperature": 0.7,
				"max_tokens":  float64(2000),
				"top_p":       0.95,
			},
		},
		{
			name: "override nested parameter",
			overrideParameters: map[string]any{
				"response_format.type": "json_object",
			},
			expectedValues: map[string]any{
				"response_format.type": "json_object",
			},
		},
		{
			name: "stream override ignored",
			overrideParameters: map[string]any{
				"stream":      true,
				"temperature": 0.8,
			},
			expectedValues: map[string]any{
				"temperature": 0.8,
			},
			unexpectedKeys: []string{"stream"},
		},
		{
			name:               "no override parameters",
			overrideParameters: nil,
			expectedValues: map[string]any{
				"temperature": 0.5,
				"max_tokens":  float64(1000),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create override parameters JSON
			var overrideParamsStr string

			if tt.overrideParameters != nil {
				data, err := json.Marshal(tt.overrideParameters)
				require.NoError(t, err)

				overrideParamsStr = string(data)
			}

			// Create mock channel with override parameters
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:              1,
					Name:            "test-channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings: &objects.ChannelSettings{
						OverrideParameters: overrideParamsStr,
					},
				},
				Outbound: &mockTransformer{},
			}

			// Create outbound transformer
			outbound := &PersistentOutboundTransformer{
				wrapped: &mockTransformer{},
				state: &PersistenceState{
					CurrentChannel: channel,
					Channels:       []*biz.Channel{channel},
					ChannelIndex:   0,
				},
			}

			// Create the middleware
			middleware := applyOverrideRequestBody(outbound)

			// Create a test request
			requestBody, err := json.Marshal(map[string]any{
				"model":       "gpt-4",
				"temperature": 0.5,
				"max_tokens":  1000,
			})
			require.NoError(t, err)

			httpRequest := &httpclient.Request{
				Method: "POST",
				URL:    "https://api.example.com/v1/chat/completions",
				Body:   requestBody,
			}

			// Apply the middleware
			modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
			require.NoError(t, err)
			require.NotNil(t, modifiedRequest)

			// Verify expected values
			bodyStr := string(modifiedRequest.Body)
			for key, expectedValue := range tt.expectedValues {
				result := gjson.Get(bodyStr, key)
				assert.True(t, result.Exists(), "key %s should exist", key)

				switch v := expectedValue.(type) {
				case float64:
					assert.Equal(t, v, result.Float(), "key %s should have value %v", key, v)
				case string:
					assert.Equal(t, v, result.String(), "key %s should have value %v", key, v)
				default:
					assert.Equal(t, v, result.Value(), "key %s should have value %v", key, v)
				}
			}

			for _, key := range tt.unexpectedKeys {
				result := gjson.Get(bodyStr, key)
				assert.False(t, result.Exists(), "key %s should not exist", key)
			}
		})
	}
}

func TestOverrideParametersMiddleware_NoChannel(t *testing.T) {
	ctx := context.Background()

	// Create outbound transformer without a channel
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: nil,
		},
	}

	// Create the middleware
	middleware := applyOverrideRequestBody(outbound)

	// Create a test request
	requestBody, err := json.Marshal(map[string]any{
		"model":       "gpt-4",
		"temperature": 0.5,
	})
	require.NoError(t, err)

	httpRequest := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
		Body:   requestBody,
	}

	// Apply the middleware - should not modify the request
	modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, modifiedRequest)

	// Verify request is unchanged
	assert.Equal(t, requestBody, modifiedRequest.Body)
}

func TestOverrideParametersMiddleware_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	// Create channel with invalid JSON
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:              1,
			Name:            "test-channel",
			SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			Settings: &objects.ChannelSettings{
				OverrideParameters: "invalid json",
			},
		},
		Outbound: &mockTransformer{},
	}

	// Create outbound transformer
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: channel,
			Channels:       []*biz.Channel{channel},
			ChannelIndex:   0,
		},
	}

	// Create the middleware
	middleware := applyOverrideRequestBody(outbound)

	// Create a test request
	requestBody, err := json.Marshal(map[string]any{
		"model":       "gpt-4",
		"temperature": 0.5,
	})
	require.NoError(t, err)

	httpRequest := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
		Body:   requestBody,
	}

	// Apply the middleware - should not modify the request due to invalid JSON
	modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, modifiedRequest)

	// Verify original values are preserved
	bodyStr := string(modifiedRequest.Body)
	temperature := gjson.Get(bodyStr, "temperature")
	assert.Equal(t, 0.5, temperature.Float())
}

func TestOverrideParametersMiddleware_EmptySettings(t *testing.T) {
	ctx := context.Background()

	// Create channel without settings
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:              1,
			Name:            "test-channel",
			SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			Settings:        nil,
		},
		Outbound: &mockTransformer{},
	}

	// Create outbound transformer
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: channel,
			Channels:       []*biz.Channel{channel},
			ChannelIndex:   0,
		},
	}

	// Create the middleware
	middleware := applyOverrideRequestBody(outbound)

	// Create a test request
	requestBody, err := json.Marshal(map[string]any{
		"model":       "gpt-4",
		"temperature": 0.5,
	})
	require.NoError(t, err)

	httpRequest := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
		Body:   requestBody,
	}

	// Apply the middleware - should not modify the request
	modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, modifiedRequest)

	// Verify original values
	bodyStr := string(modifiedRequest.Body)
	temperature := gjson.Get(bodyStr, "temperature")
	assert.Equal(t, 0.5, temperature.Float())
}

func TestOverrideParametersMiddleware_AxonHubClear(t *testing.T) {
	tests := []struct {
		name               string
		overrideParameters map[string]any
		initialBody        map[string]any
		expectedRemoved    []string
		expectedPreserved  map[string]any
	}{
		{
			name: "remove single parameter with __AXONHUB_CLEAR__",
			overrideParameters: map[string]any{
				"temperature": "__AXONHUB_CLEAR__",
			},
			initialBody: map[string]any{
				"model":       "gpt-4",
				"temperature": 0.5,
				"max_tokens":  1000,
			},
			expectedRemoved:   []string{"temperature"},
			expectedPreserved: map[string]any{"model": "gpt-4", "max_tokens": float64(1000)},
		},
		{
			name: "remove multiple parameters with __AXONHUB_CLEAR__",
			overrideParameters: map[string]any{
				"temperature": "__AXONHUB_CLEAR__",
				"max_tokens":  "__AXONHUB_CLEAR__",
				"top_p":       0.95,
			},
			initialBody: map[string]any{
				"model":             "gpt-4",
				"temperature":       0.5,
				"max_tokens":        1000,
				"frequency_penalty": 0.1,
			},
			expectedRemoved: []string{"temperature", "max_tokens"},
			expectedPreserved: map[string]any{
				"model":             "gpt-4",
				"top_p":             0.95,
				"frequency_penalty": 0.1,
			},
		},
		{
			name: "remove nested parameter with __AXONHUB_CLEAR__",
			overrideParameters: map[string]any{
				"response_format.type": "__AXONHUB_CLEAR__",
			},
			initialBody: map[string]any{
				"model": "gpt-4",
				"response_format": map[string]any{
					"type":   "json_object",
					"schema": map[string]any{},
				},
			},
			expectedRemoved:   []string{"response_format.type"},
			expectedPreserved: map[string]any{"model": "gpt-4"},
		},
		{
			name: "mix of removal and override",
			overrideParameters: map[string]any{
				"temperature": "__AXONHUB_CLEAR__",
				"max_tokens":  2000,
				"top_p":       0.95,
			},
			initialBody: map[string]any{
				"model":       "gpt-4",
				"temperature": 0.5,
				"max_tokens":  1000,
			},
			expectedRemoved: []string{"temperature"},
			expectedPreserved: map[string]any{
				"model":      "gpt-4",
				"max_tokens": float64(2000),
				"top_p":      0.95,
			},
		},
		{
			name: "attempt to remove non-existent parameter",
			overrideParameters: map[string]any{
				"non_existent": "__AXONHUB_CLEAR__",
			},
			initialBody: map[string]any{
				"model":       "gpt-4",
				"temperature": 0.5,
			},
			expectedRemoved: []string{"non_existent"},
			expectedPreserved: map[string]any{
				"model":       "gpt-4",
				"temperature": 0.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create override parameters JSON
			data, err := json.Marshal(tt.overrideParameters)
			require.NoError(t, err)

			overrideParamsStr := string(data)

			// Create mock channel with override parameters
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:              1,
					Name:            "test-channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings: &objects.ChannelSettings{
						OverrideParameters: overrideParamsStr,
					},
				},
				Outbound: &mockTransformer{},
			}

			// Create outbound transformer
			outbound := &PersistentOutboundTransformer{
				wrapped: &mockTransformer{},
				state: &PersistenceState{
					CurrentChannel: channel,
					Channels:       []*biz.Channel{channel},
					ChannelIndex:   0,
				},
			}

			// Create the middleware
			middleware := applyOverrideRequestBody(outbound)

			// Create a test request with initial body
			requestBody, err := json.Marshal(tt.initialBody)
			require.NoError(t, err)

			httpRequest := &httpclient.Request{
				Method: "POST",
				URL:    "https://api.example.com/v1/chat/completions",
				Body:   requestBody,
			}

			// Apply the middleware
			modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
			require.NoError(t, err)
			require.NotNil(t, modifiedRequest)

			// Verify removed keys don't exist
			bodyStr := string(modifiedRequest.Body)
			for _, key := range tt.expectedRemoved {
				result := gjson.Get(bodyStr, key)
				assert.False(t, result.Exists(), "key %s should not exist", key)
			}

			// Verify preserved keys exist with correct values
			for key, expectedValue := range tt.expectedPreserved {
				result := gjson.Get(bodyStr, key)
				assert.True(t, result.Exists(), "key %s should exist", key)

				switch v := expectedValue.(type) {
				case float64:
					assert.Equal(t, v, result.Float(), "key %s should have value %v", key, v)
				case string:
					assert.Equal(t, v, result.String(), "key %s should have value %v", key, v)
				default:
					assert.Equal(t, v, result.Value(), "key %s should have value %v", key, v)
				}
			}
		})
	}
}

func TestOverrideHeadersMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		overrideHeaders []objects.HeaderEntry
		existingHeaders http.Header
		expectedHeaders http.Header
	}{
		{
			name: "override single header",
			overrideHeaders: []objects.HeaderEntry{
				{Key: "User-Agent", Value: "AxonHub/1.0"},
			},
			existingHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedHeaders: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"AxonHub/1.0"},
			},
		},
		{
			name: "override multiple headers",
			overrideHeaders: []objects.HeaderEntry{
				{Key: "User-Agent", Value: "AxonHub/1.0"},
				{Key: "X-Custom-Header", Value: "custom-value"},
				{Key: "Authorization", Value: "Bearer token123"}, // This will be blocked
			},
			existingHeaders: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"Original-Agent"},
			},
			expectedHeaders: http.Header{
				"Content-Type":    []string{"application/json"},
				"User-Agent":      []string{"AxonHub/1.0"}, // Should be overridden
				"X-Custom-Header": []string{"custom-value"},
				// Authorization header should be blocked and not present
			},
		},
		{
			name:            "no override headers",
			overrideHeaders: []objects.HeaderEntry{},
			existingHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
		{
			name: "override with empty key should be ignored",
			overrideHeaders: []objects.HeaderEntry{
				{Key: "", Value: "should-be-ignored"},
				{Key: "Valid-Header", Value: "valid-value"},
			},
			existingHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedHeaders: http.Header{
				"Content-Type": []string{"application/json"},
				"Valid-Header": []string{"valid-value"},
			},
		},
		{
			name: "no existing headers",
			overrideHeaders: []objects.HeaderEntry{
				{Key: "User-Agent", Value: "AxonHub/1.0"},
				{Key: "X-API-Key", Value: "secret-key"}, // This will be blocked
			},
			existingHeaders: nil,
			expectedHeaders: http.Header{
				"User-Agent": []string{"AxonHub/1.0"},
				// X-API-Key header should be blocked and not present
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create channel with override headers
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:              1,
					Name:            "test-channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings: &objects.ChannelSettings{
						OverrideHeaders: tt.overrideHeaders,
					},
				},
				Outbound: &mockTransformer{},
			}

			// Create outbound transformer
			outbound := &PersistentOutboundTransformer{
				wrapped: &mockTransformer{},
				state: &PersistenceState{
					CurrentChannel: channel,
					Channels:       []*biz.Channel{channel},
					ChannelIndex:   0,
					RequestExec:    &ent.RequestExecution{ID: 1}, // Dummy to skip creation
				},
			}

			// Create middleware
			middleware := applyOverrideRequestHeaders(outbound)

			// Create HTTP request with existing headers
			httpRequest := &httpclient.Request{
				Method:  "POST",
				URL:     "https://api.example.com/v1/chat/completions",
				Body:    []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
				Headers: tt.existingHeaders,
			}

			// Apply middleware
			modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
			require.NoError(t, err)
			require.NotNil(t, modifiedRequest)

			// Verify headers
			assert.NotNil(t, modifiedRequest.Headers)

			for key, expectedValue := range tt.expectedHeaders {
				assert.Equal(t, expectedValue, modifiedRequest.Headers[key],
					"Header %s should have value %s", key, expectedValue)
			}
		})
	}
}

func TestOverrideHeadersMiddleware_NoChannel(t *testing.T) {
	ctx := context.Background()

	// Create outbound transformer without a channel
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: nil,
		},
	}

	// Create middleware
	middleware := applyOverrideRequestHeaders(outbound)

	// Create HTTP request
	httpRequest := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
		Body:   []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	// Apply middleware
	modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, modifiedRequest)

	// Verify request is unchanged
	assert.Equal(t, httpRequest.Headers, modifiedRequest.Headers)
}

func TestOverrideHeadersMiddleware_EmptySettings(t *testing.T) {
	ctx := context.Background()

	// Create channel without settings
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:              1,
			Name:            "test-channel",
			SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			Settings:        nil,
		},
		Outbound: &mockTransformer{},
	}

	// Create outbound transformer
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: channel,
			Channels:       []*biz.Channel{channel},
			ChannelIndex:   0,
			RequestExec:    &ent.RequestExecution{ID: 1}, // Dummy to skip creation
		},
	}

	// Create middleware
	middleware := applyOverrideRequestHeaders(outbound)

	// Create HTTP request
	httpRequest := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
		Body:   []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	// Apply middleware
	modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, modifiedRequest)

	// Verify request is unchanged
	assert.Equal(t, httpRequest.Headers, modifiedRequest.Headers)
}

func TestOverrideHeadersMiddleware_EmptyOverrideHeaders(t *testing.T) {
	ctx := context.Background()

	// Create channel with empty override headers
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:              1,
			Name:            "test-channel",
			SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			Settings: &objects.ChannelSettings{
				OverrideHeaders: []objects.HeaderEntry{},
			},
		},
		Outbound: &mockTransformer{},
	}

	// Create outbound transformer
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: channel,
			Channels:       []*biz.Channel{channel},
			ChannelIndex:   0,
			RequestExec:    &ent.RequestExecution{ID: 1}, // Dummy to skip creation
		},
	}

	// Create middleware
	middleware := applyOverrideRequestHeaders(outbound)

	// Create HTTP request
	httpRequest := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
		Body:   []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	// Apply middleware
	modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, modifiedRequest)

	// Verify request is unchanged
	assert.Equal(t, httpRequest.Headers, modifiedRequest.Headers)
}

func TestOverrideHeadersMiddleware_OverrideExistingAuth(t *testing.T) {
	ctx := context.Background()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:              1,
			Name:            "auth-channel",
			SupportedModels: []string{"gpt-4"},
			Settings: &objects.ChannelSettings{
				OverrideHeaders: []objects.HeaderEntry{
					{Key: "Authorization", Value: "Bearer override-token"},
					{Key: "Api-Key", Value: "override-key"},
					{Key: "X-Api-Key", Value: "override-x-key"},
				},
			},
		},
		Outbound: &mockTransformer{},
	}

	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: channel,
			Channels:       []*biz.Channel{channel},
			ChannelIndex:   0,
			RequestExec:    &ent.RequestExecution{ID: 1},
		},
	}

	middleware := applyOverrideRequestHeaders(outbound)

	httpRequest := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
		Body:   []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
		Headers: http.Header{
			"Authorization": []string{"Bearer original-token"},
			"Api-Key":       []string{"original-key"},
		},
	}

	modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, modifiedRequest)

	// Authorization should be overridden by channel override headers.
	assert.Equal(t, "Bearer override-token", modifiedRequest.Headers.Get("Authorization"))
	// Api-Key should be overridden.
	assert.Equal(t, "override-key", modifiedRequest.Headers.Get("Api-Key"))
	// X-Api-Key should be added because it was not present originally.
	assert.Equal(t, "override-x-key", modifiedRequest.Headers.Get("X-Api-Key"))
}

func TestOverrideHeadersMiddleware_BlockedHeaders(t *testing.T) {
	tests := []struct {
		name            string
		overrideHeaders []objects.HeaderEntry
		existingHeaders http.Header
		expectedHeaders http.Header
		shouldBlock     []string
	}{
		{
			name: "transport headers are forwarded, sensitive allowed",
			overrideHeaders: []objects.HeaderEntry{
				{Key: "Content-Length", Value: "123"},
				{Key: "Authorization", Value: "Bearer token"},
				{Key: "User-Agent", Value: "CustomAgent"},
				{Key: "X-Custom-Header", Value: "custom-value"},
			},
			existingHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedHeaders: http.Header{
				"Content-Type":    []string{"application/json"},
				"Content-Length":  []string{"123"},
				"User-Agent":      []string{"CustomAgent"},
				"X-Custom-Header": []string{"custom-value"},
				"Authorization":   []string{"Bearer token"},
			},
			shouldBlock: []string{},
		},
		{
			name: "transport headers remain forwarded",
			overrideHeaders: []objects.HeaderEntry{
				{Key: "Content-Length", Value: "123"},
				{Key: "Transfer-Encoding", Value: "chunked"},
				{Key: "Accept-Encoding", Value: "gzip"},
				{Key: "Authorization", Value: "Bearer token"},
				{Key: "Api-Key", Value: "secret"},
				{Key: "X-Api-Key", Value: "secret"},
				{Key: "X-Api-Secret", Value: "secret"},
				{Key: "X-Api-Token", Value: "secret"},
			},
			existingHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedHeaders: http.Header{
				"Content-Type":      []string{"application/json"},
				"Content-Length":    []string{"123"},
				"Transfer-Encoding": []string{"chunked"},
				"Accept-Encoding":   []string{"gzip"},
				"Authorization":     []string{"Bearer token"},
				"Api-Key":           []string{"secret"},
				"X-Api-Key":         []string{"secret"},
				"X-Api-Secret":      []string{"secret"},
				"X-Api-Token":       []string{"secret"},
			},
			shouldBlock: []string{},
		},
		{
			name: "mixed case sensitive headers allowed",
			overrideHeaders: []objects.HeaderEntry{
				{Key: "authorization", Value: "Bearer token"},
				{Key: "API-KEY", Value: "secret"},
				{Key: "x-api-key", Value: "secret"},
				{Key: "Valid-Header", Value: "valid-value"},
			},
			existingHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedHeaders: http.Header{
				"Content-Type":  []string{"application/json"},
				"Valid-Header":  []string{"valid-value"},
				"Authorization": []string{"Bearer token"},
				"Api-Key":       []string{"secret"},
				"X-Api-Key":     []string{"secret"},
			},
			shouldBlock: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create channel with override headers
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:              1,
					Name:            "test-channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings: &objects.ChannelSettings{
						OverrideHeaders: tt.overrideHeaders,
					},
				},
				Outbound: &mockTransformer{},
			}

			// Create outbound transformer
			outbound := &PersistentOutboundTransformer{
				wrapped: &mockTransformer{},
				state: &PersistenceState{
					CurrentChannel: channel,
					Channels:       []*biz.Channel{channel},
					ChannelIndex:   0,
					RequestExec:    &ent.RequestExecution{ID: 1}, // Dummy to skip creation
				},
			}

			// Create middleware
			middleware := applyOverrideRequestHeaders(outbound)

			// Create HTTP request with existing headers
			httpRequest := &httpclient.Request{
				Method:  "POST",
				URL:     "https://api.example.com/v1/chat/completions",
				Body:    []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
				Headers: tt.existingHeaders,
			}

			// Apply middleware
			modifiedRequest, err := middleware.OnOutboundRawRequest(ctx, httpRequest)
			require.NoError(t, err)
			require.NotNil(t, modifiedRequest)

			// Verify headers
			assert.NotNil(t, modifiedRequest.Headers)

			for key, expectedValue := range tt.expectedHeaders {
				assert.Equal(t, expectedValue, modifiedRequest.Headers[key],
					"Header %s should have value %s", key, expectedValue)
			}

			// Verify blocked headers are not present
			for _, blockedHeader := range tt.shouldBlock {
				_, exists := modifiedRequest.Headers[blockedHeader]
				assert.False(t, exists, "Blocked header %s should not be present", blockedHeader)
			}
		})
	}
}
