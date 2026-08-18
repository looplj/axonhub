package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/deepseek"
)

func newTestTransformer(t *testing.T) *OutboundTransformer {
	t.Helper()

	tr, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://opencode.ai/zen/go",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	})
	require.NoError(t, err)

	out, ok := tr.(*OutboundTransformer)
	require.True(t, ok)

	return out
}

func TestRouteForModel(t *testing.T) {
	tests := []struct {
		model string
		want  route
	}{
		// DeepSeek family → DeepSeek transformer
		{"deepseek-v4-pro", routeDeepseek},
		{"deepseek-v4-flash", routeDeepseek},
		// Grok/GPT family → OpenAI Responses API
		{"grok-4.5", routeResponses},
		{"gpt-5.6-luna", routeResponses},
		// MiniMax/Qwen family → Anthropic messages
		{"minimax-m3", routeAnthropic},
		{"minimax-m2.7", routeAnthropic},
		{"qwen3.8-max", routeAnthropic},
		{"qwen3.7-plus", routeAnthropic},
		// Everything else → OpenAI chat completions
		{"glm-5.2", routeChat},
		{"kimi-k3", routeChat},
		{"mimo-v2.5", routeChat},
		{"hy3", routeChat},
		{"unknown-model", routeChat},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, routeForModel(tt.model))
		})
	}
}

func TestOutboundTransformer_TransformRequest_RoutesByModel(t *testing.T) {
	tr := newTestTransformer(t)

	tests := []struct {
		name        string
		model       string
		expectedURL string
	}{
		{
			name:        "deepseek routes to DeepSeek transformer",
			model:       "deepseek-v4-flash",
			expectedURL: "https://opencode.ai/zen/go/v1/chat/completions",
		},
		{
			name:        "grok routes to Responses API",
			model:       "grok-4.5",
			expectedURL: "https://opencode.ai/zen/go/v1/responses",
		},
		{
			name:        "gpt routes to Responses API",
			model:       "gpt-5.6-luna",
			expectedURL: "https://opencode.ai/zen/go/v1/responses",
		},
		{
			name:        "minimax routes to Anthropic messages",
			model:       "minimax-m3",
			expectedURL: "https://opencode.ai/zen/go/v1/messages",
		},
		{
			name:        "qwen routes to Anthropic messages",
			model:       "qwen3.8-max",
			expectedURL: "https://opencode.ai/zen/go/v1/messages",
		},
		{
			name:        "glm routes to OpenAI chat completions",
			model:       "glm-5.2",
			expectedURL: "https://opencode.ai/zen/go/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq, err := tr.TransformRequest(context.Background(), &llm.Request{
				Model: tt.model,
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				},
			})
			require.NoError(t, err)
			assert.Equal(t, tt.expectedURL, httpReq.URL)
			assert.Equal(t, http.MethodPost, httpReq.Method)

			// The route must be recorded for response routing.
			require.NotNil(t, httpReq.TransformerMetadata)
			r, ok := httpReq.TransformerMetadata[routeMetadataKey].(string)
			require.True(t, ok)
			assert.Equal(t, string(routeForModel(tt.model)), r)
		})
	}
}

func TestOutboundTransformer_TransformRequest_DeepSeekThinking(t *testing.T) {
	tr := newTestTransformer(t)

	httpReq, err := tr.TransformRequest(context.Background(), &llm.Request{
		Model: "deepseek-v4-flash",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
		},
	})
	require.NoError(t, err)

	// DeepSeek sub-transformer must emit the thinking field.
	var dsReq deepseek.Request
	require.NoError(t, json.Unmarshal(httpReq.Body, &dsReq))
	require.NotNil(t, dsReq.Thinking)
	assert.Equal(t, "enabled", dsReq.Thinking.Type)
}

func TestOutboundTransformer_TransformResponse_RoutesByMetadata(t *testing.T) {
	tr := newTestTransformer(t)

	tests := []struct {
		name      string
		route     route
		body      string
		expectErr bool
	}{
		{
			name:  "chat route parses OpenAI chat completion",
			route: routeChat,
			body: `{
				"id": "chatcmpl-1",
				"object": "chat.completion",
				"model": "glm-5.2",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hi"}, "finish_reason": "stop"}]
			}`,
		},
		{
			name:  "anthropic route parses Anthropic message",
			route: routeAnthropic,
			body: `{
				"id": "msg_1",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "Hi"}],
				"model": "minimax-m3",
				"stop_reason": "end_turn"
			}`,
		},
		{
			name:  "responses route parses Responses API response",
			route: routeResponses,
			body: `{
				"id": "resp_1",
				"object": "response",
				"created_at": 1750000000,
				"status": "completed",
				"model": "grok-4.5",
				"output": [{"type": "message", "id": "msg_1", "role": "assistant", "content": [{"type": "output_text", "text": "Hi"}]}]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpResp := &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(tt.body),
				Request: &httpclient.Request{
					TransformerMetadata: map[string]any{
						routeMetadataKey: string(tt.route),
					},
				},
			}

			resp, err := tr.TransformResponse(context.Background(), httpResp)
			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, resp.Choices, 1)
			assert.NotEmpty(t, resp.Choices[0].Message.Content.Content)
		})
	}
}

func TestOutboundTransformer_TransformResponse_FallsBackToChatWithoutMetadata(t *testing.T) {
	tr := newTestTransformer(t)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body: []byte(`{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"model": "glm-5.2",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hi"}, "finish_reason": "stop"}]
		}`),
	}

	resp, err := tr.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message)
	assert.NotEmpty(t, resp.Choices[0].Message.Content.Content)
}

func TestOutboundTransformer_TransformStream_RoutesByMetadata(t *testing.T) {
	tr := newTestTransformer(t)

	req := &httpclient.Request{
		TransformerMetadata: map[string]any{
			routeMetadataKey: string(routeDeepseek),
		},
	}

	chunks := streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "message", Data: []byte(`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}`)},
		{Type: "message", Data: []byte("[DONE]")},
	})

	stream, err := tr.TransformStream(context.Background(), req, chunks)
	require.NoError(t, err)

	var got []*llm.Response
	for stream.Next() {
		got = append(got, stream.Current())
	}
	require.NoError(t, stream.Err())

	require.Len(t, got, 2)
	require.NotNil(t, got[0].Choices[0].Delta)
	assert.Equal(t, "Hi", *got[0].Choices[0].Delta.Content.Content)
}

func TestOutboundTransformer_TransformError_DelegatesToDefault(t *testing.T) {
	tr := newTestTransformer(t)

	errResp := tr.TransformError(context.Background(), &httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"error":{"message":"bad request","type":"invalid_request_error"}}`),
	})
	require.NotNil(t, errResp)
	assert.Equal(t, http.StatusBadRequest, errResp.StatusCode)
	assert.Equal(t, "bad request", errResp.Detail.Message)
}

func TestOutboundTransformer_TransformRequest_NilRequest(t *testing.T) {
	tr := newTestTransformer(t)

	_, err := tr.TransformRequest(context.Background(), nil)
	require.Error(t, err)
}

func TestNewOutboundTransformerWithConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr string
	}{
		{name: "nil config", wantErr: "config is nil"},
		{name: "missing api key provider", config: &Config{BaseURL: "https://opencode.ai/zen/go"}, wantErr: "API key provider is required"},
		{name: "missing base url", config: &Config{APIKeyProvider: auth.NewStaticKeyProvider("k")}, wantErr: "base URL is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOutboundTransformerWithConfig(tt.config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
