package anthropic

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestAnthropicTransformers_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	tests := []struct {
		name                 string
		anthropicRequestJSON string
		expectedModel        string
		expectedMaxTokens    int64
	}{
		{
			name: "simple text message",
			anthropicRequestJSON: `{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": "Hello, Claude!"
					}
				]
			}`,
			expectedModel:     "claude-3-sonnet-20240229",
			expectedMaxTokens: 1024,
		},
		{
			name: "message with system prompt",
			anthropicRequestJSON: `{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 2048,
				"system": "You are a helpful assistant.",
				"messages": [
					{
						"role": "user",
						"content": "What is the capital of France?"
					}
				],
				"temperature": 0.7
			}`,
			expectedModel:     "claude-3-sonnet-20240229",
			expectedMaxTokens: 2048,
		},
		{
			name: "multimodal message",
			anthropicRequestJSON: `{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": [
							{
								"type": "text",
								"text": "What's in this image?"
							},
							{
								"type": "image",
								"source": {
									"type": "base64",
									"media_type": "image/jpeg",
									"data": "/9j/4AAQSkZJRg..."
								}
							}
						]
					}
				]
			}`,
			expectedModel:     "claude-3-sonnet-20240229",
			expectedMaxTokens: 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Transform Anthropic request to ChatCompletionRequest
			httpReq := &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(tt.anthropicRequestJSON),
			}

			chatReq, err := inboundTransformer.TransformRequest(t.Context(), httpReq)
			require.NoError(t, err)
			require.NotNil(t, chatReq)

			// Verify the transformation
			require.Equal(t, tt.expectedModel, chatReq.Model)
			require.Equal(t, tt.expectedMaxTokens, *chatReq.MaxTokens)
			require.NotEmpty(t, chatReq.Messages)

			// Step 2: Transform ChatCompletionRequest to Anthropic outbound request
			outboundReq, err := outboundTransformer.TransformRequest(t.Context(), chatReq)
			require.NoError(t, err)
			require.NotNil(t, outboundReq)

			// Verify outbound request
			require.Equal(t, http.MethodPost, outboundReq.Method)
			require.Equal(t, "https://api.anthropic.com/v1/messages", outboundReq.URL)
			require.Equal(t, "application/json", outboundReq.Headers.Get("Content-Type"))
			require.Equal(t, "2023-06-01", outboundReq.Headers.Get("Anthropic-Version"))

			// Verify the outbound request body can be unmarshaled
			var anthropicReq MessageRequest

			err = json.Unmarshal(outboundReq.Body, &anthropicReq)
			require.NoError(t, err)
			require.Equal(t, tt.expectedModel, anthropicReq.Model)
			require.Equal(t, tt.expectedMaxTokens, anthropicReq.MaxTokens)

			// Step 3: Simulate Anthropic response and transform back
			anthropicResponse := &Message{
				ID:   "msg_test_123",
				Type: "message",
				Role: "assistant",
				Content: []MessageContentBlock{
					{
						Type: "text",
						Text: "This is a test response from Claude.",
					},
				},
				Model:      tt.expectedModel,
				StopReason: func() *string { s := "end_turn"; return &s }(),
				Usage: &Usage{
					InputTokens:  15,
					OutputTokens: 25,
				},
			}

			responseBody, err := json.Marshal(anthropicResponse)
			require.NoError(t, err)

			httpResp := &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       responseBody,
			}

			// Step 4: Transform Anthropic response to ChatCompletionResponse
			chatResp, err := outboundTransformer.TransformResponse(t.Context(), httpResp)
			require.NoError(t, err)
			require.NotNil(t, chatResp)

			// Verify chat response
			require.Equal(t, "msg_test_123", chatResp.ID)
			require.Equal(t, "chat.completion", chatResp.Object)
			require.Equal(t, tt.expectedModel, chatResp.Model)
			require.Equal(t, 1, len(chatResp.Choices))
			require.Equal(t, "assistant", chatResp.Choices[0].Message.Role)
			require.Equal(
				t,
				"This is a test response from Claude.",
				*chatResp.Choices[0].Message.Content.Content,
			)
			require.Equal(t, "stop", *chatResp.Choices[0].FinishReason)

			// Step 5: Transform ChatCompletionResponse back to Anthropic format
			finalHttpResp, err := inboundTransformer.TransformResponse(t.Context(), chatResp)
			require.NoError(t, err)
			require.NotNil(t, finalHttpResp)

			// Verify final response
			require.Equal(t, http.StatusOK, finalHttpResp.StatusCode)
			require.Equal(t, "application/json", finalHttpResp.Headers.Get("Content-Type"))

			var finalAnthropicResp Message

			err = json.Unmarshal(finalHttpResp.Body, &finalAnthropicResp)
			require.NoError(t, err)
			require.Equal(t, "msg_test_123", finalAnthropicResp.ID)
			require.Equal(t, "message", finalAnthropicResp.Type)
			require.Equal(t, "assistant", finalAnthropicResp.Role)
			require.Equal(t, tt.expectedModel, finalAnthropicResp.Model)
		})
	}
}

func TestTransformRequest_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	tests := []struct {
		name        string
		requestFile string
	}{
		{
			name:        "claude code",
			requestFile: `anthropic-claude-code.request.json`,
		},
		{
			name:        "claude code2",
			requestFile: `anthropic-claude-code2.request.json`,
		},
		{
			name:        "claude cache control",
			requestFile: `anthropic-cache-control-inbound.request.json`,
		},
		{
			name:        "claude thinking",
			requestFile: `anthropic-thinking.request.json`,
		},
		{
			name:        "tool result with reasoning",
			requestFile: `anthropic-tool-result-mixed.request.json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wantReq MessageRequest

			err := xtest.LoadTestData(t, tt.requestFile, &wantReq)
			require.NoError(t, err)

			var buf bytes.Buffer

			decoder := json.NewEncoder(&buf)
			decoder.SetEscapeHTML(false)

			if err := decoder.Encode(wantReq); err != nil {
				t.Fatalf("failed to marshal tool result: %v", err)
			}

			chatReq, err := inboundTransformer.TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: buf.Bytes(),
			})
			require.NoError(t, err)
			require.NotNil(t, chatReq)

			outboundReq, err := outboundTransformer.TransformRequest(t.Context(), chatReq)
			require.NoError(t, err)

			var gotReq MessageRequest

			err = json.Unmarshal(outboundReq.Body, &gotReq)
			require.NoError(t, err)

			if !xjson.Equal(wantReq, gotReq) {
				t.Errorf("wantReq != gotReq\n%s", cmp.Diff(wantReq, gotReq))
			}
		})
	}
}

func TestAnthropicTransformers_StreamingIntegration(t *testing.T) {
	outboundTransformer, _ := NewOutboundTransformer("https://api.claude.com", "xxx")

	// Simulate streaming chunks from Anthropic
	chunks := []*httpclient.StreamEvent{
		{
			Data: []byte(`{
				"type": "message_start",
				"message": {
					"id": "msg_stream_123",
					"type": "message",
					"role": "assistant",
					"content": [],
					"model": "claude-3-sonnet-20240229",
					"stop_reason": null,
					"stop_sequence": null,
					"usage": {"input_tokens": 10, "output_tokens": 0}
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_start",
				"index": 0,
				"content_block": {
					"type": "text",
					"text": ""
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "text_delta",
					"text": "Hello"
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "text_delta",
					"text": ", this is"
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "text_delta",
					"text": " a streaming response!"
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_stop",
				"index": 0
			}`),
		},
		{
			Data: []byte(`{
				"type": "message_delta",
				"delta": {
					"stop_reason": "end_turn",
					"stop_sequence": null
				},
				"usage": {"input_tokens": 10, "output_tokens": 25}
			}`),
		},
		{
			Data: []byte(`{
				"type": "message_stop"
			}`),
		},
	}

	// Aggregate the streaming chunks
	chatRespBytes, _, err := outboundTransformer.AggregateStreamChunks(t.Context(), chunks)
	require.NoError(t, err)
	require.NotNil(t, chatRespBytes)

	// Parse the response
	var chatResp Message

	err = json.Unmarshal(chatRespBytes, &chatResp)
	require.NoError(t, err)

	// Verify the aggregated response
	require.Equal(t, "msg_stream_123", chatResp.ID)
	require.Equal(t, "message", chatResp.Type)
	require.Equal(t, 1, len(chatResp.Content))
	require.Equal(t, "assistant", chatResp.Role)
	require.Equal(
		t,
		"Hello, this is a streaming response!",
		chatResp.Content[0].Text,
	)
	require.NotNil(t, chatResp.StopReason)
	require.Equal(t, "end_turn", *chatResp.StopReason)

	// Verify usage
	require.NotNil(t, chatResp.Usage)
	require.Equal(t, int64(10), chatResp.Usage.InputTokens)
	require.Equal(t, int64(25), chatResp.Usage.OutputTokens)
}

func TestTransformResponse_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	tests := []struct {
		name        string
		requestFile string
	}{
		{
			name:        "anthropic-tool.response.json",
			requestFile: `anthropic-tool.response.json`,
		},
		{
			name:        "anthropic-think.response.json",
			requestFile: `anthropic-think.response.json`,
		},
		{
			name:        "anthropic-tool2.response.json",
			requestFile: `anthropic-tool2.response.json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wantMessage Message

			err := xtest.LoadTestData(t, tt.requestFile, &wantMessage)
			require.NoError(t, err)

			var buf bytes.Buffer

			encoder := json.NewEncoder(&buf)
			encoder.SetEscapeHTML(false)

			if err := encoder.Encode(wantMessage); err != nil {
				t.Fatalf("failed to marshal tool result: %v", err)
			}

			chatResp, err := outboundTransformer.TransformResponse(t.Context(), &httpclient.Response{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: buf.Bytes(),
			})
			require.NoError(t, err)
			require.NotNil(t, chatResp)

			println("chatResp.Body:", xjson.MustMarshalString(chatResp))

			inboundResp, err := inboundTransformer.TransformResponse(t.Context(), chatResp)
			require.NoError(t, err)

			var gotMessage Message

			err = json.Unmarshal(inboundResp.Body, &gotMessage)
			require.NoError(t, err)

			if !xjson.Equal(wantMessage, gotMessage) {
				t.Errorf("wantMessage != gotMessage\n%s", cmp.Diff(wantMessage, gotMessage))
			}
		})
	}
}
