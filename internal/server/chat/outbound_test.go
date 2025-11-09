package chat

import (
	"context"
	"encoding/json"
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

func TestOverrideParameters(t *testing.T) {
	tests := []struct {
		name               string
		overrideParameters map[string]any
		expectedValues     map[string]any
	}{
		{
			name: "override temperature",
			overrideParameters: map[string]any{
				"temperature": 0.9,
			},
			expectedValues: map[string]any{
				"temperature": 0.9,
				"max_tokens":  float64(1000), // JSON numbers are float64
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

			// Parse the body
			bodyStr := string(channelRequest.Body)

			// Verify expected values
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
		})
	}
}

func TestOverrideParametersInvalidJSON(t *testing.T) {
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

	processor := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: channel,
			Channels:       []*biz.Channel{channel},
			ChannelIndex:   0,
			RequestExec:    &ent.RequestExecution{ID: 1}, // Dummy to skip creation
		},
	}

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

	// Should not error, just log warning and continue
	channelRequest, err := processor.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)
	require.NotNil(t, channelRequest)

	// Verify original values are preserved
	bodyStr := string(channelRequest.Body)
	temperature := gjson.Get(bodyStr, "temperature")
	assert.Equal(t, 0.5, temperature.Float())
}

func TestOverrideParametersEmptySettings(t *testing.T) {
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

	processor := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentChannel: channel,
			Channels:       []*biz.Channel{channel},
			ChannelIndex:   0,
			RequestExec:    &ent.RequestExecution{ID: 1}, // Dummy to skip creation
		},
	}

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

	// Should work normally without override
	channelRequest, err := processor.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)
	require.NotNil(t, channelRequest)

	// Verify original values
	bodyStr := string(channelRequest.Body)
	temperature := gjson.Get(bodyStr, "temperature")
	assert.Equal(t, 0.5, temperature.Float())
}
